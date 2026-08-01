package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/aymerick/douceur/inliner"
	"github.com/aymerick/douceur/parser"
)

const (
	// Version is package version
	Version = "0.2.0"
)

var (
	flagVersion bool
)

func init() {
	flag.BoolVar(&flagVersion, "version", false, "Display version")
}

func main() {
	flag.Parse()

	if flagVersion {
		fmt.Println(Version)
		os.Exit(0)
	}

	args := flag.Args()

	if len(args) == 0 {
		fmt.Println("No command supplied")
		os.Exit(1)
	}

	switch args[0] {
	case "parse":
		if len(args) < 2 {
			fmt.Println("Missing file path")
			os.Exit(1)
		}

		parseCSS(args[1])
	case "inline":
		if len(args) < 2 {
			fmt.Println("Missing file path")
			os.Exit(1)
		}

		inlineCSS(args[1])
	default:
		fmt.Println("Unexpected command: ", args[0])
		os.Exit(1)
	}
}

// parse and display CSS file
func parseCSS(filePath string) {
	input := readFile(filePath)

	stylesheet, err := parser.Parse(string(input))
	if err != nil {
		fmt.Println("Parsing error: ", err)
		os.Exit(1)
	}

	fmt.Println(stylesheet.String())
}

// inlines CSS into HTML and display result
func inlineCSS(filePath string) {
	input := readFile(filePath)

	output, err := inliner.Inline(string(input))
	if err != nil {
		fmt.Println("Inlining error: ", err)
		os.Exit(1)
	}

	fmt.Println(output)
}

func readFile(filePath string) []byte {
	file, err := ioutil.ReadFile(filePath)
	if err != nil {
		fmt.Println("Failed to open file: ", filePath, err)
		os.Exit(1)
	}

	return file
}
