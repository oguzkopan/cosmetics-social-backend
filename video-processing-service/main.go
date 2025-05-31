package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"cloud.google.com/go/firestore"
	"cloud.google.com/go/storage"
)

var (
	projectID = os.Getenv("GOOGLE_CLOUD_PROJECT")
	port      = "8080"
)

var (
	ctx context.Context
	fs  *firestore.Client
	st  *storage.Client
)

type pubSub struct {
	Message struct {
		Attributes map[string]string `json:"attributes"`
		Data       string            `json:"data"`
	} `json:"message"`
}

type gcsEvent struct {
	Bucket      string `json:"bucket"`
	Name        string `json:"name"`
	ContentType string `json:"contentType"`
}

func main() {
	ctx = context.Background()
	var err error
	if fs, err = firestore.NewClient(ctx, projectID); err != nil { log.Fatal(err) }
	if st, err = storage.NewClient(ctx); err != nil { log.Fatal(err) }

	http.HandleFunc("/pubsub", handle)
	http.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("video OK")) })
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func handle(w http.ResponseWriter, r *http.Request) {
	var push pubSub
	body, _ := io.ReadAll(r.Body)
	_ = json.Unmarshal(body, &push)

	// data is base64 encoded GCS event
	var ev gcsEvent
	b, _ := base64.StdEncoding.DecodeString(push.Message.Data)
	_ = json.Unmarshal(b, &ev)

	if !strings.HasSuffix(ev.Name, ".mp4") { w.WriteHeader(200); return }

	tmp := filepath.Join(os.TempDir(), "video.mp4")
	if err := download(ev.Bucket, ev.Name, tmp); err != nil { log.Println("dl:", err); return }

	thumb := filepath.Join(os.TempDir(), "thumb.jpg")
	if err := exec.Command("ffmpeg", "-y", "-i", tmp,
		"-ss", "00:00:01.0", "-vframes", "1",
		"-vf", "scale=640:-1", thumb).Run(); err != nil {
		log.Println("ffmpeg:", err); return
	}

	thumbObj := strings.TrimSuffix(ev.Name, ".mp4") + "_thumb.jpg"
	if err := upload(ev.Bucket, thumbObj, thumb); err != nil { log.Println("up:", err) }

	postID := extractPostID(ev.Name)
	if postID != "" {
		url := fmt.Sprintf("https://storage.googleapis.com/%s/%s", ev.Bucket, thumbObj)
		_, _ = fs.Collection("posts").Doc(postID).Set(ctx, map[string]any{
			"thumbnailURL": url, "processed": true,
		}, firestore.MergeAll)
	}
	w.WriteHeader(200)
}

// ——— helpers ————————————————————————————————

func download(bucket, object, dest string) error {
	rc, err := st.Bucket(bucket).Object(object).NewReader(ctx)
	if err != nil { return err }
	defer rc.Close()
	file, _ := os.Create(dest)
	defer file.Close()
	_, err = io.Copy(file, rc)
	return err
}

func upload(bucket, object, src string) error {
	file, _ := os.Open(src); defer file.Close()
	wc := st.Bucket(bucket).Object(object).NewWriter(ctx)
	defer wc.Close()
	wc.ContentType = "image/jpeg"
	_, err := io.Copy(wc, file)
	return err
}

func extractPostID(obj string) string {
	parts := strings.Split(obj, "/")
	if len(parts) < 3 { return "" }
	filename := parts[len(parts)-1] // "<postID>.mp4"
	return strings.TrimSuffix(filename, filepath.Ext(filename))
}
