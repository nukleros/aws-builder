package ec2

import (
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/nukleros/aws-builder/pkg/client"
)

// CheckUniqueTagsForSecurityGroup checks to see if a security group with a
// matching name and tags already exists.
func CheckUniqueTagsForSecurityGroup(
	client client.Client,
	groupName string,
	tags *[]types.Tag,
) (string, bool, error) {
	// add security group name to filters
	nameFilter := "group-name"
	filters := []types.Filter{
		{
			Name:   &nameFilter,
			Values: []string{groupName},
		},
	}

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

	describeSecurityGroupInput := ec2.DescribeSecurityGroupsInput{
		Filters: filters,
	}
	resp, err := svc.DescribeSecurityGroups(client.GetContext(), &describeSecurityGroupInput)
	if err != nil {
		return "", false, fmt.Errorf("failed to describe security group to check for unique tags: %w", err)
	}

	switch len(resp.SecurityGroups) {
	case 1:
		return *resp.SecurityGroups[0].GroupId, true, nil
	case 0:
		return "", false, nil
	default:
		return "", false, errors.New("found multiple security groups with matching name and tags")
	}
}
