package s3

import (
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/iam/types"
)

// CreateRole creates the IAM role needed for DNS management by
// the Kubernetes service account of an in-cluster supporting service such as
// external-dns using IRSA (IAM role for service accounts).
func (c *S3Client) CreateRole(
	tags *[]types.Tag,
	policyArn string,
	awsAccount string,
	oidcUrl string,
	serviceAccountName string,
	serviceAccountNamespace string,
) (*types.Role, error) {
	svc := iam.NewFromConfig(*c.AwsConfig)

	oidcUrlBare := strings.Trim(oidcUrl, "https://")
	roleName := serviceAccountName
	rolePolicyDocument := fmt.Sprintf(`{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Principal": {
                "Federated": "arn:aws:iam::%[1]s:oidc-provider/%[2]s"
            },
            "Action": "sts:AssumeRoleWithWebIdentity",
            "Condition": {
                "StringEquals": {
                    "%[2]s:sub": "system:serviceaccount:%[3]s:%[4]s",
                    "%[2]s:aud": "sts.amazonaws.com"
                }
            }
        }
    ]
}`, awsAccount, oidcUrlBare, serviceAccountNamespace, serviceAccountName)
	createRoleInput := iam.CreateRoleInput{
		AssumeRolePolicyDocument: &rolePolicyDocument,
		RoleName:                 &roleName,
		PermissionsBoundary:      &policyArn,
		Tags:                     *tags,
	}
	resp, err := svc.CreateRole(c.Context, &createRoleInput)
	if err != nil {
		return nil, fmt.Errorf("failed to create role %s: %w", roleName, err)
	}

	attachRolePolicyInput := iam.AttachRolePolicyInput{
		PolicyArn: &policyArn,
		RoleName:  resp.Role.RoleName,
	}
	_, err = svc.AttachRolePolicy(c.Context, &attachRolePolicyInput)
	if err != nil {
		return resp.Role, fmt.Errorf("failed to attach role policy %s to %s: %w", policyArn, roleName, err)
	}

	return resp.Role, nil
}

// DeleteRole detaches a role's policies and deletes an IAM role.
func (c *S3Client) DeleteRole(role *RoleInventory) error {
	// if roles are empty, there's nothing to delete
	if role == nil || role.RoleName == "" {
		return nil
	}

	svc := iam.NewFromConfig(*c.AwsConfig)

	for _, policyArn := range role.RolePolicyArns {
		detachRolePolicyInput := iam.DetachRolePolicyInput{
			PolicyArn: &policyArn,
			RoleName:  &role.RoleName,
		}
		_, err := svc.DetachRolePolicy(c.Context, &detachRolePolicyInput)
		if err != nil {
			var noSuchEntityErr *types.NoSuchEntityException
			if errors.As(err, &noSuchEntityErr) {
				continue
			} else {
				return fmt.Errorf("failed to detach policy %s from role %s: %w", policyArn, role.RoleName, err)
			}
		}
	}

	deleteRoleInput := iam.DeleteRoleInput{RoleName: &role.RoleName}
	_, err := svc.DeleteRole(c.Context, &deleteRoleInput)
	if err != nil {
		var noSuchEntityErr *types.NoSuchEntityException
		if errors.As(err, &noSuchEntityErr) {
			return nil
		} else {
			return fmt.Errorf("failed to delete role %s: %w", role.RoleName, err)
		}
	}

	return nil
}
