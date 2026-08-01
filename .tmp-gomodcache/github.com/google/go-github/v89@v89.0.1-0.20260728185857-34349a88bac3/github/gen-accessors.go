// Copyright 2017 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build ignore

// gen-accessors generates accessor methods for all struct fields.
// This is so that interfaces can be easily crafted by users of this repo
// within their own code bases.
// See https://github.com/google/go-github/issues/4059 for details.
//
// It is meant to be used by go-github contributors in conjunction with the
// go generate tool before sending a PR to GitHub.
// Please see the CONTRIBUTING.md file for more information.
//
// Usage:
//
//	go run gen-accessors.go [-v [file1.go file2.go ...]]
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io/fs"
	"log"
	"os"
	"slices"
	"strings"
	"text/template"
)

const (
	fileSuffix = "-accessors.go"
)

var (
	check   = flag.Bool("check", false, "Check whether generated files are up to date")
	verbose = flag.Bool("v", false, "Print verbose log messages")

	sourceTmpl = template.Must(template.New("source").Parse(source))
	testTmpl   = template.Must(template.New("test").Parse(test))

	// skipStructMethods lists "struct.method" combos to skip.
	skipStructMethods = map[string]bool{
		"AbuseRateLimitError.GetResponse": true,
		"Client.GetBaseURL":               true,
		"Client.GetUploadURL":             true,
		"ErrorResponse.GetResponse":       true,
		"MarketplaceService.GetStubbed":   true,
		"PackageVersion.GetBody":          true,
		"PackageVersion.GetMetadata":      true,
		"RateLimitError.GetResponse":      true,
		"RepositoryContent.GetContent":    true,
	}
	// skipStructs lists structs to skip.
	skipStructs = map[string]bool{
		"Client": true,
	}

	// whitelistSliceGetters lists "struct.field" to add getter method
	whitelistSliceGetters = map[string]bool{
		"PushEvent.Commits": true,
	}
)

func isCheck() bool {
	return *check || os.Getenv("CHECK") == "1"
}

func logf(fmt string, args ...any) {
	if *verbose {
		log.Printf(fmt, args...)
	}
}

func main() {
	flag.Parse()

	// For debugging purposes, processing just a single or a few files is helpful:
	var processOnly map[string]bool
	if *verbose { // Only create the map if args are provided.
		for _, arg := range flag.Args() {
			if processOnly == nil {
				processOnly = map[string]bool{}
			}
			processOnly[arg] = true
		}
	}

	fset := token.NewFileSet()

	pkgs, err := parser.ParseDir(fset, ".", sourceFilter, 0)
	if err != nil {
		log.Fatal(err)
		return
	}

	for pkgName, pkg := range pkgs {
		t := &templateData{
			filename: pkgName + fileSuffix,
			Year:     2017,
			Package:  pkgName,
			Imports:  map[string]string{},
		}
		for filename, f := range pkg.Files {
			if *verbose && processOnly != nil && !processOnly[filename] {
				continue
			}

			logf("Processing %v...", filename)
			if err := t.processAST(f); err != nil {
				log.Fatal(err)
			}
		}
		if err := t.dump(); err != nil {
			log.Fatal(err)
		}
	}
	logf("Done.")
}

