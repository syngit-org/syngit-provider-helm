package helmprovider

import (
	"regexp"

	corev1 "k8s.io/api/core/v1"
)

// Captures the Helm release name from a Helm storage secret
// name of the form "sh.helm.release.v1.<release>.v<revision>".
// Release names are DNS-safe (no dots), so the capture is
// unambiguous.
var helmSecretNameRegex = regexp.MustCompile(`^sh\.helm\.release\.v1\.([^.]+)\.v\d+$`)

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
