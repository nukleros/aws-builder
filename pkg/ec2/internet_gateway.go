package ec2

import (
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/nukleros/aws-builder/pkg/client"
)

// CheckUniqueTagsForInternetGateway checks to see if an internet gateway with
// mathcing tags already exists.
func CheckUniqueTagsForInternetGateway(
	client client.Client,
	tags *[]types.Tag,
) (*types.InternetGateway, bool, error) {
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

	describeInternetGatewayInput := ec2.DescribeInternetGatewaysInput{
		Filters: filters,
	}
	resp, err := svc.DescribeInternetGateways(client.GetContext(), &describeInternetGatewayInput)
	if err != nil {
		return nil, false, fmt.Errorf("failed to describe VPC to check for unique tags: %w", err)
	}

	switch len(resp.InternetGateways) {
	case 1:
		return &resp.InternetGateways[0], true, nil
	case 0:
		return nil, false, nil
	default:
		return nil, false, errors.New("found multiple internet gateways with matchingtags")
	}
}
