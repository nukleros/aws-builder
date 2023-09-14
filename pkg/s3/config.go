package s3

import (
	"fmt"
	"io/ioutil"

	"gopkg.in/yaml.v2"
)

// S3Config contains the configurable parameters for an S3 bucket.
type S3Config struct {
	Tags                    map[string]string `yaml:"tags"`
	AwsAccount              string            `yaml:"awsAccount"`
	Region                  string            `yaml:"region"`
	Name                    string            `yaml:"name"`
	VpcIdReadWriteAccess    string            `yaml:"vpcIdReadWriteAccess"`
	PublicReadAccess        bool              `yaml:"publicReadAccess"`
	WorkloadReadWriteAccess WorkloadAccess    `yaml:"workloadReadWriteAccess"`
}

type WorkloadAccess struct {
	ServiceAccountName      string `yaml:"serviceAccountName"`
	ServiceAccountNamespace string `yaml:"serviceAccountNamespace"`
	OidcUrl                 string `yaml:"oidcUrl"`
}

// LoadS3Config loads an S3 config from a config file and returns the
// S3Config object.
func LoadS3Config(configFile string) (*S3Config, error) {
	configYaml, err := ioutil.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}
	var s3Config S3Config
	if err := yaml.Unmarshal(configYaml, &s3Config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal yaml from config file: %w", err)
	}

	return &s3Config, nil
}
