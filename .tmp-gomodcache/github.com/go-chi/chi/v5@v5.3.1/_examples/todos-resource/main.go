// This example demonstrates a project structure that defines a subrouter and its
// handlers on a struct, and mounting them as subrouters to a parent router.
// See also _examples/rest for an in-depth example of a REST service, and apply
// those same patterns to this structure.
package main

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("."))
	})

	r.Mount("/users", usersResource{}.Routes())
	r.Mount("/todos", todosResource{}.Routes())

	http.ListenAndServe(":3333", r)
}
