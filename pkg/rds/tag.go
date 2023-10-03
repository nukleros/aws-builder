package rds

import "github.com/aws/aws-sdk-go-v2/service/rds/types"

// CreateRdsTags returns the tags for an RDS instance including a name tag and
// any custom tags provided.
func CreateRdsTags(name string, customTags map[string]string) *[]types.Tag {
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
