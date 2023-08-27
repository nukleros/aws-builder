package rds

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
)

// RdsInventory contains RDS inventory resources used for an RDS instance.
type RdsInventory struct {
	Region        string `json:"region"`
	RdsInstanceId string `json:"rdsInstanceId"`
}

// send sends the RDS inventory on the inventory channel.
func (i *RdsInventory) send(inventoryChan *chan RdsInventory) {
	if inventoryChan != nil {
		*inventoryChan <- *i
	}
}

// Write writes RDS inventory to file.
func (i *RdsInventory) Write(inventoryFile string) error {
	invJson, err := i.Marshal()
	if err != nil {
		return fmt.Errorf("failed to marshal RDS inventory to JSON: %w", err)
	}

	if err := os.WriteFile(inventoryFile, invJson, 0644); err != nil {
		return fmt.Errorf("failed to write RDS inventory to file: %w", err)
	}

	return nil
}

// Load loads the RDS inventory from a file on disk.
func (i *RdsInventory) Load(inventoryFile string) error {
	// read inventory file
	inventoryBytes, err := ioutil.ReadFile(inventoryFile)
	if err != nil {
		return err
	}

	// unmarshal JSON inventory
	return i.Unmarshal(inventoryBytes)
}

// Marshal returns the JSON RDS inventory from an RdsInventory object.
func (i *RdsInventory) Marshal() ([]byte, error) {
	invJson, err := json.MarshalIndent(i, "", "  ")
	if err != nil {
		return []byte{}, err
	}

	return invJson, nil
}

// Unmarshal populates an RdsInventory object from the JSON RDS inventory.
func (i *RdsInventory) Unmarshal(inventoryBytes []byte) error {
	if err := json.Unmarshal(inventoryBytes, i); err != nil {
		return err
	}

	return nil
}
