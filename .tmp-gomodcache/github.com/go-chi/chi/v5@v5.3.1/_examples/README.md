chi examples
============

* [custom-handler](https://github.com/go-chi/chi/blob/master/_examples/custom-handler/main.go) - Use a custom handler function signature
* [custom-method](https://github.com/go-chi/chi/blob/master/_examples/custom-method/main.go) - Add a custom HTTP method
* [fileserver](https://github.com/go-chi/chi/blob/master/_examples/fileserver/main.go) - Easily serve static files
* [graceful](https://github.com/go-chi/chi/blob/master/_examples/graceful/main.go) - Graceful context signaling and server shutdown
* [hello-world](https://github.com/go-chi/chi/blob/master/_examples/hello-world/main.go) - Hello World!
* [limits](https://github.com/go-chi/chi/blob/master/_examples/limits/main.go) - Timeouts and Throttling
* [logging](https://github.com/go-chi/chi/blob/master/_examples/logging/main.go) - Easy structured logging for any backend
* [rest](https://github.com/go-chi/chi/blob/master/_examples/rest/main.go) - REST APIs made easy, productive and maintainable
* [router-walk](https://github.com/go-chi/chi/blob/master/_examples/router-walk/main.go) - Print to stdout a router's routes
* [todos-resource](https://github.com/go-chi/chi/blob/master/_examples/todos-resource/main.go) - Struct routers/handlers, an example of another code layout style
* [versions](https://github.com/go-chi/chi/blob/master/_examples/versions/main.go) - Demo of `chi/render` subpkg
* [pathvalue](https://github.com/go-chi/chi/blob/master/_examples/pathvalue/main.go) - Demonstrates `PathValue` usage for retrieving URL parameters


## Usage

1. `go get -v -d -u ./...` - fetch example deps
2. `cd <example>/` ie. `cd rest/`
3. `go run *.go` - note, example services run on port 3333
4. Open another terminal and use curl to send some requests to your example service,
   `curl -v http://localhost:3333/`
5. Read <example>/main.go source to learn how service works and read comments for usage