func (t *templateData) processAST(f *ast.File) error {
	for _, decl := range f.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range gd.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			// Skip unexported identifiers.
			if !ts.Name.IsExported() {
				logf("Struct %v is unexported; skipping.", ts.Name)
				continue
			}
			// Check if the struct should be skipped.
			if skipStructs[ts.Name.Name] {
				logf("Struct %v is in skip list; skipping.", ts.Name)
				continue
			}
			if _, ok := ts.Type.(*ast.Ident); ok { // e.g. type SomeService service
				continue
			}
			st, ok := ts.Type.(*ast.StructType)
			if !ok {
				logf("Skipping TypeSpec of type %T", ts.Type)
				continue
			}
			for _, field := range st.Fields.List {
				if len(field.Names) == 0 {
					continue
				}

				fieldName := field.Names[0]
				// Skip unexported identifiers.
				if !fieldName.IsExported() {
					logf("Field %v is unexported; skipping.", fieldName)
					continue
				}
				// Check if "struct.method" should be skipped.
				if key := fmt.Sprintf("%v.Get%v", ts.Name, fieldName); skipStructMethods[key] {
					logf("Method %v is skip list; skipping.", key)
					continue
				}

				se, ok := field.Type.(*ast.StarExpr)
				if !ok {
					switch x := field.Type.(type) {
					case *ast.MapType:
						logf("processAST: addMapType(x, %q, %q)", ts.Name.String(), fieldName.String())
						t.addMapType(x, ts.Name.String(), fieldName.String(), false)
						continue
					case *ast.ArrayType:
						logf("processAST: addArrayType(x, %q, %q)", ts.Name.String(), fieldName.String())
						t.addArrayType(x, ts.Name.String(), fieldName.String(), false)
						continue
					case *ast.Ident:
						logf("processAST: addSimpleValueIdent(x, %q, %q)", ts.Name.String(), fieldName.String())
						t.addSimpleValueIdent(x, ts.Name.String(), fieldName.String())
						continue
					case *ast.SelectorExpr:
						logf("processAST: addSimpleValueSelectorExpr(x, %q, %q)", ts.Name.String(), fieldName.String())
						t.addSimpleValueSelectorExpr(x, ts.Name.String(), fieldName.String())
						continue
					}

					logf("Skipping field type %T, fieldName=%v", field.Type, fieldName)
					continue
				}

				switch x := se.X.(type) {
				case *ast.ArrayType:
					t.addArrayType(x, ts.Name.String(), fieldName.String(), true)
				case *ast.Ident:
					t.addIdent(x, ts.Name.String(), fieldName.String())
				case *ast.MapType:
					t.addMapType(x, ts.Name.String(), fieldName.String(), true)
				case *ast.SelectorExpr:
					t.addSelectorExpr(x, ts.Name.String(), fieldName.String())
				default:
					logf("processAST: type %q, field %q, unknown %T: %+v", ts.Name, fieldName, x, x)
				}
			}
		}
	}
	return nil
}

func sourceFilter(fi os.FileInfo) bool {
	return !strings.HasSuffix(fi.Name(), "_test.go") && !strings.HasSuffix(fi.Name(), fileSuffix)
}

func (t *templateData) dump() error {
	if len(t.Getters) == 0 {
		logf("No getters for %v; skipping.", t.filename)
		return nil
	}

	// Sort getters by ReceiverType.FieldName.
	slices.SortStableFunc(t.Getters, func(a, b *getter) int {
		return strings.Compare(a.sortVal, b.sortVal)
	})

	processTemplate := func(tmpl *template.Template, filename string) error {
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, t); err != nil {
			return err
		}
		clean, err := format.Source(buf.Bytes())
		if err != nil {
			return fmt.Errorf("format.Source:\n%v\n%v", buf.String(), err)
		}

		if isCheck() {
			logf("Checking %v...", filename)
			old, err := os.ReadFile(filename)
			if err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					return fmt.Errorf("Missing file: %v\n", filename)
				}
				return err
			}

			if !bytes.Equal(old, clean) {
				return fmt.Errorf("Generated files are out of date. Please run go generate ./... and commit the results")
			}
			return nil
		}

		logf("Writing %v...", filename)
		if err := os.Chmod(filename, 0o644); err != nil {
			return fmt.Errorf("os.Chmod(%q, 0644): %v", filename, err)
		}

		if err := os.WriteFile(filename, clean, 0o444); err != nil {
			return err
		}

		if err := os.Chmod(filename, 0o444); err != nil {
			return fmt.Errorf("os.Chmod(%q, 0444): %v", filename, err)
		}

		return nil
	}

	if err := processTemplate(sourceTmpl, t.filename); err != nil {
		return err
	}
	return processTemplate(testTmpl, strings.ReplaceAll(t.filename, ".go", "_test.go"))
}

