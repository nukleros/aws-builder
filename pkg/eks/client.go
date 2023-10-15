package eks

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"

	"github.com/nukleros/aws-builder/pkg/client"
)

// EksClient is used to manage operations on EKS clusters.
type EksClient struct {
	client.ResourceClient

	// A channel for latest version of resource inventory to be passed to client
	// as resources are created and deleted.
	InventoryChan *chan EksInventory
}

func (c *EksClient) GetMessageChan() *chan string {
	return c.MessageChan
}

func (c *EksClient) GetContext() context.Context {
	return c.Context
}

func (c *EksClient) GetAwsConfig() *aws.Config {
	return c.AwsConfig
}
