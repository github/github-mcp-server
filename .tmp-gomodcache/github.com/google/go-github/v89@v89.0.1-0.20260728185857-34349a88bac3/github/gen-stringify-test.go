// Copyright 2019 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build ignore

// gen-stringify-test generates test methods to test the String methods.
//
// These tests eliminate most of the code coverage problems so that real
// code coverage issues can be more readily identified.
//
// It is meant to be used by go-github contributors in conjunction with the
// go generate tool before sending a PR to GitHub.
// Please see the CONTRIBUTING.md file for more information.
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
	"strings"
	"text/template"
)

const (
	ignoreFilePrefix1 = "gen-"
	ignoreFilePrefix2 = "github-"
	outputFileSuffix  = "-stringify_test.go"
)

var (
	check   = flag.Bool("check", false, "Check whether generated files are up to date")
	verbose = flag.Bool("v", false, "Print verbose log messages")

	// skipStructMethods lists "struct.method" combos to skip.
	skipStructMethods = map[string]bool{}
	// skipStructs lists structs to skip.
	skipStructs = map[string]bool{
		"RateLimits": true,
	}

	funcMap = template.FuncMap{
		"isNotLast": func(index int, slice []*structField) string {
			if index+1 < len(slice) {
				return ", "
			}
			return ""
		},
		"processZeroValue": func(v string) string {
			switch v {
			case "false":
				return "false"
			case "Ptr(false)":
				return "false"
			case "Ptr(0.0)":
				return "0"
			case "0", "Ptr(0)", "Ptr(int64(0))":
				return "0"
			case `""`, `Ptr("")`:
				return `""`
			case "Timestamp{}", "&Timestamp{}":
				return "github.Timestamp{0001-01-01 00:00:00 +0000 UTC}"
			case "nil":
				return "map[]"
			case `[]int{0}`:
				return `[0]`
			case `[]string{""}`:
				return `[""]`
			case "[]Scope{ScopeNone}":
				return `["(no scope)"]`
			case "[]any{nil}":
				return "[<nil>]"
			}
			log.Fatalf("Unhandled zero value: %q", v)
			return ""
		},
	}

	sourceTmpl = template.Must(template.New("source").Funcs(funcMap).Parse(source))
)

func isCheck() bool {
	return *check || os.Getenv("CHECK") == "1"
}

