// cmd/media-service/main.go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"cloud.google.com/go/firestore"
	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/storage"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/oguzkopan/cosmetics-social-backend/shared/auth"
	"github.com/oguzkopan/cosmetics-social-backend/shared/events"
)

/* ────── env vars ─────────────────────────────────────────────────────────── */

var (
	projectID = mustEnv("GOOGLE_CLOUD_PROJECT")
	port      = "8080"

	bucket    = mustEnv("MEDIA_BUCKET")      // "<project>.appspot.com"
	postTopic = mustEnv("POST_EVENTS_TOPIC") // "post-events"
	signerSA  = mustEnv("SERVICE_ACCOUNT_EMAIL")
	signerKey = mustEnv("SERVICE_ACCOUNT_KEY_PATH")
)

/* ────── globals (initialised in main) ────────────────────────────────────── */

var (
	ctx     context.Context
	fs      *firestore.Client
	stor    *storage.Client
	pubClnt *pubsub.Client
)

/* ────── main ─────────────────────────────────────────────────────────────── */

func main() {
	ctx = context.Background()

	var err error
	if fs, err = firestore.NewClient(ctx, projectID); err != nil {
		log.Fatalf("firestore: %v", err)
	}
	if stor, err = storage.NewClient(ctx); err != nil {
		log.Fatalf("storage: %v", err)
	}
	if err = auth.Init(ctx); err != nil {
		log.Fatalf("auth init: %v", err)
	}
	if err = events.Init(ctx, projectID); err != nil {
		log.Fatalf("events init: %v", err)
	}

	r := chi.NewRouter()
	r.Use(middleware.Logger)

	r.Post("/posts", createPost)
	r.Get("/posts/{id}", getPost)
	r.Get("/", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("media-svc OK")) })

	log.Printf("media-service listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}

/* ────── handlers ─────────────────────────────────────────────────────────── */

type postRequest struct {
	Caption   string `json:"caption"`
	MediaType string `json:"mediaType"` // "image" | "video"
	FileExt   string `json:"fileExt"`   // optional; jpg/mp4 guessed if empty
}

func createPost(w http.ResponseWriter, r *http.Request) {
	authorUID, err := auth.VerifyFirebaseToken(r.Context(), r)
	if err != nil {
		http.Error(w, "unauth", http.StatusUnauthorized)
		return
	}

	var req postRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	if req.FileExt == "" {
		if req.MediaType == "video" {
			req.FileExt = "mp4"
		} else {
			req.FileExt = "jpg"
		}
	}

	// ── Firestore doc & Signed URL ────────────────────────────────────────
	postRef := fs.Collection("posts").NewDoc()
	objPath := fmt.Sprintf("posts/%s/%s.%s", authorUID, postRef.ID, req.FileExt)

	uploadURL, err := signedUploadURL(objPath, 15*time.Minute)
	if err != nil {
		http.Error(w, "signed-url err", http.StatusInternalServerError)
		return
	}

	// initial placeholder document
	if _, err = postRef.Set(r.Context(), map[string]any{
		"id":           postRef.ID,
		"authorID":     authorUID,
		"caption":      req.Caption,
		"mediaPath":    objPath,
		"mediaType":    req.MediaType,
		"likeCount":    0,
		"commentCount": 0,
		"timestamp":    firestore.ServerTimestamp,
		"processed":    false,
	}); err != nil {
		http.Error(w, "db write err", http.StatusInternalServerError)
		return
	}

	events.Publish(r.Context(), postTopic, "POST_DRAFTED", map[string]string{
		"postID": postRef.ID, "authorID": authorUID, "object": objPath,
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"postID":    postRef.ID,
		"uploadURL": uploadURL,
	})
}

func getPost(w http.ResponseWriter, r *http.Request) {
	if _, err := auth.VerifyFirebaseToken(r.Context(), r); err != nil {
		http.Error(w, "unauth", http.StatusUnauthorized)
		return
	}

	docID := chi.URLParam(r, "id")
	doc, err := fs.Collection("posts").Doc(docID).Get(r.Context())
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(doc.Data())
}

/* ────── helpers ─────────────────────────────────────────────────────────── */

func signedUploadURL(object string, ttl time.Duration) (string, error) {
	keyBytes, err := os.ReadFile(signerKey) // ↓ ioutil deprecated
	if err != nil {
		return "", err
	}
	return storage.SignedURL(bucket, object, &storage.SignedURLOptions{
		Method:         "PUT",
		GoogleAccessID: signerSA,
		PrivateKey:     keyBytes,
		Expires:        time.Now().Add(ttl),
	})
}

func mustEnv(k string) string {
	v := os.Getenv(k)
	if v == "" {
		log.Fatalf("missing env %s", k)
	}
	return v
}
