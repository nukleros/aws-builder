package ec2

import (
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/nukleros/aws-builder/pkg/client"
)

// CheckUniqueTagsForSubnet checks to see if a subnet with matching tags already
// exists.
func CheckUniqueTagsForSubnet(
	client client.Client,
	tags *[]types.Tag,
	cidrBlock string,
) (*types.Subnet, bool, error) {
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

	// add CIDR block to filters
	cidrFilterName := "cidr-block"
	cidrFilter := types.Filter{
		Name:   &cidrFilterName,
		Values: []string{cidrBlock},
	}
	filters = append(filters, cidrFilter)

	svc := ec2.NewFromConfig(*client.GetAwsConfig())

	describeSubnetInput := ec2.DescribeSubnetsInput{
		Filters: filters,
	}
	resp, err := svc.DescribeSubnets(client.GetContext(), &describeSubnetInput)
	if err != nil {
		return nil, false, fmt.Errorf("failed to describe subnet to check for unique tags: %w", err)
	}

	switch len(resp.Subnets) {
	case 1:
		return &resp.Subnets[0], true, nil
	case 0:
		return nil, false, nil
	default:
		return nil, false, fmt.Errorf("found multiple subnets with matching tags and CIDR block: %s", cidrBlock)
	}
}
