package rds

import (
	"github.com/nukleros/aws-builder/pkg/client"
)

// RdsClient is used to manage operations on RDS Instances.
type RdsClient struct {
	client.ResourceClient

	// A channel for latest version of resource inventory to be passed to client
	// as resources are created and deleted.
	InventoryChan *chan RdsInventory
}
