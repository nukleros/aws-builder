package eks

import (
	"fmt"
	"io/ioutil"

	"gopkg.in/yaml.v2"
)

const DefaultKubernetesVersion = "1.31"

// EksConfig contains the configuration options for an EKS cluster.
type EksConfig struct {
	Name                             string                   `yaml:"name"`
	Region                           string                   `yaml:"region"`
	AwsAccountId                     string                   `yaml:"awsAccountId"`
	KubernetesVersion                string                   `yaml:"kubernetesVersion"`
	ClusterCidr                      string                   `yaml:"clusterCidr"`
	DesiredAzCount                   int32                    `yaml:"desiredAzCount"`
	AvailabilityZones                []AvailabilityZoneConfig `yaml:"availabilityZones"`
	InstanceTypes                    []string                 `yaml:"instanceTypes"`
	InitialNodes                     int32                    `yaml:"initialNodes"`
	MinNodes                         int32                    `yaml:"minNodes"`
	MaxNodes                         int32                    `yaml:"maxNodes"`
	DnsManagement                    bool                     `yaml:"dnsManagement"`
	Dns01Challenge                   bool                     `yaml:"dns01Challenge"`
	SecretsManager                   bool                     `yaml:"secretsManager"`
	DnsManagementServiceAccount      ServiceAccountConfig     `yaml:"dnsManagementServiceAccount"`
	Dns01ChallengeServiceAccount     ServiceAccountConfig     `yaml:"dns01ChallengeServiceAccount"`
	SecretsManagerServiceAccount     ServiceAccountConfig     `yaml:"secretsManagerServiceAccount"`
	StorageManagementServiceAccount  ServiceAccountConfig     `yaml:"storageManagementServiceAccount"`
	ClusterAutoscaling               bool                     `yaml:"clusterAutoscaling"`
	ClusterAutoscalingServiceAccount ServiceAccountConfig     `yaml:"clusterAutoscalingServiceAccount"`
	KeyPair                          string                   `yaml:"keyPair"`
	Tags                             map[string]string        `yaml:"tags"`
}

// AvailabilityZone contains configuration options for an EKS cluster
// networking.  It also contains resource ID fields used internally during
// creation.
type AvailabilityZoneConfig struct {
	Zone              string `yaml:"zone"`
	PrivateSubnetCidr string `yaml:"privateSubnetCidr"`
	PublicSubnetCidr  string `yaml:"publicSubnetCidr"`
}

// ServiceAccountConfig contains the name and namespace for a Kubernetes service
// account.  Used to set up IAM roles for service accounts (IRSA).
type ServiceAccountConfig struct {
	Name      string `yaml:"name"`
	Namespace string `yaml:"namespace"`
}

// LoadEksConfig loads an EKS config from a config file and returns the
// EksConfig object.
func LoadEksConfig(configFile string) (*EksConfig, error) {
	configYaml, err := ioutil.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}
	var eksConfig EksConfig
	if err := yaml.Unmarshal(configYaml, &eksConfig); err != nil {
		return nil, fmt.Errorf("failed to unmarshal yaml from config file: %w", err)
	}

	return &eksConfig, nil
}

// NewEksConfig returns an EksConfig with default values set.
func NewEksConfig() *EksConfig {
	return &EksConfig{
		Name:              "default-eks-cluster",
		KubernetesVersion: DefaultKubernetesVersion,
		ClusterCidr:       "10.0.0.0/16",
		InstanceTypes:     []string{"t3.micro"},
		MinNodes:          int32(2),
		MaxNodes:          int32(4),
	}
}
