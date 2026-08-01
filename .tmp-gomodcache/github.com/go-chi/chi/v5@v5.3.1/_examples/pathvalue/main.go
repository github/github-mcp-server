package main

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func main() {
	r := chi.NewRouter()

	// Registering a handler that retrieves a path parameter using PathValue
	r.Get("/users/{userID}", pathValueHandler)

	http.ListenAndServe(":3333", r)
}

// pathValueHandler retrieves a URL parameter using PathValue and writes it to the response.
func pathValueHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("userID")

	// Respond with the extracted userID
	w.Write([]byte(fmt.Sprintf("User ID: %s", userID)))
}
