package main

import (
	"context"

	"firebase.google.com/go/v4/messaging"
)

// FirebaseMessagingClient interface defines the methods we use from Firebase messaging
type FirebaseMessagingClient interface {
	Send(ctx context.Context, message *messaging.Message) (string, error)
	SubscribeToTopic(ctx context.Context, tokens []string, topic string) (*messaging.TopicManagementResponse, error)
	UnsubscribeFromTopic(ctx context.Context, tokens []string, topic string) (*messaging.TopicManagementResponse, error)
}
