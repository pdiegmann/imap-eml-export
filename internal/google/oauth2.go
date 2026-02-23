// Package google provides Google OAuth2 IMAP authentication helpers.
//
// Gmail and Google Workspace (GSuite) accounts require OAuth2 for IMAP access.
// This package handles the full OAuth2 device-authorization flow so callers
// only need to supply a username; the browser-based login and token storage are
// managed automatically.
package google

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/oauth2"
	googleoauth "golang.org/x/oauth2/google"
)

const (
	// GmailIMAPHost is the IMAP hostname for Gmail and Google Workspace.
	GmailIMAPHost = "imap.gmail.com"
	// GmailIMAPPort is the standard IMAPS port.
	GmailIMAPPort = 993

	// gmailScope grants full access to the Gmail mailbox over IMAP.
	gmailScope = "https://mail.google.com/"
)

// tokenCache is the on-disk representation of a stored OAuth2 token.
type tokenCache struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	Expiry       time.Time `json:"expiry"`
	TokenType    string    `json:"token_type"`
}

// defaultTokenCachePath returns the path used to cache the OAuth2 token when
// no explicit path is provided.
func defaultTokenCachePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "imap-eml-export", "google-token.json"), nil
}

// oauthConfig builds an *oauth2.Config from the given credentials.
func oauthConfig(clientID, clientSecret string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint:     googleoauth.Endpoint,
		Scopes:       []string{gmailScope},
	}
}

// GetAccessToken returns a valid Google OAuth2 access token for IMAP use.
//
// Lookup order for credentials:
//  1. clientID / clientSecret parameters (non-empty values take priority)
//  2. GOOGLE_CLIENT_ID / GOOGLE_CLIENT_SECRET environment variables
//
// Token caching order:
//  1. refreshToken parameter (a previously stored refresh token)
//  2. tokenCachePath file on disk
//  3. Interactive device-authorization flow (opens browser prompt)
//
// When a new token is obtained it is written to tokenCachePath (or the
// default path when tokenCachePath is empty) for future reuse.
//
// The returned refreshToken should be persisted by the caller so subsequent
// calls can skip the interactive flow.
func GetAccessToken(ctx context.Context, clientID, clientSecret, refreshToken, tokenCachePath string) (accessToken, newRefreshToken string, err error) {
	// Resolve credentials from env if not provided.
	if clientID == "" {
		clientID = os.Getenv("GOOGLE_CLIENT_ID")
	}
	if clientSecret == "" {
		clientSecret = os.Getenv("GOOGLE_CLIENT_SECRET")
	}
	if clientID == "" || clientSecret == "" {
		return "", "", fmt.Errorf(
			"Google OAuth2 client credentials are required.\n" +
				"Set GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET environment variables,\n" +
				"or add them to the [export.oauth2] / [import.oauth2] config section.\n" +
				"See https://developers.google.com/identity/protocols/oauth2 for instructions.",
		)
	}

	cfg := oauthConfig(clientID, clientSecret)

	// 1. Try the in-memory refresh token.
	if refreshToken != "" {
		tok, err := refreshFromToken(ctx, cfg, refreshToken)
		if err == nil {
			return tok.AccessToken, tok.RefreshToken, nil
		}
		// Fall through to cache / interactive flow on failure.
	}

	// 2. Try the token cache file.
	cachePath := tokenCachePath
	if cachePath == "" {
		if p, e := defaultTokenCachePath(); e == nil {
			cachePath = p
		}
	}
	if cachePath != "" {
		if tok, err := loadCachedToken(ctx, cfg, cachePath); err == nil {
			return tok.AccessToken, tok.RefreshToken, nil
		}
	}

	// 3. Interactive device-authorization flow.
	tok, err := deviceAuthFlow(ctx, cfg)
	if err != nil {
		return "", "", fmt.Errorf("Google OAuth2 authentication failed: %w", err)
	}

	// Persist the new token.
	if cachePath != "" {
		_ = saveCachedToken(cachePath, tok)
	}

	return tok.AccessToken, tok.RefreshToken, nil
}

// refreshFromToken exchanges a refresh token for a new access token.
func refreshFromToken(ctx context.Context, cfg *oauth2.Config, refreshToken string) (*oauth2.Token, error) {
	tok := &oauth2.Token{RefreshToken: refreshToken}
	ts := cfg.TokenSource(ctx, tok)
	return ts.Token()
}

// loadCachedToken reads a token from the cache file and refreshes it if needed.
func loadCachedToken(ctx context.Context, cfg *oauth2.Config, path string) (*oauth2.Token, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cache tokenCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, err
	}
	tok := &oauth2.Token{
		AccessToken:  cache.AccessToken,
		RefreshToken: cache.RefreshToken,
		Expiry:       cache.Expiry,
		TokenType:    cache.TokenType,
	}
	ts := cfg.TokenSource(ctx, tok)
	return ts.Token()
}

// saveCachedToken writes an OAuth2 token to the cache file.
func saveCachedToken(path string, tok *oauth2.Token) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	cache := tokenCache{
		AccessToken:  tok.AccessToken,
		RefreshToken: tok.RefreshToken,
		Expiry:       tok.Expiry,
		TokenType:    tok.TokenType,
	}
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// deviceAuthFlow runs the OAuth2 device authorization grant flow, printing
// instructions to stdout so the user can complete the login in a browser.
func deviceAuthFlow(ctx context.Context, cfg *oauth2.Config) (*oauth2.Token, error) {
	resp, err := cfg.DeviceAuth(ctx, oauth2.AccessTypeOffline)
	if err != nil {
		return nil, fmt.Errorf("starting device auth: %w", err)
	}

	fmt.Printf("\n┌─────────────────────────────────────────────────────────────────────┐\n")
	fmt.Printf("│               Google Account Sign-In Required                       │\n")
	fmt.Printf("├─────────────────────────────────────────────────────────────────────┤\n")
	fmt.Printf("│ 1. Open the following URL in your browser:                          │\n")
	fmt.Printf("│    %s\n", resp.VerificationURI)
	fmt.Printf("│                                                                     │\n")
	fmt.Printf("│ 2. Enter the code: %s\n", resp.UserCode)
	fmt.Printf("│                                                                     │\n")
	fmt.Printf("│ Waiting for authentication...                                       │\n")
	fmt.Printf("└─────────────────────────────────────────────────────────────────────┘\n\n")

	tok, err := cfg.DeviceAccessToken(ctx, resp, oauth2.AccessTypeOffline)
	if err != nil {
		return nil, fmt.Errorf("completing device auth: %w", err)
	}
	return tok, nil
}
