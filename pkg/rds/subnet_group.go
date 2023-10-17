package rds

import (
	"errors"
	"fmt"

	aws_rds "github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/rds/types"
)

// CreateSubnetGroup creates a new subnet group for an RDS instances.  The
// subnet group defines which subnets the RDS instance can be deployed to and
// implicitly sets the VPC for the instance.  If a subnet group with with
// matching name and tags already exists, that subnet group will be returned and
// used in the resource stack to ensure idempotency.
func (c *RdsClient) CreateSubnetGroup(
	tags *[]types.Tag,
	instanceName string,
	subnetIds []string,
) (*types.DBSubnetGroup, error) {
	svc := aws_rds.NewFromConfig(*c.AwsConfig)

	subnetGroupName := fmt.Sprintf("%s-subnet-group", instanceName)
	subnetGroupDescription := fmt.Sprintf("database subnet group for RDS instance %s", instanceName)
	createSubnetGroupInput := aws_rds.CreateDBSubnetGroupInput{
		DBSubnetGroupName:        &subnetGroupName,
		DBSubnetGroupDescription: &subnetGroupDescription,
		SubnetIds:                subnetIds,
		Tags:                     *tags,
	}
	subnetGroupResp, err := svc.CreateDBSubnetGroup(c.Context, &createSubnetGroupInput)
	if err != nil {
		// if a subnet group with matching name and tags already exists,
		// return that subnet group
		var alreadyExists *types.DBSubnetGroupAlreadyExistsFault
		if errors.As(err, &alreadyExists) {
			subnetGroup, uniqueTagsExist, err := c.checkSubnetGroupUniqueTags(subnetGroupName, tags)
			if err != nil {
				return nil, fmt.Errorf("failed to check for unique tags on subnet group %s that already exists: %w", subnetGroupName, err)
			}
			if uniqueTagsExist {
				return subnetGroup, nil
			}
		} else {
			return nil, fmt.Errorf("failed to create DB subnet group for RDS instance %s: %w", instanceName, err)
		}
	}

	return subnetGroupResp.DBSubnetGroup, nil
}

// DeleteSubnetGroup deletes a subnet group that was used by an RDS instance.
func (c *RdsClient) DeleteSubnetGroup(subnetGroupName string) error {
	if subnetGroupName == "" {
		return nil
	}

	svc := aws_rds.NewFromConfig(*c.AwsConfig)

	deleteSubnetGroupInput := aws_rds.DeleteDBSubnetGroupInput{
		DBSubnetGroupName: &subnetGroupName,
	}
	_, err := svc.DeleteDBSubnetGroup(c.Context, &deleteSubnetGroupInput)
	if err != nil {
		return fmt.Errorf("failed to delete subnet group %s: %w", subnetGroupName, err)
	}

	return nil
}

// checkSubnetGroupUniqueTags checks to see if a subnet group with a matching
// name and tags already exists.
func (c *RdsClient) checkSubnetGroupUniqueTags(
	subnetGroupName string,
	tags *[]types.Tag,
) (*types.DBSubnetGroup, bool, error) {
	svc := aws_rds.NewFromConfig(*c.AwsConfig)

	describeSubnetGroupsInput := aws_rds.DescribeDBSubnetGroupsInput{
		DBSubnetGroupName: &subnetGroupName,
	}
	resp, err := svc.DescribeDBSubnetGroups(c.Context, &describeSubnetGroupsInput)
	if err != nil {
		return nil, false, fmt.Errorf("failed to describe subnet groups to check for unique tags: %w", err)
	}

	for _, subnetGroup := range resp.DBSubnetGroups {
		tagsMatch, err := c.CheckUniqueTagsForResource(
			*subnetGroup.DBSubnetGroupArn,
			tags,
		)
		if err != nil {
			return nil, false, fmt.Errorf("failed to check unique tags for DB subnet group %s: %w", *subnetGroup.DBSubnetGroupName, err)
		}

		if tagsMatch {
			return &subnetGroup, true, nil
		}
	}

	return nil, false, nil
}
