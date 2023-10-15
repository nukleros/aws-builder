package ec2

import (
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/nukleros/aws-builder/pkg/client"
)

// CheckUniqueTagsForNatGateway checks to see if a NAT gateway with matching
// tags in a particular subnet already exists.
func CheckUniqueTagsForNatGateway(
	client client.Client,
	tags *[]types.Tag,
	subnetId string,
) (*types.NatGateway, bool, error) {
	var filters []types.Filter

	// add tags to filters
	for _, tag := range *tags {
		tagFilter := fmt.Sprintf("tag:%s", *tag.Key)
		filter := types.Filter{
			Name:   &tagFilter,
			Values: []string{*tag.Value},
		}
		filters = append(filters, filter)
	}

	// add subnet to filters
	subnetFilterName := "subnet-id"
	subnetFilter := types.Filter{
		Name:   &subnetFilterName,
		Values: []string{subnetId},
	}
	filters = append(filters, subnetFilter)

	svc := ec2.NewFromConfig(*client.GetAwsConfig())

	describeNatGatewayInput := ec2.DescribeNatGatewaysInput{
		Filter: filters,
	}
	resp, err := svc.DescribeNatGateways(client.GetContext(), &describeNatGatewayInput)
	if err != nil {
		return nil, false, fmt.Errorf("failed to describe NAT gateway to check for unique tags: %w", err)
	}

	var relevantGateways []types.NatGateway
	for _, gateway := range resp.NatGateways {
		if gateway.State == types.NatGatewayStatePending || gateway.State == types.NatGatewayStateAvailable {
			relevantGateways = append(relevantGateways, gateway)
		}
	}

	switch len(relevantGateways) {
	case 1:
		return &relevantGateways[0], true, nil
	case 0:
		return nil, false, nil
	default:
		return nil, false, fmt.Errorf("found multiple NAT gateway with matching tags in subnet: %s", subnetId)
	}
}
