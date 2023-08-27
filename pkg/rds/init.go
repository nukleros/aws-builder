package rds

import (
	"fmt"

	"github.com/nukleros/aws-builder/pkg/client"
)

// InitCreate initializes RDS resource creation by creating an inventory
// channel, starting a goroutine to write inventory to file, creating the RDS
// client and loading the RDS configuration.
func InitCreate(
	resourceClient *client.ResourceClient,
	configFile string,
	inventoryFile string,
) (*RdsClient, *RdsConfig, error) {
	// capture inventory and write to file as resources are created
	invChan := make(chan RdsInventory)
	go func() {
		for inventory := range invChan {
			if err := inventory.Write(inventoryFile); err != nil {
				fmt.Printf("failed to write inventory file: %s", err)
			}
		}
	}()

	// create client and load config
	rdsClient := RdsClient{
		*resourceClient,
		&invChan,
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
) (*RdsClient, *RdsInventory, error) {
	// capture inventory and write to file as resources are deleted
	invChan := make(chan RdsInventory)
	go func() {
		for inventory := range invChan {
			if err := inventory.Write(inventoryFile); err != nil {
				fmt.Printf("failed to write inventory file: %s", err)
			}
		}
	}()

	// create client and load inventory to delete
	rdsClient := RdsClient{
		*resourceClient,
		&invChan,
	}
	var rdsInventory RdsInventory
	if err := rdsInventory.Load(inventoryFile); err != nil {
		return nil, nil, fmt.Errorf("failed to load RDS inventory file: %w", err)
	}

	return &rdsClient, &rdsInventory, nil
}
