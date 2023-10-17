package eks

import (
	"errors"
	"fmt"
	"time"

	aws_ec2 "github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/smithy-go"

	"github.com/nukleros/aws-builder/pkg/ec2"
)

type NatGatewayCondition string

const (
	NatGatewayConditionCreated = "NatGatewayCreated"
	NatGatewayConditionDeleted = "NatGatewayDeleted"
	NatGatewayCheckInterval    = 15 //check cluster status every 15 seconds
	NatGatewayCheckMaxCount    = 20 // check 20 times before giving up (5 minutes)
)

// CreateNatGateways creates a NAT gateway for each private subnet so that it
// may reach the public internet.
func (c *EksClient) CreateNatGateways(
	tags *[]types.Tag,
	azInventory *[]AvailabilityZoneInventory,
	elasticIpIds []string,
) error {
	svc := aws_ec2.NewFromConfig(*c.AwsConfig)

	for i, az := range *azInventory {
		eip := elasticIpIds[i]
		for _, publicSubnet := range az.PublicSubnets {
			// because NAT gateways don't have unique names we have to check for
			// existing gateways with matching tags up front
			_, uniqueTagsExist, err := ec2.CheckUniqueTagsForNatGateway(c, tags, publicSubnet.SubnetId)
			if err != nil {
				return fmt.Errorf("failed to check for unique tags on NAT gateway: %w", err)
			}
			if uniqueTagsExist {
				continue
			}

			createNatGatewayInput := aws_ec2.CreateNatGatewayInput{
				SubnetId:     &publicSubnet.SubnetId,
				AllocationId: &eip,
				TagSpecifications: []types.TagSpecification{
					{
						ResourceType: types.ResourceTypeNatgateway,
						Tags:         *tags,
					},
				},
			}
			_, err = svc.CreateNatGateway(c.Context, &createNatGatewayInput)
			if err != nil {
				return fmt.Errorf("failed to create NAT gateway in subnet with ID %s: %w", publicSubnet.SubnetId, err)
			}
		}
	}

	return nil
}

// DeleteNatGateways deletes the NAT gateway for each availability zone.
func (c *EksClient) DeleteNatGateways(azInventory *[]AvailabilityZoneInventory) ([]string, error) {
	// collect NAT gateway inventory
	var natGatewayIds []string
	for _, azInv := range *azInventory {
		if azInv.NatGatewayId != "" {
			natGatewayIds = append(natGatewayIds, azInv.NatGatewayId)
		}
	}

	// if there are no subnet IDs there is nothing to do
	if len(natGatewayIds) == 0 {
		return natGatewayIds, nil
	}

	svc := aws_ec2.NewFromConfig(*c.AwsConfig)

	for _, natGatewayId := range natGatewayIds {
		deleteNatGatewayInput := aws_ec2.DeleteNatGatewayInput{NatGatewayId: &natGatewayId}
		_, err := svc.DeleteNatGateway(c.Context, &deleteNatGatewayInput)
		if err != nil {
			var ae smithy.APIError
			if errors.As(err, &ae) {
				if ae.ErrorCode() == "InvalidNATGatewayID.NotFound" {
					// attempting to delete a NAT gateway that doesn't exist so
					// continue to the next
					continue
				} else {
					return natGatewayIds, fmt.Errorf("failed to delete NAT gateway with ID %s: %w", natGatewayId, err)
				}
			} else {
				return natGatewayIds, fmt.Errorf("failed to delete NAT gateway with ID %s: %w", natGatewayId, err)
			}
		}
	}

	return natGatewayIds, nil
}

