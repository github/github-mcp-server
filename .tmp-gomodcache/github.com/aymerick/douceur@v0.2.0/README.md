# douceur [![Build Status](https://secure.travis-ci.org/aymerick/douceur.svg?branch=master)](http://travis-ci.org/aymerick/douceur)

A simple CSS parser and inliner in Golang.

![Douceur Logo](https://github.com/aymerick/douceur/blob/master/douceur.png?raw=true "Douceur")

Parser is vaguely inspired by [CSS Syntax Module Level 3](http://www.w3.org/TR/css3-syntax) and [corresponding JS parser](https://github.com/tabatkins/parse-css).

Inliner only parses CSS defined in HTML document, it *DOES NOT* fetch external stylesheets (for now).

Inliner inserts additional attributes when possible, for example:

```html
<html>
  <head>
  <style type="text/css">
    body {
      background-color: #f2f2f2;
    }
  </style>
  </head>
  <body>
    <p>Inline me !</p>
  </body>
</html>`
```

Becomes:

```html
<html>
  <head>
  </head>
  <body style="background-color: #f2f2f2;" bgcolor="#f2f2f2">
    <p>Inline me !</p>
  </body>
</html>`
```

The `bgcolor` attribute is inserted, in addition to the inlined `background-color` style.


## Tool usage

Install tool:

    $ go install github.com/aymerick/douceur

Parse a CSS file and display result:

    $ douceur parse inputfile.css

Inline CSS in an HTML document and display result:

    $ douceur inline inputfile.html


## Library usage

Fetch package:

    $ go get github.com/aymerick/douceur


### Parse CSS

```go
package main

import (
    "fmt"

    "github.com/aymerick/douceur/parser"
)

func main() {
    input := `body {
    /* D4rK s1T3 */
    background-color: black;
        }

  p     {
    /* Try to read that ! HAHA! */
    color: red; /* L O L */
 }
`

    stylesheet, err := parser.Parse(input)
    if err != nil {
        panic("Please fill a bug :)")
    }

    fmt.Print(stylesheet.String())
}
```

Displays:

```css
body {
  background-color: black;
}
p {
  color: red;
}
```


### Inline HTML

```go
package main

import (
    "fmt"

    "github.com/aymerick/douceur/inliner"
)

func main() {
    input := `<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.0 Strict//EN" "http://www.w3.org/TR/xhtml1/DTD/xhtml1-strict.dtd">
<html xmlns="http://www.w3.org/1999/xhtml">
  <head>
<meta http-equiv="Content-Type" content="text/html; charset=utf-8"/>
<style type="text/css">
  p {
    font-family: 'Helvetica Neue', Verdana, sans-serif;
    color: #eee;
  }
</style>
  </head>
  <body>
    <p>
      Inline me please!
    </p>
</body>
</html>`

    html, err := inliner.Inline(input)
    if err != nil {
        panic("Please fill a bug :)")
    }

    fmt.Print(html)
}
```

Displays:

```css
<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.0 Strict//EN" "http://www.w3.org/TR/xhtml1/DTD/xhtml1-strict.dtd"><html xmlns="http://www.w3.org/1999/xhtml"><head>
<meta http-equiv="Content-Type" content="text/html; charset=utf-8"/>

  </head>
  <body>
    <p style="color: #eee; font-family: &#39;Helvetica Neue&#39;, Verdana, sans-serif;">
      Inline me please!
    </p>

</body></html>
```

## Test

    go test ./... -v


## Dependencies

  - Parser uses [Gorilla CSS3 tokenizer](https://github.com/gorilla/css).
  - Inliner uses [goquery](github.com/PuerkitoBio/goquery) to manipulate HTML.


## Similar projects

  - [premailer](https://github.com/premailer/premailer)
  - [roadie](https://github.com/Mange/roadie)
