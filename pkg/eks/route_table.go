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

// CreatePublicRouteTable creates the route tables for the subnets used by the EKS
// cluster.  A single route table is shared by all the public subnets.
func (c *EksClient) CreatePublicRouteTable(
	tags *[]types.Tag,
	vpcId string,
	internetGatewayId string,
	azInventory *[]AvailabilityZoneInventory,
) (*types.RouteTable, error) {
	svc := aws_ec2.NewFromConfig(*c.AwsConfig)

	destinationCidr := "0.0.0.0/0"

	// collect public subnets from AZ inventory
	var publicSubnetIds []string
	for _, az := range *azInventory {
		for _, subnet := range az.PublicSubnets {
			if subnet.SubnetId != "" {
				publicSubnetIds = append(publicSubnetIds, subnet.SubnetId)
			}
		}
	}

	// in order to uniquely reference route tables created for an EKS cluster, we
	// add a PublicRouteTableRef tag
	publicRtRefTagKey := "PublicRouteTableRef"
	publicRtRefTagValue := "1"
	publicRtRefTag := types.Tag{
		Key:   &publicRtRefTagKey,
		Value: &publicRtRefTagValue,
	}
	publicRtTags := append(*tags, publicRtRefTag)

	// because route tables don't have unique names we have to check for
	// existing route tables with matching tags up front
	routeTables, uniqueTagsExist, err := ec2.CheckUniqueTagsForRouteTables(c, &publicRtTags)
	if err != nil {
		return nil, fmt.Errorf("failed to check for unique tags on route tables: %w", err)
	}
	if routeTables != nil && len(*routeTables) > 1 {
		return nil, errors.New("multiple route tables with matching tags found")
	}
	if uniqueTagsExist {
		rts := *routeTables

		// check route to internet gateway - create if missing
		igwFound := false
		for _, route := range rts[0].Routes {
			if route.GatewayId != nil && *route.GatewayId == internetGatewayId {
				igwFound = true
				break
			}
		}
		if !igwFound {
			if err := c.createRouteToInternetGateway(
				&rts[0],
				internetGatewayId,
				destinationCidr,
			); err != nil {
				return nil, err
			}
		}

		// check route table association with public subnets - create if missing
		for _, subnetId := range publicSubnetIds {
			associationFound := false
			for _, assoc := range rts[0].Associations {
				if assoc.SubnetId != nil && *assoc.SubnetId == subnetId {
					associationFound = true
					break
				}
			}
			if !associationFound {
				if err := c.associateRouteTableWithSubnet(
					*rts[0].RouteTableId,
					publicSubnetIds,
				); err != nil {
					return nil, err
				}
			}
		}

		return &rts[0], nil
	}

	// create a single route table for public subnets
	createPublicRouteTableInput := aws_ec2.CreateRouteTableInput{
		VpcId: &vpcId,
		TagSpecifications: []types.TagSpecification{
			{
				ResourceType: types.ResourceTypeRouteTable,
				Tags:         publicRtTags,
			},
		},
	}
	publicResp, err := svc.CreateRouteTable(c.Context, &createPublicRouteTableInput)
	if err != nil {
		return nil, fmt.Errorf("failed to create public route table for VPC ID %s: %w", vpcId, err)
	}

	// add route to internet gateway for public subnets' route table
	if err := c.createRouteToInternetGateway(
		publicResp.RouteTable,
		internetGatewayId,
		destinationCidr,
	); err != nil {
		return nil, err
	}

	// associate the public route table with the public subnet in each AZ
	if err := c.associateRouteTableWithSubnet(
		*publicResp.RouteTable.RouteTableId,
		publicSubnetIds,
	); err != nil {
		return nil, err
	}

	return publicResp.RouteTable, nil
}

