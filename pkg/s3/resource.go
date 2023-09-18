package s3

import (
	"errors"
	"fmt"

	"github.com/nukleros/aws-builder/internal/util"
	"github.com/nukleros/aws-builder/pkg/iam"
)

var ErrResourceNotFound = errors.New("resource not found")

// CreateResourceStack creates all the resources for an RDS instance.
func (c *S3Client) CreateS3ResourceStack(resourceConfig *S3Config) error {
	var inventory S3Inventory
	if resourceConfig.Region != "" {
		inventory.Region = resourceConfig.Region
		c.AwsConfig.Region = resourceConfig.Region
	} else {
		inventory.Region = c.AwsConfig.Region
		resourceConfig.Region = c.AwsConfig.Region
	}

	// Tags
	s3Tags := CreateS3Tags(resourceConfig.Name, resourceConfig.Tags)
	iamTags := iam.CreateIamTags(resourceConfig.Name, resourceConfig.Tags)

	// Bucket
	bucketName, err := c.CreateBucket(
		s3Tags,
		resourceConfig.Name,
		resourceConfig.Region,
	)
	if bucketName != "" {
		inventory.BucketName = bucketName
		inventory.send(c.InventoryChan)
	}
	if err != nil {
		return err
	}
	c.SendMessage(fmt.Sprintf("S3 bucket %s created", bucketName))

	// Access Point
	accessPointName, err := c.CreateAccessPoint(
		resourceConfig.Name,
		bucketName,
		resourceConfig.AwsAccount,
		resourceConfig.VpcIdReadWriteAccess,
	)
	if accessPointName != "" {
		inventory.AccessPointName = accessPointName
		inventory.AwsAccount = resourceConfig.AwsAccount
		inventory.send(c.InventoryChan)
	}
	if err != nil {
		return err
	}
	c.SendMessage(fmt.Sprintf("S3 bucket access point %s created", accessPointName))

	// Access Control List
	if err := c.CreateAcl(
		s3Tags,
		bucketName,
		resourceConfig.PublicReadAccess,
	); err != nil {
		return err
	}

	// IAM Policy
	nameSuffix := util.RandomAlphaNumericString(12)
	policy, err := c.CreatePolicy(
		iamTags,
		bucketName,
		resourceConfig.WorkloadReadWriteAccess.ServiceAccountName,
		nameSuffix,
	)
	if policy != nil {
		inventory.PolicyArn = *policy.Arn
		inventory.send(c.InventoryChan)
	}
	if err != nil {
		return err
	}
	c.SendMessage(fmt.Sprintf("IAM policy %s created", *policy.PolicyName))

	// IAM Role
	role, err := c.CreateRole(
		iamTags,
		*policy.Arn,
		resourceConfig.AwsAccount,
		resourceConfig.WorkloadReadWriteAccess.OidcUrl,
		resourceConfig.WorkloadReadWriteAccess.ServiceAccountName,
		resourceConfig.WorkloadReadWriteAccess.ServiceAccountNamespace,
		nameSuffix,
	)
	if role != nil {
		roleInventory := RoleInventory{
			RoleName:       *role.RoleName,
			RoleArn:        *role.Arn,
			RolePolicyArns: []string{*policy.Arn},
		}
		inventory.Role = roleInventory
		inventory.send(c.InventoryChan)
	}
	if err != nil {
		return err
	}
	c.SendMessage(fmt.Sprintf("IAM role %s created", *role.RoleName))

	return nil
}

// DeleteResourceStack deletes all the resources for an RDS instance.
func (c *S3Client) DeleteS3ResourceStack(inventory *S3Inventory) error {
	c.AwsConfig.Region = inventory.Region

	// Access Point
	if err := c.DeleteAccessPoint(inventory.AccessPointName, inventory.AwsAccount); err != nil {
		return err
	}
	c.SendMessage(fmt.Sprintf("S3 bucket access point %s deleted\n", inventory.AccessPointName))
	inventory.AccessPointName = ""
	inventory.send(c.InventoryChan)

	// Bucket
	if err := c.DeleteBucket(inventory.BucketName); err != nil {
		return err
	}
	c.SendMessage(fmt.Sprintf("S3 bucket %s deleted\n", inventory.BucketName))
	inventory.BucketName = ""
	inventory.send(c.InventoryChan)

	// IAM Role
	if err := c.DeleteRole(&inventory.Role); err != nil {
		return err
	}
	c.SendMessage(fmt.Sprintf("IAM role %s deleted\n", inventory.Role.RoleName))
	inventory.Role = RoleInventory{}
	inventory.send(c.InventoryChan)

	// IAM Policy
	if err := c.DeletePolicy(inventory.PolicyArn); err != nil {
		return err
	}
	c.SendMessage(fmt.Sprintf("IAM policy with ARN %s deleted\n", inventory.PolicyArn))
	inventory.PolicyArn = ""
	inventory.send(c.InventoryChan)

	return nil
}
