/*
Package main demonstrates a HTML email cleaner.

It should be noted that this uses bluemonday to sanitize the HTML but as it
preserves the styling of the email this should not be considered a safe or XSS
secure approach.

It does function as a basic demonstration of how to take HTML emails, which are
notorious for having inconsistent, obselete and poorly formatted HTML, and to
use bluemonday to normalise the output.
*/
package main
