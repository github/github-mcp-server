// This example demonstrates the use of Timeout, and Throttle middlewares.
//
// Timeout: cancel a request if processing takes longer than 2.5 seconds,
// server will respond with a http.StatusGatewayTimeout.
//
// Throttle: limit the number of in-flight requests along a particular
// routing path and backlog the others.
package main

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("root."))
	})

	r.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("pong"))
	})

	r.Get("/panic", func(w http.ResponseWriter, r *http.Request) {
		panic("test")
	})

	// Slow handlers/operations.
	r.Group(func(r chi.Router) {
		// Stop processing after 2.5 seconds.
		r.Use(middleware.Timeout(2500 * time.Millisecond))

		r.Get("/slow", func(w http.ResponseWriter, r *http.Request) {
			rand.Seed(time.Now().Unix())

			// Processing will take 1-5 seconds.
			processTime := time.Duration(rand.Intn(4)+1) * time.Second

			select {
			case <-r.Context().Done():
				return

			case <-time.After(processTime):
				// The above channel simulates some hard work.
			}

			w.Write([]byte(fmt.Sprintf("Processed in %v seconds\n", processTime)))
		})
	})

	// Throttle very expensive handlers/operations.
	r.Group(func(r chi.Router) {
		// Stop processing after 30 seconds.
		r.Use(middleware.Timeout(30 * time.Second))

		// Only one request will be processed at a time.
		r.Use(middleware.Throttle(1))

		r.Get("/throttled", func(w http.ResponseWriter, r *http.Request) {
			select {
			case <-r.Context().Done():
				switch r.Context().Err() {
				case context.DeadlineExceeded:
					w.WriteHeader(504)
					w.Write([]byte("Processing too slow\n"))
				default:
					w.Write([]byte("Canceled\n"))
				}
				return

			case <-time.After(5 * time.Second):
				// The above channel simulates some hard work.
			}

			w.Write([]byte("Processed\n"))
		})
	})

	http.ListenAndServe(":3333", r)
}
