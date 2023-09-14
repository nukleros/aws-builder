package s3

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
)

type S3Inventory struct {
	AwsAccount      string        `json:"awsAccount"`
	Region          string        `json:"region"`
	BucketName      string        `json:"bucketName"`
	AccessPointName string        `json:"accessPointName"`
	PolicyArn       string        `json:"policyArn"`
	Role            RoleInventory `json:"role"`
}

// RoleInventory contains the details for a created role.
type RoleInventory struct {
	RoleName       string   `json:"roleName"`
	RoleArn        string   `json:"roleArn"`
	RolePolicyArns []string `json:"rolePolicyArns"`
}

// send sends the S3 inventory on the inventory channel.
func (i *S3Inventory) send(inventoryChan *chan S3Inventory) {
	if inventoryChan != nil {
		*inventoryChan <- *i
	}
}

// Write writes S3 inventory to file.
func (i *S3Inventory) Write(inventoryFile string) error {
	invJson, err := i.Marshal()
	if err != nil {
		return fmt.Errorf("failed to marshal S3 inventory to JSON: %w", err)
	}

	if err := os.WriteFile(inventoryFile, invJson, 0644); err != nil {
		return fmt.Errorf("failed to write S3 inventory to file: %w", err)
	}

	return nil
}

// Load loads the S3 inventory from a file on disk.
func (i *S3Inventory) Load(inventoryFile string) error {
	// read inventory file
	inventoryBytes, err := ioutil.ReadFile(inventoryFile)
	if err != nil {
		return err
	}

	// unmarshal JSON inventory
	return i.Unmarshal(inventoryBytes)
}

// Marshal returns the JSON S3 inventory from an S3Inventory object.
func (i *S3Inventory) Marshal() ([]byte, error) {
	invJson, err := json.MarshalIndent(i, "", "  ")
	if err != nil {
		return []byte{}, err
	}

	return invJson, nil
}

// Unmarshal populates an S3Inventory object from the JSON S3 inventory.
func (i *S3Inventory) Unmarshal(inventoryBytes []byte) error {
	if err := json.Unmarshal(inventoryBytes, i); err != nil {
		return err
	}

	return nil
}
