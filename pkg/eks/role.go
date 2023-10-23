package eks

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/aws/smithy-go"
)

const (
	ClusterRoleName            = "cluster-role"
	WorkerRoleName             = "worker-role"
	DnsManagementRoleName      = "dns-mgmt-role"
	Dns01ChallengeRoleName     = "dns-chlg-role"
	ClusterAutoscalingRoleName = "ca-role"
	StorageManagementRoleName  = "csi-role"
)

// CreateClusterRole creates the IAM roles needed for EKS clusters.
func (c *EksClient) CreateClusterRole(
	tags *[]types.Tag,
	clusterName string,
) (*types.Role, error) {
	svc := iam.NewFromConfig(*c.AwsConfig)

	clusterRoleName := fmt.Sprintf("%s-%s", ClusterRoleName, clusterName)
	if err := CheckRoleName(clusterRoleName); err != nil {
		return nil, err
	}
	clusterPolicyArn := ClusterPolicyArn
	clusterRolePolicyDocument := `{
  "Version": "2012-10-17",
  "Statement": [
	  {
		  "Effect": "Allow",
		  "Principal": {
			  "Service": [
				  "eks.amazonaws.com"
			  ]
		  },
		  "Action": "sts:AssumeRole"
	  }
  ]
}`
	createClusterRoleInput := iam.CreateRoleInput{
		AssumeRolePolicyDocument: &clusterRolePolicyDocument,
		RoleName:                 &clusterRoleName,
		PermissionsBoundary:      &clusterPolicyArn,
		Tags:                     *tags,
	}
	clusterRoleResp, err := svc.CreateRole(c.Context, &createClusterRoleInput)
	if err != nil {
		var ae smithy.APIError
		if errors.As(err, &ae) {
			if ae.ErrorCode() == "EntityAlreadyExists" {
				// ensure policy is attached to role
				listPoliciesInput := iam.ListAttachedRolePoliciesInput{
					RoleName: &clusterRoleName,
				}
				listPoliciesOutput, err := svc.ListAttachedRolePolicies(c.Context, &listPoliciesInput)
				if err != nil {
					return nil, fmt.Errorf("failed to list policies for role %s: %w", clusterRoleName, err)
				}
				attachedPolicyFound := false
				for _, policy := range listPoliciesOutput.AttachedPolicies {
					if *policy.PolicyArn == clusterPolicyArn {
						attachedPolicyFound = true
						break
					}
				}
				// if not attached, attach it
				if !attachedPolicyFound {
					if err := c.attachPolicyToRole(
						clusterRoleName,
						clusterPolicyArn,
					); err != nil {
						return nil, err
					}
				}
				// get the role by name to return
				getRoleInput := iam.GetRoleInput{RoleName: &clusterRoleName}
				getRoleOutput, err := svc.GetRole(c.Context, &getRoleInput)
				if err != nil {
					return nil, fmt.Errorf("failed to existing role with name %s: %w", clusterRoleName, err)
				}

				return getRoleOutput.Role, nil
			}
		}
		return nil, fmt.Errorf("failed to create role %s: %w", clusterRoleName, err)
	}

	// attach policy to role
	if err := c.attachPolicyToRole(
		clusterRoleName,
		clusterPolicyArn,
	); err != nil {
		return nil, err
	}

	return clusterRoleResp.Role, nil
}

