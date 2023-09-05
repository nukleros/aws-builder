package rds

import (
	"fmt"
	"sync"

	"github.com/nukleros/aws-builder/pkg/client"
)

// InitCreate initializes RDS resource creation by creating an inventory
// channel, starting a goroutine to write inventory to file, creating the RDS
// client and loading the RDS configuration.
func InitCreate(
	resourceClient *client.ResourceClient,
	configFile string,
	inventoryFile string,
	inventoryChan *chan RdsInventory,
	createWait *sync.WaitGroup,
) (*RdsClient, *RdsConfig, error) {
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
	rdsClient := RdsClient{
		*resourceClient,
		inventoryChan,
	}
	rdsConfig, err := LoadRdsConfig(configFile)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load RDS config file: %w", err)
	}

	return &rdsClient, rdsConfig, nil
}

// InitDelete initializes RDS resource deletion by creating an inventory
// channel, starting a goroutine to write inventory updates to file, creating
// the RDS client and loading the inventory to be deleted.
func InitDelete(
	resourceClient *client.ResourceClient,
	inventoryFile string,
	inventoryChan *chan RdsInventory,
	deleteWait *sync.WaitGroup,
) (*RdsClient, *RdsInventory, error) {
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
	rdsClient := RdsClient{
		*resourceClient,
		inventoryChan,
	}
	var rdsInventory RdsInventory
	if err := rdsInventory.Load(inventoryFile); err != nil {
		return nil, nil, fmt.Errorf("failed to load RDS inventory file: %w", err)
	}

	return &rdsClient, &rdsInventory, nil
}
