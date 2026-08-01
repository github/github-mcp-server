package locafero

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/spf13/afero"
)

func toAbsOsPath(s string) string {
	if runtime.GOOS == "windows" {
		return filepath.Join("C:", s)
	}

	return s
}

func eachToAbsOsPath(paths []string) []string {
	if paths == nil {
		return nil
	}

	newPaths := make([]string, len(paths))

	for i, p := range paths {
		newPaths[i] = toAbsOsPath(p)
	}

	return newPaths
}

func toOsPath(s string) string {
	if runtime.GOOS == "windows" {
		return filepath.Clean(s)
	}

	return s
}

func eachToOsPath(paths []string) []string {
	if paths == nil {
		return nil
	}

	newPaths := make([]string, len(paths))

	for i, p := range paths {
		newPaths[i] = toOsPath(p)
	}

	return newPaths
}

func TestFinder_Find(t *testing.T) {
	fsys := afero.NewMemMapFs()

	files := []string{
		"/home/user/.config/app/config.yaml",
		"/home/user/app/config.yaml",
		"/home/user/config.json",
		"/home/user/config.yaml",
		"/home/user/config/app.yaml",
		"/home/user/config/config.yaml",
		"/etc/app/config.yaml",
		"/etc/config.json",
		"/etc/config.yaml",
		"/etc/config/app.yaml",
		"/etc/config/config.yaml",
	}

	for _, file := range files {
		dir := filepath.Dir(toAbsOsPath(file))

		err := fsys.MkdirAll(dir, 0o777)
		mustNotBeError(t, err)

		_, err = fsys.Create(toAbsOsPath(file))
		mustNotBeError(t, err)
	}

	testCases := []struct {
		name    string
		finder  Finder
		results []string
	}{
		{
			name:    "nothing to find",
			finder:  Finder{},
			results: nil,
		},
		{
			name: "no names to find",
			finder: Finder{
				Paths: []string{"/home/user"},
			},
			results: nil,
		},
		{
			name: "no paths to find in",
			finder: Finder{
				Names: []string{"config.yaml"},
			},
			results: nil,
		},
		{
			name: "find in path",
			finder: Finder{
				Paths: []string{"/home/user"},
				Names: []string{"config.yaml"},
			},
			results: []string{
				"/home/user/config.yaml",
			},
		},
		{
			name: "find in multiple paths",
			finder: Finder{
				Paths: []string{"/home/user", "/etc"},
				Names: []string{"config.yaml"},
			},
			results: []string{
				"/home/user/config.yaml",
				"/etc/config.yaml",
			},
		},
		{
			name: "find multiple names in multiple paths",
			finder: Finder{
				Paths: []string{"/home/user", "/etc"},
				Names: []string{"config", "config.yaml"},
			},
			results: []string{
				"/home/user/config",
				"/home/user/config.yaml",
				"/etc/config",
				"/etc/config.yaml",
			},
		},
		{
			name: "find in subdirs of each other",
			finder: Finder{
				Paths: []string{"/home/user", "/home/user/app"},
				Names: []string{"config.yaml"},
			},
			results: []string{
				"/home/user/config.yaml",
				"/home/user/app/config.yaml",
			},
		},
		{
			name: "find files only",
			finder: Finder{
				Paths: []string{"/home/user", "/etc"},
				Names: []string{"config", "config.yaml"},
				Type:  FileTypeFile,
			},
			results: []string{
				"/home/user/config.yaml",
				"/etc/config.yaml",
			},
		},
		{
			name: "find dirs only",
			finder: Finder{
				Paths: []string{"/home/user", "/etc"},
				Names: []string{"config", "config.yaml"},
				Type:  FileTypeDir,
			},
			results: []string{
				"/home/user/config",
				"/etc/config",
			},
		},
		{
			name: "glob match",
			finder: Finder{
				Paths: []string{"/home/user", "/etc"},
				Names: []string{"config*"},
			},
			results: []string{
				"/home/user/config",
				"/home/user/config.json",
				"/home/user/config.yaml",
				"/etc/config",
				"/etc/config.json",
				"/etc/config.yaml",
			},
		},
		{
			name: "glob match",
			finder: Finder{
				Paths: []string{"/home/user", "/etc"},
				Names: []string{"config.*"},
			},
			results: []string{
				"/home/user/config.json",
				"/home/user/config.yaml",
				"/etc/config.json",
				"/etc/config.yaml",
			},
		},
		{
			name: "glob match files",
			finder: Finder{
				Paths: []string{"/home/user", "/etc"},
				Names: []string{"config*"},
				Type:  FileTypeFile,
			},
			results: []string{
				"/home/user/config.json",
				"/home/user/config.yaml",
				"/etc/config.json",
				"/etc/config.yaml",
			},
		},
		{
			name: "glob match dirs",
			finder: Finder{
				Paths: []string{"/home/user", "/etc"},
				Names: []string{"config*"},
				Type:  FileTypeDir,
			},
			results: []string{
				"/home/user/config",
				"/etc/config",
			},
		},
		{
			name: "glob match in subdirs of each other",
			finder: Finder{
				Paths: []string{"/home/user", "/home/user/config", "/etc", "/etc/config"},
				Names: []string{"config*"},
			},
			results: []string{
				"/home/user/config",
				"/home/user/config.json",
				"/home/user/config.yaml",
				"/home/user/config/config.yaml",
				"/etc/config",
				"/etc/config.json",
				"/etc/config.yaml",
				"/etc/config/config.yaml",
			},
		},
		{
			name: "glob match files in subdirs of each other",
			finder: Finder{
				Paths: []string{"/home/user", "/home/user/config", "/etc", "/etc/config"},
				Names: []string{"config*"},
				Type:  FileTypeFile,
			},
			results: []string{
				"/home/user/config.json",
				"/home/user/config.yaml",
				"/home/user/config/config.yaml",
				"/etc/config.json",
				"/etc/config.yaml",
				"/etc/config/config.yaml",
			},
		},
		{
			name: "glob match dirs in subdirs of each other",
			finder: Finder{
				Paths: []string{"/home/user", "/home/user/config", "/etc", "/etc/config"},
				Names: []string{"config*"},
				Type:  FileTypeDir,
			},
			results: []string{
				"/home/user/config",
				"/etc/config",
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			testCase.finder.Paths = eachToAbsOsPath(testCase.finder.Paths)

			results, err := testCase.finder.Find(fsys)
			mustNotBeError(t, err)

			beEqual(t, eachToAbsOsPath(testCase.results), results)
		})
	}
}

