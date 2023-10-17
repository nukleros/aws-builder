package eks

import (
	"bytes"
	"crypto/sha1"
	"crypto/tls"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/aws/smithy-go"
)

// CreateOidcProvider creates a new identity provider in IAM for the EKS cluster.
// This enables IAM roles for Kubernetes service accounts (IRSA).
func (c *EksClient) CreateOidcProvider(
	tags *[]types.Tag,
	providerUrl string,
) (string, error) {
	svc := iam.NewFromConfig(*c.AwsConfig)

	var oidcProviderArn string
	// get the OIDC provider server certificate thumbprint
	parsedUrl, err := url.Parse(providerUrl)
	if err != nil {
		return oidcProviderArn, fmt.Errorf("failed to parse OIDC provider URL: %w", err)
	}
	conn, err := tls.Dial("tcp", fmt.Sprintf("%s:%d", parsedUrl.Hostname(), 443), &tls.Config{})
	if err != nil {
		return oidcProviderArn, fmt.Errorf("failed to connect to OIDC provider: %w", err)
	}
	cert := conn.ConnectionState().PeerCertificates[len(conn.ConnectionState().PeerCertificates)-1]
	thumbprint := sha1.Sum(cert.Raw)
	var thumbprintString string
	for _, t := range thumbprint {
		var buf bytes.Buffer
		fmt.Fprintf(&buf, "%02X", t)
		thumbprintString = thumbprintString + strings.ToLower(buf.String())
	}

	createOidcProviderInput := iam.CreateOpenIDConnectProviderInput{
		ClientIDList:   []string{"sts.amazonaws.com"},
		ThumbprintList: []string{thumbprintString},
		Url:            &providerUrl,
	}
	resp, err := svc.CreateOpenIDConnectProvider(c.Context, &createOidcProviderInput)
	if err != nil {
		var ae smithy.APIError
		if errors.As(err, &ae) {
			if ae.ErrorCode() == "EntityAlreadyExists" {
				// find existing provider to return ARN
				listOidcProvidersOutput, err := svc.ListOpenIDConnectProviders(c.Context, &iam.ListOpenIDConnectProvidersInput{})
				if err != nil {
					return "", fmt.Errorf("failed to list OIDC providers to find existing provider: %w", err)
				}
				providerFound := false
				var oidcProviderArn string
				for _, providerArn := range listOidcProvidersOutput.OpenIDConnectProviderList {
					if strings.Contains(*providerArn.Arn, parsedUrl.Hostname()) {
						oidcProviderArn = *providerArn.Arn
						providerFound = true
					}
				}
				if !providerFound {
					return "", fmt.Errorf("failed to find existing OIDC provider with URL %s", providerUrl)
				}

				return oidcProviderArn, nil
			}
		}
		return oidcProviderArn, fmt.Errorf("failed to create IAM identity provider: %w", err)
	}
	oidcProviderArn = *resp.OpenIDConnectProviderArn

	return oidcProviderArn, nil
}

// DeleteOIdCProvider deletes an OIDC identity cluster in IAM.  If  an empty ARN
// is provided or if not found it returns without error.
func (c *EksClient) DeleteOidcProvider(oidcProviderArn string) error {
	// if clusterName is empty, there's nothing to delete
	if oidcProviderArn == "" {
		return nil
	}

	svc := iam.NewFromConfig(*c.AwsConfig)

	deleteOidcProviderInput := iam.DeleteOpenIDConnectProviderInput{
		OpenIDConnectProviderArn: &oidcProviderArn,
	}
	_, err := svc.DeleteOpenIDConnectProvider(c.Context, &deleteOidcProviderInput)
	if err != nil {
		var noSuchEntityErr *types.NoSuchEntityException
		if errors.As(err, &noSuchEntityErr) {
			return nil
		} else {
			return fmt.Errorf("failed to delete IAM identity provider %s: %w", oidcProviderArn, err)
		}
	}

	return nil
}
