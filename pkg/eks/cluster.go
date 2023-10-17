package eks

import (
	"errors"
	"fmt"
	"time"

	aws_eks "github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/aws/smithy-go"

	"github.com/nukleros/aws-builder/pkg/util"
)

type ClusterCondition string

const (
	ClusterConditionCreated = "ClusterCreated"
	ClusterConditionDeleted = "ClusterDeleted"
	ClusterCheckInterval    = 15 //check cluster status every 15 seconds
	ClusterCheckMaxCount    = 60 // check 60 times before giving up (15 minutes)
)

// CreateCluster creates a new EKS Cluster.
func (c *EksClient) CreateCluster(
	tags *map[string]string,
	clusterName string,
	kubernetesVersion string,
	roleArn string,
	azInventory *[]AvailabilityZoneInventory,
) (*types.Cluster, error) {
	svc := aws_eks.NewFromConfig(*c.AwsConfig)

	// collect subnet IDs
	var subnetIds []string
	for _, az := range *azInventory {
		for _, privateSubnet := range az.PrivateSubnets {
			if privateSubnet.SubnetId != "" {
				subnetIds = append(subnetIds, privateSubnet.SubnetId)
			}
		}
	}

	privateAccess := true
	publicAccess := true
	vpcConfig := types.VpcConfigRequest{
		EndpointPrivateAccess: &privateAccess,
		EndpointPublicAccess:  &publicAccess,
		SubnetIds:             subnetIds,
	}

	createClusterInput := aws_eks.CreateClusterInput{
		Name:               &clusterName,
		ResourcesVpcConfig: &vpcConfig,
		RoleArn:            &roleArn,
		Version:            &kubernetesVersion,
		Tags:               *tags,
	}
	resp, err := svc.CreateCluster(c.Context, &createClusterInput)
	if err != nil {
		var ae smithy.APIError
		if errors.As(err, &ae) {
			if ae.ErrorCode() == "ResourceInUseException" {
				// cluster already exists - get cluster to return
				describeClusterInput := aws_eks.DescribeClusterInput{
					Name: &clusterName,
				}
				describeClusterOutput, err := svc.DescribeCluster(c.Context, &describeClusterInput)
				if err != nil {
					return nil, fmt.Errorf("failed to describe cluster %s that already exists: %w", clusterName, err)
				}

				return describeClusterOutput.Cluster, nil
			}
		}
		return nil, fmt.Errorf("failed to create cluster: %w", err)
	}

	return resp.Cluster, nil
}

// DeleteCluster deletes an EKS cluster.  If  an empty cluster name is supplied,
// or if the cluster is not found it returns without error.
func (c *EksClient) DeleteCluster(clusterName string) error {
	// if clusterName is empty, there's nothing to delete
	if clusterName == "" {
		return nil
	}

	svc := aws_eks.NewFromConfig(*c.AwsConfig)

	deleteClusterInput := aws_eks.DeleteClusterInput{Name: &clusterName}
	_, err := svc.DeleteCluster(c.Context, &deleteClusterInput)
	if err != nil {
		var notFoundErr *types.ResourceNotFoundException
		if errors.As(err, &notFoundErr) {
			return nil
		} else {
			return fmt.Errorf("failed to delete cluster: %w", err)
		}
	}

	return nil
}

// WaitForCluster waits until a cluster reaches a certain condition.  One of:
// * ClusterConditionCreated
// * ClusterConditionDeleted
func (c *EksClient) WaitForCluster(
	clusterName string,
	clusterCondition ClusterCondition,
) (string, error) {
	var oicdIssuer string

	// if no cluster, there's nothing to check
	if clusterName == "" {
		return oicdIssuer, nil
	}

	clusterCheckCount := 0
	for {
		clusterCheckCount += 1
		if clusterCheckCount > ClusterCheckMaxCount {
			return oicdIssuer, errors.New("cluster condition check timed out")
		}

		cluster, err := c.getCluster(clusterName)
		if err != nil {
			if errors.Is(err, util.ErrResourceNotFound) && clusterCondition == ClusterConditionDeleted {
				// resource was not found and we're waiting for it to be
				// deleted so condition is met
				break
			} else {
				return oicdIssuer, fmt.Errorf("failed to get cluster status while waiting for %s: %w", clusterName, err)
			}
		}
		if cluster.Status == types.ClusterStatusActive && clusterCondition == ClusterConditionCreated {
			// resource is available and we're waiting for it to be created
			// so condition is met
			oicdIssuer = *cluster.Identity.Oidc.Issuer
			break
		}
		time.Sleep(time.Second * 15)
	}

	return oicdIssuer, nil
}

// getCluster retrieves the cluster for a given cluster name.
func (c *EksClient) getCluster(clusterName string) (*types.Cluster, error) {
	svc := aws_eks.NewFromConfig(*c.AwsConfig)

	describeClusterInput := aws_eks.DescribeClusterInput{
		Name: &clusterName,
	}
	resp, err := svc.DescribeCluster(c.Context, &describeClusterInput)
	if err != nil {
		var notFoundErr *types.ResourceNotFoundException
		if errors.As(err, &notFoundErr) {
			return nil, util.ErrResourceNotFound
		} else {
			return nil, fmt.Errorf("failed to describe cluster %s: %w", clusterName, err)
		}
	}

	return resp.Cluster, nil
}