func newGetter(receiverType, fieldName, fieldType, zeroValue string, namedStruct bool) *getter {
	return &getter{
		sortVal:      strings.ToLower(receiverType) + "." + strings.ToLower(fieldName),
		ReceiverVar:  strings.ToLower(receiverType[:1]),
		ReceiverType: receiverType,
		FieldName:    fieldName,
		FieldType:    fieldType,
		ZeroValue:    zeroValue,
		NamedStruct:  namedStruct,
	}
}

func (t *templateData) addArrayType(x *ast.ArrayType, receiverType, fieldName string, isAPointer bool) {
	var eltType string
	var ng *getter
	switch elt := x.Elt.(type) {
	case *ast.Ident:
		eltType = elt.String()
		ng = newGetter(receiverType, fieldName, "[]"+eltType, "nil", false)
	case *ast.StarExpr:
		ident, ok := elt.X.(*ast.Ident)
		if !ok {
			return
		}
		ng = newGetter(receiverType, fieldName, "[]*"+ident.String(), "nil", false)
	default:
		logf("addArrayType: type %q, field %q: unknown elt type: %T %+v; skipping.", receiverType, fieldName, elt, elt)
		return
	}

	ng.ArrayType = !isAPointer
	t.Getters = append(t.Getters, ng)
}

func (t *templateData) addSimpleValueIdent(x *ast.Ident, receiverType, fieldName string) {
	getter := genIdentGetter(x, receiverType, fieldName)
	getter.IsSimpleValue = true
	logf("addSimpleValueIdent: Processing %q - fieldName=%q, getter.ZeroValue=%q, x.Obj=%#v", x.String(), fieldName, getter.ZeroValue, x.Obj)
	if getter.ZeroValue == "nil" {
		if x.Obj == nil {
			switch x.String() {
			case "any": // NOOP - leave as `nil`
			default:
				getter.ZeroValue = x.String() + "{}"
			}
		} else {
			if ts, ok := x.Obj.Decl.(*ast.TypeSpec); ok {
				logf("addSimpleValueIdent: Processing %q of type %T", x.String(), ts.Type)
				switch xX := ts.Type.(type) {
				case *ast.Ident:
					logf("addSimpleValueIdent: Processing %q of type %T - zero value is %q", x.String(), ts.Type, getter.ZeroValue)
					getter.ZeroValue = zeroValueOfIdent(xX)
				case *ast.StructType:
					getter.ZeroValue = x.String() + "{}"
					logf("addSimpleValueIdent: Processing %q of type %T - zero value is %q", x.String(), ts.Type, getter.ZeroValue)
				case *ast.InterfaceType, *ast.ArrayType: // NOOP - leave as `nil`
					logf("addSimpleValueIdent: Processing %q of type %T - zero value is %q", x.String(), ts.Type, getter.ZeroValue)
				default:
					log.Fatalf("addSimpleValueIdent: unhandled case %T", xX)
				}
			}
		}
	}
	t.Getters = append(t.Getters, getter)
}

func (t *templateData) addIdent(x *ast.Ident, receiverType, fieldName string) {
	getter := genIdentGetter(x, receiverType, fieldName)
	t.Getters = append(t.Getters, getter)
}

func zeroValueOfIdent(x *ast.Ident) string {
	switch x.String() {
	case "int", "int64", "float64", "uint8", "uint16":
		return "0"
	case "string":
		return `""`
	case "bool":
		return "false"
	case "Timestamp":
		return "Timestamp{}"
	default:
		return "nil"
	}
}

func genIdentGetter(x *ast.Ident, receiverType, fieldName string) *getter {
	zeroValue := zeroValueOfIdent(x)
	namedStruct := zeroValue == "nil"
	return newGetter(receiverType, fieldName, x.String(), zeroValue, namedStruct)
}

