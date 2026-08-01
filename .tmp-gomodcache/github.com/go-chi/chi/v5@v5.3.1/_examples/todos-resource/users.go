package main

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

type usersResource struct{}

// Routes creates a REST router for the todos resource
func (rs usersResource) Routes() chi.Router {
	r := chi.NewRouter()
	// r.Use() // some middleware..

	r.Get("/", rs.List)    // GET /users - read a list of users
	r.Post("/", rs.Create) // POST /users - create a new user and persist it
	r.Put("/", rs.Delete)

	r.Route("/{id}", func(r chi.Router) {
		// r.Use(rs.TodoCtx) // lets have a users map, and lets actually load/manipulate
		r.Get("/", rs.Get)       // GET /users/{id} - read a single user by :id
		r.Put("/", rs.Update)    // PUT /users/{id} - update a single user by :id
		r.Delete("/", rs.Delete) // DELETE /users/{id} - delete a single user by :id
	})

	return r
}

func (rs usersResource) List(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("users list of stuff.."))
}

func (rs usersResource) Create(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("users create"))
}

func (rs usersResource) Get(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("user get"))
}

func (rs usersResource) Update(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("user update"))
}

func (rs usersResource) Delete(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("user delete"))
}
