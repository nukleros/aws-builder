package rds

import (
	"fmt"

	awsrds "github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/rds/types"
)

// CreateSubnetGroup creates a new subnet group for an RDS instances.  The
// subnet group defines which subnets the RDS instance can be deployed to and
// implicitly sets the VPC for the instance.
func (c *RdsClient) CreateSubnetGroup(
	tags *[]types.Tag,
	instanceName string,
	subnetIds []string,
) (*types.DBSubnetGroup, error) {
	svc := awsrds.NewFromConfig(*c.AwsConfig)

	subnetGroupName := fmt.Sprintf("%s-subnet-group", instanceName)
	subnetGroupDescription := fmt.Sprintf("database subnet group for RDS instance %s", instanceName)
	createSubnetGroupInput := awsrds.CreateDBSubnetGroupInput{
		DBSubnetGroupName:        &subnetGroupName,
		DBSubnetGroupDescription: &subnetGroupDescription,
		SubnetIds:                subnetIds,
		Tags:                     *tags,
	}
	subnetGroupResp, err := svc.CreateDBSubnetGroup(c.Context, &createSubnetGroupInput)
	if err != nil {
		return nil, fmt.Errorf("failed to create DB subnet group for RDS instance %s: %w", instanceName, err)
	}

	return subnetGroupResp.DBSubnetGroup, nil
}

// DeleteSubnetGroup deletes a subnet group that was used by an RDS instance.
func (c *RdsClient) DeleteSubnetGroup(subnetGroupName string) error {
	if subnetGroupName == "" {
		return nil
	}

	svc := awsrds.NewFromConfig(*c.AwsConfig)

	deleteSubnetGroupInput := awsrds.DeleteDBSubnetGroupInput{
		DBSubnetGroupName: &subnetGroupName,
	}
	_, err := svc.DeleteDBSubnetGroup(c.Context, &deleteSubnetGroupInput)
	if err != nil {
		return fmt.Errorf("failed to delete subnet group %s: %w", subnetGroupName, err)
	}

	return nil
}
