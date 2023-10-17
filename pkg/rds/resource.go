package rds

import (
	"fmt"

	"github.com/nukleros/aws-builder/pkg/ec2"
)

// CreateResourceStack creates all the resources for an RDS instance.  If
// inventory for pre-existing resources are provided, it will not re-create
// those resources but instead use them as a part of the stack.
func (c *RdsClient) CreateRdsResourceStack(
	resourceConfig *RdsConfig,
	inventory *RdsInventory,
) error {
	// create an empty inventory object to refer to if nil
	if inventory == nil {
		inventory = &RdsInventory{}
	}

	// return an error if resource config and inventory regions do not match
	// inventory.Region and resourceConfig.Region can both be empty strings in
	// which case the user's default region will be used according their local
	// AWS config
	if inventory.Region != "" && inventory.Region != resourceConfig.Region {
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
	if inventory.SecurityGroupId == "" {
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
		c.SendMessage(fmt.Sprintf("security group with ID %s created", sgId))
	} else {
		c.SendMessage(fmt.Sprintf("security group with ID %s found in inventory", inventory.SecurityGroupId))
	}

	// Subnet Group
	if inventory.SubnetGroupName == "" {
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
		c.SendMessage(fmt.Sprintf("subnet group %s created", *subnetGroup.DBSubnetGroupName))
	} else {
		c.SendMessage(fmt.Sprintf("subnet group %s found in inventory", inventory.SubnetGroupName))
	}

	// RDS Instance
	if inventory.RdsInstanceId == "" {
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
		c.SendMessage(fmt.Sprintf("RDS instance %s created", *rdsInstance.DBInstanceIdentifier))
	} else {
		c.SendMessage(fmt.Sprintf("RDS instance %s found in inventory", inventory.RdsInstanceId))
	}

	// RDS Instance Endpoint
	if inventory.RdsInstanceEndpoint == "" {
		c.SendMessage(fmt.Sprintf("waiting for RDS instance %s to become available", inventory.RdsInstanceId))
		endpoint, err := c.WaitForRdsInstance(inventory.RdsInstanceId, RdsConditionCreated)
		if endpoint != "" && c.InventoryChan != nil {
			inventory.RdsInstanceEndpoint = endpoint
			inventory.send(c.InventoryChan)
		}
		if err != nil {
			return err
		}
		c.SendMessage(fmt.Sprintf("RDS instance %s is available", inventory.RdsInstanceId))
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
	c.SendMessage(fmt.Sprintf("RDS instance %s deleted", inventory.RdsInstanceId))
	c.SendMessage(fmt.Sprintf("waiting for RDS instance %s to be removed", inventory.RdsInstanceId))
	_, err := c.WaitForRdsInstance(inventory.RdsInstanceId, RdsConditionDeleted)
	if err != nil {
		return err
	}
	c.SendMessage(fmt.Sprintf("RDS instance %s has been removed", inventory.RdsInstanceId))
	inventory.RdsInstanceId = ""
	inventory.RdsInstanceEndpoint = ""
	inventory.send(c.InventoryChan)

	// Subnet Group
	if err := c.DeleteSubnetGroup(inventory.SubnetGroupName); err != nil {
		return err
	}
	c.SendMessage(fmt.Sprintf("subnet group %s deleted", inventory.SubnetGroupName))
	inventory.SubnetGroupName = ""
	inventory.send(c.InventoryChan)

	// Security Group
	if err := c.DeleteSecurityGroup(inventory.SecurityGroupId); err != nil {
		return err
	}
	c.SendMessage(fmt.Sprintf("security group %s deleted", inventory.SecurityGroupId))
	inventory.SecurityGroupId = ""
	inventory.send(c.InventoryChan)

	return nil
}
