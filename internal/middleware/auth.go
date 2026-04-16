package middleware

import (
	"context"
	"net/http"

	"dataset-tracker/internal/models"
)

type contextKey string

const userKey contextKey = "user"

// GetUser retrieves the authenticated user from the request context.
// Returns nil if the request is unauthenticated.
func GetUser(r *http.Request) *models.User {
	u, _ := r.Context().Value(userKey).(*models.User)
	return u
}

// Auth is an HTTP middleware that loads the session cookie, looks up the
// user, and injects it into the request context. It does NOT require auth —
// routes that need auth should use RequireAuth.
func Auth(userRepo *models.UserStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if cookie, err := r.Cookie("session"); err == nil && cookie.Value != "" {
				if user, err := userRepo.GetSession(cookie.Value); err == nil {
					ctx := context.WithValue(r.Context(), userKey, user)
					r = r.WithContext(ctx)
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireAuth wraps a HandlerFunc and redirects to /login when no session exists.
func RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if GetUser(r) == nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		next(w, r)
	}
}

// RequireManager wraps a HandlerFunc and returns 403 for non-managers.
func RequireManager(next http.HandlerFunc) http.HandlerFunc {
	return RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		if !GetUser(r).IsManager() {
			http.Error(w, "Forbidden — manager access required", http.StatusForbidden)
			return
		}
		next(w, r)
	})
}
