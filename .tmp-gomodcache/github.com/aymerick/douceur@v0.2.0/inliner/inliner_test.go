package inliner

import (
	"fmt"
	"testing"
)

// Simple rule inlining with two declarations
func TestInliner(t *testing.T) {
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

	expectedOutput := `<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.0 Strict//EN" "http://www.w3.org/TR/xhtml1/DTD/xhtml1-strict.dtd"><html xmlns="http://www.w3.org/1999/xhtml"><head>
<meta http-equiv="Content-Type" content="text/html; charset=utf-8"/>

  </head>
  <body>
    <p style="color: #eee; font-family: &#39;Helvetica Neue&#39;, Verdana, sans-serif;">
      Inline me please!
    </p>

</body></html>`

	output, err := Inline(input)
	if err != nil {
		t.Fatal("Failed to inline html:", err)
	}

	if output != expectedOutput {
		t.Fatal(fmt.Sprintf("CSS inliner error\nExpected:\n\"%s\"\nGot:\n\"%s\"", expectedOutput, output))
	}
}

// Already inlined style has more priority than <style>
func TestInlineStylePriority(t *testing.T) {
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
    <p style="color: #222;">
      Inline me please!
    </p>
</body>
</html>`

	expectedOutput := `<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.0 Strict//EN" "http://www.w3.org/TR/xhtml1/DTD/xhtml1-strict.dtd"><html xmlns="http://www.w3.org/1999/xhtml"><head>
<meta http-equiv="Content-Type" content="text/html; charset=utf-8"/>

  </head>
  <body>
    <p style="color: #222; font-family: &#39;Helvetica Neue&#39;, Verdana, sans-serif;">
      Inline me please!
    </p>

</body></html>`

	output, err := Inline(input)
	if err != nil {
		t.Fatal("Failed to inline html:", err)
	}

	if output != expectedOutput {
		t.Fatal(fmt.Sprintf("CSS inliner error\nExpected:\n\"%s\"\nGot:\n\"%s\"", expectedOutput, output))
	}
}

// !important has highest priority
func TestImportantPriority(t *testing.T) {
	input := `<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.0 Strict//EN" "http://www.w3.org/TR/xhtml1/DTD/xhtml1-strict.dtd">
<html xmlns="http://www.w3.org/1999/xhtml">
  <head>
<meta http-equiv="Content-Type" content="text/html; charset=utf-8"/>
<style type="text/css">
  p#test {
    color: #000;
  }
  p {
    color: #eee !important;
  }
</style>
  </head>
  <body>
    <p id="test" style="color: #222;">
      Inline me please!
    </p>
</body>
</html>`

	expectedOutput := `<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.0 Strict//EN" "http://www.w3.org/TR/xhtml1/DTD/xhtml1-strict.dtd"><html xmlns="http://www.w3.org/1999/xhtml"><head>
<meta http-equiv="Content-Type" content="text/html; charset=utf-8"/>

  </head>
  <body>
    <p id="test" style="color: #eee;">
      Inline me please!
    </p>

</body></html>`

	output, err := Inline(input)
	if err != nil {
		t.Fatal("Failed to inline html:", err)
	}

	if output != expectedOutput {
		t.Fatal(fmt.Sprintf("CSS inliner error\nExpected:\n\"%s\"\nGot:\n\"%s\"", expectedOutput, output))
	}
}

// Pseudo-class selectors can't be inlined
func TestNotInlinable(t *testing.T) {
	input := `<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.0 Strict//EN" "http://www.w3.org/TR/xhtml1/DTD/xhtml1-strict.dtd">
<html xmlns="http://www.w3.org/1999/xhtml">
  <head>
<meta http-equiv="Content-Type" content="text/html; charset=utf-8"/>
<style type="text/css">
    @media only screen and (max-width: 600px) {
      table[class="body"] .container {
        width: 95% !important;
      }
    }

    a:hover {
      color: #2795b6 !important;
    }

    a:active {
      color: #2795b6 !important;
    }

    a:visited {
      color: #2795b6 !important;
    }
</style>
  </head>
  <body>
    <p>
      <a href="http://aymerick.com">Superbe website</a>
    </p>
