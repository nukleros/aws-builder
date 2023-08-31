package rds

import (
	"errors"
	"fmt"

	"github.com/nukleros/aws-builder/pkg/ec2"
)

var ErrResourceNotFound = errors.New("resource not found")

// CreateResourceStack creates all the resources for an RDS instance.
func (c *RdsClient) CreateRdsResourceStack(resourceConfig *RdsConfig) error {
	var inventory RdsInventory
	if resourceConfig.Region != "" {
		inventory.Region = resourceConfig.Region
		c.AwsConfig.Region = resourceConfig.Region
	} else {
		inventory.Region = c.AwsConfig.Region
		resourceConfig.Region = c.AwsConfig.Region
	}

	// Tags
	rdsTags := CreateRdsTags(resourceConfig.Name, resourceConfig.Tags)
	ec2Tags := ec2.CreateEc2Tags(resourceConfig.Name, resourceConfig.Tags)

	// Security Group
	securityGroupId, err := c.CreateSecurityGroup(
		ec2Tags,
		resourceConfig.Name,
		resourceConfig.VpcId,
		resourceConfig.DbPort,
		resourceConfig.SourceSecurityGroupId,
		resourceConfig.AwsAccount,
	)
	if securityGroupId != "" {
		inventory.SecurityGroupId = securityGroupId
		inventory.send(c.InventoryChan)
	}
	if err != nil {
		return err
	}
	c.SendMessage(fmt.Sprintf("security group with ID %s created\n", securityGroupId))

	// Subnet Group
	subnetGroup, err := c.CreateSubnetGroup(
		rdsTags,
		resourceConfig.Name,
		resourceConfig.SubnetIds,
	)
	if subnetGroup != nil {
		inventory.SubnetGroupName = *subnetGroup.DBSubnetGroupName
		inventory.send(c.InventoryChan)
	}
	if err != nil {
		return err
	}
	c.SendMessage(fmt.Sprintf("subnet group %s created\n", *subnetGroup.DBSubnetGroupName))

	// RDS Instance
	rdsInstance, err := c.CreateRdsInstance(
		rdsTags,
		resourceConfig.Name,
		resourceConfig.DbName,
		resourceConfig.Class,
		resourceConfig.Engine,
		resourceConfig.EngineVersion,
		resourceConfig.StorageGb,
		resourceConfig.BackupDays,
		resourceConfig.DbUser,
		resourceConfig.DbUserPassword,
		securityGroupId,
		*subnetGroup.DBSubnetGroupName,
	)
	if rdsInstance != nil {
		inventory.RdsInstanceId = *rdsInstance.DBInstanceIdentifier
		inventory.send(c.InventoryChan)
	}
	if err != nil {
		return err
	}
	c.SendMessage(fmt.Sprintf("RDS instance %s created\n", *rdsInstance.DBInstanceIdentifier))
	c.SendMessage(fmt.Sprintf("waiting for RDS instance %s to become available\n", *rdsInstance.DBInstanceIdentifier))
	if err := c.WaitForRdsInstance(*rdsInstance.DBInstanceIdentifier, RdsConditionCreated); err != nil {
		return err
	}
	c.SendMessage(fmt.Sprintf("RDS instance %s is available\n", *rdsInstance.DBInstanceIdentifier))

	return nil
}

// DeleteResourceStack deletes all the resources for an RDS instance.
func (c *RdsClient) DeleteRdsResourceStack(inventory *RdsInventory) error {
	c.AwsConfig.Region = inventory.Region

	// RDS Instance
	if err := c.DeleteRdsInstance(inventory.RdsInstanceId); err != nil {
		return err
	}
	c.SendMessage(fmt.Sprintf("RDS instance %s deleted\n", inventory.RdsInstanceId))
	c.SendMessage(fmt.Sprintf("waiting for RDS instance %s to be removed\n", inventory.RdsInstanceId))
	if err := c.WaitForRdsInstance(inventory.RdsInstanceId, RdsConditionDeleted); err != nil {
		return err
	}
	c.SendMessage(fmt.Sprintf("RDS instance %s has been removed\n", inventory.RdsInstanceId))
	inventory.RdsInstanceId = ""
	inventory.send(c.InventoryChan)

	// Subnet Group
	if err := c.DeleteSubnetGroup(inventory.SubnetGroupName); err != nil {
		return err
	}
	c.SendMessage(fmt.Sprintf("subnet group %s deleted\n", inventory.SubnetGroupName))
	inventory.SubnetGroupName = ""
	inventory.send(c.InventoryChan)

	// Security Group
	if err := c.DeleteSecurityGroup(inventory.SecurityGroupId); err != nil {
		return err
	}
	c.SendMessage(fmt.Sprintf("security group %s deleted\n", inventory.SecurityGroupId))
	inventory.SecurityGroupId = ""
	inventory.send(c.InventoryChan)

	return nil
}
