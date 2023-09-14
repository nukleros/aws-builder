package s3

import (
	"github.com/nukleros/aws-builder/pkg/client"
)

// S3Client is used to manage operations on S3 buckets.
type S3Client struct {
	client.ResourceClient

	// A channel for latest version of resource inventory to be passed to client
	// as resources are created and deleted.
	InventoryChan *chan S3Inventory
}
