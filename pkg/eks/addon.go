package eks

import (
	"errors"
	"fmt"

	aws_eks "github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/smithy-go"
)

const EbsStorageAddonName = "aws-ebs-csi-driver"

// CreateEbsStorageAddon creates installs the EBS CSI driver addon on the EKS
// cluster.
func (c *EksClient) CreateEbsStorageAddon(
	tags *map[string]string,
	clusterName string,
	storageManagementRoleArn string,
) (string, error) {
	svc := aws_eks.NewFromConfig(*c.AwsConfig)

	ebsAddonName := EbsStorageAddonName

	createEbsAddonInput := aws_eks.CreateAddonInput{
		AddonName:             &ebsAddonName,
		ClusterName:           &clusterName,
		ServiceAccountRoleArn: &storageManagementRoleArn,
		Tags:                  *tags,
	}
	resp, err := svc.CreateAddon(c.Context, &createEbsAddonInput)
	if err != nil {
		var ae smithy.APIError
		if errors.As(err, &ae) {
			if ae.ErrorCode() == "ResourceInUseException" {
				// addon already exists - return name
				return EbsStorageAddonName, nil
			}
		}
		return "", fmt.Errorf("failed to create cluster: %w", err)
	}

	return *resp.Addon.AddonName, nil
}
