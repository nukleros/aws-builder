package s3

import (
	"fmt"
	"sync"

	"github.com/nukleros/aws-builder/pkg/client"
)

// InitCreate initializes S3 resource creation by creating an inventory
// channel, starting a goroutine to write inventory to file, creating the S3
// client and loading the S3 configuration.
func InitCreate(
	resourceClient *client.ResourceClient,
	configFile string,
	inventoryFile string,
	inventoryChan *chan S3Inventory,
	createWait *sync.WaitGroup,
) (*S3Client, *S3Config, error) {
	// capture inventory and write to file as resources are created
	createWait.Add(1)
	go func() {
		defer createWait.Done()
		for inventory := range *inventoryChan {
			if err := inventory.Write(inventoryFile); err != nil {
				fmt.Printf("failed to write inventory file: %s", err)
			}
		}
	}()

	// create client and load config
	s3Client := S3Client{
		*resourceClient,
		inventoryChan,
	}
	s3Config, err := LoadS3Config(configFile)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load S3 config file: %w", err)
	}

	return &s3Client, s3Config, nil
}

// InitDelete initializes S3 resource deletion by creating an inventory
// channel, starting a goroutine to write inventory updates to file, creating
// the S3 client and loading the inventory to be deleted.
func InitDelete(
	resourceClient *client.ResourceClient,
	inventoryFile string,
	inventoryChan *chan S3Inventory,
	deleteWait *sync.WaitGroup,
) (*S3Client, *S3Inventory, error) {
	// capture inventory and write to file as resources are deleted
	deleteWait.Add(1)
	go func() {
		defer deleteWait.Done()
		for inventory := range *inventoryChan {
			if err := inventory.Write(inventoryFile); err != nil {
				fmt.Printf("failed to write inventory file: %s", err)
			}
		}
	}()

	// create client and load inventory to delete
	s3Client := S3Client{
		*resourceClient,
		inventoryChan,
	}
	var s3Inventory S3Inventory
	if err := s3Inventory.Load(inventoryFile); err != nil {
		return nil, nil, fmt.Errorf("failed to load S3 inventory file: %w", err)
	}

	return &s3Client, &s3Inventory, nil
}
