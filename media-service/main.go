package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
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

var (
	projectID = mustEnv("GOOGLE_CLOUD_PROJECT")
	port      = "8080"

	// env specific to this service
	bucket     = mustEnv("MEDIA_BUCKET")      // "<project>.appspot.com"
	postTopic  = mustEnv("POST_EVENTS_TOPIC") // "post-events"
	signerSA   = mustEnv("SERVICE_ACCOUNT_EMAIL")
	signerKey  = mustEnv("SERVICE_ACCOUNT_KEY_PATH")
)

var (
	ctx    context.Context
	fs     *firestore.Client
	stor   *storage.Client
	pubCli *pubsub.Client
)

func main() {
	ctx = context.Background()

	var err error
	if fs, err = firestore.NewClient(ctx, projectID); err != nil { log.Fatal(err) }
	if stor, err = storage.NewClient(ctx); err != nil { log.Fatal(err) }
	if err = auth.Init(ctx); err != nil { log.Fatal(err) }
	if err = events.Init(ctx, projectID); err != nil { log.Fatal(err) }

	r := chi.NewRouter()
	r.Use(middleware.Logger)

	r.Post("/posts", createPost)
	r.Get("/posts/{id}", getPost)
	r.Get("/", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("media-svc OK")) })

	log.Printf("media-service listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}

// POST /posts  – returns {postID, uploadURL}
func createPost(w http.ResponseWriter, r *http.Request) {
	author, err := auth.VerifyFirebaseToken(r.Context(), r)
	if err != nil { http.Error(w, "unauth", http.StatusUnauthorized); return }

	var req struct{ Caption, MediaType, FileExt string }
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.FileExt == "" {
		if req.MediaType == "video" { req.FileExt = "mp4" } else { req.FileExt = "jpg" }
	}

	postRef := fs.Collection("posts").NewDoc()
	objPath := fmt.Sprintf("posts/%s/%s.%s", author, postRef.ID, req.FileExt)

	uploadURL, err := signedUploadURL(objPath, 15*time.Minute)
	if err != nil { http.Error(w, "signed-url err", 500); return }

	// initial post document (mediaURL added later by video-processing or client)
	_ = postRef.Set(r.Context(), map[string]any{
		"id":           postRef.ID,
		"authorID":     author,
		"caption":      req.Caption,
		"mediaPath":    objPath,
		"mediaType":    req.MediaType,
		"likeCount":    0,
		"commentCount": 0,
		"timestamp":    firestore.ServerTimestamp,
		"processed":    false,
	})

	events.Publish(r.Context(), postTopic, "POST_DRAFTED", map[string]string{
		"postID": postRef.ID, "authorID": author, "object": objPath,
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"postID":    postRef.ID,
		"uploadURL": uploadURL,
	})
}

// GET /posts/{id}
func getPost(w http.ResponseWriter, r *http.Request) {
	_, err := auth.VerifyFirebaseToken(r.Context(), r)
	if err != nil { http.Error(w, "unauth", 401); return }

	pid := chi.URLParam(r, "id")
	doc, err := fs.Collection("posts").Doc(pid).Get(r.Context())
	if err != nil { http.Error(w, "not found", 404); return }

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(doc.Data())
}

// ——— helpers ———————————————————————————————————————————————

func signedUploadURL(object string, ttl time.Duration) (string, error) {
	keyBytes, err := ioutil.ReadFile(signerKey)
	if err != nil { return "", err }
	return storage.SignedURL(bucket, object, &storage.SignedURLOptions{
		Method:         "PUT",
		GoogleAccessID: signerSA,
		PrivateKey:     keyBytes,
		Expires:        time.Now().Add(ttl),
	})
}

func mustEnv(k string) string {
	v := os.Getenv(k)
	if v == "" { log.Fatalf("missing env %s", k) }
	return v
}
