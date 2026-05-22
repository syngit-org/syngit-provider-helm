package helmprovider

import (
	"errors"
	"fmt"

	release "helm.sh/helm/v4/pkg/release/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"
)

const (
	helmSecretType   = "helm.sh/release.v1"
	helmOwnerLabel   = "owner"
	helmOwnerValue   = "helm"
	helmNameLabel    = "name"
	helmStatusLabel  = "status"
	helmVersionLabel = "version"
	releaseDataKey   = "release"
)

const ResourceFinderCommentPrefix = "syngit.resource-finder/v1: "

// ExtractValues decodes a Helm release secret and returns the user-supplied values.
func ExtractValues(secret *corev1.Secret) (*HelmValues, error) {
	rel, err := ExtractRelease(secret)
	if err != nil {
		return nil, err
	}

	if rel.Config == nil {
		rel.Config = map[string]interface{}{}
	}

	rawValues, err := yaml.Marshal(rel.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal values to YAML: %w", err)
	}

	rawValuesWithHeader := fmt.Sprintf("# %s%s/%s\n%s", ResourceFinderCommentPrefix, rel.Namespace, secret.Name, string(rawValues))

	return &HelmValues{
		Namespace:        rel.Namespace,
		Name:             rel.Name,
		RawValues:        rawValuesWithHeader,
		StructuredValues: rel.Config,
	}, nil
}

// ExtractRelease decodes a Helm release secret and returns the full chart and
// release information stored in it, using the upstream Helm v4 Release type.
// Labels are sourced from the secret metadata, since Helm stores them there
// rather than in the encoded release blob.
func ExtractRelease(secret *corev1.Secret) (*release.Release, error) {
	if !IsHelmSecret(secret) {
		return nil, errors.New("secret is not a valid Helm release secret")
	}

	releaseData, ok := secret.Data[releaseDataKey]
	if !ok {
		return nil, fmt.Errorf("secret does not contain %q key", releaseDataKey)
	}

	rel, err := decodeRelease(releaseData)
	if err != nil {
		return nil, fmt.Errorf("failed to decode release data: %w", err)
	}

	rel.Labels = secret.Labels

	return rel, nil
}
