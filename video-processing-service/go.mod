module github.com/oguzkopan/cosmetics-social-backend/video-processing-service

go 1.22

require (
	cloud.google.com/go/firestore v1.11.0
	cloud.google.com/go/storage   v1.39.0
)

replace github.com/oguzkopan/cosmetics-social-backend/shared => ../shared
