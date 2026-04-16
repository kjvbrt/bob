package auth

import (
	"context"
	"fmt"
	"os"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

const CERNIssuer = "https://auth.cern.ch/auth/realms/cern"

// Client wraps the OIDC provider and OAuth2 config for CERN SSO.
type Client struct {
	Provider *oidc.Provider
	Config   oauth2.Config
	Verifier *oidc.IDTokenVerifier
}

// Claims holds the CERN SSO user claims we care about.
type Claims struct {
	Sub               string `json:"sub"`
	PreferredUsername string `json:"preferred_username"`
	Name              string `json:"name"`
	Email             string `json:"email"`
}

func NewClient(ctx context.Context) (*Client, error) {
	clientID := os.Getenv("OIDC_CLIENT_ID")
	if clientID == "" {
		return nil, fmt.Errorf("OIDC_CLIENT_ID env var is required")
	}
	clientSecret := os.Getenv("OIDC_CLIENT_SECRET")
	if clientSecret == "" {
		return nil, fmt.Errorf("OIDC_CLIENT_SECRET env var is required")
	}
	redirectURL := os.Getenv("OIDC_REDIRECT_URL")
	if redirectURL == "" {
		return nil, fmt.Errorf("OIDC_REDIRECT_URL env var is required")
	}

	provider, err := oidc.NewProvider(ctx, CERNIssuer)
	if err != nil {
		return nil, fmt.Errorf("init OIDC provider: %w", err)
	}

	cfg := oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Endpoint:     provider.Endpoint(),
		Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
	}

	verifier := provider.Verifier(&oidc.Config{ClientID: clientID})

	return &Client{
		Provider: provider,
		Config:   cfg,
		Verifier: verifier,
	}, nil
}
