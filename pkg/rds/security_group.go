package rds

import (
	"errors"
	"fmt"

	aws_ec2 "github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/smithy-go"

	"github.com/nukleros/aws-builder/pkg/ec2"
)

// CreateSecurityGroup creates a security group for the RDS instance and adds an
// ingress and egress rule that allows connections from workloads in the VPC
// where the runtime is deployed.  The sourceSecurityGroupId argument provided
// must be for the VPC where the workloads are running.  If a security group
// with matching name and tags already exists, that security group ID will be
// returned and used in the resource stack to ensure idempotency.
func (c *RdsClient) CreateSecurityGroup(
	tags *[]types.Tag,
	instanceName string,
	vpcId string,
	port int32,
	sourceSecurityGroupId string,
	awsAccount string,
) (string, error) {
	svc := aws_ec2.NewFromConfig(*c.AwsConfig)

	// create security group
	description := fmt.Sprintf("security group for RDS instance %s", instanceName)
	groupName := fmt.Sprintf("%s-rds-sg", instanceName)
	createSecurityGroupInput := aws_ec2.CreateSecurityGroupInput{
		Description: &description,
		GroupName:   &groupName,
		VpcId:       &vpcId,
		TagSpecifications: []types.TagSpecification{
			{
				ResourceType: types.ResourceTypeSecurityGroup,
				Tags:         *tags,
			},
		},
	}
	createSgResp, err := svc.CreateSecurityGroup(c.Context, &createSecurityGroupInput)
	if err != nil {
		// if a security group with matching name and tags already exists,
		// return that security group ID
		var apiErr *smithy.GenericAPIError
		if ok := errors.As(err, &apiErr); ok {
			if apiErr.Code == "InvalidGroup.Duplicate" {
				sgId, uniqueTagsExist, err := ec2.CheckUniqueTagsForSecurityGroup(c, groupName, tags)
				if err != nil {
					return "", fmt.Errorf("failed to check for unique tags on security group with name %s: %w", groupName, err)
				}
				if uniqueTagsExist {
					return sgId, nil
				}
			}
		}
		return "", fmt.Errorf("failed to create security group for RDS instance %s: %w", instanceName, err)
	}

	// create ingress rule
	protocol := "tcp"
	ruleDescription := "allow DB clients from local VPC"
	ingressIpPermission := types.IpPermission{
		FromPort:   &port,
		ToPort:     &port,
		IpProtocol: &protocol,
		UserIdGroupPairs: []types.UserIdGroupPair{
			{
				Description: &ruleDescription,
				GroupId:     &sourceSecurityGroupId,
				UserId:      &awsAccount,
				VpcId:       &vpcId,
			},
		},
	}
	authIngressInput := aws_ec2.AuthorizeSecurityGroupIngressInput{
		GroupId:       createSgResp.GroupId,
		IpPermissions: []types.IpPermission{ingressIpPermission},
		TagSpecifications: []types.TagSpecification{
			{
				ResourceType: types.ResourceTypeSecurityGroupRule,
				Tags:         *tags,
			},
		},
	}
	_, err = svc.AuthorizeSecurityGroupIngress(c.Context, &authIngressInput)
	if err != nil {
		return "", fmt.Errorf("failed to authorize ingress rule on security group for RDS instance %s: %w", instanceName, err)
	}

	// create egress rule
	egressPort := int32(-1)
	egressIpPermission := types.IpPermission{
		FromPort:   &egressPort,
		ToPort:     &egressPort,
		IpProtocol: &protocol,
	}
	authEgressInput := aws_ec2.AuthorizeSecurityGroupEgressInput{
		GroupId:       createSgResp.GroupId,
		IpPermissions: []types.IpPermission{egressIpPermission},
		TagSpecifications: []types.TagSpecification{
			{
				ResourceType: types.ResourceTypeSecurityGroupRule,
				Tags:         *tags,
			},
		},
	}
	_, err = svc.AuthorizeSecurityGroupEgress(c.Context, &authEgressInput)
	if err != nil {
		return "", fmt.Errorf("failed to authorize egress rule on security group for RDS instance %s: %w", instanceName, err)
	}

	return *createSgResp.GroupId, nil
}

// DeleteSecurityGroup deletes a security group that was used by an RDS
// instance.
func (c *RdsClient) DeleteSecurityGroup(securityGroupId string) error {
	if securityGroupId == "" {
		return nil
	}

	svc := aws_ec2.NewFromConfig(*c.AwsConfig)

	deleteSecurityGroupInput := aws_ec2.DeleteSecurityGroupInput{
		GroupId: &securityGroupId,
	}
	_, err := svc.DeleteSecurityGroup(c.Context, &deleteSecurityGroupInput)
	if err != nil {
		return fmt.Errorf("failed to delete security group with ID %s: %w", securityGroupId, err)
	}

	return nil
}