func main() {
	flag.Parse()
	fset := token.NewFileSet()

	pkgs, err := parser.ParseDir(fset, ".", sourceFilter, 0)
	if err != nil {
		log.Fatal(err)
		return
	}

	for pkgName, pkg := range pkgs {
		t := &templateData{
			filename:     pkgName + outputFileSuffix,
			Year:         2019, // No need to change this once set (even in following years).
			Package:      pkgName,
			Imports:      map[string]string{"testing": "testing"},
			StringFuncs:  map[string]bool{},
			StructFields: map[string][]*structField{},
		}
		for filename, f := range pkg.Files {
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

func sourceFilter(fi os.FileInfo) bool {
	return !strings.HasSuffix(fi.Name(), "_test.go") &&
		!strings.HasPrefix(fi.Name(), ignoreFilePrefix1) &&
		!strings.HasPrefix(fi.Name(), ignoreFilePrefix2)
}

type templateData struct {
	filename     string
	Year         int
	Package      string
	Imports      map[string]string
	StringFuncs  map[string]bool
	StructFields map[string][]*structField
}

type structField struct {
	sortVal      string // Lower-case version of "ReceiverType.FieldName".
	ReceiverVar  string // The one-letter variable name to match the ReceiverType.
	ReceiverType string
	FieldName    string
	FieldType    string
	ZeroValue    string
	NamedStruct  bool // Getter for named struct.
}

func (t *templateData) processAST(f *ast.File) error {
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok {
			if fn.Recv != nil && len(fn.Recv.List) > 0 {
				id, ok := fn.Recv.List[0].Type.(*ast.Ident)
				if ok && fn.Name.Name == "String" {
					logf("Got FuncDecl: Name=%q, id.Name=%#v", fn.Name.Name, id.Name)
					t.StringFuncs[id.Name] = true
				} else {
					star, ok := fn.Recv.List[0].Type.(*ast.StarExpr)
					if ok && fn.Name.Name == "String" {
						id, ok := star.X.(*ast.Ident)
						if ok {
							logf("Got FuncDecl: Name=%q, id.Name=%#v", fn.Name.Name, id.Name)
							t.StringFuncs[id.Name] = true
						} else {
							logf("Ignoring FuncDecl: Name=%q, Type=%T", fn.Name.Name, fn.Recv.List[0].Type)
						}
					} else {
						logf("Ignoring FuncDecl: Name=%q, Type=%T", fn.Name.Name, fn.Recv.List[0].Type)
					}
				}
			} else {
				logf("Ignoring FuncDecl: Name=%q, fn=%#v", fn.Name.Name, fn)
			}
			continue
		}

		gd, ok := decl.(*ast.GenDecl)
		if !ok {
			logf("Ignoring AST decl type %T", decl)
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
			st, ok := ts.Type.(*ast.StructType)
			if !ok {
				logf("Ignoring AST type %T, Name=%q", ts.Type, ts.Name)
				continue
			}
			for _, field := range st.Fields.List {
				if len(field.Names) == 0 {
					continue
				}

				fieldName := field.Names[0]
				if id, ok := field.Type.(*ast.Ident); ok {
					t.addIdent(id, ts.Name.String(), fieldName.String())
					continue
				}

				if at, ok := field.Type.(*ast.ArrayType); ok {
					if id, ok := at.Elt.(*ast.Ident); ok {
						t.addIdentSlice(id, ts.Name.String(), fieldName.String())
						continue
					}
				}

				se, ok := field.Type.(*ast.StarExpr)
				if !ok {
					logf("Ignoring type %T for Name=%q, FieldName=%q", field.Type, ts.Name, fieldName)
					continue
				}

				// Skip unexported identifiers.
				if !fieldName.IsExported() {
					logf("Field %v is unexported; skipping.", fieldName)
					continue
				}
				// Check if "struct.method" should be skipped.
				if key := fmt.Sprintf("%v.Get%v", ts.Name, fieldName); skipStructMethods[key] {
					logf("Method %v is in skip list; skipping.", key)
					continue
				}

				switch x := se.X.(type) {
				case *ast.ArrayType:
				case *ast.Ident:
					t.addIdentPtr(x, ts.Name.String(), fieldName.String())
				case *ast.MapType:
				case *ast.SelectorExpr:
				default:
					logf("processAST: type %q, field %q, unknown %T: %+v", ts.Name, fieldName, x, x)
				}
			}
		}
	}
	return nil
}

func (t *templateData) addMapType(receiverType, fieldName string) {
	t.StructFields[receiverType] = append(t.StructFields[receiverType], newStructField(receiverType, fieldName, "map[]", "nil", false))
}

func (t *templateData) addIdent(x *ast.Ident, receiverType, fieldName string) {
	var zeroValue string
	var namedStruct bool
	switch x.String() {
	case "int":
		zeroValue = "0"
	case "int64":
		zeroValue = "0"
	case "float64":
		zeroValue = "0.0"
	case "string":
		zeroValue = `""`
	case "bool":
		zeroValue = "false"
	case "Timestamp":
		zeroValue = "Timestamp{}"
	default:
		zeroValue = "nil"
		namedStruct = true
	}

	t.StructFields[receiverType] = append(t.StructFields[receiverType], newStructField(receiverType, fieldName, x.String(), zeroValue, namedStruct))
}

func (t *templateData) addIdentPtr(x *ast.Ident, receiverType, fieldName string) {
	var zeroValue string
	var namedStruct bool
	switch x.String() {
	case "int":
		zeroValue = "Ptr(0)"
	case "int64":
		zeroValue = "Ptr(int64(0))"
	case "float64":
		zeroValue = "Ptr(0.0)"
	case "string":
		zeroValue = `Ptr("")`
	case "bool":
		zeroValue = "Ptr(false)"
	case "Timestamp":
		zeroValue = "&Timestamp{}"
	default:
		zeroValue = "nil"
		namedStruct = true
	}

	t.StructFields[receiverType] = append(t.StructFields[receiverType], newStructField(receiverType, fieldName, x.String(), zeroValue, namedStruct))
}

func (t *templateData) addIdentSlice(x *ast.Ident, receiverType, fieldName string) {
	var zeroValue string
	var namedStruct bool
	switch x.String() {
	case "int":
		zeroValue = "[]int{0}"
	case "int64":
		zeroValue = "[]int64{0}"
	case "float64":
		zeroValue = "[]float64{0}"
	case "string":
		zeroValue = `[]string{""}`
	case "bool":
		zeroValue = "[]bool{false}"
	case "Scope":
		zeroValue = "[]Scope{ScopeNone}"
	case "any":
		zeroValue = "[]any{nil}"
	// case "Timestamp":
	// 	zeroValue = "&Timestamp{}"
	default:
		zeroValue = "nil"
		namedStruct = true
	}

	t.StructFields[receiverType] = append(t.StructFields[receiverType], newStructField(receiverType, fieldName, x.String(), zeroValue, namedStruct))
}

func (t *templateData) dump() error {
	if len(t.StructFields) == 0 {
		logf("No StructFields for %v; skipping.", t.filename)
		return nil
	}

	// Remove unused structs.
	var toDelete []string
	for k := range t.StructFields {
		if !t.StringFuncs[k] {
			toDelete = append(toDelete, k)
			continue
		}
	}
	for _, k := range toDelete {
		delete(t.StructFields, k)
	}

	var buf bytes.Buffer
	if err := sourceTmpl.Execute(&buf, t); err != nil {
		return err
	}
	clean, err := format.Source(buf.Bytes())
	if err != nil {
		log.Printf("failed-to-format source:\n%v", buf)
		return err
	}

	if isCheck() {
		logf("Checking %v...", t.filename)
		old, err := os.ReadFile(t.filename)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return fmt.Errorf("Missing file: %v\n", t.filename)
			}
			return err
		}

		if !bytes.Equal(old, clean) {
			return fmt.Errorf("Generated files are out of date. Please run go generate ./... and commit the results")
		}
		return nil
	}

	logf("Writing %v...", t.filename)
	if err := os.Chmod(t.filename, 0o644); err != nil {
		return fmt.Errorf("os.Chmod(%q, 0644): %v", t.filename, err)
	}

	if err := os.WriteFile(t.filename, clean, 0o444); err != nil {
		return err
	}

	if err := os.Chmod(t.filename, 0o444); err != nil {
		return fmt.Errorf("os.Chmod(%q, 0444): %v", t.filename, err)
	}

	return nil
}

