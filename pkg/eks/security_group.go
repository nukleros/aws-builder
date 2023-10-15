package eks

import (
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// GetClusterSecurityGroup retrieves the security group created for the EKS
// cluster by AWS during provisioning.
func (c *EksClient) GetClusterSecurityGroup(clusterName string) (string, error) {
	svc := ec2.NewFromConfig(*c.AwsConfig)

	filterName := fmt.Sprintf("tag:aws:eks:cluster-name")
	filters := []types.Filter{
		{
			Name:   &filterName,
			Values: []string{clusterName},
		},
	}
	describeSecurityGroupsInput := ec2.DescribeSecurityGroupsInput{
		Filters: filters,
	}
	resp, err := svc.DescribeSecurityGroups(c.Context, &describeSecurityGroupsInput)
	if err != nil {
		return "", fmt.Errorf("failed to describe security groups filtered by cluster name %s: %w", clusterName, err)
	}

	if len(resp.SecurityGroups) == 0 {
		return "", fmt.Errorf("found zero security groups filtered by cluster name %s", clusterName)
	}
	if len(resp.SecurityGroups) > 1 {
		return "", fmt.Errorf("found multiple security groups filtered by cluster name %s", clusterName)
	}

	return *resp.SecurityGroups[0].GroupId, nil
}