</body>
</html>`

	expectedOutput := `<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.0 Strict//EN" "http://www.w3.org/TR/xhtml1/DTD/xhtml1-strict.dtd"><html xmlns="http://www.w3.org/1999/xhtml"><head>
<meta http-equiv="Content-Type" content="text/html; charset=utf-8"/>

  <style type="text/css">
@media only screen and (max-width: 600px) {
  table[class="body"] .container {
    width: 95% !important;
  }
}
a:hover {
  color: #2795b6 !important;
}
a:active {
  color: #2795b6 !important;
}
a:visited {
  color: #2795b6 !important;
}
</style></head>
  <body>
    <p>
      <a href="http://aymerick.com">Superbe website</a>
    </p>

</body></html>`

	output, err := Inline(input)
	if err != nil {
		t.Fatal("Failed to inline html:", err)
	}

	if output != expectedOutput {
		t.Fatal(fmt.Sprintf("CSS inliner error\nExpected:\n\"%s\"\nGot:\n\"%s\"", expectedOutput, output))
	}
}

// Some styles causes insertion of additional element attributes
func TestStyleToAttr(t *testing.T) {
	input := `<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.0 Strict//EN" "http://www.w3.org/TR/xhtml1/DTD/xhtml1-strict.dtd">
<html xmlns="http://www.w3.org/1999/xhtml">
  <head>
<meta http-equiv="Content-Type" content="text/html; charset=utf-8"/>
<style type="text/css">
  h1, h2, h3, h4, h5, h6, p, div, blockquote, tr, th, td {
    text-align: left;
  }
  body, table, tr, th, td {
    background-color: #f2f2f2;
  }
  table {
    background-image: url(my_image.png);
  }
  th, td {
    vertical-align: top;
  }
  img {
    float: left;
  }
</style>
  </head>
  <body>
    <h1>test</h1>
    <h2>test</h2>
    <h3>test</h3>
    <h4>test</h4>
    <h5>test</h5>
    <h6>test</h6>
    <p>
      <img src="my_image.png"/>
    </p>
    <div>test</div>
    <blockquote>test</blockquote>
    <table>
      <tbody>
        <tr>
          <th>
          </th>
          <td>
          </td>
        </tr>
      </tbody>
    </table>
</body>
</html>`

	expectedOutput := `<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.0 Strict//EN" "http://www.w3.org/TR/xhtml1/DTD/xhtml1-strict.dtd"><html xmlns="http://www.w3.org/1999/xhtml"><head>
<meta http-equiv="Content-Type" content="text/html; charset=utf-8"/>

  </head>
  <body style="background-color: #f2f2f2;" bgcolor="#f2f2f2">
    <h1 style="text-align: left;" align="left">test</h1>
    <h2 style="text-align: left;" align="left">test</h2>
    <h3 style="text-align: left;" align="left">test</h3>
    <h4 style="text-align: left;" align="left">test</h4>
    <h5 style="text-align: left;" align="left">test</h5>
    <h6 style="text-align: left;" align="left">test</h6>
    <p style="text-align: left;" align="left">
      <img src="my_image.png" style="float: left;" align="left"/>
    </p>
    <div style="text-align: left;" align="left">test</div>
    <blockquote style="text-align: left;" align="left">test</blockquote>
    <table style="background-color: #f2f2f2; background-image: url(my_image.png);" bgcolor="#f2f2f2" background="url(my_image.png)">
      <tbody>
        <tr style="background-color: #f2f2f2; text-align: left;" bgcolor="#f2f2f2" align="left">
          <th style="background-color: #f2f2f2; text-align: left; vertical-align: top;" bgcolor="#f2f2f2" align="left" valign="top">
          </th>
          <td style="background-color: #f2f2f2; text-align: left; vertical-align: top;" bgcolor="#f2f2f2" align="left" valign="top">
          </td>
        </tr>
      </tbody>
    </table>

</body></html>`

	output, err := Inline(input)
	if err != nil {
		t.Fatal("Failed to inline html:", err)
	}

	if output != expectedOutput {
		t.Fatal(fmt.Sprintf("CSS inliner error\nExpected:\n\"%s\"\nGot:\n\"%s\"", expectedOutput, output))
	}
}