func (t *templateData) addMapType(x *ast.MapType, receiverType, fieldName string, isAPointer bool) {
	var keyType string
	switch key := x.Key.(type) {
	case *ast.Ident:
		keyType = key.String()
	default:
		logf("addMapType: type %q, field %q: unknown key type: %T %+v; skipping.", receiverType, fieldName, key, key)
		return
	}

	var valueType string
	switch value := x.Value.(type) {
	case *ast.Ident:
		valueType = value.String()
	default:
		logf("addMapType: type %q, field %q: unknown value type: %T %+v; skipping.", receiverType, fieldName, value, value)
		return
	}

	fieldType := fmt.Sprintf("map[%v]%v", keyType, valueType)
	zeroValue := fmt.Sprintf("map[%v]%v{}", keyType, valueType)
	ng := newGetter(receiverType, fieldName, fieldType, zeroValue, false)
	ng.MapType = !isAPointer
	t.Getters = append(t.Getters, ng)
}

func (t *templateData) addSimpleValueSelectorExpr(x *ast.SelectorExpr, receiverType, fieldName string) {
	getter := t.genSelectorExprGetter(x, receiverType, fieldName)
	if getter == nil {
		return
	}
	getter.IsSimpleValue = true
	logf("addSimpleValueSelectorExpr: Processing field name %q - %#v - zero value is %q", fieldName, x, getter.ZeroValue)
	t.Getters = append(t.Getters, getter)
}

func (t *templateData) addSelectorExpr(x *ast.SelectorExpr, receiverType, fieldName string) {
	getter := t.genSelectorExprGetter(x, receiverType, fieldName)
	if getter == nil {
		return
	}
	t.Getters = append(t.Getters, getter)
}

func (t *templateData) genSelectorExprGetter(x *ast.SelectorExpr, receiverType, fieldName string) *getter {
	if strings.ToLower(fieldName[:1]) == fieldName[:1] { // Non-exported field.
		return nil
	}

	var xX string
	if xx, ok := x.X.(*ast.Ident); ok {
		xX = xx.String()
	}

	switch xX {
	case "time", "json":
		if xX == "json" {
			t.Imports["encoding/json"] = "encoding/json"
		} else {
			t.Imports[xX] = xX
		}
		fieldType := fmt.Sprintf("%v.%v", xX, x.Sel.Name)
		zeroValue := fmt.Sprintf("%v.%v{}", xX, x.Sel.Name)
		if xX == "time" && x.Sel.Name == "Duration" {
			zeroValue = "0"
		}
		return newGetter(receiverType, fieldName, fieldType, zeroValue, false)
	default:
		logf("addSelectorExpr: xX %q, type %q, field %q: unknown x=%+v; skipping.", xX, receiverType, fieldName, x)
	}

	return nil
}

type templateData struct {
	filename string
	Year     int
	Package  string
	Imports  map[string]string
	Getters  []*getter
}

type getter struct {
	sortVal       string // Lower-case version of "ReceiverType.FieldName".
	ReceiverVar   string // The one-letter variable name to match the ReceiverType.
	ReceiverType  string
	FieldName     string
	FieldType     string
	ZeroValue     string
	NamedStruct   bool // Getter for named struct.
	MapType       bool
	ArrayType     bool
	IsSimpleValue bool
}

