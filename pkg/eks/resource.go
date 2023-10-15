package eks

import (
	"fmt"

	"github.com/nukleros/aws-builder/pkg/ec2"
	"github.com/nukleros/aws-builder/pkg/iam"
	"github.com/nukleros/aws-builder/pkg/util"
)

// CreateResourceStack creates all the resources for an EKS cluster.  If
// inventory for pre-existing resources are provided, it will not re-create
// those resources but instead use them as a part of the stack.
func (c *EksClient) CreateEksResourceStack(
	resourceConfig *EksConfig,
	inventory *EksInventory,
) error {
	// create an empty inventory object to refer to if nil
	if inventory == nil {
		inventory = &EksInventory{}
	}

	// return an error if resource config and inventory regions do not match
	// inventory.Region and resourceConfig.Region can both be empty strings in
	// which case the user's default region will be used according their local
	// AWS config
	if inventory.Region != "" && inventory.Region != resourceConfig.Region {
		return fmt.Errorf(
			"config region %s and inventory region %s do not match",
			resourceConfig.Region,
			inventory.Region,
		)
	}

	// resource config region takes precedence
	// if not set, use the region defined in AWS config
	if resourceConfig.Region != "" {
		inventory.Region = resourceConfig.Region
		c.AwsConfig.Region = resourceConfig.Region
	} else {
		inventory.Region = c.AwsConfig.Region
		resourceConfig.Region = c.AwsConfig.Region
	}

	// Tags
	ec2Tags := ec2.CreateEc2Tags(resourceConfig.Name, resourceConfig.Tags)
	iamTags := iam.CreateIamTags(resourceConfig.Name, resourceConfig.Tags)
	mapTags := util.CreateMapTags(resourceConfig.Name, resourceConfig.Tags)

	// Availability Zones
	if len(inventory.AvailabilityZones) == 0 {
		azInventory, err := c.SetAvailabilityZones(
			resourceConfig.Region,
			resourceConfig.DesiredAzCount,
			&resourceConfig.AvailabilityZones,
		)
		if azInventory != nil {
			inventory.AvailabilityZones = *azInventory
			inventory.send(c.InventoryChan)
		}
		if err != nil {
			return err
		}
		c.SendMessage("Availability zones set up")
	} else {
		c.SendMessage("Availability zones found in inventory")
	}

	// VPC
	if inventory.VpcId == "" {
		vpc, err := c.CreateVpc(
			ec2Tags,
			resourceConfig.ClusterCidr,
			resourceConfig.Name,
		)
		if vpc != nil {
			inventory.VpcId = *vpc.VpcId
			inventory.send(c.InventoryChan)
		}
		if err != nil {
			return err
		}
		c.SendMessage(fmt.Sprintf("VPC created: %s", *vpc.VpcId))
	} else {
		c.SendMessage(fmt.Sprintf("VPC found in inventory: %s", inventory.VpcId))
	}

	// Internet Gateway
	if inventory.InternetGatewayId == "" {
		igw, err := c.CreateInternetGateway(
			ec2Tags,
			inventory.VpcId,
			resourceConfig.Name,
		)
		if igw != nil {
			inventory.InternetGatewayId = *igw.InternetGatewayId
			inventory.send(c.InventoryChan)
		}
		if err != nil {
			return err
		}
		c.SendMessage(fmt.Sprintf("Internet gateway created: %s", *igw.InternetGatewayId))
	} else {
		c.SendMessage(fmt.Sprintf("Internet gateway found in inventory: %s", inventory.InternetGatewayId))
	}

	// Public Subnets
	var inventoryPublicSubnetIds []string
	allPublicSubnetsFound := true
	for _, az := range inventory.AvailabilityZones {
		for _, subnet := range az.PublicSubnets {
			if subnet.SubnetId != "" {
				inventoryPublicSubnetIds = append(inventoryPublicSubnetIds, subnet.SubnetId)
			} else {
				allPublicSubnetsFound = false
			}
		}
	}
	if !allPublicSubnetsFound {
		azInventory, publicSubnetIds, err := c.CreatePublicSubnets(
			ec2Tags,
			inventory.VpcId,
			resourceConfig.Name,
			&inventory.AvailabilityZones,
		)
		if azInventory != nil {
			inventory.AvailabilityZones = *azInventory
			inventory.send(c.InventoryChan)
		}
		if err != nil {
			return err
		}
		c.SendMessage(fmt.Sprintf("Public subnets created: %s", publicSubnetIds))
	} else {
		c.SendMessage(fmt.Sprintf("Public subnets found in inventory: %s", inventoryPublicSubnetIds))
	}

	// Private Subnets
	var inventoryPrivateSubnetIds []string
	allPrivateSubnetsFound := true
	for _, az := range inventory.AvailabilityZones {
		for _, subnet := range az.PrivateSubnets {
			if subnet.SubnetId != "" {
				inventoryPrivateSubnetIds = append(inventoryPrivateSubnetIds, subnet.SubnetId)
			} else {
				allPrivateSubnetsFound = false
			}
		}
	}
	if !allPrivateSubnetsFound {
		var privateSubnetIds []string
		azInventory, privateSubnetIds, err := c.CreatePrivateSubnets(
			ec2Tags,
			inventory.VpcId,
			resourceConfig.Name,
			&inventory.AvailabilityZones,
		)
		if azInventory != nil {
			inventory.AvailabilityZones = *azInventory
			inventory.send(c.InventoryChan)
		}
		if err != nil {
			return err
		}
		c.SendMessage(fmt.Sprintf("Private subnets created: %s", privateSubnetIds))
	} else {
		c.SendMessage(fmt.Sprintf("Private subnets found in inventory: %s", inventoryPrivateSubnetIds))
	}

	// Elastic IPs
	if len(inventory.ElasticIpIds) == 0 {
		elasticIpIds, err := c.CreateElasticIps(
			ec2Tags,
			&inventory.AvailabilityZones,
		)
		inventory.ElasticIpIds = elasticIpIds
		inventory.send(c.InventoryChan)
		if err != nil {
			return err
		}
		c.SendMessage(fmt.Sprintf("Elastic IPs created: %s", elasticIpIds))
	} else {
		c.SendMessage(fmt.Sprintf("Elastic IPs found in inventory: %s", inventory.ElasticIpIds))
	}

	// NAT Gateways
	var inventoryNatGatewayIds []string
	allNatGatewaysFound := true
	for _, az := range inventory.AvailabilityZones {
		if az.NatGatewayId != "" {
			inventoryNatGatewayIds = append(inventoryNatGatewayIds, az.NatGatewayId)
		} else {
			allNatGatewaysFound = false
		}
	}
	if !allNatGatewaysFound {
		if err := c.CreateNatGateways(
			ec2Tags,
			&inventory.AvailabilityZones,
			inventory.ElasticIpIds,
		); err != nil {
			return err
		}
		c.SendMessage("NAT gateways created")
		c.SendMessage("Waiting for NAT gateways to become active")
		updatedAzInventory, natGatewayIds, err := c.WaitForNatGateways(
			inventory.VpcId,
			&inventory.AvailabilityZones,
			NatGatewayConditionCreated,
		)
		if updatedAzInventory != nil {
			inventory.AvailabilityZones = *updatedAzInventory
			inventory.send(c.InventoryChan)
		}
		if err != nil {
			return err
		}
		c.SendMessage(fmt.Sprintf("NAT gateways ready: %s", natGatewayIds))
	} else {
		c.SendMessage(fmt.Sprintf("NAT gateways found in inventory: %s", inventoryNatGatewayIds))
	}

	// Public Route Table
	if inventory.PublicRouteTableId == "" {
		publicRouteTable, err := c.CreatePublicRouteTable(
			ec2Tags,
			inventory.VpcId,
			inventory.InternetGatewayId,
			&inventory.AvailabilityZones,
		)
		if publicRouteTable != nil {
			inventory.PublicRouteTableId = *publicRouteTable.RouteTableId
			inventory.send(c.InventoryChan)
		}
		if err != nil {
			return err
		}
		c.SendMessage(fmt.Sprintf("Public route table created: %s", *publicRouteTable.RouteTableId))
	} else {
		c.SendMessage(fmt.Sprintf("Public route table found in inventory: %s", inventory.PublicRouteTableId))
	}

	// Private Route Tables
	if len(inventory.PrivateRouteTableIds) == 0 {
		var privateRouteTableIds []string
		privateRouteTables, err := c.CreatePrivateRouteTables(
			ec2Tags,
			inventory.VpcId,
			&inventory.AvailabilityZones,
		)
		if privateRouteTables != nil {
			for _, rt := range *privateRouteTables {
				privateRouteTableIds = append(privateRouteTableIds, *rt.RouteTableId)
			}
			inventory.PrivateRouteTableIds = privateRouteTableIds
			inventory.send(c.InventoryChan)
		}
		if err != nil {
			return err
		}
		c.SendMessage(fmt.Sprintf("Private route tables created: %s", privateRouteTableIds))
	} else {
		c.SendMessage(fmt.Sprintf("Private route tables found in inventory: %s", inventory.PrivateRouteTableIds))
	}

	// IAM Role for cluster
	if inventory.ClusterRole.RoleName == "" {
		clusterRole, err := c.CreateClusterRole(
			iamTags,
			resourceConfig.Name,
		)
		if clusterRole != nil {
			inventory.ClusterRole = RoleInventory{
				RoleName:       *clusterRole.RoleName,
				RoleArn:        *clusterRole.Arn,
				RolePolicyArns: []string{ClusterPolicyArn},
			}
			inventory.send(c.InventoryChan)
		}
		if err != nil {
			return err
		}
		c.SendMessage(fmt.Sprintf("IAM role for cluster created: %s", *clusterRole.RoleName))
	} else {
		c.SendMessage(fmt.Sprintf("IAM role for cluster found in inventory: %s", inventory.ClusterRole.RoleName))
	}

	// IAM Role for worker nodes
	if inventory.WorkerRole.RoleName == "" {
		nodeRole, err := c.CreateNodeRole(
			iamTags,
			resourceConfig.Name,
		)
		if nodeRole != nil {
			inventory.WorkerRole = RoleInventory{
				RoleName:       *nodeRole.RoleName,
				RoleArn:        *nodeRole.Arn,
				RolePolicyArns: getWorkerPolicyArns(),
			}
			inventory.send(c.InventoryChan)
		}
		if err != nil {
			return err
		}
		c.SendMessage(fmt.Sprintf("IAM role for worker nodes created: %s", *nodeRole.RoleName))
	} else {
		c.SendMessage(fmt.Sprintf("IAM role for worker nodes found in inventory: %s", inventory.WorkerRole.RoleName))
	}

	// EKS Cluster
	if inventory.Cluster.ClusterName == "" {
		cluster, err := c.CreateCluster(
			&mapTags,
			resourceConfig.Name,
			resourceConfig.KubernetesVersion,
			inventory.ClusterRole.RoleArn,
			&inventory.AvailabilityZones,
		)
		if cluster != nil {
			inventory.Cluster.ClusterName = *cluster.Name
			inventory.Cluster.ClusterArn = *cluster.Arn
			inventory.send(c.InventoryChan)
		}
		if err != nil {
			return err
		}
		c.SendMessage(fmt.Sprintf("EKS cluster created: %s", *cluster.Name))
	} else {
		c.SendMessage(fmt.Sprintf("EKS cluster found in inventory: %s", inventory.Cluster.ClusterName))
	}
	if inventory.Cluster.OidcProviderUrl == "" {
		c.SendMessage(fmt.Sprintf("Waiting for EKS cluster to become active: %s", inventory.Cluster.ClusterName))
		oidcIssuer, err := c.WaitForCluster(
			inventory.Cluster.ClusterName,
			ClusterConditionCreated,
		)
		if oidcIssuer != "" {
			inventory.Cluster.OidcProviderUrl = oidcIssuer
			inventory.send(c.InventoryChan)
		}
		if err != nil {
			return err
		}
		c.SendMessage(fmt.Sprintf("EKS cluster ready: %s", inventory.Cluster.ClusterName))
	} else {
		c.SendMessage(fmt.Sprintf("EKS cluster found in inventory is ready: %s", inventory.Cluster.ClusterName))
	}

	// EKS Cluster Security Group
	if inventory.SecurityGroupId == "" {
		securityGroupId, err := c.GetClusterSecurityGroup(resourceConfig.Name)
		if securityGroupId != "" {
			inventory.SecurityGroupId = securityGroupId
			inventory.send(c.InventoryChan)
		}
		if err != nil {
			return err
		}
		c.SendMessage(fmt.Sprintf("EKS cluster security group ID %s retrieved", securityGroupId))
	} else {
		c.SendMessage(fmt.Sprintf("EKS cluster security group ID %s found in inventory", inventory.SecurityGroupId))
	}

	// Node Groups
	if len(inventory.NodeGroupNames) == 0 {
		var nodeGroupNames []string
		nodeGroups, err := c.CreateNodeGroups(
			&mapTags,
			inventory.Cluster.ClusterName,
			resourceConfig.KubernetesVersion,
			inventory.WorkerRole.RoleArn,
			&inventory.AvailabilityZones,
			resourceConfig.InstanceTypes,
			resourceConfig.InitialNodes,
			resourceConfig.MinNodes,
			resourceConfig.MaxNodes,
			resourceConfig.KeyPair,
		)
		if nodeGroups != nil {
			for _, nodeGroup := range *nodeGroups {
				nodeGroupNames = append(nodeGroupNames, *nodeGroup.NodegroupName)
			}
			inventory.NodeGroupNames = nodeGroupNames
			inventory.send(c.InventoryChan)
		}
		if err != nil {
			return err
		}
		c.SendMessage(fmt.Sprintf("EKS node groups created: %s", nodeGroupNames))
		c.SendMessage(fmt.Sprintf("Waiting for EKS node groups to become active: %s", nodeGroupNames))
		if err := c.WaitForNodeGroups(
			inventory.Cluster.ClusterName,
			nodeGroupNames,
			NodeGroupConditionCreated,
		); err != nil {
			return err
		}
		c.SendMessage(fmt.Sprintf("EKS node groups ready: %s", nodeGroupNames))
	} else {
		c.SendMessage(fmt.Sprintf("EKS node groups found in inventory: %s", inventory.NodeGroupNames))
	}

	// OIDC Provider
	if inventory.OidcProviderArn == "" {
		oidcProviderArn, err := c.CreateOidcProvider(
			iamTags,
			inventory.Cluster.OidcProviderUrl,
		)
		if oidcProviderArn != "" {
			inventory.OidcProviderArn = oidcProviderArn
			inventory.send(c.InventoryChan)
		}
		if err != nil {
			return err
		}
		c.SendMessage(fmt.Sprintf("OIDC provider created: %s", oidcProviderArn))
	} else {
		c.SendMessage(fmt.Sprintf("OIDC provider found in inventory: %s", inventory.OidcProviderArn))
	}

	// IAM Policy for DNS Management
	if resourceConfig.DnsManagement {
		if len(inventory.DnsManagementRole.RolePolicyArns) == 0 {
			dnsPolicy, err := c.CreateDnsManagementPolicy(
				iamTags,
				resourceConfig.Name,
			)
			if dnsPolicy != nil {
				inventory.PolicyArns = append(inventory.PolicyArns, *dnsPolicy.Arn)
				inventory.DnsManagementRole = RoleInventory{
					RolePolicyArns: []string{*dnsPolicy.Arn},
				}
				inventory.send(c.InventoryChan)
			}
			if err != nil {
				return err
			}
			c.SendMessage(fmt.Sprintf("IAM policy created: %s", *dnsPolicy.PolicyName))
		} else {
			c.SendMessage(fmt.Sprintf("IAM policy found in inventory: %s", inventory.DnsManagementRole.RolePolicyArns))
		}
	} else {
		c.SendMessage("IAM policy for DNS management not requested")
	}

	// IAM Role for DNS Management
	if resourceConfig.DnsManagement {
		if inventory.DnsManagementRole.RoleName == "" {
			if len(inventory.DnsManagementRole.RolePolicyArns) != 1 {
				return fmt.Errorf("expected 1 policy for DNS management role but found %d in inventory", len(inventory.DnsManagementRole.RolePolicyArns))
			}
			dnsManagementRole, err := c.CreateDnsManagementRole(
				iamTags,
				inventory.DnsManagementRole.RolePolicyArns[0],
				resourceConfig.AwsAccountId,
				inventory.Cluster.OidcProviderUrl,
				&resourceConfig.DnsManagementServiceAccount,
				resourceConfig.Name,
			)
			if dnsManagementRole != nil {
				inventory.DnsManagementRole.RoleName = *dnsManagementRole.RoleName
				inventory.DnsManagementRole.RoleArn = *dnsManagementRole.Arn
				inventory.send(c.InventoryChan)
			}
			if err != nil {
				return err
			}
			c.SendMessage(fmt.Sprintf("IAM role for DNS management created: %s", *dnsManagementRole.RoleName))
		} else {
			c.SendMessage(fmt.Sprintf("IAM role for DNS management found in inventory: %s", inventory.DnsManagementRole.RoleName))
		}
	} else {
		c.SendMessage("IAM role for DNS management not requested")
	}

	// IAM Policy for DNS01 Challenge
	if resourceConfig.Dns01Challenge {
		if len(inventory.Dns01ChallengeRole.RolePolicyArns) == 0 {
			dns01ChallengePolicy, err := c.CreateDns01ChallengePolicy(
				iamTags,
				resourceConfig.Name,
			)
			if dns01ChallengePolicy != nil {
				inventory.PolicyArns = append(inventory.PolicyArns, *dns01ChallengePolicy.Arn)
				inventory.Dns01ChallengeRole = RoleInventory{
					RolePolicyArns: []string{*dns01ChallengePolicy.Arn},
				}
				inventory.send(c.InventoryChan)
			}
			if err != nil {
				return err
			}
			c.SendMessage(fmt.Sprintf("IAM policy created: %s", *dns01ChallengePolicy.PolicyName))
		} else {
			c.SendMessage(fmt.Sprintf("IAM policy found in inventory: %s", inventory.Dns01ChallengeRole.RolePolicyArns))
		}
	} else {
		c.SendMessage("IAM policy for DNS01 challenge not requested")
	}

	// IAM Role for DNS01 Challenges
	if resourceConfig.Dns01Challenge {
		if inventory.Dns01ChallengeRole.RoleName == "" {
			if len(inventory.Dns01ChallengeRole.RolePolicyArns) != 1 {
				return fmt.Errorf("expected 1 policy for DNS01 challenge role but found %d in inventory", len(inventory.Dns01ChallengeRole.RolePolicyArns))
			}
			dns01ChallengeRole, err := c.CreateDns01ChallengeRole(
				iamTags,
				inventory.Dns01ChallengeRole.RolePolicyArns[0],
				resourceConfig.AwsAccountId,
				inventory.Cluster.OidcProviderUrl,
				&resourceConfig.Dns01ChallengeServiceAccount,
				resourceConfig.Name,
			)
			if dns01ChallengeRole != nil {
				inventory.Dns01ChallengeRole.RoleName = *dns01ChallengeRole.RoleName
				inventory.Dns01ChallengeRole.RoleArn = *dns01ChallengeRole.Arn
				inventory.send(c.InventoryChan)
			}
			if err != nil {
				return err
			}
			c.SendMessage(fmt.Sprintf("IAM role for DNS01 challenges created: %s", *dns01ChallengeRole.RoleName))
		} else {
			c.SendMessage(fmt.Sprintf("IAM role for DNS01 challenges found in inventory: %s", inventory.Dns01ChallengeRole.RolePolicyArns))
		}
	} else {
		c.SendMessage("IAM role for DNS01 challenge not requested")
	}

	// IAM Policy for Cluster Autoscaling
	if resourceConfig.ClusterAutoscaling {
		if len(inventory.ClusterAutoscalingRole.RolePolicyArns) == 0 {
			clusterAutoscalingPolicy, err := c.CreateClusterAutoscalingPolicy(
				iamTags,
				resourceConfig.Name,
			)
			if clusterAutoscalingPolicy != nil {
				inventory.PolicyArns = append(inventory.PolicyArns, *clusterAutoscalingPolicy.Arn)
				inventory.ClusterAutoscalingRole = RoleInventory{
					RolePolicyArns: []string{*clusterAutoscalingPolicy.Arn},
				}
				inventory.send(c.InventoryChan)
			}
			if err != nil {
				return err
			}
			c.SendMessage(fmt.Sprintf("IAM policy created: %s", *clusterAutoscalingPolicy.PolicyName))
		} else {
			c.SendMessage(fmt.Sprintf("IAM policy found in inventory: %s", inventory.ClusterAutoscalingRole.RolePolicyArns))
		}
	} else {
		c.SendMessage("IAM policy for cluster autoscaling not requested")
	}

	// IAM Role for Cluster Autoscaling
	if resourceConfig.ClusterAutoscaling {
		if inventory.ClusterAutoscalingRole.RoleName == "" {
			if len(inventory.ClusterAutoscalingRole.RolePolicyArns) != 1 {
				return fmt.Errorf("expected 1 policy for cluster autoscaling role but found %d in inventory", len(inventory.ClusterAutoscalingRole.RolePolicyArns))
			}
			clusterAutoscalingRole, err := c.CreateClusterAutoscalingRole(
				iamTags,
				inventory.ClusterAutoscalingRole.RolePolicyArns[0],
				resourceConfig.AwsAccountId,
				inventory.Cluster.OidcProviderUrl,
				&resourceConfig.ClusterAutoscalingServiceAccount,
				resourceConfig.Name,
			)
			if clusterAutoscalingRole != nil {
				inventory.ClusterAutoscalingRole = RoleInventory{
					RoleName: *clusterAutoscalingRole.RoleName,
					RoleArn:  *clusterAutoscalingRole.Arn,
				}
				inventory.send(c.InventoryChan)
			}
			if err != nil {
				return err
			}
			c.SendMessage(fmt.Sprintf("IAM role for cluster autoscaling created: %s", *clusterAutoscalingRole.RoleName))
		} else {
			c.SendMessage(fmt.Sprintf("IAM role for cluster autoscaling found in inventory: %s", inventory.ClusterAutoscalingRole.RoleName))
		}
	} else {
		c.SendMessage("IAM role for cluster autoscaling not requested")
	}

	// IAM Role for Storage Management
	if inventory.StorageManagementRole.RoleName == "" {
		storageManagementRole, err := c.CreateStorageManagementRole(
			iamTags,
			resourceConfig.AwsAccountId,
			inventory.Cluster.OidcProviderUrl,
			&resourceConfig.StorageManagementServiceAccount,
			resourceConfig.Name,
		)
		if storageManagementRole != nil {
			inventory.StorageManagementRole = RoleInventory{
				RoleName:       *storageManagementRole.RoleName,
				RoleArn:        *storageManagementRole.Arn,
				RolePolicyArns: []string{CsiDriverPolicyArn},
			}
			inventory.send(c.InventoryChan)
		}
		if err != nil {
			return err
		}
		c.SendMessage(fmt.Sprintf("IAM role for storage management created: %s", *storageManagementRole.RoleName))
	} else {
		c.SendMessage(fmt.Sprintf("IAM role for storage management found in inventory: %s", inventory.StorageManagementRole.RoleName))
	}

	// EBS CSI Addon
	ebsStorageAddonName, err := c.CreateEbsStorageAddon(
		&mapTags,
		inventory.Cluster.ClusterName,
		inventory.StorageManagementRole.RoleArn,
	)
	if err != nil {
		return err
	}
	c.SendMessage(fmt.Sprintf("EBS storage addon created: %s", ebsStorageAddonName))

	c.SendMessage(fmt.Sprintf("EKS cluster creation complete: %s", inventory.Cluster.ClusterName))

	return nil
}

