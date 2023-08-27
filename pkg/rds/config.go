package rds

import (
	"fmt"
	"io/ioutil"

	"gopkg.in/yaml.v3"
)

// RdsConfig contains the configurable parameters for an RDS instance.
type RdsConfig struct {
	Tags          map[string]string `yaml:"tags"`
	Region        string            `yaml:"region"`
	Name          string            `yaml:"name"`
	DbName        string            `yaml:"dbName"`
	Class         string            `yaml:"class"`
	Engine        string            `yaml:"engine"`
	EngineVersion string            `yaml:"engineVersion"`
	StorageGb     int32             `yaml:"storageGb"`
	BackupDays    int32             `yaml:"backupDays"`
	DbUser        string            `yaml:"dbUser"`
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
