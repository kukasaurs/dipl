package push

import (
	"context"
	firebase "firebase.google.com/go"
	"firebase.google.com/go/messaging"
	"google.golang.org/api/option"
)

type FCMClient struct {
	App       *firebase.App
	Messaging *messaging.Client
}

// NewFCMClient creates a new Firebase Cloud Messaging client
// The serviceAccountPath parameter should be the path to a Firebase service account credentials JSON file
func NewFCMClient(serviceAccountPath string) (*FCMClient, error) {
	// Create Firebase configuration with project ID explicitly specified
	// The project ID will be extracted from the service account credentials file
	opt := option.WithCredentialsFile(serviceAccountPath)

	// Initialize Firebase app with proper configuration
	app, err := firebase.NewApp(context.Background(), nil, opt)
	if err != nil {
		return nil, err
	}

	// Create Firebase messaging client
	client, err := app.Messaging(context.Background())
	if err != nil {
		return nil, err
	}

	return &FCMClient{App: app, Messaging: client}, nil
}

// SendPushNotification sends a push notification to a specific device token
func (f *FCMClient) SendPushNotification(token, title, body string) error {
	msg := &messaging.Message{
		Notification: &messaging.Notification{
			Title: title,
			Body:  body,
		},
		Token: token,
	}
	_, err := f.Messaging.Send(context.Background(), msg)
	return err
}
