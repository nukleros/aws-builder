package eks

import (
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/aws/smithy-go"
)

const (
	DnsPolicyName              = "DNSUpdates"
	Dns01ChallengePolicyName   = "DNS01Challenge"
	SecretsManagerPolicyName   = "SecretsManager"
	AutoscalingPolicyName      = "ClusterAutoscaler"
	ClusterPolicyArn           = "arn:aws:iam::aws:policy/AmazonEKSClusterPolicy"
	WorkerNodePolicyArn        = "arn:aws:iam::aws:policy/AmazonEKSWorkerNodePolicy"
	ContainerRegistryPolicyArn = "arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly"
	CniPolicyArn               = "arn:aws:iam::aws:policy/AmazonEKS_CNI_Policy"
	CsiDriverPolicyArn         = "arn:aws:iam::aws:policy/service-role/AmazonEBSCSIDriverPolicy"
)

// CreateDnsManagementPolicy creates the IAM policy to be used for managing
// Route53 DNS records.
func (c *EksClient) CreateDnsManagementPolicy(
	tags *[]types.Tag,
	clusterName string,
) (*types.Policy, error) {
	svc := iam.NewFromConfig(*c.AwsConfig)

	dnsPolicyName := fmt.Sprintf("%s-%s", DnsPolicyName, clusterName)
	dnsPolicyPath := fmt.Sprintf("/%s/", clusterName)
	dnsPolicyDescription := "Allow cluster services to update Route53 records"
	dnsPolicyDocument := `{
"Version": "2012-10-17",
"Statement": [
{
  "Effect": "Allow",
  "Action": [
	"route53:ChangeResourceRecordSets"
  ],
  "Resource": [
	"arn:aws:route53:::hostedzone/*"
  ]
},
{
  "Effect": "Allow",
  "Action": [
	"route53:ListHostedZones",
	"route53:ListResourceRecordSets"
  ],
  "Resource": [
	"*"
  ]
}
]
}`
	createR53PolicyInput := iam.CreatePolicyInput{
		PolicyName:     &dnsPolicyName,
		Path:           &dnsPolicyPath,
		Description:    &dnsPolicyDescription,
		PolicyDocument: &dnsPolicyDocument,
	}
	r53PolicyResp, err := svc.CreatePolicy(c.Context, &createR53PolicyInput)
	if err != nil {
		var ae smithy.APIError
		if errors.As(err, &ae) {
			if ae.ErrorCode() == "EntityAlreadyExists" {
				listPoliciesInput := iam.ListPoliciesInput{
					PathPrefix: &dnsPolicyPath,
					Scope:      types.PolicyScopeTypeLocal,
				}
				listPoliciesOutput, err := svc.ListPolicies(c.Context, &listPoliciesInput)
				if err != nil {
					return nil, fmt.Errorf("failed to list policies to find existing %s policy: %w", dnsPolicyName, err)
				}
				for _, policy := range listPoliciesOutput.Policies {
					if *policy.PolicyName == dnsPolicyName {
						return &policy, nil
					}
				}

				return nil, fmt.Errorf("failed to find existing policy with name %s and path %s", dnsPolicyName, dnsPolicyPath)
			}
		}
		return nil, fmt.Errorf("failed to create DNS management policy %s: %w", dnsPolicyName, err)
	}

	return r53PolicyResp.Policy, nil
}

