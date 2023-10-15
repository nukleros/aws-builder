package eks

import (
	"errors"
	"fmt"

	aws_ec2 "github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/smithy-go"

	"github.com/nukleros/aws-builder/pkg/ec2"
)

// CreatePublicSubnets creates the public subnets used by an EKS cluster for
// each availability zone in use by the cluster.  It also tags each subnet so
// the load balancers may be correctly applied to them.
func (c *EksClient) CreatePublicSubnets(
	tags *[]types.Tag,
	vpcId string,
	clusterName string,
	azInventory *[]AvailabilityZoneInventory,
) (*[]AvailabilityZoneInventory, []string, error) {
	svc := aws_ec2.NewFromConfig(*c.AwsConfig)

	// make a copy of inventory for changes so we don't change existing AZ
	// inventory incrementally - wWe want to apply all changes or none at all
	modifiedAzInventory := *azInventory
	var publicSubnetIds []string

	// add ELB tag
	elbTagKey := "kubernetes.io/role/elb"
	elbTagValue := "1"
	subnetTags := *tags
	elbTag := types.Tag{
		Key:   &elbTagKey,
		Value: &elbTagValue,
	}
	subnetTags = append(subnetTags, elbTag)
	for azIdx, az := range modifiedAzInventory {
		for subnetIdx, subnet := range az.PublicSubnets {
			// because subnets don't have unique names we have to check for
			// existing subnets with matching tags up front
			existingSubnet, uniqueTagsExist, err := ec2.CheckUniqueTagsForSubnet(c, &subnetTags, subnet.SubnetCidr)
			if err != nil {
				return nil, publicSubnetIds, fmt.Errorf("failed to check for unique tags on subnet: %w", err)
			}
			if uniqueTagsExist {
				if !*existingSubnet.MapPublicIpOnLaunch {
					if err := c.mapPublicIpsForSubnet(*existingSubnet.SubnetId); err != nil {
						return nil, publicSubnetIds, err
					}
				}
				// subnet already exists - record its SubnetID
				modifiedAzInventory[azIdx].PublicSubnets[subnetIdx].SubnetId = *existingSubnet.SubnetId
				publicSubnetIds = append(publicSubnetIds, *existingSubnet.SubnetId)
				continue
			}

			// subnet does not exist - create it
			publicCreateSubnetInput := aws_ec2.CreateSubnetInput{
				VpcId:            &vpcId,
				AvailabilityZone: &az.Zone,
				CidrBlock:        &subnet.SubnetCidr,
				TagSpecifications: []types.TagSpecification{
					{
						ResourceType: types.ResourceTypeSubnet,
						Tags:         subnetTags,
					},
				},
			}
			publicResp, err := svc.CreateSubnet(c.Context, &publicCreateSubnetInput)
			if err != nil {
				return nil, publicSubnetIds, fmt.Errorf("failed to create public subnet for VPC with ID %s: %w", vpcId, err)
			}
			modifiedAzInventory[azIdx].PublicSubnets[subnetIdx].SubnetId = *publicResp.Subnet.SubnetId
			publicSubnetIds = append(publicSubnetIds, *publicResp.Subnet.SubnetId)

			if err := c.mapPublicIpsForSubnet(*publicResp.Subnet.SubnetId); err != nil {
				return nil, publicSubnetIds, err
			}
		}
	}

	return &modifiedAzInventory, publicSubnetIds, nil
}