func TestFinder_Find_RelativePaths(t *testing.T) {
	fsys := afero.NewOsFs()

	finder := Finder{
		Paths: []string{
			"testdata/home/user",
			"testdata/etc",
		},
		Names: []string{"config.*"},
		Type:  FileTypeFile,
	}

	results, err := finder.Find(fsys)
	mustNotBeError(t, err)

	expected := []string{
		"testdata/home/user/config.yaml",
		"testdata/etc/config.yaml",
	}

	beEqual(t, eachToOsPath(expected), results)
}

func TestFinder_Find_AbsolutePaths(t *testing.T) {
	abs := func(t *testing.T, s string) string {
		t.Helper()

		a, err := filepath.Abs(s)
		mustNotBeError(t, err)

		return a
	}

	fsys := afero.NewOsFs()

	finder := Finder{
		Paths: []string{
			abs(t, "testdata/home/user"),
			abs(t, "testdata/etc"),
		},
		Names: []string{"config.*"},
		Type:  FileTypeFile,
	}

	results, err := finder.Find(fsys)
	mustNotBeError(t, err)

	expected := []string{
		abs(t, "testdata/home/user/config.yaml"),
		abs(t, "testdata/etc/config.yaml"),
	}

	beEqual(t, expected, results)
}

func FuzzFinder_Find(f *testing.F) {
	f.Add("test")     // A simple pattern
	f.Add("*")        // A wildcard
	f.Add("???[abc]") // Something with pattern syntax

	fsys := afero.NewMemMapFs()

	_ = afero.WriteFile(fsys, "foo.txt", []byte("Hello world"), 0o644)
	_ = afero.WriteFile(fsys, "bar.txt", []byte("Hello again"), 0o644)

	f.Fuzz(func(_ *testing.T, pattern string) {
		finder := Finder{
			Paths: []string{""},
			Names: []string{pattern},
			Type:  FileTypeFile,
		}

		_, _ = finder.Find(fsys)
	})
}

func beEqual[T comparable](t *testing.T, expected, actual []T) {
	t.Helper()

	if len(expected) != len(actual) {
		t.Errorf(
			"expected both lists to be the same length\nwant: %d\ngot:  %d",
			len(expected),
			len(actual),
		)
	}

	for i := range expected {
		if expected[i] != actual[i] {
			t.Errorf(
				"expected %d. element to be equal\nwant: %v\ngot:  %v",
				i+1,
				expected[i],
				actual[i],
			)
		}
	}
}

func mustNotBeError(t *testing.T, err error) {
	t.Helper()

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}
