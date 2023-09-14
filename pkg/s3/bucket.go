package s3

import (
	"errors"
	"fmt"

	"github.com/google/uuid"

	aws_s3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// CreateBucket creates a new S3 bucket.
func (c *S3Client) CreateBucket(
	tags *[]types.Tag,
	bucketName string,
	region string,
) (string, error) {
	svc := aws_s3.NewFromConfig(*c.AwsConfig)

	// determine if region is among the supported bucket location constraints
	createBucketConfig := types.CreateBucketConfiguration{}
	var bucketLocation types.BucketLocationConstraint
	bucketRegions := bucketLocation.Values()
	bucketLocationFound := false
	for _, r := range bucketRegions {
		if string(r) == region {
			createBucketConfig = types.CreateBucketConfiguration{
				LocationConstraint: r,
			}
			bucketLocationFound = true
			break
		}
	}
	if !bucketLocationFound && region != "us-east-1" {
		return "", errors.New(fmt.Sprintf(
			"supplied region %s is not a supported region for S3 buckets",
			region,
		))
	}

	uniqueBucketName := fmt.Sprintf("%s-%s", bucketName, uuid.New())
	createBucketInput := aws_s3.CreateBucketInput{
		Bucket:          &uniqueBucketName,
		ObjectOwnership: types.ObjectOwnershipObjectWriter,
	}

	// if using us-east-1 region, this field cannot be applied - it is the
	// default region (but the default region has no BucketLocationConstraint)
	if bucketLocationFound {
		createBucketInput.CreateBucketConfiguration = &createBucketConfig
	}

	_, err := svc.CreateBucket(c.Context, &createBucketInput)
	if err != nil {
		return "", fmt.Errorf("failed to create S3 bucket %s: %w", uniqueBucketName, err)
	}

	// enable object versioning
	putBucketVersioningInput := aws_s3.PutBucketVersioningInput{
		Bucket: &uniqueBucketName,
		VersioningConfiguration: &types.VersioningConfiguration{
			Status: types.BucketVersioningStatusEnabled,
		},
	}
	_, err = svc.PutBucketVersioning(c.Context, &putBucketVersioningInput)
	if err != nil {
		return "", fmt.Errorf("failed to enable object versioning for bucket %s: %w", uniqueBucketName, err)
	}

	// add tags to bucket
	tagging := types.Tagging{TagSet: *tags}
	putBucketTaggingInput := aws_s3.PutBucketTaggingInput{
		Bucket:  &uniqueBucketName,
		Tagging: &tagging,
	}
	_, err = svc.PutBucketTagging(c.Context, &putBucketTaggingInput)
	if err != nil {
		return "", fmt.Errorf("failed to add tags to bucket: %s: %w", uniqueBucketName, err)
	}

	return uniqueBucketName, nil
}

// DeleteBucket deletes an S3 bucket.
func (c *S3Client) DeleteBucket(bucketName string) error {
	if bucketName == "" {
		return nil
	}

	svc := aws_s3.NewFromConfig(*c.AwsConfig)

	deleteBucketInput := aws_s3.DeleteBucketInput{
		Bucket: &bucketName,
	}
	_, err := svc.DeleteBucket(c.Context, &deleteBucketInput)
	if err != nil {
		return fmt.Errorf("failed to delete S3 bucket %s: %w", bucketName, err)
	}

	return nil
}
