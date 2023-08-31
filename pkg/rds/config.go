package rds

import (
	"fmt"
	"io/ioutil"

	"gopkg.in/yaml.v3"
)

// RdsConfig contains the configurable parameters for an RDS instance.
type RdsConfig struct {
	Tags                  map[string]string `yaml:"tags"`
	AwsAccount            string            `yaml:"awsAccount"`
	Region                string            `yaml:"region"`
	VpcId                 string            `yaml:"vpcId"`
	SubnetIds             []string          `yaml:"subnetIds"`
	Name                  string            `yaml:"name"`
	DbName                string            `yaml:"dbName"`
	Class                 string            `yaml:"class"`
	Engine                string            `yaml:"engine"`
	EngineVersion         string            `yaml:"engineVersion"`
	DbPort                int32             `yaml:"dbPort"`
	StorageGb             int32             `yaml:"storageGb"`
	BackupDays            int32             `yaml:"backupDays"`
	DbUser                string            `yaml:"dbUser"`
	DbUserPassword        string            `yaml:"dbUserPassword"`
	SourceSecurityGroupId string            `yaml:"sourceSecurityGroupId"`
}

// LoadRdsConfig loads an RDS config from a config file and returns the
// RdsConfig object.
func LoadRdsConfig(configFile string) (*RdsConfig, error) {
	configYaml, err := ioutil.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}
	var rdsConfig RdsConfig
	if err := yaml.Unmarshal(configYaml, &rdsConfig); err != nil {
		return nil, fmt.Errorf("failed to unmarshal yaml from config file: %w", err)
	}

	return &rdsConfig, nil
}
