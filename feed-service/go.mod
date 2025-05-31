module github.com/oguzkopan/cosmetics-social-backend/feed-service

go 1.22

require (
	cloud.google.com/go/firestore v1.11.0
	github.com/go-chi/chi/v5      v5.0.10
	github.com/go-redis/redis/v8  v8.11.5
)

replace github.com/example/cosmetics-social-backend/shared => ../shared
