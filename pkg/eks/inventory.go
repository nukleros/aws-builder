package eks

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
)

// EksInventory contains a record of all resources created so they can be
// referenced and cleaned up.
type EksInventory struct {
	Region                 string                      `json:"region"`
	AvailabilityZones      []AvailabilityZoneInventory `json:"availabilityZones"`
	VpcId                  string                      `json:"vpcId"`
	InternetGatewayId      string                      `json:"internetGatewayId"`
	ElasticIpIds           []string                    `json:"elasticIpIds"`
	PublicRouteTableId     string                      `json:"publicRouteTableId"`
	PrivateRouteTableIds   []string                    `json:"privateRouteTableIds"`
	ClusterRole            RoleInventory               `json:"clusterRole"`
	WorkerRole             RoleInventory               `json:"workerRole"`
	DnsManagementRole      RoleInventory               `json:"dnsManagementRole"`
	Dns01ChallengeRole     RoleInventory               `json:"dns01ChallengeRole"`
	SecretsManagerRole     RoleInventory               `json:"SecretsManagerRole"`
	ClusterAutoscalingRole RoleInventory               `json:"clusterAutoscalingRole"`
	StorageManagementRole  RoleInventory               `json:"storageManagementRole"`
	PolicyArns             []string                    `json:"policyArns"`
	Cluster                ClusterInventory            `json:"cluster"`
	ClusterAddon           bool                        `json:"clusterAddon"`
	NodeGroupNames         []string                    `json:"nodeGroupNames"`
	OidcProviderArn        string                      `json:"oidcProviderArn"`
	SecurityGroupId        string                      `json:"securityGroupId"`
}

// AvailabilityZoneInventory
type AvailabilityZoneInventory struct {
	Zone           string            `json:"zone"`
	PublicSubnets  []SubnetInventory `json:"publicSubnets"`
	PrivateSubnets []SubnetInventory `json:"privateSubnets"`
	NatGatewayId   string            `json:"natGatewayId"`
}

// SubnetInventory contains the details for each subnet created.
type SubnetInventory struct {
	SubnetId   string `json:"subnetId"`
	SubnetCidr string `json:"subnetCidr"`
}

// RoleInventory contains the details for each role created.
type RoleInventory struct {
	RoleName       string   `json:"roleName"`
	RoleArn        string   `json:"roleArn"`
	RolePolicyArns []string `json:"rolePolicyArns"`
}

// ClusterInventory contains the details for the EKS cluster.
type ClusterInventory struct {
	ClusterName     string `json:"clusterName"`
	ClusterArn      string `json:"clusterArn"`
	OidcProviderUrl string `json:"oidcProviderUrl"`
}

// send sends the EKS inventory on the inventory channel.
func (i *EksInventory) send(inventoryChan *chan EksInventory) {
	if inventoryChan != nil {
		*inventoryChan <- *i
	}
}

// Write writes EKS inventory to file.
func (i *EksInventory) Write(inventoryFile string) error {
	invJson, err := i.Marshal()
	if err != nil {
		return fmt.Errorf("failed to marshal EKS inventory to JSON: %w", err)
	}

	if err := os.WriteFile(inventoryFile, invJson, 0644); err != nil {
		return fmt.Errorf("failed to write EKS inventory to file: %w", err)
	}

	return nil
}

// Load loads the EKS inventory from a file on disk.
func (i *EksInventory) Load(inventoryFile string) error {
	// read inventory file
	inventoryBytes, err := ioutil.ReadFile(inventoryFile)
	if err != nil {
		return err
	}

	// unmarshal JSON inventory
	return i.Unmarshal(inventoryBytes)
}

// Marshal returns the JSON EKS inventory from an EksInventory object.
func (i *EksInventory) Marshal() ([]byte, error) {
	invJson, err := json.MarshalIndent(i, "", "  ")
	if err != nil {
		return []byte{}, err
	}

	return invJson, nil
}

// Unmarshal populates an EksInventory object from the JSON EKS inventory.
func (i *EksInventory) Unmarshal(inventoryBytes []byte) error {
	if err := json.Unmarshal(inventoryBytes, i); err != nil {
		return err
	}

	return nil
}