// DeleteResourceStack deletes all the resources in the resource inventory.
func (c *EksClient) DeleteEksResourceStack(inventory *EksInventory) error {
	c.AwsConfig.Region = inventory.Region

	// OIDC Provider
	if err := c.DeleteOidcProvider(inventory.OidcProviderArn); err != nil {
		return err
	}
	c.SendMessage(fmt.Sprintf("OIDC provider deleted: %s", inventory.OidcProviderArn))
	inventory.OidcProviderArn = ""
	inventory.send(c.InventoryChan)

	// Node Groups
	if err := c.DeleteNodeGroups(inventory.Cluster.ClusterName, inventory.NodeGroupNames); err != nil {
		return err
	}
	c.SendMessage(fmt.Sprintf("Node groups deletion initiated: %s", inventory.NodeGroupNames))
	c.SendMessage(fmt.Sprintf("Waiting for node groups to be deleted: %s", inventory.NodeGroupNames))
	if err := c.WaitForNodeGroups(inventory.Cluster.ClusterName, inventory.NodeGroupNames, NodeGroupConditionDeleted); err != nil {
		return err
	}
	c.SendMessage(fmt.Sprintf("Node groups deletion complete: %s", inventory.NodeGroupNames))
	inventory.NodeGroupNames = []string{}
	inventory.send(c.InventoryChan)

	// EKS Cluster
	if err := c.DeleteCluster(inventory.Cluster.ClusterName); err != nil {
		return err
	}
	c.SendMessage(fmt.Sprintf("EKS cluster deletion initiated: %s", inventory.Cluster.ClusterName))
	c.SendMessage(fmt.Sprintf("Waiting for EKS cluster to be deleted: %s", inventory.Cluster.ClusterName))
	if _, err := c.WaitForCluster(inventory.Cluster.ClusterName, ClusterConditionDeleted); err != nil {
		return err
	}
	c.SendMessage(fmt.Sprintf("EKS cluster deletion complete: %s", inventory.Cluster.ClusterName))
	inventory.Cluster = ClusterInventory{}
	inventory.send(c.InventoryChan)

	// IAM Roles
	iamRoles := []RoleInventory{
		inventory.ClusterRole,
		inventory.WorkerRole,
		inventory.DnsManagementRole,
		inventory.Dns01ChallengeRole,
		inventory.ClusterAutoscalingRole,
		inventory.StorageManagementRole,
	}
	if err := c.DeleteRoles(&iamRoles); err != nil {
		return err
	}
	c.SendMessage(fmt.Sprintf("IAM roles deleted: %s", iamRoles))
	inventory.ClusterRole = RoleInventory{}
	inventory.WorkerRole = RoleInventory{}
	inventory.DnsManagementRole = RoleInventory{}
	inventory.Dns01ChallengeRole = RoleInventory{}
	inventory.ClusterAutoscalingRole = RoleInventory{}
	inventory.StorageManagementRole = RoleInventory{}
	inventory.send(c.InventoryChan)

	// IAM Policies
	policyArns, err := c.DeletePolicies(inventory.PolicyArns)
	if err != nil {
		return err
	}
	c.SendMessage(fmt.Sprintf("IAM policies deleted: %s", policyArns))
	inventory.PolicyArns = []string{}
	inventory.send(c.InventoryChan)

	// NAT Gateways
	natGatewayIds, err := c.DeleteNatGateways(&inventory.AvailabilityZones)
	if err != nil {
		return err
	}
	c.SendMessage(fmt.Sprintf("NAT gateway deletion initiated: %s", natGatewayIds))
	c.SendMessage("Waiting for NAT gateways to be deleted")
	updatedAzInventory, natGatewayIds, err := c.WaitForNatGateways(
		inventory.VpcId,
		&inventory.AvailabilityZones,
		NatGatewayConditionDeleted,
	)
	if err != nil {
		return err
	}
	c.SendMessage(fmt.Sprintf("NAT gateway deletion complete: %s", natGatewayIds))
	inventory.AvailabilityZones = *updatedAzInventory
	inventory.send(c.InventoryChan)

	// Internet Gateway
	if err := c.DeleteInternetGateway(inventory.InternetGatewayId, inventory.VpcId); err != nil {
		return err
	}
	c.SendMessage(fmt.Sprintf("Internet gateway deleted: %s", inventory.InternetGatewayId))
	inventory.InternetGatewayId = ""
	inventory.send(c.InventoryChan)

	// Elastic IPs
	if err := c.DeleteElasticIps(inventory.ElasticIpIds); err != nil {
		return err
	}
	c.SendMessage(fmt.Sprintf("Elastic IPs deleted: %s", inventory.ElasticIpIds))
	inventory.ElasticIpIds = []string{}
	inventory.send(c.InventoryChan)

	// Subnets
	updatedAzInventory, subnetIds, err := c.DeleteSubnets(&inventory.AvailabilityZones)
	if err != nil {
		return err
	}
	c.SendMessage(fmt.Sprintf("Subnets deleted: %s", subnetIds))
	inventory.AvailabilityZones = *updatedAzInventory
	inventory.send(c.InventoryChan)

	// Route Tables
	if err := c.DeleteRouteTables(
		inventory.PrivateRouteTableIds,
		inventory.PublicRouteTableId,
	); err != nil {
		return err
	}
	c.SendMessage(
		fmt.Sprintf("Route tables deleted: [%s %s]",
			inventory.PrivateRouteTableIds, inventory.PublicRouteTableId,
		),
	)
	inventory.PrivateRouteTableIds = []string{}
	inventory.PublicRouteTableId = ""
	inventory.send(c.InventoryChan)

	// VPC
	if err := c.DeleteVpc(inventory.VpcId); err != nil {
		return err
	}
	c.SendMessage(fmt.Sprintf("VPC deleted: %s", inventory.VpcId))
	inventory.VpcId = ""
	inventory.send(c.InventoryChan)

	return nil
}
