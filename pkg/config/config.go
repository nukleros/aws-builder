package config

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
)

// LoadAWSConfig loads the AWS config from environment or shared config profile
// and overrides the default region if provided.
func LoadAwsConfig(configProfile, region string) (*aws.Config, error) {
	awsConfig, err := config.LoadDefaultConfig(
		context.Background(),
		config.WithRegion(region),
		config.WithSharedConfigProfile(configProfile),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	return &awsConfig, err
}
