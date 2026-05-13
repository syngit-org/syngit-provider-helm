package helmprovider

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"

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

var helmSecretNameRegex = regexp.MustCompile(`^sh\.helm\.release\.v1\..+\.v\d+$`)

// HelmValues contains the extracted user-supplied values from a Helm release secret.
type HelmValues struct {
	Namespace        string
	Name             string
	RawValues        string
	StructuredValues map[string]interface{}
}

// helmRelease represents the minimal structure of a Helm release JSON.
type helmRelease struct {
	Name      string                 `json:"name"`
	Namespace string                 `json:"namespace"`
	Config    map[string]interface{} `json:"config"`
}

// IsHelmSecret checks whether the given secret is a valid Helm release secret
// by validating its name pattern, type, and metadata labels.
func IsHelmSecret(secret *corev1.Secret) bool {
	if secret == nil {
		return false
	}

	if !helmSecretNameRegex.MatchString(secret.Name) {
		return false
	}

	if secret.Type != corev1.SecretType(helmSecretType) {
		return false
	}

	labels := secret.Labels
	if labels == nil {
		return false
	}

	if labels[helmOwnerLabel] != helmOwnerValue {
		return false
	}

	if _, ok := labels[helmNameLabel]; !ok {
		return false
	}

	if _, ok := labels[helmStatusLabel]; !ok {
		return false
	}

	if _, ok := labels[helmVersionLabel]; !ok {
		return false
	}

	return true
}

func IsHelmSecretByName(secretName string) bool {
	return helmSecretNameRegex.MatchString(secretName)
}

const ResourceFinderCommentPrefix = "syngit.resource-finder/v1: "

// ExtractValues decodes a Helm release secret and returns the user-supplied values.
func ExtractValues(secret *corev1.Secret) (*HelmValues, error) {
	if !IsHelmSecret(secret) {
		return nil, errors.New("secret is not a valid Helm release secret")
	}

	releaseData, ok := secret.Data[releaseDataKey]
	if !ok {
		return nil, fmt.Errorf("secret does not contain %q key", releaseDataKey)
	}

	release, err := decodeRelease(releaseData)
	if err != nil {
		return nil, fmt.Errorf("failed to decode release data: %w", err)
	}

	if release.Config == nil {
		release.Config = map[string]interface{}{}
	}

	rawValues, err := yaml.Marshal(release.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal values to YAML: %w", err)
	}

	rawValuesWithHeader := fmt.Sprintf("# %s%s/%s\n%s", ResourceFinderCommentPrefix, release.Namespace, secret.Name, string(rawValues))

	return &HelmValues{
		Namespace:        release.Namespace,
		Name:             release.Name,
		RawValues:        rawValuesWithHeader,
		StructuredValues: release.Config,
	}, nil
}

// decodeRelease decodes the Helm release data from its encoded form.
// Decoding chain: base64 (Helm's encoding) -> gzip decompress -> JSON unmarshal.
// Note: Kubernetes already handles its own base64 layer when populating secret.Data.
func decodeRelease(data []byte) (*helmRelease, error) {
	decoded, err := base64.StdEncoding.DecodeString(string(data))
	if err != nil {
		return nil, fmt.Errorf("base64 decode failed: %w", err)
	}

	reader, err := gzip.NewReader(bytes.NewReader(decoded))
	if err != nil {
		return nil, fmt.Errorf("gzip reader creation failed: %w", err)
	}
	defer reader.Close() // nolint:errcheck

	decompressed, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("gzip decompression failed: %w", err)
	}

	var release helmRelease
	if err := json.Unmarshal(decompressed, &release); err != nil {
		return nil, fmt.Errorf("JSON unmarshal failed: %w", err)
	}

	return &release, nil
}
