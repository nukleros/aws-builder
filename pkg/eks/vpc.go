package eks

import (
	"errors"
	"fmt"

	aws_ec2 "github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/smithy-go"

	"github.com/nukleros/aws-builder/pkg/ec2"
)

// CreateVpc creates a VPC for an EKS cluster.  It adds the necessary tags and
// enables the DNS attributes.
func (c *EksClient) CreateVpc(
	tags *[]types.Tag,
	cidrBlock string,
	clusterName string,
) (*types.Vpc, error) {
	svc := aws_ec2.NewFromConfig(*c.AwsConfig)

	// because VPCs don't have unique names we have to check for an existing VPC
	// with matching tags up front
	vpc, uniqueTagsExist, err := ec2.CheckUniqueTagsForVpc(c, tags)
	if err != nil {
		return nil, fmt.Errorf("failed to check for unique tags on VPC: %w", err)
	}
	if uniqueTagsExist {
		// check to ensure DNS resolution is enabled
		dnsResolutionEnabled, err := ec2.CheckDnsResolutionForVpc(c, *vpc.VpcId)
		if err != nil {
			return nil, fmt.Errorf("failed to check DNS resolution attribute on VPC: %w", err)
		}
		if !dnsResolutionEnabled {
			if err := c.enableDnsResolutionOnVpc(*vpc.VpcId); err != nil {
				return nil, err
			}
		}

		// check to ensure DNS hostnames are enabled
		dnsHostnamesEnabled, err := ec2.CheckDnsHostnamesForVpc(c, *vpc.VpcId)
		if err != nil {
			return nil, fmt.Errorf("failed to check DNS hostnames attribute on VPC: %w", err)
		}
		if !dnsHostnamesEnabled {
			if err := c.enableDnsHostnamesOnVpc(*vpc.VpcId); err != nil {
				return nil, err
			}
		}

		return vpc, nil
	}

	vpcTags := *tags

	clusterNameTagKey := "kubernetes.io/cluster/cluster-name"
	clusterNameTagValue := clusterName
	clusterNameTag := types.Tag{
		Key:   &clusterNameTagKey,
		Value: &clusterNameTagValue,
	}
	vpcTags = append(vpcTags, clusterNameTag)

	clusterNameSharedTagKey := fmt.Sprintf("kubernetes.io/cluster/%s", clusterName)
	clusterNameSharedTagValue := "shared"
	clusterNameSharedTag := types.Tag{
		Key:   &clusterNameSharedTagKey,
		Value: &clusterNameSharedTagValue,
	}
	vpcTags = append(vpcTags, clusterNameSharedTag)

	createVpcInput := aws_ec2.CreateVpcInput{
		CidrBlock: &cidrBlock,
		TagSpecifications: []types.TagSpecification{
			{
				ResourceType: types.ResourceTypeVpc,
				Tags:         vpcTags,
			},
		},
	}
	resp, err := svc.CreateVpc(c.Context, &createVpcInput)
	if err != nil {
		return nil, fmt.Errorf("failed to create VPC for cluster %s: %w", clusterName, err)
	}

	// enable DNS resolution for VPC
	if err := c.enableDnsResolutionOnVpc(*resp.Vpc.VpcId); err != nil {
		return nil, err
	}

	// enable DNS hostnames for instances launcehd in VPC
	if err := c.enableDnsHostnamesOnVpc(*resp.Vpc.VpcId); err != nil {
		return nil, err
	}

	return resp.Vpc, nil
}

// DeleteVpc deletes the VPC used by an EKS cluster.  If the VPC ID is empty, or
// if the VPC is not found it returns without error.
func (c *EksClient) DeleteVpc(vpcId string) error {
	// if both vpcId is empty, there's nothing to delete
	if vpcId == "" {
		return nil
	}

	svc := aws_ec2.NewFromConfig(*c.AwsConfig)

	deleteVpcInput := aws_ec2.DeleteVpcInput{VpcId: &vpcId}
	_, err := svc.DeleteVpc(c.Context, &deleteVpcInput)
	if err != nil {
		var ae smithy.APIError
		if errors.As(err, &ae) {
			if ae.ErrorCode() == "InvalidVpcID.NotFound" {
				// attempting to delete a VPC that doesn't exist so return
				// without error
				return nil
			} else {
				return fmt.Errorf("failed to delete VPC with ID %s: %w", vpcId, err)
			}
		} else {
			return fmt.Errorf("failed to delete VPC with ID %s: %w", vpcId, err)
		}
	}

	return nil
}

// enableDnsResolutionOnVpc takes a VPC ID and enables DNS resolution for it.
func (c *EksClient) enableDnsResolutionOnVpc(vpcId string) error {
	svc := aws_ec2.NewFromConfig(*c.AwsConfig)

	valueTrue := true
	attributeTrue := types.AttributeBooleanValue{Value: &valueTrue}
	modifyVpcAttributeDnsSupportInput := aws_ec2.ModifyVpcAttributeInput{
		VpcId:            &vpcId,
		EnableDnsSupport: &attributeTrue,
	}
	_, err := svc.ModifyVpcAttribute(c.Context, &modifyVpcAttributeDnsSupportInput)
	if err != nil {
		return fmt.Errorf("failed to modify VPC attribute to enable DNS support for VPC with ID %s: %w", vpcId, err)
	}

	return nil
}

// enableDnsHostnamesOnVpc takes a VPC ID and enables DNS hostnames for
// instances launched in that VPC.
func (c *EksClient) enableDnsHostnamesOnVpc(vpcId string) error {
	svc := aws_ec2.NewFromConfig(*c.AwsConfig)

	valueTrue := true
	attributeTrue := types.AttributeBooleanValue{Value: &valueTrue}
	modifyVpcAttributeDnsHostnamesInput := aws_ec2.ModifyVpcAttributeInput{
		VpcId:              &vpcId,
		EnableDnsHostnames: &attributeTrue,
	}
	_, err := svc.ModifyVpcAttribute(c.Context, &modifyVpcAttributeDnsHostnamesInput)
	if err != nil {
		return fmt.Errorf("failed to modify VPC attribute to enable DNS hostnames for VPC with ID %s: %w", vpcId, err)
	}

	return nil
}
