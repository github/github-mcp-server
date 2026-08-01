module github.com/microcosm-cc/bluemonday

go 1.19

require (
	github.com/aymerick/douceur v0.2.0
	golang.org/x/net v0.26.0
)

require github.com/gorilla/css v1.0.1 // indirect

retract [v1.0.0, v1.0.25] // Retract older versions as only latest is to be depended upon
