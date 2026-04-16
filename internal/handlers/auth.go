package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"dataset-tracker/internal/middleware"
	"dataset-tracker/internal/models"
)

const sessionCookie = "session"
const stateCookie = "oidc_state"
const sessionTTL = 24 * time.Hour
const stateTTL = 10 * time.Minute

// managerUsernames returns the set of CERN usernames with manager role,
// read from the MANAGER_USERNAMES env var (comma-separated).
func managerUsernames() map[string]bool {
	raw := os.Getenv("MANAGER_USERNAMES")
	m := map[string]bool{}
	for _, u := range strings.Split(raw, ",") {
		if u = strings.TrimSpace(u); u != "" {
			m[u] = true
		}
	}
	return m
}

func (h *Handler) ShowLogin(w http.ResponseWriter, r *http.Request) {
	if middleware.GetUser(r) != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	h.renderPage(w, r, "login", PageData{Title: "Login", DevMode: h.devMode})
}

// DevLogin is only active when DEV_MODE=true. It accepts a plain form with
// a username and role and creates a local session — no OIDC involved.
func (h *Handler) DevLogin(w http.ResponseWriter, r *http.Request) {
	if !h.devMode {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", 400)
		return
	}

	username := strings.TrimSpace(r.FormValue("username"))
	if username == "" {
		username = "devuser"
	}
	role := models.Role(r.FormValue("role"))
	if role != models.RoleManager && role != models.RoleRequester {
		role = models.RoleRequester
	}

	user, err := h.users.Upsert(username, username, username+"@dev.local", role)
	if err != nil {
		slog.Error("dev login upsert", "error", err)
		http.Error(w, "Internal Server Error", 500)
		return
	}

	token := generateToken()
	expiresAt := time.Now().Add(sessionTTL)
	if err := h.users.CreateSession(user.ID, token, expiresAt); err != nil {
		http.Error(w, "Internal Server Error", 500)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	slog.Warn("dev login", "user", username, "role", role)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}


// Login initiates the CERN SSO OIDC flow.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	if h.oidc == nil {
		http.Error(w, "OIDC not configured — set OIDC_CLIENT_ID, OIDC_CLIENT_SECRET, OIDC_REDIRECT_URL", 503)
		return
	}

	state := generateToken()
	http.SetCookie(w, &http.Cookie{
		Name:     stateCookie,
		Value:    state,
		Path:     "/",
		MaxAge:   int(stateTTL.Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	url := h.oidc.Config.AuthCodeURL(state)
	http.Redirect(w, r, url, http.StatusFound)
}

// Callback handles the CERN SSO redirect after authentication.
func (h *Handler) Callback(w http.ResponseWriter, r *http.Request) {
	if h.oidc == nil {
		http.Error(w, "OIDC not configured", 503)
		return
	}

	// Verify state
	stateCk, err := r.Cookie(stateCookie)
	if err != nil || stateCk.Value != r.URL.Query().Get("state") {
		http.Error(w, "Invalid state — possible CSRF", http.StatusBadRequest)
		return
	}
	// Clear state cookie
	http.SetCookie(w, &http.Cookie{Name: stateCookie, Value: "", MaxAge: -1, Path: "/"})

	// Exchange code for token
	token, err := h.oidc.Config.Exchange(context.Background(), r.URL.Query().Get("code"))
	if err != nil {
		slog.Error("oidc token exchange", "error", err)
		http.Error(w, "Authentication failed", http.StatusInternalServerError)
		return
	}

	// Verify ID token
	rawID, ok := token.Extra("id_token").(string)
	if !ok {
		http.Error(w, "No id_token in response", http.StatusInternalServerError)
		return
	}
	idToken, err := h.oidc.Verifier.Verify(context.Background(), rawID)
	if err != nil {
		slog.Error("oidc id_token verify", "error", err)
		http.Error(w, "Token verification failed", http.StatusInternalServerError)
		return
	}

	// Extract claims
	var claims struct {
		Sub               string `json:"sub"`
		PreferredUsername string `json:"preferred_username"`
		Name              string `json:"name"`
		Email             string `json:"email"`
	}
	if err := idToken.Claims(&claims); err != nil {
		http.Error(w, "Failed to parse claims", http.StatusInternalServerError)
		return
	}

	// Determine role
	role := models.RoleRequester
	if managerUsernames()[claims.PreferredUsername] {
		role = models.RoleManager
	}

	// Upsert user (role only set on first creation; subsequent logins keep existing role)
	user, err := h.users.Upsert(claims.PreferredUsername, claims.Name, claims.Email, role)
	if err != nil {
		slog.Error("upsert user", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Create session
	sessionToken := generateToken()
	expiresAt := time.Now().Add(sessionTTL)
	if err := h.users.CreateSession(user.ID, sessionToken, expiresAt); err != nil {
		slog.Error("create session", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    sessionToken,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	slog.Info("user logged in", "user", claims.PreferredUsername, "role", user.Role)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// Logout clears the session and redirects to the CERN SSO logout endpoint.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	if ck, err := r.Cookie(sessionCookie); err == nil {
		h.users.DeleteSession(ck.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: "", Path: "/", MaxAge: -1,
	})
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func generateToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}
