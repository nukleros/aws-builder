package eks

import (
	"errors"
	"fmt"
	"strconv"

	aws_ec2 "github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/smithy-go"

	"github.com/nukleros/aws-builder/pkg/ec2"
)

// CreateElasticIps allocates elastic IP addresses for use by NAT gateways.
func (c *EksClient) CreateElasticIps(
	tags *[]types.Tag,
	azInventory *[]AvailabilityZoneInventory,
) ([]string, error) {
	svc := aws_ec2.NewFromConfig(*c.AwsConfig)

	var elasticIpIds []string

	// in order to uniquely reference EIPs created for an EKS cluster, we
	// add an ElasticIpRef tag with auto-incrementing IDs
	eipRefTagKey := "ElasticIpRef"
	eipRefTagValue := 1
	for _, az := range *azInventory {
		for _, _ = range az.PublicSubnets {
			eipTags := *tags
			tk := eipRefTagKey
			tv := strconv.Itoa(eipRefTagValue)
			eipRefTag := types.Tag{
				Key:   &tk,
				Value: &tv,
			}
			eipTags = append(eipTags, eipRefTag)
			eipRefTagValue++

			// because elastic IPs don't have unique names we have to check for
			// existing elastic IPs with matching tags up front
			eip, uniqueTagsExist, err := ec2.CheckUniqueTagsForElasticIp(c, &eipTags)
			if err != nil {
				return nil, fmt.Errorf("failed to check for unique tags on elastic IP: %w", err)
			}
			if uniqueTagsExist {
				elasticIpIds = append(elasticIpIds, *eip.AllocationId)
				continue
			}

			allocateAddressInput := aws_ec2.AllocateAddressInput{
				Domain: types.DomainTypeVpc,
				TagSpecifications: []types.TagSpecification{
					{
						ResourceType: types.ResourceTypeElasticIp,
						Tags:         eipTags,
					},
				},
			}
			resp, err := svc.AllocateAddress(c.Context, &allocateAddressInput)
			if err != nil {
				return elasticIpIds, fmt.Errorf("failed to create elastic IP: %w", err)
			}
			elasticIpIds = append(elasticIpIds, *resp.AllocationId)
		}
	}

	return elasticIpIds, nil
}

// DeleteElasticIps releases elastic IP addresses.  If no IDs are supplied, or
// if the address IDs are not found it exits without error.
func (c *EksClient) DeleteElasticIps(elasticIpIds []string) error {
	// if elasticIpiDs are empty, there's nothing to delete
	if len(elasticIpIds) == 0 {
		return nil
	}

	svc := aws_ec2.NewFromConfig(*c.AwsConfig)

	for _, elasticIpId := range elasticIpIds {
		deleteElasticIpInput := aws_ec2.ReleaseAddressInput{AllocationId: &elasticIpId}
		_, err := svc.ReleaseAddress(c.Context, &deleteElasticIpInput)
		if err != nil {
			var ae smithy.APIError
			if errors.As(err, &ae) {
				if ae.ErrorCode() == "InvalidAllocationID.NotFound" {
					// attempting to delete a elastic IP that doesn't exist so return
					// without error
					return nil
				} else {
					return fmt.Errorf("failed to delete elastic IP with ID %s: %w", elasticIpId, err)
				}
			} else {
				return fmt.Errorf("failed to delete elastic IP with ID %s: %w", elasticIpId, err)
			}
		}
	}

	return nil
}
