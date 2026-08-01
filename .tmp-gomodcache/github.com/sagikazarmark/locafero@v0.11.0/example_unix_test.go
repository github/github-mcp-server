//go:build !windows && !plan9

package locafero

import (
	"fmt"

	"github.com/spf13/afero"
)

func ExampleFinder_Find() {
	fsys := afero.NewBasePathFs(afero.NewOsFs(), "testdata")

	finder := Finder{
		Paths: []string{
			"/home/user",
			"/etc",
		},
		Names: []string{"config.*"},
		Type:  FileTypeFile,
	}

	results, err := finder.Find(fsys)
	if err != nil {
		panic(err)
	}

	fmt.Println("On Unix:", results)

	// Output:
	// On Unix: [/home/user/config.yaml /etc/config.yaml]
}
