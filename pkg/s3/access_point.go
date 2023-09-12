package s3

import (
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/s3control"
	"github.com/aws/aws-sdk-go-v2/service/s3control/types"
)

// CreateAccessPoint creates an access point for an S3 bucket.
func (c *S3Client) CreateAccessPoint(
	name string,
	bucketName string,
	awsAccount string,
	putAccessVpcId string,
) (string, error) {
	svc := s3control.NewFromConfig(*c.AwsConfig)

	createAccessPointInput := s3control.CreateAccessPointInput{
		AccountId: &awsAccount,
		Name:      &name,
		Bucket:    &bucketName,
		VpcConfiguration: &types.VpcConfiguration{
			VpcId: &putAccessVpcId,
		},
	}
	_, err := svc.CreateAccessPoint(c.Context, &createAccessPointInput)
	if err != nil {
		return "", fmt.Errorf("failed to create S3 access point %s: %w", name, err)
	}

	return name, nil
}

// DeleteAccessPoint deletes an access point for an S3 bucket.
func (c *S3Client) DeleteAccessPoint(
	accessPointName string,
	awsAccount string,
) error {
	if accessPointName == "" {
		return nil
	}

	svc := s3control.NewFromConfig(*c.AwsConfig)

	deleteAccessPointInput := s3control.DeleteAccessPointInput{
		AccountId: &awsAccount,
		Name:      &accessPointName,
	}
	_, err := svc.DeleteAccessPoint(c.Context, &deleteAccessPointInput)
	if err != nil {
		return fmt.Errorf("failed to delete S3 access point: %s: %w", accessPointName, err)
	}

	return nil
}