// CreatePrivateRouteTables creates the route tables for the subnets used by the EKS
// cluster.  A single route table is shared by all the public subnets, however a
// separate route table is needed for each private subnet because they each get
// a route to a different NAT gateway.
func (c *EksClient) CreatePrivateRouteTables(
	tags *[]types.Tag,
	vpcId string,
	azInventory *[]AvailabilityZoneInventory,
) (*[]types.RouteTable, error) {
	svc := aws_ec2.NewFromConfig(*c.AwsConfig)

	destinationCidr := "0.0.0.0/0"

	var privateRouteTables []types.RouteTable

	// in order to uniquely reference route tables created for an EKS cluster, we
	// add a PublicRouteTableRef tag
	privateRtRefTagKey := "PrivateRouteTableRef"
	privateRtRefTagValue := 1

	// create a route table for each private subnet
	for _, az := range *azInventory {
		for _, privateSubnet := range az.PrivateSubnets {
			privateRtTags := *tags
			tk := privateRtRefTagKey
			tv := strconv.Itoa(privateRtRefTagValue)
			privateRtRefTag := types.Tag{
				Key:   &tk,
				Value: &tv,
			}
			privateRtTags = append(privateRtTags, privateRtRefTag)
			privateRtRefTagValue++

			// because route tables don't have unique names we have to check for
			// existing route tables with matching tags up front
			routeTables, uniqueTagsExist, err := ec2.CheckUniqueTagsForRouteTables(c, &privateRtTags)
			if err != nil {
				return nil, fmt.Errorf("failed to check for unique tags on route tables: %w", err)
			}
			if routeTables != nil && len(*routeTables) > 1 {
				return nil, errors.New("multiple route tables with matching tags found")
			}
			if uniqueTagsExist {
				rts := *routeTables

				// check to ensure route table is associated with private subnet
				associationFound := false
				for _, assoc := range rts[0].Associations {
					if assoc.SubnetId != nil && *assoc.SubnetId == privateSubnet.SubnetId {
						associationFound = true
						break
					}
				}
				if !associationFound {
					if err := c.associateRouteTableWithSubnet(
						*rts[0].RouteTableId,
						[]string{privateSubnet.SubnetId},
					); err != nil {
						return nil, err
					}
				}

				// check to ensure route table has a route to the NAT gateway
				natGatewayFound := false
				for _, route := range rts[0].Routes {
					if route.NatGatewayId != nil && *route.NatGatewayId == az.NatGatewayId {
						natGatewayFound = true
						break
					}
				}
				if !natGatewayFound {
					if err := c.createRouteToNatGateway(
						&rts[0],
						az.NatGatewayId,
						destinationCidr,
					); err != nil {
						return nil, err
					}
				}

				privateRouteTables = append(privateRouteTables, rts[0])
				continue
			}

			createPrivateRouteTableInput := aws_ec2.CreateRouteTableInput{
				VpcId: &vpcId,
				TagSpecifications: []types.TagSpecification{
					{
						ResourceType: types.ResourceTypeRouteTable,
						Tags:         privateRtTags,
					},
				},
			}

			privateResp, err := svc.CreateRouteTable(c.Context, &createPrivateRouteTableInput)
			if err != nil {
				return &privateRouteTables, fmt.Errorf("failed to create private route table for VPC ID %s: %w", vpcId, err)
			}

			// associate the private route table with the private subnet for this
			// availability zone
			if err := c.associateRouteTableWithSubnet(
				*privateResp.RouteTable.RouteTableId,
				[]string{privateSubnet.SubnetId},
			); err != nil {
				return nil, err
			}

			// add a route to the NAT gateway for the private subnet
			if err := c.createRouteToNatGateway(
				privateResp.RouteTable,
				az.NatGatewayId,
				destinationCidr,
			); err != nil {
				return nil, err
			}

			privateRouteTables = append(privateRouteTables, *privateResp.RouteTable)
		}
	}

	return &privateRouteTables, nil
}

