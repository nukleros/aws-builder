package rds

import (
	"errors"
	"fmt"
	"time"

	awsrds "github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/rds/types"
)

type RdsCondition string

const (
	RdsConditionCreated     RdsCondition = "RdsCreated"
	RdsConditionDeleted     RdsCondition = "RdsDelete"
	RdsCheckIntervalSeconds              = 15 // check cluster status every 15 seconds
	RdsCheckMaxCount                     = 60 // check 60 time before giving up (15 minutes)
)

// CreateRdsInstance creates a new RDS instance.
func (c *RdsClient) CreateRdsInstance(
	tags *[]types.Tag,
	instanceName string,
	dbName string,
	class string,
	engine string,
	engineVersion string,
	storageGb int32,
	backupDays int32,
	dbUser string,
	dbPassword string,
	securityGroupId string,
	subnetGroupName string,
) (*types.DBInstance, error) {
	svc := awsrds.NewFromConfig(*c.AwsConfig)

	copyTagsToSnapshot := true
	//managePassword := true
	monitoringInterval := int32(0)
	multiAz := false
	public := false
	createRdsInput := awsrds.CreateDBInstanceInput{
		DBInstanceIdentifier:  &instanceName,
		DBName:                &dbName,
		DBInstanceClass:       &class,
		Engine:                &engine,
		EngineVersion:         &engineVersion,
		AllocatedStorage:      &storageGb,
		BackupRetentionPeriod: &backupDays,
		CopyTagsToSnapshot:    &copyTagsToSnapshot,
		//ManageMasterUserPassword: &managePassword,
		MasterUsername:      &dbUser,
		MasterUserPassword:  &dbPassword,
		MonitoringInterval:  &monitoringInterval,
		MultiAZ:             &multiAz,
		PubliclyAccessible:  &public,
		VpcSecurityGroupIds: []string{securityGroupId},
		DBSubnetGroupName:   &subnetGroupName,
		Tags:                *tags,
		//CustomIamInstanceProfile: &iamProfileName,
	}
	rdsResp, err := svc.CreateDBInstance(c.Context, &createRdsInput)
	if err != nil {
		return nil, fmt.Errorf("failed to create RDS instance %s: %w", instanceName, err)
	}

	return rdsResp.DBInstance, nil
}

// DeleteRdsInstance removes an existing RDS instance.
func (c *RdsClient) DeleteRdsInstance(rdsInstanceId string) error {
	if rdsInstanceId == "" {
		return nil
	}

	svc := awsrds.NewFromConfig(*c.AwsConfig)

	deleteRdsInput := awsrds.DeleteDBInstanceInput{
		DBInstanceIdentifier: &rdsInstanceId,
		SkipFinalSnapshot:    true,
	}
	_, err := svc.DeleteDBInstance(c.Context, &deleteRdsInput)
	if err != nil {
		return fmt.Errorf("failed to delete RDS instance %s: %w", rdsInstanceId, err)
	}

	return nil
}

// WaitForRdsInstance waits for an RDS instance to become available and times
// out after 10 min if it fails to become ready in that time.
func (c *RdsClient) WaitForRdsInstance(rdsInstanceId string, rdsCondition RdsCondition) error {
	// if no instance ID, nothing to check
	if rdsInstanceId == "" {
		return nil
	}

	rdsCheckCount := 0
	for {
		rdsCheckCount += 1
		if rdsCheckCount > RdsCheckMaxCount {
			return errors.New("RDS instance check timed out waiting for it be ready")
		}

		rdsInstanceStatus, err := c.getRdsInstanceStatus(rdsInstanceId)
		if err != nil {
			if errors.Is(err, ErrResourceNotFound) && rdsCondition == RdsConditionDeleted {
				// RDS instance was not found and we're waiting for deletion so
				// condition is met
				break
			} else {
				return fmt.Errorf("failed to get RDS instance status with identifier %s: %w", rdsInstanceId, err)
			}
		}

		if rdsInstanceStatus == "available" && rdsCondition == RdsConditionCreated {
			// RDS instance is available and we're waiting for creation so
			// condition is met
			break
		}

		time.Sleep(time.Second * time.Duration(RdsCheckIntervalSeconds))
	}

	return nil
}

// getRdsInstanceStatus retrieves the status of an RDS instance.
func (c *RdsClient) getRdsInstanceStatus(rdsInstanceId string) (string, error) {
	svc := awsrds.NewFromConfig(*c.AwsConfig)

	describeRdsInput := awsrds.DescribeDBInstancesInput{
		DBInstanceIdentifier: &rdsInstanceId,
	}
	resp, err := svc.DescribeDBInstances(c.Context, &describeRdsInput)
	if err != nil {
		var notFoundErr *types.DBInstanceNotFoundFault
		if errors.As(err, &notFoundErr) {
			return "", ErrResourceNotFound
		} else {
			return "", fmt.Errorf("failed to describe RDS instance with identifier %s: %w", rdsInstanceId, err)
		}
	}

	switch {
	case len(resp.DBInstances) == 0:
		return "", errors.New(fmt.Sprintf("failed to find any RDS instances with identifier %s", rdsInstanceId))
	case len(resp.DBInstances) > 1:
		return "", errors.New(fmt.Sprintf("received back more than one RDS instance with identifier %s", rdsInstanceId))
	}

	return *resp.DBInstances[0].DBInstanceStatus, nil
}