// CreateNodeRole creates the IAM roles needed for EKS worker node groups.
func (c *EksClient) CreateNodeRole(
	tags *[]types.Tag,
	clusterName string,
) (*types.Role, error) {
	svc := iam.NewFromConfig(*c.AwsConfig)

	workerRoleName := fmt.Sprintf("%s-%s", WorkerRoleName, clusterName)
	if err := CheckRoleName(workerRoleName); err != nil {
		return nil, err
	}
	workerRolePolicyDocument := `{
  "Version": "2012-10-17",
  "Statement": [
  	{
  		"Effect": "Allow",
  		"Principal": {
  			"Service": [
  				"ec2.amazonaws.com"
  			]
  		},
  		"Action": [
  			"sts:AssumeRole"
  		]
  	}
  ]
}`
	createWorkerRoleInput := iam.CreateRoleInput{
		AssumeRolePolicyDocument: &workerRolePolicyDocument,
		RoleName:                 &workerRoleName,
		Tags:                     *tags,
	}
	workerRoleResp, err := svc.CreateRole(c.Context, &createWorkerRoleInput)
	if err != nil {
		var ae smithy.APIError
		if errors.As(err, &ae) {
			if ae.ErrorCode() == "EntityAlreadyExists" {
				// ensure policies are attached to role
				listPoliciesInput := iam.ListAttachedRolePoliciesInput{
					RoleName: &workerRoleName,
				}
				listPoliciesOutput, err := svc.ListAttachedRolePolicies(c.Context, &listPoliciesInput)
				if err != nil {
					return nil, fmt.Errorf("failed to list policies for role %s: %w", workerRoleName, err)
				}
				attachedPoliciesFound := true
				for _, expectedPolicy := range getWorkerPolicyArns() {
					expectedPolicyFound := false
					for _, policy := range listPoliciesOutput.AttachedPolicies {
						if *policy.PolicyArn == expectedPolicy {
							expectedPolicyFound = true
							break
						}
					}
					if !expectedPolicyFound {
						attachedPoliciesFound = false
						break
					}
				}
				// if not attached, attach them
				if !attachedPoliciesFound {
					for _, policyArn := range getWorkerPolicyArns() {
						if err := c.attachPolicyToRole(
							workerRoleName,
							policyArn,
						); err != nil {
							return nil, err
						}
					}
				}
				// get the role by name to return
				getRoleInput := iam.GetRoleInput{RoleName: &workerRoleName}
				getRoleOutput, err := svc.GetRole(c.Context, &getRoleInput)
				if err != nil {
					return nil, fmt.Errorf("failed to existing role with name %s: %w", workerRoleName, err)
				}

				return getRoleOutput.Role, nil
			}
		}
		return nil, fmt.Errorf("failed to create role %s: %w", workerRoleName, err)
	}

	// attach policies
	for _, policyArn := range getWorkerPolicyArns() {
		if err := c.attachPolicyToRole(
			workerRoleName,
			policyArn,
		); err != nil {
			return nil, err
		}
	}

	return workerRoleResp.Role, nil
}

// CreateDnsManagementRole creates the IAM role needed for DNS management by
// the Kubernetes service account of an in-cluster supporting service such as
// external-dns using IRSA (IAM role for service accounts).
func (c *EksClient) CreateDnsManagementRole(
	tags *[]types.Tag,
	dnsPolicyArn string,
	awsAccountId string,
	oidcProvider string,
	serviceAccount *ServiceAccountConfig,
	clusterName string,
) (*types.Role, error) {
	svc := iam.NewFromConfig(*c.AwsConfig)

	oidcProviderBare := strings.Trim(oidcProvider, "https://")
	dnsManagementRoleName := fmt.Sprintf("%s-%s", DnsManagementRoleName, clusterName)
	dnsManagementRolePath := fmt.Sprintf("/%s/", clusterName)
	if err := CheckRoleName(dnsManagementRoleName); err != nil {
		return nil, err
	}
	dnsManagementRolePolicyDocument := fmt.Sprintf(`{
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
}`, awsAccountId, oidcProviderBare, serviceAccount.Namespace, serviceAccount.Name)
	createDnsManagementRoleInput := iam.CreateRoleInput{
		AssumeRolePolicyDocument: &dnsManagementRolePolicyDocument,
		RoleName:                 &dnsManagementRoleName,
		Path:                     &dnsManagementRolePath,
		PermissionsBoundary:      &dnsPolicyArn,
		Tags:                     *tags,
	}
	dnsManagementRoleResp, err := svc.CreateRole(c.Context, &createDnsManagementRoleInput)
	if err != nil {
		var ae smithy.APIError
		if errors.As(err, &ae) {
			if ae.ErrorCode() == "EntityAlreadyExists" {
				// ensure policy is attached to role
				listPoliciesInput := iam.ListAttachedRolePoliciesInput{
					RoleName: &dnsManagementRoleName,
				}
				listPoliciesOutput, err := svc.ListAttachedRolePolicies(c.Context, &listPoliciesInput)
				if err != nil {
					return nil, fmt.Errorf("failed to list policies for role %s: %w", dnsManagementRoleName, err)
				}
				attachedPolicyFound := false
				for _, policy := range listPoliciesOutput.AttachedPolicies {
					if *policy.PolicyArn == dnsPolicyArn {
						attachedPolicyFound = true
						break
					}
				}
				// if not attached, attach it
				if !attachedPolicyFound {
					if err := c.attachPolicyToRole(
						dnsManagementRoleName,
						dnsPolicyArn,
					); err != nil {
						return nil, err
					}
				}
				// get the role by name to return
				getRoleInput := iam.GetRoleInput{RoleName: &dnsManagementRoleName}
				getRoleOutput, err := svc.GetRole(c.Context, &getRoleInput)
				if err != nil {
					return nil, fmt.Errorf("failed to existing role with name %s: %w", dnsManagementRoleName, err)
				}

				return getRoleOutput.Role, nil
			}
		}
		return nil, fmt.Errorf("failed to create role %s: %w", dnsManagementRoleName, err)
	}

	// attach policy to role
	if err := c.attachPolicyToRole(
		dnsManagementRoleName,
		dnsPolicyArn,
	); err != nil {
		return nil, err
	}

	return dnsManagementRoleResp.Role, nil
}

