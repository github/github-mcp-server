// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package gosdk

import (
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestCopyrightHeaders(t *testing.T) {
	var re = regexp.MustCompile(`Copyright \d{4} The Go MCP SDK Authors. All rights reserved.
Use of this source code is governed by (the license\n|an MIT-style\nlicense )that can be found in the LICENSE file.`)

	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip directories starting with "." or "_", and testdata directories.
		if d.IsDir() && d.Name() != "." &&
			(strings.HasPrefix(d.Name(), ".") ||
				strings.HasPrefix(d.Name(), "_") ||
				filepath.Base(d.Name()) == "testdata") {

			return filepath.SkipDir
		}

		// Skip non-go files.
		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		// Check the copyright header.
		f, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ParseComments|parser.PackageClauseOnly)
		if err != nil {
			return fmt.Errorf("parsing %s: %v", path, err)
		}
		if len(f.Comments) == 0 {
			t.Errorf("File %s must start with a copyright header matching %s", path, re)
		} else if !re.MatchString(f.Comments[0].Text()) {
			t.Errorf("Header comment for %s does not match expected copyright header.\ngot:\n%s\nwant matching:%s", path, f.Comments[0].Text(), re)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