const source = `// Code generated by gen-accessors; DO NOT EDIT.
// Instead, please run "go generate ./..." as described here:
// https://github.com/google/go-github/blob/master/CONTRIBUTING.md#submitting-a-patch

// Copyright {{.Year}} The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package {{.Package}}
{{with .Imports}}
import (
  {{- range . -}}
  "{{.}}"
  {{end -}}
)
{{end}}
{{range .Getters}}
{{if .IsSimpleValue}}
// Get{{.FieldName}} returns the {{.FieldName}} field.
func ({{.ReceiverVar}} *{{.ReceiverType}}) Get{{.FieldName}}() {{.FieldType}} {
  if {{.ReceiverVar}} == nil {
    return {{.ZeroValue}}
  }
  return {{.ReceiverVar}}.{{.FieldName}}
}
{{else if .NamedStruct}}
// Get{{.FieldName}} returns the {{.FieldName}} field.
func ({{.ReceiverVar}} *{{.ReceiverType}}) Get{{.FieldName}}() *{{.FieldType}} {
  if {{.ReceiverVar}} == nil {
    return {{.ZeroValue}}
  }
  return {{.ReceiverVar}}.{{.FieldName}}
}
{{else if or .MapType .ArrayType }}
// Get{{.FieldName}} returns the {{.FieldName}} {{if .MapType}}map{{else if .ArrayType }}slice{{end}} if it's non-nil, {{if .MapType}}an empty map{{else if .ArrayType }}nil{{end}} otherwise.
func ({{.ReceiverVar}} *{{.ReceiverType}}) Get{{.FieldName}}() {{.FieldType}} {
  if {{.ReceiverVar}} == nil || {{.ReceiverVar}}.{{.FieldName}} == nil {
    return {{.ZeroValue}}
  }
  return {{.ReceiverVar}}.{{.FieldName}}
}
{{else}}
// Get{{.FieldName}} returns the {{.FieldName}} field if it's non-nil, zero value otherwise.
func ({{.ReceiverVar}} *{{.ReceiverType}}) Get{{.FieldName}}() {{.FieldType}} {
  if {{.ReceiverVar}} == nil || {{.ReceiverVar}}.{{.FieldName}} == nil {
    return {{.ZeroValue}}
  }
  return *{{.ReceiverVar}}.{{.FieldName}}
}
{{end}}
{{end}}
`

const test = `// Code generated by gen-accessors; DO NOT EDIT.
// Instead, please run "go generate ./..." as described here:
// https://github.com/google/go-github/blob/master/CONTRIBUTING.md#submitting-a-patch

// Copyright {{.Year}} The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package {{.Package}}
{{with .Imports}}
import (
  "testing"
  {{range . -}}
  "{{.}}"
  {{end -}}
)
{{end}}
{{range .Getters}}
{{if .IsSimpleValue}}
func Test{{.ReceiverType}}_Get{{.FieldName}}(tt *testing.T) {
  tt.Parallel()
  {{.ReceiverVar}} := &{{.ReceiverType}}{}
  {{.ReceiverVar}}.Get{{.FieldName}}()
  {{.ReceiverVar}} = nil
  {{.ReceiverVar}}.Get{{.FieldName}}()
}
{{else if .NamedStruct}}
func Test{{.ReceiverType}}_Get{{.FieldName}}(tt *testing.T) {
  tt.Parallel()
  {{.ReceiverVar}} := &{{.ReceiverType}}{}
  {{.ReceiverVar}}.Get{{.FieldName}}()
  {{.ReceiverVar}} = nil
  {{.ReceiverVar}}.Get{{.FieldName}}()
}
{{else if or .MapType .ArrayType}}
func Test{{.ReceiverType}}_Get{{.FieldName}}(tt *testing.T) {
  tt.Parallel()
  zeroValue := {{.FieldType}}{}
  {{.ReceiverVar}} := &{{.ReceiverType}}{ {{.FieldName}}: zeroValue }
  {{.ReceiverVar}}.Get{{.FieldName}}()
  {{.ReceiverVar}} = &{{.ReceiverType}}{}
  {{.ReceiverVar}}.Get{{.FieldName}}()
  {{.ReceiverVar}} = nil
  {{.ReceiverVar}}.Get{{.FieldName}}()
}
{{else}}
func Test{{.ReceiverType}}_Get{{.FieldName}}(tt *testing.T) {
  tt.Parallel()
  var zeroValue {{.FieldType}}
  {{.ReceiverVar}} := &{{.ReceiverType}}{ {{.FieldName}}: &zeroValue }
  {{.ReceiverVar}}.Get{{.FieldName}}()
  {{.ReceiverVar}} = &{{.ReceiverType}}{}
  {{.ReceiverVar}}.Get{{.FieldName}}()
  {{.ReceiverVar}} = nil
  {{.ReceiverVar}}.Get{{.FieldName}}()
}
{{end}}
{{end}}
`
