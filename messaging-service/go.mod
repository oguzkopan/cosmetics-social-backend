module github.com/oguzkopan/cosmetics-social-backend/messaging-service

go 1.22

require (
	cloud.google.com/go/firestore v1.11.0
	cloud.google.com/go/pubsub    v1.36.0
    firebase.google.com/go/v4     v4.15.0    // ← latest valid tag
    github.com/go-chi/chi/v5      v5.0.10
)

replace github.com/oguzkopan/cosmetics-social-backend/shared => ../shared
