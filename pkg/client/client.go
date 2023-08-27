package client

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
)

// ResourceClient contains the elements needed to manage resources.
type ResourceClient struct {
	// A channel for messages to be passed to client as resources are created
	// and deleted.
	MessageChan *chan string

	// A context object available for passing data across operations.
	Context context.Context

	// The AWS configuration for default settings and credentials.
	AwsConfig *aws.Config
}

// CreateResourceClient configures a resource client and returns it.
func CreateResourceClient(awsConfig *aws.Config) *ResourceClient {
	msgChan := make(chan string)
	ctx := context.Background()
	resourceClient := ResourceClient{&msgChan, ctx, awsConfig}

	return &resourceClient
}

// SendMessage sends a message on the resource client's message channel if
// present.
func (c *ResourceClient) SendMessage(message string) {
	if c.MessageChan != nil {
		*c.MessageChan <- message
	}
}
