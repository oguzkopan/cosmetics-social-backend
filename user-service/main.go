package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"cloud.google.com/go/firestore"
	"github.com/oguzkopan/cosmetics-social-backend/shared/auth"
	"github.com/oguzkopan/cosmetics-social-backend/shared/events"
)

var (
	projectID = os.Getenv("GOOGLE_CLOUD_PROJECT")
	port      = "8080"

	fs  *firestore.Client
	ctx context.Context

	topic = os.Getenv("SOCIAL_EVENTS_TOPIC") // e.g. "social-events"
)

func main() {
	ctx = context.Background()

	// —— bootstrap clients
	var err error
	fs, err = firestore.NewClient(ctx, projectID)
	if err != nil { log.Fatalf("firestore: %v", err) }
	if err = auth.Init(ctx); err != nil { log.Fatalf("auth init: %v", err) }
	if err = events.Init(ctx, projectID); err != nil { log.Fatalf("pubsub init: %v", err) }

	// —— router
	r := chi.NewRouter()
	r.Use(middleware.Logger, middleware.RequestID, middleware.Timeout(15*time.Second))

	r.Get("/users/{id}", getProfile)
	r.Put("/users/{id}", updateProfile)
	r.Post("/users/{id}/follow", followUser)
	r.Delete("/users/{id}/follow", unfollowUser)
	r.Get("/", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("user-svc OK")) })

	log.Printf("user-service listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}

func getProfile(w http.ResponseWriter, r *http.Request) {
	uid := chi.URLParam(r, "id")
	doc, err := fs.Collection("users").Doc(uid).Get(r.Context())
	if err != nil { http.Error(w, "not found", 404); return }
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(doc.Data())
}

func updateProfile(w http.ResponseWriter, r *http.Request) {
	uid := chi.URLParam(r, "id")
	me, err := auth.VerifyFirebaseToken(r.Context(), r)
	if err != nil { http.Error(w, "unauth", 401); return }
	if me != uid { http.Error(w, "forbidden", 403); return }

	var updates map[string]any
	_ = json.NewDecoder(r.Body).Decode(&updates)
	_, err = fs.Collection("users").Doc(uid).Set(r.Context(), updates, firestore.MergeAll)
	if err != nil { http.Error(w, err.Error(), 500); return }
	w.WriteHeader(204)
}

func followUser(w http.ResponseWriter, r *http.Request) {
	target := chi.URLParam(r, "id")
	follower, err := auth.VerifyFirebaseToken(r.Context(), r)
	if err != nil { http.Error(w, "unauth", 401); return }
	if follower == target { http.Error(w, "bad request", 400); return }

	b := fs.Batch()
	b.Set(fs.Collection("users").Doc(follower).Collection("following").Doc(target), map[string]any{})
	b.Set(fs.Collection("users").Doc(target).Collection("followers").Doc(follower), map[string]any{})
	b.Update(fs.Collection("users").Doc(follower), []firestore.Update{{Path: "followingCount", Value: firestore.Increment(1)}})
	b.Update(fs.Collection("users").Doc(target),   []firestore.Update{{Path: "followersCount", Value: firestore.Increment(1)}})
	if _, err := b.Commit(r.Context()); err != nil {
		http.Error(w, err.Error(), 500); return
	}
	events.Publish(r.Context(), topic, "USER_FOLLOWED", map[string]string{
		"followerID": follower, "targetID": target,
	})
	w.WriteHeader(204)
}

func unfollowUser(w http.ResponseWriter, r *http.Request) {
	target := chi.URLParam(r, "id")
	follower, err := auth.VerifyFirebaseToken(r.Context(), r)
	if err != nil { http.Error(w, "unauth", 401); return }

	b := fs.Batch()
	b.Delete(fs.Collection("users").Doc(follower).Collection("following").Doc(target))
	b.Delete(fs.Collection("users").Doc(target).Collection("followers").Doc(follower))
	b.Update(fs.Collection("users").Doc(follower), []firestore.Update{{Path: "followingCount", Value: firestore.Increment(-1)}})
	b.Update(fs.Collection("users").Doc(target),   []firestore.Update{{Path: "followersCount", Value: firestore.Increment(-1)}})
	_, _ = b.Commit(r.Context())
	w.WriteHeader(204)
}
