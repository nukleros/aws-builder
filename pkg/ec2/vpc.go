package ec2

import (
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/nukleros/aws-builder/pkg/client"
)

// CheckUniqueTagsForVpc checks to see if a VPC with a mathcing tags already
// exists.
func CheckUniqueTagsForVpc(
	client client.Client,
	tags *[]types.Tag,
) (*types.Vpc, bool, error) {
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

	svc := ec2.NewFromConfig(*client.GetAwsConfig())

	describeVpcInput := ec2.DescribeVpcsInput{
		Filters: filters,
	}
	resp, err := svc.DescribeVpcs(client.GetContext(), &describeVpcInput)
	if err != nil {
		return nil, false, fmt.Errorf("failed to describe VPC to check for unique tags: %w", err)
	}

	switch len(resp.Vpcs) {
	case 1:
		return &resp.Vpcs[0], true, nil
	case 0:
		return nil, false, nil
	default:
		return nil, false, errors.New("found multiple VPCs with matching tags")
	}
}

// CheckDnsResolutionForVpc takes a VPC ID and returns true if DNS resolution is
// enabled for that VPC.
func CheckDnsResolutionForVpc(
	client client.Client,
	vpcId string,
) (bool, error) {
	svc := ec2.NewFromConfig(*client.GetAwsConfig())

	describeAttributeInput := ec2.DescribeVpcAttributeInput{
		VpcId:     &vpcId,
		Attribute: types.VpcAttributeNameEnableDnsSupport,
	}
	resp, err := svc.DescribeVpcAttribute(client.GetContext(), &describeAttributeInput)
	if err != nil {
		return false, fmt.Errorf("failed to describe VPC attribute: %w", err)
	}

	if resp.EnableDnsSupport != nil && *resp.EnableDnsSupport.Value {
		return true, nil
	}

	return false, nil
}

// CheckDnsHostnamesForVpc takes a VPC ID and returns true if DNS hostnames are
// enabled for that VPC.
func CheckDnsHostnamesForVpc(
	client client.Client,
	vpcId string,
) (bool, error) {
	svc := ec2.NewFromConfig(*client.GetAwsConfig())

	describeAttributeInput := ec2.DescribeVpcAttributeInput{
		VpcId:     &vpcId,
		Attribute: types.VpcAttributeNameEnableDnsHostnames,
	}
	resp, err := svc.DescribeVpcAttribute(client.GetContext(), &describeAttributeInput)
	if err != nil {
		return false, fmt.Errorf("failed to describe VPC attribute: %w", err)
	}

	if resp.EnableDnsHostnames != nil && *resp.EnableDnsHostnames.Value {
		return true, nil
	}

	return false, nil
}
