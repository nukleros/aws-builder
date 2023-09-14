package iam

import (
	"github.com/aws/aws-sdk-go-v2/service/iam/types"
)

// CreateIamTags creates tags for IAM resources.
func CreateIamTags(name string, tags map[string]string) *[]types.Tag {
	nameKey := "Name"
	ec2Tags := []types.Tag{
		{
			Key:   &nameKey,
			Value: &name,
		},
	}
	for k, v := range tags {
		t := types.Tag{
			Key:   &k,
			Value: &v,
		}
		ec2Tags = append(ec2Tags, t)
	}

	return &ec2Tags
}
