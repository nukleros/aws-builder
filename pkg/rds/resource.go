package rds

import (
	"errors"
	"fmt"

	"github.com/nukleros/aws-builder/pkg/ec2"
)

var ErrResourceNotFound = errors.New("resource not found")

// CreateResourceStack creates all the resources for an RDS instance.  If
// inventory for pre-existing resources are provided, it will not re-create
// those resources but instead use them as a part of the stack.
func (c *RdsClient) CreateRdsResourceStack(
	resourceConfig *RdsConfig,
	inventory *RdsInventory,
) error {
	// return an error if resource config and inventory regions do not match
	if inventory != nil && inventory.Region != "" && inventory.Region != resourceConfig.Region {
		return errors.New("different regions provided in config and inventory")
		return fmt.Errorf(
			"config region %s and inventory region %s do not match",
			resourceConfig.Region,
			inventory.Region,
		)
	}

	// resource config region takes precedence
	// if not set, use the region defined in AWS config
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
	if inventory != nil && inventory.SecurityGroupId == "" {
		sgId, err := c.CreateSecurityGroup(
			ec2Tags,
			resourceConfig.Name,
			resourceConfig.VpcId,
			resourceConfig.DbPort,
			resourceConfig.SourceSecurityGroupId,
			resourceConfig.AwsAccount,
		)
		if sgId != "" {
			inventory.SecurityGroupId = sgId
			inventory.send(c.InventoryChan)
		}
		if err != nil {
			return err
		}
		c.SendMessage(fmt.Sprintf("security group with ID %s created\n", sgId))
	} else {
		c.SendMessage(fmt.Sprintf("security group with ID %s found in inventory\n", inventory.SecurityGroupId))
	}

	// Subnet Group
	if inventory != nil && inventory.SubnetGroupName == "" {
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
	} else {
		c.SendMessage(fmt.Sprintf("subnet group %s found in inventory\n", inventory.SubnetGroupName))
	}

	// RDS Instance
	if inventory != nil && inventory.RdsInstanceId == "" {
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
			inventory.SecurityGroupId,
			inventory.SubnetGroupName,
		)
		if rdsInstance != nil {
			inventory.RdsInstanceId = *rdsInstance.DBInstanceIdentifier
			inventory.send(c.InventoryChan)
		}
		if err != nil {
			return err
		}
		c.SendMessage(fmt.Sprintf("RDS instance %s created\n", *rdsInstance.DBInstanceIdentifier))
	} else {
		c.SendMessage(fmt.Sprintf("RDS instance %s found in inventory\n", inventory.RdsInstanceId))
	}

	// RDS Instance Endpoint
	if inventory != nil && inventory.RdsInstanceEndpoint == "" {
		c.SendMessage(fmt.Sprintf("waiting for RDS instance %s to become available\n", inventory.RdsInstanceId))
		endpoint, err := c.WaitForRdsInstance(inventory.RdsInstanceId, RdsConditionCreated)
		fmt.Println(c.InventoryChan)
		if endpoint != "" && c.InventoryChan != nil {
			inventory.RdsInstanceEndpoint = endpoint
			inventory.send(c.InventoryChan)
		}
		if err != nil {
			return err
		}
		c.SendMessage(fmt.Sprintf("RDS instance %s is available\n", inventory.RdsInstanceId))
	}

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
	_, err := c.WaitForRdsInstance(inventory.RdsInstanceId, RdsConditionDeleted)
	if err != nil {
		return err
	}
	c.SendMessage(fmt.Sprintf("RDS instance %s has been removed\n", inventory.RdsInstanceId))
	inventory.RdsInstanceId = ""
	inventory.RdsInstanceEndpoint = ""
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
