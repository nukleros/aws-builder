package config

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
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

// LoadAWSConfigFromAPIKeys returns an AWS config from static API keys and
// overrides the default region if provided.  The token parameter can be an
// empty string.
func LoadAWSConfigFromAPIKeys(accessKeyID, secretAccessKey, token, region string) (*aws.Config, error) {
	awsConfig, err := config.LoadDefaultConfig(
		context.Background(),
		config.WithRegion(region),
		config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(
				accessKeyID,
				secretAccessKey,
				token,
			),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config from static API keys: %w", err)
	}

	return &awsConfig, err
}
