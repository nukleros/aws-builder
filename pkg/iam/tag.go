package iam

import "github.com/aws/aws-sdk-go-v2/service/iam/types"

// CreateIamTags creates tags for IAM resources.
func CreateIamTags(name string, customTags map[string]string) *[]types.Tag {
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
