package eks

import (
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

const maxAzCount = int32(3)

// defaultCidrs returns a set of Cidr blocks for subnets.
func defaultCidrs() []string {
	return []string{
		"10.0.0.0/22",
		"10.0.4.0/22",
		"10.0.8.0/22",
		"10.0.12.0/22",
		"10.0.16.0/22",
		"10.0.20.0/22",
	}
}

// SetAvailabilityZones sets the number of availability zones if not provided
// and assigns CIDR blocks for a public and private subnet in each zone.  If
// availability zone config is provided by client this function returns without
// doing anything.
func (c *EksClient) SetAvailabilityZones(
	region string,
	desiredAzCount int32,
	azConfig *[]AvailabilityZoneConfig,
) (*[]AvailabilityZoneInventory, error) {
	// ensure region is in resource config
	if region == "" {
		return nil, errors.New("region is not set in resource config")
	}

	// if availability zones provided in config set the availability zone
	// inventory and return that
	if len(*azConfig) > 0 {
		var availabilityZones []AvailabilityZoneInventory
		for _, az := range *azConfig {
			az := AvailabilityZoneInventory{
				Zone: az.Zone,
				PublicSubnets: []SubnetInventory{
					{
						SubnetCidr: az.PublicSubnetCidr,
					},
				},
				PrivateSubnets: []SubnetInventory{
					{
						SubnetCidr: az.PrivateSubnetCidr,
					},
				},
			}
			availabilityZones = append(availabilityZones, az)
		}
		return &availabilityZones, nil
	}

	// no explicit availability zone config provided
	// default to 2 availability zones if not specified
	var desiredAZs int32
	if desiredAzCount == 0 {
		desiredAZs = 2
	} else {
		desiredAZs = desiredAzCount
	}

	// get availability zones from AWS and set CIDRs for the client
	availabilityZones, err := c.SetupAvailabilityZoneForRegion(
		region,
		desiredAZs,
		defaultCidrs(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get availability zones for region %s: %w", region, err)
	}

	return availabilityZones, nil
}

// SetupAvailabilityZoneForRegion gets the availability zones for a given region
// and assigns CIDR blocks to the public and private subnets for each AZ.
func (c *EksClient) SetupAvailabilityZoneForRegion(
	region string,
	desiredAZs int32,
	cidrBlocks []string,
) (*[]AvailabilityZoneInventory, error) {
	svc := ec2.NewFromConfig(*c.AwsConfig)
	var availabilityZones []AvailabilityZoneInventory

	filterName := "region-name"
	describeAZInput := ec2.DescribeAvailabilityZonesInput{
		Filters: []types.Filter{
			{
				Name:   &filterName,
				Values: []string{region},
			},
		},
	}
	resp, err := svc.DescribeAvailabilityZones(c.Context, &describeAZInput)
	if err != nil {
		return &availabilityZones, fmt.Errorf("failed to describe availability zones for region %s: %w", region, err)
	}

	azsSet := int32(0)
	var azCount int32
	if desiredAZs > maxAzCount {
		azCount = maxAzCount
	} else {
		azCount = desiredAZs
	}
	cidrIndex := 0
	for _, az := range resp.AvailabilityZones {
		if azsSet < azCount {
			newAz := AvailabilityZoneInventory{
				Zone: *az.ZoneName,
				PublicSubnets: []SubnetInventory{
					{
						SubnetCidr: cidrBlocks[cidrIndex],
					},
				},
				PrivateSubnets: []SubnetInventory{
					{
						SubnetCidr: cidrBlocks[cidrIndex+1],
					},
				},
			}
			availabilityZones = append(availabilityZones, newAz)
			cidrIndex += 2
			azsSet++
		} else {
			break
		}
	}

	return &availabilityZones, nil
}
