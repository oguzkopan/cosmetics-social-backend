package auth

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	firebase "firebase.google.com/go"
	"firebase.google.com/go/auth"
)

// Lazy-init – every service calls auth.Init(ctx) once in main()
var client *auth.Client

// Init must be called from main() *before* Verify().
func Init(ctx context.Context) error {
	if client != nil { return nil }
	app, err := firebase.NewApp(ctx, nil)
	if err != nil { return err }
	client, err = app.Auth(ctx)
	return err
}

// VerifyFirebaseToken extracts and validates the Firebase ID-token.
// Returns UID on success.
func VerifyFirebaseToken(ctx context.Context, r *http.Request) (string, error) {
	if client == nil { return "", fmt.Errorf("auth not initialised") }

	raw := r.Header.Get("Authorization")
	if raw == "" { return "", fmt.Errorf("missing Authorization header") }
	tokenString := strings.TrimPrefix(raw, "Bearer ")
	tok, err := client.VerifyIDToken(ctx, tokenString)
	if err != nil { return "", err }
	return tok.UID, nil
}
