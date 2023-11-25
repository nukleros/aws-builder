package s3

import (
	"fmt"
	"time"

	aws_s3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// CreateAcl puts an access control list on the created bucket to allow public
// read access or private read access only based on client config.
func (c *S3Client) CreateAcl(
	tags *[]types.Tag,
	bucketName string,
	publicGetAccess bool,
) error {
	svc := aws_s3.NewFromConfig(*c.AwsConfig)

	// set public or private access on bucket
	var cannedAcl types.BucketCannedACL
	if publicGetAccess {
		cannedAcl = types.BucketCannedACLPublicRead
	} else {
		cannedAcl = types.BucketCannedACLPrivate
	}

	// if public read access we need to first update the
	// PublicAccessBlockConfiguration
	if publicGetAccess {
		deletePublicAccessBlockInput := aws_s3.DeletePublicAccessBlockInput{
			Bucket: &bucketName,
		}
		_, err := svc.DeletePublicAccessBlock(c.Context, &deletePublicAccessBlockInput)
		if err != nil {
			return fmt.Errorf("failed to apply configuration to allow public ACLs to bucket %s: %w", bucketName, err)
		}

		// aws seems to need some time to let the above change propagate so
		// we have a retry loop on this operation
		putPolicySucceeded := false
		putPolicyAttempts := 0
		putPolicyMaxAttempts := 20

		for !putPolicySucceeded {
			// add a bucket policy that grants public read access to all objects
			policy := fmt.Sprintf(`{
				"Version": "2012-10-17",
				"Statement": [{
					"Sid": "PublicReadGetObject",
					"Effect": "Allow",
					"Principal": "*",
					"Action": "s3:GetObject",
					"Resource": "arn:aws:s3:::%s/*"
				}]
			}`, bucketName)

			putBucketPolicyInput := aws_s3.PutBucketPolicyInput{
				Bucket: &bucketName,
				Policy: &policy,
			}
			_, err = svc.PutBucketPolicy(c.Context, &putBucketPolicyInput)
			if err != nil {
				if putPolicyAttempts > putPolicyMaxAttempts {
					return fmt.Errorf("failed to apply bucket policy to bucket %s: %w", bucketName, err)
				}
				putPolicyAttempts += 1
				time.Sleep(time.Second * 2)
				continue
			}

			putPolicySucceeded = true
		}
	}

	putBucketAclInput := aws_s3.PutBucketAclInput{
		Bucket: &bucketName,
		ACL:    cannedAcl,
	}
	_, err := svc.PutBucketAcl(c.Context, &putBucketAclInput)
	if err != nil {
		return fmt.Errorf("failed to apply bucket ACL to bucket %s: %w", bucketName, err)
	}

	return nil
}
