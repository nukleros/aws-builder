package config

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// LoadAWSConfig loads the AWS config from environment or shared config profile
// and overrides the default region if provided.
func LoadAWSConfig(
	configProfile,
	region,
	roleArn,
	externalId,
	serialNumber string,
) (*aws.Config, error) {
	configOptions := []func(*config.LoadOptions) error{
		config.WithSharedConfigProfile(configProfile),
		config.WithRegion(region),
		config.WithAssumeRoleCredentialOptions(
			func(o *stscreds.AssumeRoleOptions) {
				o.TokenProvider = stscreds.StdinTokenProvider
				o.ExternalID = aws.String(externalId)
			}),
	}

	// load config from filesystem
	awsConfig, err := config.LoadDefaultConfig(
		context.Background(),
		configOptions...,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// if serialNumber is provided, request MFA token and get temporary credentials
	if serialNumber != "" {
		stsClient := sts.NewFromConfig(awsConfig)

		// get MFA token from user
		tokenCode, err := stscreds.StdinTokenProvider()
		if err != nil {
			return nil, fmt.Errorf("failed to get token code: %w", err)
		}

		// generate temporary credentials
		sessionToken, err := stsClient.GetSessionToken(
			context.Background(),
			&sts.GetSessionTokenInput{
				SerialNumber: &serialNumber,
				TokenCode:    &tokenCode,
			},
		)
		if err != nil {
			return nil, fmt.Errorf("failed to get session token: %w", err)
		}

		// update configOptions with session token
		configOptions = append(
			configOptions,
			config.WithCredentialsProvider(
				credentials.NewStaticCredentialsProvider(
					*sessionToken.Credentials.AccessKeyId,
					*sessionToken.Credentials.SecretAccessKey,
					*sessionToken.Credentials.SessionToken,
				),
			))

		// update aws config with session token
		awsConfig, err = config.LoadDefaultConfig(
			context.Background(),
			configOptions...,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to load AWS config: %w", err)
		}
	}

	// assume role if roleArn is provided
	if roleArn != "" {
		awsConfig, err = assumeRole(roleArn, externalId, awsConfig, configOptions)
		if err != nil {
			return nil, fmt.Errorf("failed to assume role: %w", err)
		}
		return &awsConfig, err
	}

	return &awsConfig, err
}

// LoadAWSConfigFromAPIKeys returns an AWS config from static API keys and
// overrides the default region if provided.  The token parameter can be an
// empty string.
func LoadAWSConfigFromAPIKeys(
	accessKeyID,
	secretAccessKey,
	token,
	region,
	roleArn,
	externalId string,
) (*aws.Config, error) {
	configOptions := []func(*config.LoadOptions) error{
		config.WithRegion(region),
		config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(
				accessKeyID,
				secretAccessKey,
				token,
			),
		),
	}

	awsConfig, err := config.LoadDefaultConfig(
		context.Background(),
		configOptions...,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config from static API keys: %w", err)
	}

	// assume role if roleArn is provided
	if roleArn != "" {
		awsConfig, err = assumeRole(roleArn, externalId, awsConfig, configOptions)
		if err != nil {
			return nil, fmt.Errorf("failed to assume role: %w", err)
		}
		return &awsConfig, err
	}

	return &awsConfig, err
}

// assumeRole returns an AWS config with temporary credentials
// from an assumed role.
func assumeRole(
	roleArn,
	externalId string,
	awsConfig aws.Config,
	configOptions []func(*config.LoadOptions) error,
) (aws.Config, error) {
	// create assume role provider
	assumeRoleProvider := stscreds.NewAssumeRoleProvider(
		sts.NewFromConfig(awsConfig),
		roleArn,
		func(o *stscreds.AssumeRoleOptions) {
			o.ExternalID = aws.String(externalId)
		})

	// update configOptions with assume role provider
	configOptions = append(
		configOptions,
		config.WithCredentialsProvider(assumeRoleProvider),
	)

	// load config with assume role provider
	awsConfig, err := config.LoadDefaultConfig(
		context.Background(),
		configOptions...,
	)
	if err != nil {
		return aws.Config{}, fmt.Errorf("failed to load AWS config: %w", err)
	}

	return awsConfig, err
}
