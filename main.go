package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"
	"go.uber.org/zap/exp/zapslog"
)

type Response struct {
	Error string `json:"error,omitempty"` // The error field is omitted from JSON responses if it is empty.
	Data  any    `json:"data,omitempty"`  // The data field is omitted from JSON responses if it is empty.
}

func sendJSON(w http.ResponseWriter, response Response, status int) {
	data, err := json.Marshal(response)
	if err != nil {
		slog.Error(
			"Error marshalling response",
			slog.String("error", err.Error()),
		)

		sendJSON(w, Response{Error: "Error marshalling response"}, http.StatusInternalServerError)

		return
	}

	w.WriteHeader(status)
	if _, err := w.Write(data); err != nil {
		slog.Error(
			"Error writing response",
			slog.String("error", err.Error()),
		)

		return
	}
}

type User struct {
	Username string `json:"username"`
	ID       int64  `json:"id,string"`
	Role     string `json:"role"`
	Password string `json:"-"` // The password field is omitted from JSON responses for security reasons.
}

type Password string

func (p Password) String() string {
	return "[REDACTED]" // The password is represented as "[REDACTED]" to avoid exposing sensitive information.
}

func (p Password) LogValue() slog.Value {
	return slog.StringValue("[REDACTED]") // The password is logged as "[REDACTED]" to avoid exposing sensitive information.
}

func (u User) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("username", u.Username),
		slog.Int64("id", u.ID),
		slog.String("role", u.Role),
		slog.String("password", "[REDACTED]"), // The password is logged as "[REDACTED]" to avoid exposing sensitive information.
	)
}

const LowestLevel = slog.Level(-50) // Define a custom log level that is lower than the default levels (Debug, Info, Warn, Error).

func main() {
	// CONFIG ZAP LOGGER
	zapLogger, _ := zap.NewProduction()
	defer zapLogger.Sync() // Flush any buffered log entries before the program exits.

	zapHandler := zapslog.NewHandler(
		zapLogger.Core(),
		nil,
	)

	zapWithSlog := slog.New(zapHandler)

	zapWithSlog.Info(
		"Log with zap logger",
		slog.String("environment", "development"),
	)

	// CONFIG LOGGER
	opts := &slog.HandlerOptions{
		AddSource: true,        // AddSource: true enables the inclusion of source code information (file name and line number) in log entries.
		Level:     LowestLevel, // Minimum log level to be logged.
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr { // ReplaceAttr: A function that allows you to modify log attributes before they are logged.
			if a.Key == "level" {
				level := a.Value.String()
				if level == "DEBUG-46" {
					a.Value = slog.StringValue("LOWEST")
				}
			}

			return a
		},
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, opts))

	slog.SetDefault(logger)

	// CONFIG HTTP SERVER WITH CUSTOM ERROR LOGGING
	// server := http.Server{
	// 	ErrorLog: slog.NewLogLogger(slog.NewJSONHandler(os.Stdout, nil), slog.LevelError),
	// }
	// _ = &server

	// LOGGING
	slog.Debug(
		"Debug: This is a debug message",
		slog.String("version", "1.0.0"),
	) // Debug level logs are typically used for development and debugging purposes, and may not be shown in production environments. (obs: by default slog just logs info, warn and error levels)

	slog.Info(
		"Starting application",
		slog.String("version", "1.0.0"),
	)

	slog.Warn(
		"Warning: This is a warning message",
		slog.String("version", "1.0.0"),
	)

	slog.Error(
		"Error: This is a error message",
		slog.String("version", "1.0.0"),
	)

	logger.LogAttrs(
		context.Background(),
		slog.LevelInfo,
		"We have a http request",
		slog.Group(
			"htt_data",
			slog.String("method", http.MethodDelete),
			slog.Int("status", http.StatusNotFound),
		),
		slog.Duration("time_taken", time.Second),
		slog.String("user_agent", "Go-http-client/1.1"),
	)

	logger.LogAttrs(
		context.Background(),
		LowestLevel,
		"Lowest level log message",
	)

	// LOGGING WITH
	loggerWithVersion := logger.With(
		slog.Group(
			"app_info",
			slog.String("version", "1.0.0"),
		),
	)

	loggerWithVersion.Info("Starting application with version context")

	// PERSONALIZE VALUES WITH INTERFACE IMPLEMENTATION
	password := Password("mysecretpassword")
	slog.Info(
		"User login attempt",
		slog.String("username", "johndoe"),
		slog.Any("password", password), // The password will be logged as "[REDACTED]" due to the LogValue method implementation.
	)

	// OBS: if we implement on User struct the Password struct it won't hide the password, because the LogValue method is not called recursively on nested structs. The LogValue method is only called on the top-level struct that implements it. So, if we want to hide the password in the User struct, we need to implement the LogValue method on the User struct and return a slog.GroupValue that includes the password field with a value of "[REDACTED]".
	user := User{
		Username: "johndoe",
		ID:       1,
		Role:     "admin",
		Password: "mysecretpassword",
	}

	slog.Info(
		"User details",
		slog.Any("user", user), // The password will be logged as "[REDACTED]" due to the LogValue method implementation.
	)

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

			slog.Error(
				"Something went wrong while reading the request body",
				slog.String("error", err.Error()),
			)
			sendJSON(w, Response{Error: "Something went wrong"}, http.StatusInternalServerError)
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
