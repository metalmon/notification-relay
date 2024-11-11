package mocks

import (
	"context"

	"firebase.google.com/go/v4/messaging"
	"github.com/stretchr/testify/mock"
)

// MockFirebaseMessagingClient is a mock implementation of FirebaseMessagingClient
type MockFirebaseMessagingClient struct {
	mock.Mock
}

func (m *MockFirebaseMessagingClient) Send(ctx context.Context, message *messaging.Message) (string, error) {
	args := m.Called(ctx, message)
	return args.String(0), args.Error(1)
}

func (m *MockFirebaseMessagingClient) SubscribeToTopic(ctx context.Context, tokens []string, topic string) (*messaging.TopicManagementResponse, error) {
	args := m.Called(ctx, tokens, topic)
	return args.Get(0).(*messaging.TopicManagementResponse), args.Error(1)
}

func (m *MockFirebaseMessagingClient) UnsubscribeFromTopic(ctx context.Context, tokens []string, topic string) (*messaging.TopicManagementResponse, error) {
	args := m.Called(ctx, tokens, topic)
	return args.Get(0).(*messaging.TopicManagementResponse), args.Error(1)
}
