package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type Response struct {
	Error string `json:"error,omitempty"` // The error field is omitted from JSON responses if it is empty.
	Data  any    `json:"data,omitempty"`  // The data field is omitted from JSON responses if it is empty.
}

func sendJSON(w http.ResponseWriter, response Response, status int) {
	data, err := json.Marshal(response)
	if err != nil {
		sendJSON(w, Response{Error: "Error marshalling response"}, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(status)
	if _, err := w.Write(data); err != nil {
		fmt.Println("Error writing response:", err)
		return
	}
}

type User struct {
	Username string `json:"username"`
	ID       int64  `json:"id,string"`
	Role     string `json:"role"`
	Password string `json:"-"` // The password field is omitted from JSON responses for security reasons.
}

func main() {
	// BUILD ROUTER
	r := chi.NewMux()

	// MIDDLEWARES
	r.Use(middleware.Recoverer) // A middleware that recovers from panics and returns a 500 error.
	r.Use(middleware.RequestID) // A middleware that injects a request ID into the context of each request.
	r.Use(middleware.Logger)    // A middleware that logs the start and end of each request, along with some useful data about what was requested and how long it took.

	// MOCK DATA
	db := map[int64]User{
		1: {Username: "John Doe", ID: 1, Role: "admin", Password: "password123"},
		2: {Username: "Jane Smith", ID: 2, Role: "user", Password: "password456"},
		3: {Username: "Alice Johnson", ID: 3, Role: "user", Password: "password789"},
		4: {Username: "Bob Brown", ID: 4, Role: "user", Password: "password012"},
	}

	r.Group(func(r chi.Router) {
		r.Use(jsonMiddleware)

		r.Get("/users/{id:[0-9]+}", handleGetUser(db))
		r.Post("/users", handlePostUser(db))
	})

	if err := http.ListenAndServe(":8080", r); err != nil {
		panic(err)
	}
}

func jsonMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}

func handleGetUser(db map[int64]User) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		id, _ := strconv.ParseInt(idStr, 10, 64)

		user, ok := db[id]

		if !ok {
			sendJSON(w, Response{Error: "User not found"}, http.StatusNotFound)
			return
		}

		sendJSON(w, Response{Data: user}, http.StatusCreated)
	}
}

func handlePostUser(db map[int64]User) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// LIMIT REQUEST BODY SIZE
		r.Body = http.MaxBytesReader(w, r.Body, 10000) // Limit the request body size to 10KB

		data, err := io.ReadAll(r.Body)
		if err != nil {
			var maxErr *http.MaxBytesError
			if ok := errors.As(err, &maxErr); ok {
				sendJSON(w, Response{Error: "Request body too large"}, http.StatusRequestEntityTooLarge)
				return
			}

			sendJSON(w, Response{Error: "Error reading request body"}, http.StatusInternalServerError)
			return
		}
		defer r.Body.Close()

		var newUser User
		if err := json.Unmarshal(data, &newUser); err != nil {
			sendJSON(w, Response{Error: "Invalid JSON in request body"}, http.StatusUnprocessableEntity)
			return
		}

		newID := int64(len(db) + 1)
		newUser.ID = newID
		db[newID] = newUser

		w.WriteHeader(http.StatusCreated)
		sendJSON(w, Response{Data: newUser}, http.StatusCreated)
	}
}
