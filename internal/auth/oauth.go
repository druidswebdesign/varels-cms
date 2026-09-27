package auth

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"net/http"

	"github.com/gorilla/sessions"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	"github.com/markbates/goth/providers/google"
)

// SetupOAuth configures gothic's state cookie store and registers the Google
// provider. Call once at startup.
func SetupOAuth(sessionSecret string, secure bool, providers ...goth.Provider) {
	store := sessions.NewCookieStore([]byte(sessionSecret))
	store.Options = &sessions.Options{
		Path:     "/",
		HttpOnly: true,
		MaxAge:   3600,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
	}
	gothic.Store = store
	goth.UseProviders(providers...)
}

// GoogleProvider builds the Goth Google provider.
func GoogleProvider(clientID, secret, redirectURL string) *google.Provider {
	return google.New(clientID, secret, redirectURL, "email", "profile")
}

// RandomSecret returns a hex-encoded 32-byte secret for the cookie stores.
func RandomSecret() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		log.Fatalf("auth: crypto/rand: %v", err)
	}
	return hex.EncodeToString(b)
}

// WithProvider tags the request so gothic can resolve the provider from query.
func WithProvider(r *http.Request, name string) *http.Request {
	q := r.URL.Query()
	q.Set("provider", name)
	r.URL.RawQuery = q.Encode()
	return r
}
