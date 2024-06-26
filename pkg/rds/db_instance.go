package rds

import (
	"errors"
	"fmt"
	"time"

	aws_rds "github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/nukleros/aws-builder/pkg/util"
)

type RdsCondition string

const (
	RdsConditionCreated     RdsCondition = "RdsCreated"
	RdsConditionDeleted     RdsCondition = "RdsDelete"
	RdsCheckIntervalSeconds              = 15 // check cluster status every 15 seconds
	RdsCheckMaxCount                     = 60 // check 60 time before giving up (15 minutes)
)

// CreateRdsInstance creates a new RDS instance.  If an RDS instance with
// matching name and tags already exists, that DB instance will be returned and
// used in the resource stack to ensure idempotency.
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
	svc := aws_rds.NewFromConfig(*c.AwsConfig)

	copyTagsToSnapshot := true
	//managePassword := true
	monitoringInterval := int32(0)
	multiAz := false
	public := false
	createRdsInput := aws_rds.CreateDBInstanceInput{
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
		// if an RDS instance with matching name and tags already exists,
		// return that instance
		var alreadyExists *types.DBInstanceAlreadyExistsFault
		if errors.As(err, &alreadyExists) {
			dbInstance, uniqueTagsExist, err := c.checkRdsInstanceUniqueTags(instanceName, tags)
			if err != nil {
				return nil, fmt.Errorf("failed to check for unique tags on RDS instance %s that already exists: %w", instanceName, err)
			}
			if uniqueTagsExist {
				return dbInstance, nil
			}
		} else {
			return nil, fmt.Errorf("failed to create RDS instance %s: %w", instanceName, err)
		}
	}

	return rdsResp.DBInstance, nil
}

// DeleteRdsInstance removes an existing RDS instance.
func (c *RdsClient) DeleteRdsInstance(rdsInstanceId string) error {
	if rdsInstanceId == "" {
		return nil
	}

	svc := aws_rds.NewFromConfig(*c.AwsConfig)

	skipFinalSnapshot := true
	deleteRdsInput := aws_rds.DeleteDBInstanceInput{
		DBInstanceIdentifier: &rdsInstanceId,
		SkipFinalSnapshot:    &skipFinalSnapshot,
	}
	_, err := svc.DeleteDBInstance(c.Context, &deleteRdsInput)
	if err != nil {
		return fmt.Errorf("failed to delete RDS instance %s: %w", rdsInstanceId, err)
	}

	return nil
}

// WaitForRdsInstance waits for an instance to reach the desired condition.  If
// waiting for creation, it returns when the DB instance is available and
// returns its endpoint.  If waiting for deletion, it returns when the DB
// instance is not found.  In either case, it times out after 10 min if the
// desired condition is not reached.
func (c *RdsClient) WaitForRdsInstance(
	rdsInstanceId string,
	rdsCondition RdsCondition,
) (string, error) {
	var dbEndpoint string

	if rdsInstanceId == "" {
		return dbEndpoint, nil
	}

	rdsCheckCount := 0
	for {
		rdsCheckCount += 1
		if rdsCheckCount > RdsCheckMaxCount {
			return dbEndpoint, errors.New("RDS instance check timed out waiting for the desired condition")
		}

		rdsInstance, err := c.getRdsInstance(rdsInstanceId)
		if err != nil {
			if errors.Is(err, util.ErrResourceNotFound) && rdsCondition == RdsConditionDeleted {
				// RDS instance was not found and we're waiting for deletion so
				// condition is met
				break
			} else {
				return dbEndpoint, fmt.Errorf("failed to get RDS instance status with identifier %s: %w", rdsInstanceId, err)
			}
		}

		if *rdsInstance.DBInstanceStatus == "available" && rdsCondition == RdsConditionCreated {
			dbEndpoint = *rdsInstance.Endpoint.Address
			// RDS instance is available and we're waiting for creation so
			// condition is met
			break
		}

		time.Sleep(time.Second * time.Duration(RdsCheckIntervalSeconds))
	}

	return dbEndpoint, nil
}

// getRdsInstance retrieves an RDS DBInstance.
func (c *RdsClient) getRdsInstance(rdsInstanceId string) (*types.DBInstance, error) {
	svc := aws_rds.NewFromConfig(*c.AwsConfig)

	describeRdsInput := aws_rds.DescribeDBInstancesInput{
		DBInstanceIdentifier: &rdsInstanceId,
	}
	resp, err := svc.DescribeDBInstances(c.Context, &describeRdsInput)
	if err != nil {
		var notFoundErr *types.DBInstanceNotFoundFault
		if errors.As(err, &notFoundErr) {
			return nil, util.ErrResourceNotFound
		} else {
			return nil, fmt.Errorf("failed to describe RDS instance with identifier %s: %w", rdsInstanceId, err)
		}
	}

	switch {
	case len(resp.DBInstances) == 0:
		return nil, errors.New(fmt.Sprintf("failed to find any RDS instances with identifier %s", rdsInstanceId))
	case len(resp.DBInstances) > 1:
		return nil, errors.New(fmt.Sprintf("received back more than one RDS instance with identifier %s", rdsInstanceId))
	}

	return &resp.DBInstances[0], nil
}

// checkRdsInstanceUniqueTags checks to see if an RDS instance with matching
// name and tags already exists.
func (c *RdsClient) checkRdsInstanceUniqueTags(
	instanceName string,
	tags *[]types.Tag,
) (*types.DBInstance, bool, error) {
	svc := aws_rds.NewFromConfig(*c.AwsConfig)

	describeRdsInput := aws_rds.DescribeDBInstancesInput{
		DBInstanceIdentifier: &instanceName,
	}
	resp, err := svc.DescribeDBInstances(c.Context, &describeRdsInput)
	if err != nil {
		return nil, false, fmt.Errorf("failed to describe RDS instances to check for unique tags: %w", err)
	}

	for _, rdsInstance := range resp.DBInstances {
		tagsMatch, err := c.CheckUniqueTagsForResource(
			*rdsInstance.DBInstanceArn,
			tags,
		)
		if err != nil {
			return nil, false, fmt.Errorf("failed to check unique tags for RDS instance %s: %w", *rdsInstance.DBInstanceIdentifier, err)
		}

		if tagsMatch {
			return &rdsInstance, true, nil
		}
	}

	return nil, false, nil
}
