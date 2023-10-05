package rds

import (
	"fmt"

	aws_rds "github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/rds/types"
)

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

// CheckUniqueTagsForResource takes a resource ARN for an RDS resource and a
// slice of tags and checks to see if the resource has all of the provided tags.
// It returns true if the resource has the tags or false if not.
func (c *RdsClient) CheckUniqueTagsForResource(
	resourceArn string,
	tags *[]types.Tag,
) (bool, error) {
	svc := aws_rds.NewFromConfig(*c.AwsConfig)

	listTagsInput := aws_rds.ListTagsForResourceInput{
		ResourceName: &resourceArn,
	}
	tagsResp, err := svc.ListTagsForResource(c.Context, &listTagsInput)
	if err != nil {
		return false, fmt.Errorf("failed to list tags for RDS resource %s: %w", resourceArn, err)
	}
	allTagsFound := true
	for _, resourceTag := range tagsResp.TagList {
		tagFound := false
		for _, providedTag := range *tags {
			if *providedTag.Key == *resourceTag.Key && *providedTag.Value == *resourceTag.Value {
				tagFound = true
				break
			}
		}
		if !tagFound {
			allTagsFound = false
			break
		}
	}
	if allTagsFound {
		return true, nil
	}

	return false, nil
}