// CreateDns01ChallengePolicy creates the IAM policy to be used for completing
// DNS01 challenges.
func (c *EksClient) CreateDns01ChallengePolicy(
	tags *[]types.Tag,
	clusterName string,
) (*types.Policy, error) {
	svc := iam.NewFromConfig(*c.AwsConfig)

	dnsPolicyName := fmt.Sprintf("%s-%s", Dns01ChallengePolicyName, clusterName)
	dnsPolicyPath := fmt.Sprintf("/%s/", clusterName)
	dnsPolicyDescription := "Allow cluster services to complete DNS01 challenges"

	// NOTE: As of 8/8/2023, the cert-manager documentation for the DNS01 challenge
	// IAM policy is incorrect.  The correct policy is below, and was taken from
	// this stack overflow post:
	// https://github.com/cert-manager/cert-manager/issues/3079#issuecomment-657795131
	dnsPolicyDocument := `{
"Version": "2012-10-17",
"Statement": [
{
  "Effect": "Allow",
  "Action": [
	"route53:ChangeResourceRecordSets"
  ],
  "Resource": [
	"arn:aws:route53:::hostedzone/*"
  ]
},
{
  "Effect": "Allow",
  "Action": [
	"route53:GetChange",
	"route53:ListHostedZones",
	"route53:ListResourceRecordSets",
	"route53:ListHostedZonesByName"
  ],
  "Resource": [
	"*"
  ]
}
]
}`
	createR53PolicyInput := iam.CreatePolicyInput{
		PolicyName:     &dnsPolicyName,
		Path:           &dnsPolicyPath,
		Description:    &dnsPolicyDescription,
		PolicyDocument: &dnsPolicyDocument,
	}
	r53PolicyResp, err := svc.CreatePolicy(c.Context, &createR53PolicyInput)
	if err != nil {
		var ae smithy.APIError
		if errors.As(err, &ae) {
			if ae.ErrorCode() == "EntityAlreadyExists" {
				listPoliciesInput := iam.ListPoliciesInput{
					PathPrefix: &dnsPolicyPath,
					Scope:      types.PolicyScopeTypeLocal,
				}
				listPoliciesOutput, err := svc.ListPolicies(c.Context, &listPoliciesInput)
				if err != nil {
					return nil, fmt.Errorf("failed to list policies to find existing %s policy: %w", dnsPolicyName, err)
				}
				for _, policy := range listPoliciesOutput.Policies {
					if *policy.PolicyName == dnsPolicyName {
						return &policy, nil
					}
				}

				return nil, fmt.Errorf("failed to find existing policy with name %s and path %s", dnsPolicyName, dnsPolicyPath)
			}
		}
		return nil, fmt.Errorf("failed to create DNS01 challenge policy %s: %w", dnsPolicyName, err)
	}

	return r53PolicyResp.Policy, nil
}

// CreateSecretsManagerPolicy creates the IAM policy to be used for managing
// secrets.
func (c *EksClient) CreateSecretsManagerPolicy(
	tags *[]types.Tag,
	clusterName string,
) (*types.Policy, error) {
	svc := iam.NewFromConfig(*c.AwsConfig)

	secretsManagerPolicyName := fmt.Sprintf("%s-%s", SecretsManagerPolicyName, clusterName)
	secretsManagerPolicyPath := fmt.Sprintf("/%s/", clusterName)
	secretsManagerPolicyDescription := "Allow cluster services to manage manage secrets"

	secretsManagerPolicyDocument := `{
"Version": "2012-10-17",
"Statement": [
{
  "Effect": "Allow",
  "Sid": "SecretsManagerPermissions",
  "Action": [
	"secretsmanager:BatchGetSecretValue",
	"secretsmanager:ListSecrets",
	"secretsmanager:CreateSecret",
	"secretsmanager:DeleteSecret",
	"secretsmanager:GetSecretValue"
  ],
  "Resource": [
	"*"
  ]
}
]
}`
	createSecretsManagerPolicyInput := iam.CreatePolicyInput{
		PolicyName:     &secretsManagerPolicyName,
		Path:           &secretsManagerPolicyPath,
		Description:    &secretsManagerPolicyDescription,
		PolicyDocument: &secretsManagerPolicyDocument,
	}
	secrectsManagerPolicyResp, err := svc.CreatePolicy(c.Context, &createSecretsManagerPolicyInput)
	if err != nil {
		var ae smithy.APIError
		if errors.As(err, &ae) {
			if ae.ErrorCode() == "EntityAlreadyExists" {
				listPoliciesInput := iam.ListPoliciesInput{
					PathPrefix: &secretsManagerPolicyPath,
					Scope:      types.PolicyScopeTypeLocal,
				}
				listPoliciesOutput, err := svc.ListPolicies(c.Context, &listPoliciesInput)
				if err != nil {
					return nil, fmt.Errorf("failed to list policies to find existing %s policy: %w", secretsManagerPolicyName, err)
				}
				for _, policy := range listPoliciesOutput.Policies {
					if *policy.PolicyName == secretsManagerPolicyName {
						return &policy, nil
					}
				}

				return nil, fmt.Errorf("failed to find existing policy with name %s and path %s", secretsManagerPolicyName, secretsManagerPolicyPath)
			}
		}
		return nil, fmt.Errorf("failed to create DNS01 challenge policy %s: %w", secretsManagerPolicyName, err)
	}

	return secrectsManagerPolicyResp.Policy, nil
}