// CreateDns01ChallengeRole creates the IAM role needed for DNS01 challenges by
// the Kubernetes service account of an in-cluster supporting service such as
// cert-manager using IRSA (IAM role for service accounts).
func (c *EksClient) CreateDns01ChallengeRole(
	tags *[]types.Tag,
	dnsPolicyArn string,
	awsAccountId string,
	oidcProvider string,
	serviceAccount *ServiceAccountConfig,
	clusterName string,
) (*types.Role, error) {
	svc := iam.NewFromConfig(*c.AwsConfig)

	oidcProviderBare := strings.Trim(oidcProvider, "https://")
	dns01ChallengeRoleName := fmt.Sprintf("%s-%s", Dns01ChallengeRoleName, clusterName)
	if err := CheckRoleName(dns01ChallengeRoleName); err != nil {
		return nil, err
	}
	dns01ChallengeRolePolicyDocument := fmt.Sprintf(`{
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
}`, awsAccountId, oidcProviderBare, serviceAccount.Namespace, serviceAccount.Name)
	createdDns01ChallengeRoleInput := iam.CreateRoleInput{
		AssumeRolePolicyDocument: &dns01ChallengeRolePolicyDocument,
		RoleName:                 &dns01ChallengeRoleName,
		PermissionsBoundary:      &dnsPolicyArn,
		Tags:                     *tags,
	}
	dns01ChallengeRoleResp, err := svc.CreateRole(c.Context, &createdDns01ChallengeRoleInput)
	if err != nil {
		var ae smithy.APIError
		if errors.As(err, &ae) {
			if ae.ErrorCode() == "EntityAlreadyExists" {
				// ensure policy is attached to role
				listPoliciesInput := iam.ListAttachedRolePoliciesInput{
					RoleName: &dns01ChallengeRoleName,
				}
				listPoliciesOutput, err := svc.ListAttachedRolePolicies(c.Context, &listPoliciesInput)
				if err != nil {
					return nil, fmt.Errorf("failed to list policies for role %s: %w", dns01ChallengeRoleName, err)
				}
				attachedPolicyFound := false
				for _, policy := range listPoliciesOutput.AttachedPolicies {
					if *policy.PolicyArn == dnsPolicyArn {
						attachedPolicyFound = true
						break
					}
				}
				// if not attached, attach it
				if !attachedPolicyFound {
					if err := c.attachPolicyToRole(
						dns01ChallengeRoleName,
						dnsPolicyArn,
					); err != nil {
						return nil, err
					}
				}
				// get the role by name to return
				getRoleInput := iam.GetRoleInput{RoleName: &dns01ChallengeRoleName}
				getRoleOutput, err := svc.GetRole(c.Context, &getRoleInput)
				if err != nil {
					return nil, fmt.Errorf("failed to existing role with name %s: %w", dns01ChallengeRoleName, err)
				}

				return getRoleOutput.Role, nil
			}
		}
		return nil, fmt.Errorf("failed to create role %s: %w", dns01ChallengeRoleName, err)
	}

	// attach policy to role
	if err := c.attachPolicyToRole(
		dns01ChallengeRoleName,
		dnsPolicyArn,
	); err != nil {
		return nil, err
	}

	return dns01ChallengeRoleResp.Role, nil
}

