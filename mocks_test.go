package main

import (
	"context"
	"fmt"

	"firebase.google.com/go/v4/messaging"
)

// MockMessagingClient implements messaging.Client interface for tests
type MockMessagingClient struct {
	shouldFail bool
}

func (m *MockMessagingClient) Send(ctx context.Context, message *messaging.Message) (string, error) {
	if m.shouldFail {
		return "", fmt.Errorf("mock firebase error")
	}
	return "message_id", nil
}

func (m *MockMessagingClient) SendMulticast(ctx context.Context, message *messaging.MulticastMessage) (*messaging.BatchResponse, error) {
	if m.shouldFail {
		return nil, fmt.Errorf("mock firebase error")
	}
	return &messaging.BatchResponse{
		SuccessCount: 1,
		FailureCount: 0,
	}, nil
}

func (m *MockMessagingClient) SendEachForMulticast(ctx context.Context, message *messaging.MulticastMessage) (*messaging.BatchResponse, error) {
	if m.shouldFail {
		return nil, fmt.Errorf("mock firebase error")
	}
	return &messaging.BatchResponse{
		SuccessCount: 1,
		FailureCount: 0,
		Responses: []*messaging.SendResponse{
			{Success: true},
		},
	}, nil
}

func (m *MockMessagingClient) SubscribeToTopic(ctx context.Context, tokens []string, topic string) (*messaging.TopicManagementResponse, error) {
	if m.shouldFail {
		return nil, fmt.Errorf("mock firebase error")
	}
	return &messaging.TopicManagementResponse{
		SuccessCount: 1,
		FailureCount: 0,
	}, nil
}

func (m *MockMessagingClient) UnsubscribeFromTopic(ctx context.Context, tokens []string, topic string) (*messaging.TopicManagementResponse, error) {
	if m.shouldFail {
		return nil, fmt.Errorf("mock firebase error")
	}
	return &messaging.TopicManagementResponse{
		SuccessCount: 1,
		FailureCount: 0,
	}, nil
}

func (m *MockMessagingClient) SendDryRun(ctx context.Context, message *messaging.Message) (string, error) {
	if m.shouldFail {
		return "", fmt.Errorf("mock firebase error")
	}
	return "message_id", nil
}

func (m *MockMessagingClient) SendMulticastDryRun(ctx context.Context, message *messaging.MulticastMessage) (*messaging.BatchResponse, error) {
	if m.shouldFail {
		return nil, fmt.Errorf("mock firebase error")
	}
	return &messaging.BatchResponse{
		SuccessCount: 1,
		FailureCount: 0,
	}, nil
}

func (m *MockMessagingClient) ValidateRegistrationTokens(ctx context.Context, tokens []string) (*messaging.TopicManagementResponse, error) {
	if m.shouldFail {
		return nil, fmt.Errorf("mock firebase error")
	}
	return &messaging.TopicManagementResponse{
		SuccessCount: len(tokens),
		FailureCount: 0,
	}, nil
}
