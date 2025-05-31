package events

import (
	"context"
	"encoding/json"
	"log"

	"cloud.google.com/go/pubsub"
)

var topicCache = map[string]*pubsub.Topic{}
var client *pubsub.Client

func Init(ctx context.Context, projectID string) error {
	if client != nil { return nil }
	c, err := pubsub.NewClient(ctx, projectID)
	if err != nil { return err }
	client = c
	return nil
}

// Publish serialises payload as JSON and sends it with attribute "type".
func Publish(ctx context.Context, topicName, eventType string, payload any) {
	if client == nil {
		log.Println("events: client not initialised")
		return
	}
	topic := topicCache[topicName]
	if topic == nil {
		topic = client.Topic(topicName)
		topicCache[topicName] = topic
	}
	data, _ := json.Marshal(payload)
	res := topic.Publish(ctx, &pubsub.Message{
		Data:       data,
		Attributes: map[string]string{"type": eventType},
	})
	if _, err := res.Get(ctx); err != nil {
		log.Printf("events: publish err: %v", err)
	}
}
