package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	// BUILD ROUTER
	r := chi.NewMux()

	// MIDDLEWARES
	r.Use(middleware.Recoverer) // A middleware that recovers from panics and returns a 500 error.
	r.Use(middleware.RequestID) // A middleware that injects a request ID into the context of each request.
	r.Use(middleware.Logger)    // A middleware that logs the start and end of each request, along with some useful data about what was requested and how long it took.

	// GROUP ROUTES
	r.Route("/api", func(r chi.Router) {
		r.Route("/v1", func(r chi.Router) {
			r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprintln(w, "OK")
			})
		})

		r.Route("/v2", func(r chi.Router) {
			r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprintln(w, "OK")
			})
		})

		// ADD MIDDLEWARE TO A SPECIFIC ROUTE
		r.With(middleware.ClientIPFromRemoteAddr).
			Get("/ip", func(w http.ResponseWriter, r *http.Request) {
				ip := middleware.GetClientIP(r.Context())
				fmt.Fprintln(w, ip)
			})

		// GROUPING HANDLERS (allow you to apply middleware to a group of routes)
		r.Group(func(r chi.Router) {
			r.Use(middleware.BasicAuth("", map[string]string{
				"admin": "admin",
			}))

			r.Get("/admin", func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprintln(w, "Welcome to the admin panel!")
			})
		})
	})

	r.Get("/hour", func(w http.ResponseWriter, r *http.Request) {
		now := time.Now()
		fmt.Fprintln(w, now)
	})

	if err := http.ListenAndServe(":8080", r); err != nil {
		panic(err)
	}
}
