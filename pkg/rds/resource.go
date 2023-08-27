package rds

import "fmt"

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
	tags := CreateRdsTags(resourceConfig.Name, resourceConfig.Tags)

	// RDS Instance
	rdsInstance, err := c.CreateRdsInstance(
		tags,
		resourceConfig.Name,
		resourceConfig.DbName,
		resourceConfig.Class,
		resourceConfig.Engine,
		resourceConfig.EngineVersion,
		resourceConfig.StorageGb,
		resourceConfig.BackupDays,
		resourceConfig.DbUser,
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
	if err := c.WaitForRdsInstance(*rdsInstance.DBInstanceIdentifier); err != nil {
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
	inventory.RdsInstanceId = ""
	inventory.send(c.InventoryChan)

	return nil
}
