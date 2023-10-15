package ec2

import (
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/nukleros/aws-builder/pkg/client"
)

// CheckUniqueTagsForRouteTable checks to see if a NAT gateway with matching
// tags in a particular subnet already exists.
func CheckUniqueTagsForRouteTables(
	client client.Client,
	tags *[]types.Tag,
) (*[]types.RouteTable, bool, error) {
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

	describeRouteTableInput := ec2.DescribeRouteTablesInput{
		Filters: filters,
	}
	resp, err := svc.DescribeRouteTables(client.GetContext(), &describeRouteTableInput)
	if err != nil {
		return nil, false, fmt.Errorf("failed to describe NAT gateway to check for unique tags: %w", err)
	}

	if len(resp.RouteTables) == 0 {
		return nil, false, nil
	}

	return &resp.RouteTables, true, nil
}
