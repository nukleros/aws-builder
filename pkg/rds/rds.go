package rds

import (
	"errors"
	"fmt"
	"time"

	awsrds "github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/rds/types"
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
) (*types.DBInstance, error) {
	svc := awsrds.NewFromConfig(*c.AwsConfig)

	copyTagsToSnapshot := true
	managePassword := true
	monitoringInterval := int32(0)
	multiAz := false
	public := false
	createRdsInput := awsrds.CreateDBInstanceInput{
		DBInstanceIdentifier:     &instanceName,
		DBName:                   &dbName,
		DBInstanceClass:          &class,
		Engine:                   &engine,
		EngineVersion:            &engineVersion,
		AllocatedStorage:         &storageGb,
		BackupRetentionPeriod:    &backupDays,
		CopyTagsToSnapshot:       &copyTagsToSnapshot,
		ManageMasterUserPassword: &managePassword,
		MasterUsername:           &dbUser,
		MonitoringInterval:       &monitoringInterval,
		MultiAZ:                  &multiAz,
		PubliclyAccessible:       &public,
		Tags:                     *tags,
		//CustomIamInstanceProfile: &iamProfileName,
	}
	resp, err := svc.CreateDBInstance(c.Context, &createRdsInput)
	if err != nil {
		return nil, fmt.Errorf("failed to create RDS instance %s: %w", instanceName, err)
	}

	return resp.DBInstance, nil
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
func (c *RdsClient) WaitForRdsInstance(rdsInstanceId string) error {
	rdsInstCheckCount := 0
	rdsInstCheckIntervalSeconds := 10
	rdsInstMaxCheck := 60

	for {
		rdsInstCheckCount += 1
		if rdsInstCheckCount > rdsInstMaxCheck {
			return errors.New("RDS instance check timed out waiting for it be ready")
		}

		rdsInstanceStatus, err := c.getRdsInstanceStatus(rdsInstanceId)
		if err != nil {
			return fmt.Errorf("failed to get RDS instance status with identifier %s: %w", rdsInstanceId, err)
		}

		if rdsInstanceStatus == "available" {
			break
		}

		time.Sleep(time.Second * time.Duration(rdsInstCheckIntervalSeconds))
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
		return "", fmt.Errorf("failed to describe RDS instance with identifier %s: %w", rdsInstanceId, err)
	}

	switch {
	case len(resp.DBInstances) == 0:
		return "", errors.New(fmt.Sprintf("failed to find any RDS instances with identifier %s", rdsInstanceId))
	case len(resp.DBInstances) > 1:
		return "", errors.New(fmt.Sprintf("received back more than one RDS instance with identifier %s", rdsInstanceId))
	}

	return *resp.DBInstances[0].DBInstanceStatus, nil
}
