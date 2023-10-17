package eks

import (
	"errors"
	"fmt"

	aws_ec2 "github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/smithy-go"

	"github.com/nukleros/aws-builder/pkg/ec2"
)

// CreateInternetGateway creates an internet gateway for the VPC in which an EKS
// cluster is provisioned.
func (c *EksClient) CreateInternetGateway(
	tags *[]types.Tag,
	vpcId string,
	clusterName string,
) (*types.InternetGateway, error) {
	svc := aws_ec2.NewFromConfig(*c.AwsConfig)

	// because internet gateways don't have unique names we have to check for
	// an existing gateway with matching tags up front
	igw, uniqueTagsExist, err := ec2.CheckUniqueTagsForInternetGateway(c, tags)
	if err != nil {
		return nil, fmt.Errorf("failed to check for unique tags on VPC: %w", err)
	}

	var createdIgw types.InternetGateway
	if !uniqueTagsExist {
		createIGWInput := aws_ec2.CreateInternetGatewayInput{
			TagSpecifications: []types.TagSpecification{
				{
					ResourceType: types.ResourceTypeInternetGateway,
					Tags:         *tags,
				},
			},
		}
		resp, err := svc.CreateInternetGateway(c.Context, &createIGWInput)
		if err != nil {
			return nil, fmt.Errorf("failed to create internet gateway: %w", err)
		}
		createdIgw = *resp.InternetGateway
	} else {
		createdIgw = *igw
	}

	attachIGWInput := aws_ec2.AttachInternetGatewayInput{
		InternetGatewayId: createdIgw.InternetGatewayId,
		VpcId:             &vpcId,
	}
	_, err = svc.AttachInternetGateway(c.Context, &attachIGWInput)
	if err != nil {
		// if an existing internet gateway with matching tags is already
		// attached, return the IGW
		var apiErr *smithy.GenericAPIError
		if ok := errors.As(err, &apiErr); ok {
			if apiErr.Code == "Resource.AlreadyAssociated" {
				return &createdIgw, nil
			}
		}
		return nil, fmt.Errorf(
			"failed to attach internet gateway with ID %s to VPC with ID %s",
			*createdIgw.InternetGatewayId, vpcId)
	}

	return &createdIgw, nil
}

// DeleteInternetGateway deletes an internet gateway.  If an empty ID is
// supplied, or if the internet gateway is not found, it returns without error.
func (c *EksClient) DeleteInternetGateway(internetGatewayId, vpcId string) error {
	// if internetGatewayId is empty, there's nothing to delete
	if internetGatewayId == "" {
		return nil
	}

	svc := aws_ec2.NewFromConfig(*c.AwsConfig)

	detachInternetGatewayInput := aws_ec2.DetachInternetGatewayInput{
		InternetGatewayId: &internetGatewayId,
		VpcId:             &vpcId,
	}
	_, err := svc.DetachInternetGateway(c.Context, &detachInternetGatewayInput)
	if err != nil {
		var ae smithy.APIError
		if errors.As(err, &ae) {
			if ae.ErrorCode() == "InvalidInternetGatewayID.NotFound" {
				// attempting to detach a internet gateway that doesn't exist so return
				// without error
				return nil
			} else {
				return fmt.Errorf("failed to detach internet gateway with ID %s: %w", internetGatewayId, err)
			}
		} else {
			return fmt.Errorf("failed to detach internet gateway with ID %s: %w", internetGatewayId, err)
		}
	}

	deleteInternetGatewayInput := aws_ec2.DeleteInternetGatewayInput{InternetGatewayId: &internetGatewayId}
	_, err = svc.DeleteInternetGateway(c.Context, &deleteInternetGatewayInput)
	if err != nil {
		var ae smithy.APIError
		if errors.As(err, &ae) {
			if ae.ErrorCode() == "InvalidInternetGatewayID.NotFound" {
				// attempting to delete a internet gateway that doesn't exist so return
				// without error
				return nil
			} else {
				return fmt.Errorf("failed to delete internet gateway with ID %s: %w", internetGatewayId, err)
			}
		} else {
			return fmt.Errorf("failed to delete internet gateway with ID %s: %w", internetGatewayId, err)
		}
	}

	return nil
}
