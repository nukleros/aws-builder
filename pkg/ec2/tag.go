package ec2

import "github.com/aws/aws-sdk-go-v2/service/ec2/types"

// CreateEc2Tags creates tags for EC2 resources.
func CreateEc2Tags(name string, customTags map[string]string) *[]types.Tag {
	nameKey := "Name"
	tags := []types.Tag{
		{
			Key:   &nameKey,
			Value: &name,
		},
	}
	for k, v := range customTags {
		key := k
		val := v
		t := types.Tag{
			Key:   &key,
			Value: &val,
		}
		tags = append(tags, t)
	}

	return &tags
}
