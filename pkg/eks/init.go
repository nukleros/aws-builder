package eks

import (
	"fmt"
	"sync"

	"github.com/nukleros/aws-builder/pkg/client"
)

// InitCreate initializes EKS resource creation by creating an inventory
// channel, starting a goroutine to write inventory to file, creating the EKS
// client and loading the EKS configuration.
func InitCreate(
	resourceClient *client.ResourceClient,
	configFile string,
	inventoryFile string,
	inventoryChan *chan EksInventory,
	createWait *sync.WaitGroup,
) (*EksClient, *EksConfig, error) {
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
	eksClient := EksClient{
		*resourceClient,
		inventoryChan,
	}
	eksConfig, err := LoadEksConfig(configFile)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load EKS config file: %w", err)
	}

	return &eksClient, eksConfig, nil
}

// InitDelete initializes EKS resource deletion by creating an inventory
// channel, starting a goroutine to write inventory updates to file, creating
// the EKS client and loading the inventory to be deleted.
func InitDelete(
	resourceClient *client.ResourceClient,
	inventoryFile string,
	inventoryChan *chan EksInventory,
	deleteWait *sync.WaitGroup,
) (*EksClient, *EksInventory, error) {
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
	eksClient := EksClient{
		*resourceClient,
		inventoryChan,
	}
	var eksInventory EksInventory
	if err := eksInventory.Load(inventoryFile); err != nil {
		return nil, nil, fmt.Errorf("failed to load EKS inventory file: %w", err)
	}

	return &eksClient, &eksInventory, nil
}
