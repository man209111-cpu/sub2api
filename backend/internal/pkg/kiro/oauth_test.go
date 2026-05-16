//go:build unit

package kiro

import (
	"fmt"
	"testing"
	"time"
)

func TestBuildSocialSignInURLUsesAppPortal(t *testing.T) {
	got := BuildSocialSignInURL("http://localhost:49153", "challenge123", "state456")
	want := "https://app.kiro.dev/signin?code_challenge=challenge123&code_challenge_method=S256&redirect_from=KiroIDE&redirect_uri=http%3A%2F%2Flocalhost%3A49153&state=state456"
	if got != want {
		t.Fatalf("BuildSocialSignInURL() = %q, want %q", got, want)
	}
}

func TestBuildSocialTokenRedirectURI(t *testing.T) {
	got := BuildSocialTokenRedirectURI("http://localhost:49153", "/oauth/callback", "github")
	want := "http://localhost:49153/oauth/callback?login_option=github"
	if got != want {
		t.Fatalf("BuildSocialTokenRedirectURI() = %q, want %q", got, want)
	}
}

func TestSessionStoreGetDeletesExpiredSession(t *testing.T) {
	store := NewSessionStore()
	store.Set("expired", &AuthSession{CreatedAt: time.Now().Add(-2 * sessionTTL)})

	session, ok := store.Get("expired")
	if ok || session != nil {
		t.Fatalf("Get(expired) = (%v, %v), want (nil, false)", session, ok)
	}
	if _, exists := store.data["expired"]; exists {
		t.Fatalf("expired session should be deleted from the store")
	}
}

func TestSessionStoreSetPrunesExpiredSessions(t *testing.T) {
	store := NewSessionStore()
	now := time.Now()
	for i := 0; i < sessionCleanupMin; i++ {
		store.data[fmt.Sprintf("expired-%d", i)] = &AuthSession{CreatedAt: now.Add(-2 * sessionTTL)}
	}
	store.setCount = sessionCleanupEvery - 1

	store.Set("fresh", &AuthSession{CreatedAt: now})

	if len(store.data) != 1 {
		t.Fatalf("store size = %d, want 1", len(store.data))
	}
	if _, ok := store.data["fresh"]; !ok {
		t.Fatalf("fresh session should remain after pruning")
	}
}

func TestParseImportedRefreshTokenAcceptsRefreshTokenOnlyPayload(t *testing.T) {
	token, refreshOnly, err := ParseImportedRefreshToken(`{"refreshToken":"rt","provider":"Google"}`)
	if err != nil {
		t.Fatalf("ParseImportedRefreshToken() error = %v", err)
	}
	if !refreshOnly {
		t.Fatalf("refreshOnly = false, want true")
	}
	if token.RefreshToken != "rt" {
		t.Fatalf("refresh token = %q, want rt", token.RefreshToken)
	}
	if token.Provider != "Google" {
		t.Fatalf("provider = %q, want Google", token.Provider)
	}
	if token.AuthMethod != "social" {
		t.Fatalf("auth method = %q, want social", token.AuthMethod)
	}
}

func TestParseImportedRefreshTokenKeepsFullTokenAsNonRefreshOnly(t *testing.T) {
	token, refreshOnly, err := ParseImportedRefreshToken(`{"accessToken":"at","refreshToken":"rt"}`)
	if err != nil {
		t.Fatalf("ParseImportedRefreshToken() error = %v", err)
	}
	if refreshOnly {
		t.Fatalf("refreshOnly = true, want false")
	}
	if token.Provider != "Google" {
		t.Fatalf("provider = %q, want default Google", token.Provider)
	}
}
