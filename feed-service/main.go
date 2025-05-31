package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"sort"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-redis/redis/v8"

	"github.com/oguzkopan/cosmetics-social-backend/shared/auth"
)

var (
	projectID = os.Getenv("GOOGLE_CLOUD_PROJECT")
	port      = "8080"
	redisAddr = os.Getenv("REDIS_ADDR") // optional
)

var (
	ctx context.Context
	fs  *firestore.Client
	rdb *redis.Client
)

func main() {
	ctx = context.Background()
	var err error
	if fs, err = firestore.NewClient(ctx, projectID); err != nil { log.Fatal(err) }
	if err = auth.Init(ctx); err != nil { log.Fatal(err) }
	if redisAddr != "" {
		rdb = redis.NewClient(&redis.Options{Addr: redisAddr})
	}

	r := chi.NewRouter()
	r.Use(middleware.Logger)

	r.Get("/feed/global", globalFeed)
	r.Get("/feed/following", followingFeed)
	r.Get("/", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("feed-svc OK")) })

	log.Printf("feed-service listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}

func globalFeed(w http.ResponseWriter, r *http.Request) {
	cacheKey := "feed:global"
	if maybeServeCache(w, r, cacheKey) { return }

	docs, _ := fs.Collection("posts").OrderBy("timestamp", firestore.Desc).
		Limit(50).Documents(r.Context()).GetAll()

	posts := make([]map[string]any, 0, len(docs))
	for _, d := range docs { posts = append(posts, d.Data()) }
	respondAndCache(w, r, cacheKey, posts, 5*time.Minute)
}

func followingFeed(w http.ResponseWriter, r *http.Request) {
	uid, err := auth.VerifyFirebaseToken(r.Context(), r)
	if err != nil { http.Error(w, "unauth", 401); return }

	cacheKey := "feed:user:" + uid
	if maybeServeCache(w, r, cacheKey) { return }

	// gather following
	followDocs, _ := fs.Collection("users").Doc(uid).Collection("following").
		Documents(r.Context()).GetAll()
	if len(followDocs) == 0 { respondAndCache(w, r, cacheKey, []any{}, 2*time.Minute); return }

	ids := make([]string, 0, len(followDocs))
	for _, d := range followDocs { ids = append(ids, d.Ref.ID) }

	var posts []map[string]any
	for _, chunk := range chunks(ids, 10) {
		q := fs.Collection("posts").Where("authorID", "in", chunk).
			OrderBy("timestamp", firestore.Desc).Limit(50)
		iter := q.Documents(r.Context())
		for {
			doc, err := iter.Next()
			if err != nil { break }
			posts = append(posts, doc.Data())
		}
	}
	sort.Slice(posts, func(i, j int) bool {
		ti, _ := posts[i]["timestamp"].(time.Time)
		tj, _ := posts[j]["timestamp"].(time.Time)
		return ti.After(tj)
	})
	if len(posts) > 100 { posts = posts[:100] }

	respondAndCache(w, r, cacheKey, posts, 2*time.Minute)
}

// ——— helpers ————————————————————

func maybeServeCache(w http.ResponseWriter, r *http.Request, key string) bool {
	if rdb == nil { return false }
	val, err := rdb.Get(ctx, key).Result()
	if err != nil { return false }
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(val))
	return true
}

func respondAndCache(w http.ResponseWriter, r *http.Request, key string, data any, ttl time.Duration) {
	b, _ := json.Marshal(data)
	w.Header().Set("Content-Type", "application/json")
	w.Write(b)
	if rdb != nil { _ = rdb.Set(ctx, key, b, ttl).Err() }
}

func chunks(s []string, n int) [][]string {
	var out [][]string
	for len(s) > 0 {
		if len(s) < n { n = len(s) }
		out = append(out, s[:n])
		s = s[n:]
	}
	return out
}