// CreateClusterAutoscalingRole creates the IAM role needed for cluster
// autoscaler to manage node pool sizes using IRSA (IAM role for service
// accounts).
func (c *EksClient) CreateClusterAutoscalingRole(
	tags *[]types.Tag,
	autoscalingPolicyArn string,
	awsAccountId string,
	oidcProvider string,
	serviceAccount *ServiceAccountConfig,
	clusterName string,
) (*types.Role, error) {
	svc := iam.NewFromConfig(*c.AwsConfig)

	oidcProviderBare := strings.Trim(oidcProvider, "https://")
	clusterAutoscalingRoleName := fmt.Sprintf("%s-%s", ClusterAutoscalingRoleName, clusterName)
	if err := CheckRoleName(clusterAutoscalingRoleName); err != nil {
		return nil, err
	}
	clusterAutoscalingRolePolicyDocument := fmt.Sprintf(`{
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
}`, awsAccountId, oidcProviderBare, serviceAccount.Namespace, serviceAccount.Name)
	createClusterAutoscalingRoleInput := iam.CreateRoleInput{
		AssumeRolePolicyDocument: &clusterAutoscalingRolePolicyDocument,
		RoleName:                 &clusterAutoscalingRoleName,
		PermissionsBoundary:      &autoscalingPolicyArn,
		Tags:                     *tags,
	}
	clusterAutoscalingRoleResp, err := svc.CreateRole(c.Context, &createClusterAutoscalingRoleInput)
	if err != nil {
		var ae smithy.APIError
		if errors.As(err, &ae) {
			if ae.ErrorCode() == "EntityAlreadyExists" {
				// ensure policy is attached to role
				listPoliciesInput := iam.ListAttachedRolePoliciesInput{
					RoleName: &clusterAutoscalingRoleName,
				}
				listPoliciesOutput, err := svc.ListAttachedRolePolicies(c.Context, &listPoliciesInput)
				if err != nil {
					return nil, fmt.Errorf("failed to list policies for role %s: %w", clusterAutoscalingRoleName, err)
				}
				attachedPolicyFound := false
				for _, policy := range listPoliciesOutput.AttachedPolicies {
					if *policy.PolicyArn == autoscalingPolicyArn {
						attachedPolicyFound = true
						break
					}
				}
				// if not attached, attach it
				if !attachedPolicyFound {
					if err := c.attachPolicyToRole(
						clusterAutoscalingRoleName,
						autoscalingPolicyArn,
					); err != nil {
						return nil, err
					}
				}
				// get the role by name to return
				getRoleInput := iam.GetRoleInput{RoleName: &clusterAutoscalingRoleName}
				getRoleOutput, err := svc.GetRole(c.Context, &getRoleInput)
				if err != nil {
					return nil, fmt.Errorf("failed to existing role with name %s: %w", clusterAutoscalingRoleName, err)
				}

				return getRoleOutput.Role, nil
			}
		}
		return nil, fmt.Errorf("failed to create role %s: %w", clusterAutoscalingRoleName, err)
	}

	// attach policy to role
	if err := c.attachPolicyToRole(
		clusterAutoscalingRoleName,
		autoscalingPolicyArn,
	); err != nil {
		return nil, err
	}

	return clusterAutoscalingRoleResp.Role, nil
}