// DeleteRouteTables deletes the route tables for the public and private subnets
// that are used by EKS.
func (c *EksClient) DeleteRouteTables(privateRouteTableIds []string, publicRouteTable string) error {
	svc := aws_ec2.NewFromConfig(*c.AwsConfig)

	var allRouteTableIds []string
	switch {
	case len(privateRouteTableIds) == 0 && publicRouteTable == "":
		// if route table IDs are empty, there's nothing to delete
		return nil
	case publicRouteTable == "":
		// don't want to add an empty string to slice of route table IDs
		allRouteTableIds = privateRouteTableIds
	default:
		// there are private and public route table IDs to delete
		allRouteTableIds = append(privateRouteTableIds, publicRouteTable)
	}

	for _, routeTableId := range allRouteTableIds {
		deleteRouteTableInput := aws_ec2.DeleteRouteTableInput{RouteTableId: &routeTableId}
		_, err := svc.DeleteRouteTable(c.Context, &deleteRouteTableInput)
		if err != nil {
			var ae smithy.APIError
			if errors.As(err, &ae) {
				if ae.ErrorCode() == "InvalidRouteTableID.NotFound" {
					// attempting to delete a route table that doesn't exist so return
					// without error
					return nil
				} else {
					return fmt.Errorf("failed to delete route table with ID %s: %w", routeTableId, err)
				}
			} else {
				return fmt.Errorf("failed to delete route table with ID %s: %w", routeTableId, err)
			}
		}
	}

	return nil
}

// createRouteToInternetGateway adds a route to an internet gateway for a route
// table.
func (c *EksClient) createRouteToInternetGateway(
	routeTable *types.RouteTable,
	internetGatewayId string,
	destinationCidr string,
) error {
	svc := aws_ec2.NewFromConfig(*c.AwsConfig)

	createRouteInput := aws_ec2.CreateRouteInput{
		RouteTableId:         routeTable.RouteTableId,
		GatewayId:            &internetGatewayId,
		DestinationCidrBlock: &destinationCidr,
	}
	_, err := svc.CreateRoute(c.Context, &createRouteInput)
	if err != nil {
		return fmt.Errorf(
			"failed to create route to internet gateway with ID %s for route table with ID %s: %w",
			internetGatewayId, *routeTable.RouteTableId, err,
		)
	}

	return nil
}

// createRouteToNatGateway adds a route to an NAT gateway for a route
// table.
func (c *EksClient) createRouteToNatGateway(
	routeTable *types.RouteTable,
	natGatewayId string,
	destinationCidr string,
) error {
	svc := aws_ec2.NewFromConfig(*c.AwsConfig)

	createRouteInput := aws_ec2.CreateRouteInput{
		RouteTableId:         routeTable.RouteTableId,
		NatGatewayId:         &natGatewayId,
		DestinationCidrBlock: &destinationCidr,
	}
	_, err := svc.CreateRoute(c.Context, &createRouteInput)
	if err != nil {
		return fmt.Errorf(
			"failed to create route to NAT gateway with ID %s for route table with ID %s: %w",
			natGatewayId, *routeTable.RouteTableId, err,
		)
	}

	return nil
}

// associateRouteTableWithSubnet takes a route table ID and a slice of subnet
// IDs and associates the route table with each subnet.
func (c *EksClient) associateRouteTableWithSubnet(
	routeTableId string,
	subnetIds []string,
) error {
	svc := aws_ec2.NewFromConfig(*c.AwsConfig)

	for _, subnetId := range subnetIds {
		associatePublicRouteTableInput := aws_ec2.AssociateRouteTableInput{
			RouteTableId: &routeTableId,
			SubnetId:     &subnetId,
		}
		_, err := svc.AssociateRouteTable(c.Context, &associatePublicRouteTableInput)
		if err != nil {
			return fmt.Errorf(
				"failed to associate route table with ID %s to subnet with ID %s: %w",
				routeTableId, subnetId, err,
			)
		}
	}

	return nil
}
