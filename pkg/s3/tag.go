package s3

import "github.com/aws/aws-sdk-go-v2/service/s3/types"

// CreateS3Tags returns the tags for an S3 bucket including a name tag and
// any custom tags provided.
func CreateS3Tags(name string, customTags map[string]string) *[]types.Tag {
	nameKey := "Name"
	tags := []types.Tag{
		{
			Key:   &nameKey,
			Value: &name,
		},
	}
	for k, v := range customTags {
		t := types.Tag{
			Key:   &k,
			Value: &v,
		}
		tags = append(tags, t)
	}

	return &tags
}
