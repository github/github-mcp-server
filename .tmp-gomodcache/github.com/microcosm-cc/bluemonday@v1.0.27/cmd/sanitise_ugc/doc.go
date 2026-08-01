/*
Package main demonstrates a simple user generated content sanitizer.

This is the configuration I use on the sites that I run, it allows a lot of safe
HTML that in my case comes from the blackfriday markdown package. As markdown
itself allows HTML the UGCPolicy includes most common HTML.

CSS and JavaScript is excluded (not white-listed), as are form elements and most
embedded media that isn't just an image or image map.

As I'm paranoid, I also do not allow data-uri images and embeds.
*/
package main
