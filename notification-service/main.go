package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"

	"cloud.google.com/go/firestore"
	firebase "firebase.google.com/go"
	"firebase.google.com/go/messaging"
)

var (
	projectID = os.Getenv("GOOGLE_CLOUD_PROJECT")
	port      = "8080"
)

var (
	ctx    context.Context
	fs     *firestore.Client
	fcm    *messaging.Client
)

type pushMsg struct {
	Message struct {
		Data       string            `json:"data"`
		Attributes map[string]string `json:"attributes"`
	} `json:"message"`
}

func main() {
	ctx = context.Background()
	var err error
	if fs, err = firestore.NewClient(ctx, projectID); err != nil { log.Fatal(err) }

	app, _ := firebase.NewApp(ctx, nil)
	fcm, _ = app.Messaging(ctx)

	http.HandleFunc("/pubsub", handlePubSub)
	http.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("notif OK")) })

	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func handlePubSub(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	var m pushMsg
	_ = json.Unmarshal(body, &m)

	eventType := m.Message.Attributes["type"]
	payload := map[string]string{}
	if m.Message.Data != "" {
		b, _ := base64.StdEncoding.DecodeString(m.Message.Data)
		_ = json.Unmarshal(b, &payload)
	}

	switch eventType {
	case "USER_FOLLOWED":
		sendPush(payload["targetID"],
			"New follower",
			"Someone started following you")
	case "POST_LIKED":
		sendPushToPostOwner(payload, " liked your post")
	case "POST_COMMENTED":
		sendPushToPostOwner(payload, " commented on your post")
	case "MESSAGE_SENT":
		sendPush(payload["recipientID"],
			"New message",
			payload["text"])
	}
	w.WriteHeader(200)
}

func sendPush(uid, title, body string) {
	if uid == "" { return }
	doc, _ := fs.Collection("users").Doc(uid).Get(ctx)
	token, _ := doc.Data()["fcmToken"].(string)
	if token == "" { return }
	msg := &messaging.Message{
		Token: token,
		Notification: &messaging.Notification{Title: title, Body: body},
	}
	_, _ = fcm.Send(ctx, msg)
}

func sendPushToPostOwner(p map[string]string, suffix string) {
	postID := p["postID"]
	if postID == "" { return }
	doc, _ := fs.Collection("posts").Doc(postID).Get(ctx)
	owner := doc.Data()["authorID"].(string)
	sendPush(owner, "CosmeticSocial", p["likedBy"]+suffix)
}