// CreateStorageManagementRole creates the IAM role needed for storage
// management by the CSI driver's service account using IRSA (IAM role for
// service accounts).
func (c *EksClient) CreateStorageManagementRole(
	tags *[]types.Tag,
	awsAccountId string,
	oidcProvider string,
	serviceAccount *ServiceAccountConfig,
	clusterName string,
) (*types.Role, error) {
	svc := iam.NewFromConfig(*c.AwsConfig)

	oidcProviderBare := strings.Trim(oidcProvider, "https://")
	storageManagementRoleName := fmt.Sprintf("%s-%s", StorageManagementRoleName, clusterName)
	if err := CheckRoleName(storageManagementRoleName); err != nil {
		return nil, err
	}
	storagePolicyArn := CsiDriverPolicyArn
	storageManagementRolePolicyDocument := fmt.Sprintf(`{
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
}`, awsAccountId, oidcProviderBare, serviceAccount.Namespace, serviceAccount.Name)
	createStorageManagementRoleInput := iam.CreateRoleInput{
		AssumeRolePolicyDocument: &storageManagementRolePolicyDocument,
		RoleName:                 &storageManagementRoleName,
		PermissionsBoundary:      &storagePolicyArn,
		Tags:                     *tags,
	}
	storageManagementRoleResp, err := svc.CreateRole(c.Context, &createStorageManagementRoleInput)
	if err != nil {
		var ae smithy.APIError
		if errors.As(err, &ae) {
			if ae.ErrorCode() == "EntityAlreadyExists" {
				// ensure policy is attached to role
				listPoliciesInput := iam.ListAttachedRolePoliciesInput{
					RoleName: &storageManagementRoleName,
				}
				listPoliciesOutput, err := svc.ListAttachedRolePolicies(c.Context, &listPoliciesInput)
				if err != nil {
					return nil, fmt.Errorf("failed to list policies for role %s: %w", storageManagementRoleName, err)
				}
				attachedPolicyFound := false
				for _, policy := range listPoliciesOutput.AttachedPolicies {
					if *policy.PolicyArn == storagePolicyArn {
						attachedPolicyFound = true
						break
					}
				}
				// if not attached, attach it
				if !attachedPolicyFound {
					if err := c.attachPolicyToRole(
						storageManagementRoleName,
						storagePolicyArn,
					); err != nil {
						return nil, err
					}
				}
				// get the role by name to return
				getRoleInput := iam.GetRoleInput{RoleName: &storageManagementRoleName}
				getRoleOutput, err := svc.GetRole(c.Context, &getRoleInput)
				if err != nil {
					return nil, fmt.Errorf("failed to existing role with name %s: %w", storageManagementRoleName, err)
				}

				return getRoleOutput.Role, nil
			}
		}
		return nil, fmt.Errorf("failed to create role %s: %w", storageManagementRoleName, err)
	}

	// attach policy to role
	if err := c.attachPolicyToRole(
		storageManagementRoleName,
		storagePolicyArn,
	); err != nil {
		return nil, err
	}

	return storageManagementRoleResp.Role, nil
}

// DeleteRoles deletes the IAM roles used by EKS.  If empty role names are
// provided, or if the roles are not found it returns without error.
func (c *EksClient) DeleteRoles(roles *[]RoleInventory) error {
	// if roles are empty, there's nothing to delete
	if len(*roles) == 0 {
		return nil
	}

	svc := iam.NewFromConfig(*c.AwsConfig)

	for _, role := range *roles {
		if role.RoleName == "" {
			// role is empty - skip
			continue
		}
		for _, policyArn := range role.RolePolicyArns {
			detachRolePolicyInput := iam.DetachRolePolicyInput{
				PolicyArn: &policyArn,
				RoleName:  &role.RoleName,
			}
			_, err := svc.DetachRolePolicy(c.Context, &detachRolePolicyInput)
			if err != nil {
				var noSuchEntityErr *types.NoSuchEntityException
				if errors.As(err, &noSuchEntityErr) {
					return nil
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
	}

	return nil
}

// attachPolicyToRole takes an IAM role name and policy ARN and attaches the
// policy to the role.
func (c *EksClient) attachPolicyToRole(roleName string, policyArn string) error {

	svc := iam.NewFromConfig(*c.AwsConfig)

	attachRolePolicyInput := iam.AttachRolePolicyInput{
		RoleName:  &roleName,
		PolicyArn: &policyArn,
	}
	_, err := svc.AttachRolePolicy(c.Context, &attachRolePolicyInput)
	if err != nil {
		return fmt.Errorf("failed to attach policy %s to %s: %w", policyArn, roleName, err)
	}

	return nil
}

// getWorkerPolicyArns returns the IAM policy ARNs needed for clusters and node
// groups.
func getWorkerPolicyArns() []string {
	return []string{
		WorkerNodePolicyArn,
		ContainerRegistryPolicyArn,
		CniPolicyArn,
	}
}

// CheckRoleName ensures role names do not exceed the AWS limit for role name
// lengths (64 characters).
func CheckRoleName(name string) error {
	if utf8.RuneCountInString(name) > 64 {
		return errors.New(fmt.Sprintf(
			"role name %s too long, must be 64 characters or less", name,
		))
	}

	return nil
}