func newStructField(receiverType, fieldName, fieldType, zeroValue string, namedStruct bool) *structField {
	return &structField{
		sortVal:      strings.ToLower(receiverType) + "." + strings.ToLower(fieldName),
		ReceiverVar:  strings.ToLower(receiverType[:1]),
		ReceiverType: receiverType,
		FieldName:    fieldName,
		FieldType:    fieldType,
		ZeroValue:    zeroValue,
		NamedStruct:  namedStruct,
	}
}

func logf(fmt string, args ...any) {
	if *verbose {
		log.Printf(fmt, args...)
	}
}

const source = `// Code generated by gen-stringify-tests; DO NOT EDIT.
// Instead, please run "go generate ./..." as described here:
// https://github.com/google/go-github/blob/master/CONTRIBUTING.md#submitting-a-patch

// Copyright {{.Year}} The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package {{ $package := .Package}}{{$package}}
{{with .Imports}}
import (
  {{- range . -}}
  "{{.}}"
  {{end -}}
)
{{end}}
{{range $key, $value := .StructFields}}
func Test{{ $key }}_String(t *testing.T) {
  t.Parallel()
  v := {{ $key }}{ {{range .}}{{if .NamedStruct}}
    {{ .FieldName }}: &{{ .FieldType }}{},{{else}}
    {{ .FieldName }}: {{.ZeroValue}},{{end}}{{end}}
  }
 	want := ` + "`" + `{{ $package }}.{{ $key }}{{ $slice := . }}{
{{- range $ind, $val := .}}{{if .NamedStruct}}{{ .FieldName }}:{{ $package }}.{{ .FieldType }}{}{{else}}{{ .FieldName }}:{{ processZeroValue .ZeroValue }}{{end}}{{ isNotLast $ind $slice }}{{end}}}` + "`" + `
	if got := v.String(); got != want {
		t.Errorf("{{ $key }}.String = %v, want %v", got, want)
	}
}
{{end}}
`
