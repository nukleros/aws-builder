package rds

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"

	"github.com/nukleros/aws-builder/pkg/client"
)

// RdsClient is used to manage operations on RDS instances.
type RdsClient struct {
	client.ResourceClient

	// A channel for latest version of resource inventory to be passed to client
	// as resources are created and deleted.
	InventoryChan *chan RdsInventory
}

func (c *RdsClient) GetMessageChan() *chan string {
	return c.MessageChan
}

func (c *RdsClient) GetContext() context.Context {
	return c.Context
}

func (c *RdsClient) GetAwsConfig() *aws.Config {
	return c.AwsConfig
}