// WaitForNatGateways waits for a NAT gateway to reach a given condition.  One of:
// * NatGatewayConditionCreated
// * NatGatewayConditionDeleted
func (c *EksClient) WaitForNatGateways(
	vpcId string,
	azInventory *[]AvailabilityZoneInventory,
	natGatewayCondition NatGatewayCondition,
) (*[]AvailabilityZoneInventory, []string, error) {
	natGatewayCheckCount := 0

	var updatedAzInventory []AvailabilityZoneInventory
	var natGatewayIds []string

	for {
		natGatewayCheckCount += 1
		if natGatewayCheckCount > NatGatewayCheckMaxCount {
			return nil, natGatewayIds, errors.New("NAT gateway condition check timed out")
		}

		natGatewayStates, updatedAzInv, err := c.getNatGatewayStatuses(vpcId, azInventory)
		if err != nil {
			return nil, natGatewayIds, fmt.Errorf("failed to get NAT gateway statuses for VPC with ID %s: %w", vpcId, err)
		}

		if len(*natGatewayStates) == 0 && natGatewayCondition == NatGatewayConditionDeleted {
			// no NAT gateway resources found for this VPC while waiting for
			// deletion so condition is met
			updatedAzInventory = *updatedAzInv
			break
		}

		allConditionsMet := true
		for _, state := range *natGatewayStates {
			if state != types.NatGatewayStateAvailable && natGatewayCondition == NatGatewayConditionCreated {
				// resource is not available but we're waiting for it to be
				// created so condition is not met
				allConditionsMet = false
				break
			} else if state != types.NatGatewayStateDeleted && natGatewayCondition == NatGatewayConditionDeleted {
				// resource is not in deleted state but we're waiting for it to
				// be deleted so condition is not met
				allConditionsMet = false
				break
			}
		}

		if allConditionsMet {
			updatedAzInventory = *updatedAzInv
			break
		}

		time.Sleep(time.Second * 15)
	}

	for _, az := range updatedAzInventory {
		if az.NatGatewayId != "" {
			natGatewayIds = append(natGatewayIds, az.NatGatewayId)
		}
	}

	return &updatedAzInventory, natGatewayIds, nil
}

// getNatGatewayStatuses returns the state of each NAT gateway found in the
// the public subnets of the provided VPC.  The gateways in a "failed" or
// "deleted" state are filtered out as irrlevant to allow for retries.  It also
// returns an updated availability zone inventory with NAT gateway IDs populated.
func (c *EksClient) getNatGatewayStatuses(
	vpcId string,
	availabilityZoneInventory *[]AvailabilityZoneInventory,
) (*[]types.NatGatewayState, *[]AvailabilityZoneInventory, error) {
	svc := aws_ec2.NewFromConfig(*c.AwsConfig)

	updatedAzInventory := *availabilityZoneInventory
	var natGatewayStates []types.NatGatewayState

	filterName := "vpc-id"
	describeNatGatewaysInput := aws_ec2.DescribeNatGatewaysInput{
		Filter: []types.Filter{
			{
				Name:   &filterName,
				Values: []string{vpcId},
			},
		},
	}
	resp, err := svc.DescribeNatGateways(c.Context, &describeNatGatewaysInput)
	if err != nil {
		return nil, &updatedAzInventory, fmt.Errorf("failed to describe NAT gateways for VPC with ID %s: %w", vpcId, err)
	}

	// update the AZ inventory with the NAT gateway ID when available
	for _, natGateway := range resp.NatGateways {
		if natGateway.SubnetId != nil && natGateway.NatGatewayId != nil {
			for i, az := range updatedAzInventory {
				for _, subnet := range az.PublicSubnets {
					if subnet.SubnetId == *natGateway.SubnetId && stateRelevant(natGateway.State) {
						updatedAzInventory[i].NatGatewayId = *natGateway.NatGatewayId
						natGatewayStates = append(natGatewayStates, natGateway.State)
					}
				}
			}
		}
	}

	return &natGatewayStates, &updatedAzInventory, nil
}

// stateRelevant checks the state of a NAT gateway to see if it's relevant to
// the creation or deletion of NAT gateways for a resource stack.  For these
// purposes, we don't care about deleted or failed NAT gateways.
func stateRelevant(natGatewayState types.NatGatewayState) bool {
	ignoreStates := []types.NatGatewayState{
		types.NatGatewayStateFailed,
		types.NatGatewayStateDeleted,
	}

	for _, state := range ignoreStates {
		if natGatewayState == state {
			return false
		}
	}

	return true
}
