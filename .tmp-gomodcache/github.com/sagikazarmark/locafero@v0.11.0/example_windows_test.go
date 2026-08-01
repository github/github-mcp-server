//go:build windows

package locafero

import (
	"fmt"

	"github.com/spf13/afero"
)

func ExampleFinder_Find() {
	// Use relative paths for Windows, because we do not know the absolute path.
	// fsys := afero.NewBasePathFs(afero.NewOsFs(), "testdata")
	fsys := afero.NewOsFs()

	finder := Finder{
		Paths: []string{
			"testdata\\home\\user",
			"testdata\\etc",
		},
		Names: []string{"config.*"},
		Type:  FileTypeFile,
	}

	results, err := finder.Find(fsys)
	if err != nil {
		panic(err)
	}

	fmt.Println("On Windows:", results)

	// Output:
	// On Windows: [testdata\home\user\config.yaml testdata\etc\config.yaml]
}
