package s3

import (
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/iam/types"
)

// CreatePolicy creates the IAM policy to be used for reading, creating,
// updating and deleting objects in the bucket.
func (c *S3Client) CreatePolicy(
	tags *[]types.Tag,
	bucketName string,
	serviceAccountName string,
	nameSuffix string,
) (*types.Policy, error) {
	svc := iam.NewFromConfig(*c.AwsConfig)

	policyName := fmt.Sprintf("%s-%s", serviceAccountName, nameSuffix)
	policyDescription := "Allow read, create, update and delete of objects in specified bucket"
	policyDocument := fmt.Sprintf(`{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": [
                "s3:GetObject",
                "s3:ListBucket"
            ],
            "Resource": [
                "arn:aws:s3:::%[1]s/*",
                "arn:aws:s3:::%[1]s"
            ]
        },
        {
            "Effect": "Allow",
            "Action": [
                "s3:PutObject",
                "s3:PutObjectAcl",
                "s3:DeleteObject"
            ],
            "Resource": "arn:aws:s3:::%[1]s/*"
        }
    ]
}`, bucketName)
	createPolicyInput := iam.CreatePolicyInput{
		PolicyName:     &policyName,
		Description:    &policyDescription,
		PolicyDocument: &policyDocument,
		Tags:           *tags,
	}
	resp, err := svc.CreatePolicy(c.Context, &createPolicyInput)
	if err != nil {
		return nil, fmt.Errorf("failed to create S3 bucket read write policy %s: %w", policyName, err)
	}

	return resp.Policy, nil
}

// DeletePolicy deletes an IAM policy by policy ARN.
func (c *S3Client) DeletePolicy(policyArn string) error {
	if policyArn == "" {
		return nil
	}

	svc := iam.NewFromConfig(*c.AwsConfig)

	deletePolicyInput := iam.DeletePolicyInput{
		PolicyArn: &policyArn,
	}
	_, err := svc.DeletePolicy(c.Context, &deletePolicyInput)
	if err != nil {
		var noSuchEntityErr *types.NoSuchEntityException
		if errors.As(err, &noSuchEntityErr) {
			return nil
		} else {
			return fmt.Errorf("failed to delete policy %s: %w", policyArn, err)
		}
	}

	return nil
}