// CreatePrivateSubnets creates the private subnets used by an EKS cluster for
// each availability zone in use by the cluster.  It also tags each subnet so
// the load balancers may be correctly applied to them.
func (c *EksClient) CreatePrivateSubnets(
	tags *[]types.Tag,
	vpcId string,
	clusterName string,
	azInventory *[]AvailabilityZoneInventory,
) (*[]AvailabilityZoneInventory, []string, error) {
	svc := aws_ec2.NewFromConfig(*c.AwsConfig)

	// make a copy of inventory for changes so we don't change existing AZ
	// inventory incrementally - wWe want to apply all changes or none at all
	modifiedAzInventory := *azInventory
	var privateSubnetIds []string

	// add internal ELB tag
	internalElbTagKey := "kubernetes.io/role/internal-elb"
	internalElbTagValue := "1"
	subnetTags := *tags
	internalElbTag := types.Tag{
		Key:   &internalElbTagKey,
		Value: &internalElbTagValue,
	}
	subnetTags = append(subnetTags, internalElbTag)
	for azIdx, az := range modifiedAzInventory {
		for subnetIdx, subnet := range az.PrivateSubnets {
			// because subnets don't have unique names we have to check for a
			// existing subnets with matching tags up front
			existingSubnet, uniqueTagsExist, err := ec2.CheckUniqueTagsForSubnet(c, &subnetTags, subnet.SubnetCidr)
			if err != nil {
				return nil, privateSubnetIds, fmt.Errorf("failed to check for unique tags on subnet: %w", err)
			}
			if uniqueTagsExist {
				// subnet already exists - record its SubnetId
				modifiedAzInventory[azIdx].PrivateSubnets[subnetIdx].SubnetId = *existingSubnet.SubnetId
				privateSubnetIds = append(privateSubnetIds, *existingSubnet.SubnetId)
				continue
			}

			// subnet does not exist - create it
			privateCreateSubnetInput := aws_ec2.CreateSubnetInput{
				VpcId:            &vpcId,
				AvailabilityZone: &az.Zone,
				CidrBlock:        &subnet.SubnetCidr,
				TagSpecifications: []types.TagSpecification{
					{
						ResourceType: types.ResourceTypeSubnet,
						Tags:         subnetTags,
					},
				},
			}
			privateResp, err := svc.CreateSubnet(c.Context, &privateCreateSubnetInput)
			if err != nil {
				return nil, privateSubnetIds, fmt.Errorf("failed to create private subnet for VPC with ID %s: %w", vpcId, err)
			}
			modifiedAzInventory[azIdx].PrivateSubnets[subnetIdx].SubnetId = *privateResp.Subnet.SubnetId
			privateSubnetIds = append(privateSubnetIds, *privateResp.Subnet.SubnetId)
		}
	}

	return &modifiedAzInventory, privateSubnetIds, nil
}

// DeleteSubnets deletes the subnets used by the EKS cluster.  If no subnet IDs
// are supplied, or if the subnets are not found it returns without error.
func (c *EksClient) DeleteSubnets(
	azInventory *[]AvailabilityZoneInventory,
) (*[]AvailabilityZoneInventory, []string, error) {
	// make a copy of AZ inventory to make updates and send when all changes are
	// successful
	updatedAzInventory := *azInventory

	// collect subnet inventory
	var subnetIds []string
	for azIdx, azInv := range updatedAzInventory {
		for publicSubnetIdx, publicSubnet := range azInv.PublicSubnets {
			if publicSubnet.SubnetId != "" {
				subnetIds = append(subnetIds, publicSubnet.SubnetId)
			}
			updatedAzInventory[azIdx].PublicSubnets[publicSubnetIdx].SubnetId = ""
		}
		for privateSubnetIdx, privateSubnet := range azInv.PrivateSubnets {
			if privateSubnet.SubnetId != "" {
				subnetIds = append(subnetIds, privateSubnet.SubnetId)
			}
			updatedAzInventory[azIdx].PrivateSubnets[privateSubnetIdx].SubnetId = ""
		}
	}

	// if there are no subnet IDs there is nothing to do
	if len(subnetIds) == 0 {
		return nil, subnetIds, nil
	}

	svc := aws_ec2.NewFromConfig(*c.AwsConfig)

	for _, id := range subnetIds {
		deleteSubnetInput := aws_ec2.DeleteSubnetInput{SubnetId: &id}
		_, err := svc.DeleteSubnet(c.Context, &deleteSubnetInput)
		if err != nil {
			var ae smithy.APIError
			if errors.As(err, &ae) {
				if ae.ErrorCode() == "InvalidSubnetID.NotFound" {
					// attempting to delete a subnet that doesn't exist so
					// continue with deleting any other subnets
					continue
				} else {
					return nil, subnetIds, fmt.Errorf("failed to delete subnet with ID %s: %w", id, err)
				}
			} else {
				return nil, subnetIds, fmt.Errorf("failed to delete subnet with ID %s: %w", id, err)
			}
		}
	}

	return &updatedAzInventory, subnetIds, nil
}

// mapPublicIpsForSubnet configures a subnet to have instances launched in it
// get a public IP address.
func (c *EksClient) mapPublicIpsForSubnet(subnetId string) error {
	svc := aws_ec2.NewFromConfig(*c.AwsConfig)

	mapPublicIp := true
	modifySubnetAttributeInput := aws_ec2.ModifySubnetAttributeInput{
		SubnetId:            &subnetId,
		MapPublicIpOnLaunch: &types.AttributeBooleanValue{Value: &mapPublicIp},
	}
	_, err := svc.ModifySubnetAttribute(c.Context, &modifySubnetAttributeInput)
	if err != nil {
		return fmt.Errorf("failed to modify subnet attribute for subnet with ID %s: %w",
			subnetId, err)
	}

	return nil

}