// CreateClusterAutoscalingPolicy creates the IAM policy to be used for cluster
// autoscaling to manage node pool sizes.
func (c *EksClient) CreateClusterAutoscalingPolicy(
	tags *[]types.Tag,
	clusterName string,
) (*types.Policy, error) {
	svc := iam.NewFromConfig(*c.AwsConfig)

	autoscalingPolicyName := fmt.Sprintf("%s-%s", AutoscalingPolicyName, clusterName)
	autoscalingPolicyPath := fmt.Sprintf("/%s/", clusterName)
	autoscalingPolicyDescription := "Allow cluster autoscaler to manage node pool sizes"
	autoscalingPolicyDocument := fmt.Sprintf(`{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": [
                "autoscaling:SetDesiredCapacity",
                "autoscaling:TerminateInstanceInAutoScalingGroup"
            ],
            "Resource": "*",
            "Condition": {
                "StringEquals": {
                    "aws:ResourceTag/k8s.io/cluster-autoscaler/%s": "owned"
                }
            }
        },
        {
            "Effect": "Allow",
            "Action": [
                "autoscaling:DescribeAutoScalingInstances",
                "autoscaling:DescribeAutoScalingGroups",
                "ec2:DescribeLaunchTemplateVersions",
                "autoscaling:DescribeTags",
                "autoscaling:DescribeLaunchConfigurations",
                "ec2:DescribeInstanceTypes"
            ],
            "Resource": "*"
        }
    ]
}`, clusterName)
	createAutoscalingPolicyInput := iam.CreatePolicyInput{
		PolicyName:     &autoscalingPolicyName,
		Path:           &autoscalingPolicyPath,
		Description:    &autoscalingPolicyDescription,
		PolicyDocument: &autoscalingPolicyDocument,
	}
	autoscalingPolicyResp, err := svc.CreatePolicy(c.Context, &createAutoscalingPolicyInput)
	if err != nil {
		var ae smithy.APIError
		if errors.As(err, &ae) {
			if ae.ErrorCode() == "EntityAlreadyExists" {
				listPoliciesInput := iam.ListPoliciesInput{
					PathPrefix: &autoscalingPolicyPath,
					Scope:      types.PolicyScopeTypeLocal,
				}
				listPoliciesOutput, err := svc.ListPolicies(c.Context, &listPoliciesInput)
				if err != nil {
					return nil, fmt.Errorf("failed to list policies to find existing %s policy: %w", autoscalingPolicyName, err)
				}
				for _, policy := range listPoliciesOutput.Policies {
					if *policy.PolicyName == autoscalingPolicyName {
						return &policy, nil
					}
				}

				return nil, fmt.Errorf("failed to find existing policy with name %s and path %s", autoscalingPolicyName, autoscalingPolicyPath)
			}
		}
		return nil, fmt.Errorf("failed to create cluster autoscaler management policy %s: %w", autoscalingPolicyName, err)
	}

	return autoscalingPolicyResp.Policy, nil
}

// DeletePolicies deletes the IAM policies.  If the policyArns slice is empty it
// returns without error.
func (c *EksClient) DeletePolicies(policyArns []string) ([]string, error) {
	// if roleArn is empty, there's nothing to delete
	if len(policyArns) == 0 {
		return policyArns, nil
	}

	for _, policyArn := range policyArns {
		svc := iam.NewFromConfig(*c.AwsConfig)

		deletePolicyInput := iam.DeletePolicyInput{
			PolicyArn: &policyArn,
		}
		_, err := svc.DeletePolicy(c.Context, &deletePolicyInput)
		if err != nil {
			var noSuchEntityErr *types.NoSuchEntityException
			if errors.As(err, &noSuchEntityErr) {
				continue
			} else {
				return policyArns, fmt.Errorf("failed to delete policy %s: %w", policyArn, err)
			}
		}
	}

	return policyArns, nil
}
