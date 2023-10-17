package ec2

import (
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/nukleros/aws-builder/pkg/client"
)

// CheckUniqueTagsForElasticIp checks to see if an elastic IP with matching tags
// already exists
func CheckUniqueTagsForElasticIp(
	client client.Client,
	tags *[]types.Tag,
) (*types.Address, bool, error) {
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

	describeElasticIpInput := ec2.DescribeAddressesInput{
		Filters: filters,
	}
	resp, err := svc.DescribeAddresses(client.GetContext(), &describeElasticIpInput)
	if err != nil {
		return nil, false, fmt.Errorf("failed to describe elastic IP to check for unique tags: %w", err)
	}

	switch len(resp.Addresses) {
	case 1:
		return &resp.Addresses[0], true, nil
	case 0:
		return nil, false, nil
	default:
		return nil, false, errors.New("found multiple elastic IPs with matching name and tags")
	}
}
