package main

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

type todosResource struct{}

// Routes creates a REST router for the todos resource
func (rs todosResource) Routes() chi.Router {
	r := chi.NewRouter()
	// r.Use() // some middleware..

	r.Get("/", rs.List)    // GET /todos - read a list of todos
	r.Post("/", rs.Create) // POST /todos - create a new todo and persist it
	r.Put("/", rs.Delete)

	r.Route("/{id}", func(r chi.Router) {
		// r.Use(rs.TodoCtx) // lets have a todos map, and lets actually load/manipulate
		r.Get("/", rs.Get)       // GET /todos/{id} - read a single todo by :id
		r.Put("/", rs.Update)    // PUT /todos/{id} - update a single todo by :id
		r.Delete("/", rs.Delete) // DELETE /todos/{id} - delete a single todo by :id
		r.Get("/sync", rs.Sync)
	})

	return r
}

func (rs todosResource) List(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("todos list of stuff.."))
}

func (rs todosResource) Create(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("todos create"))
}

func (rs todosResource) Get(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("todo get"))
}

func (rs todosResource) Update(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("todo update"))
}

func (rs todosResource) Delete(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("todo delete"))
}

func (rs todosResource) Sync(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("todo sync"))
}
