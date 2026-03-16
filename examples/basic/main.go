package main

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"

	helmprovider "github.com/syngit-org/syngit-provider-helm/pkg"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func main() {
	// Build a fake Helm release secret as it would appear in a cluster.
	// In production, you would obtain this from the Kubernetes API instead.
	secret := buildFakeHelmSecret()

	// 1. Check that the secret is actually a Helm release secret.
	if !helmprovider.IsHelmSecret(secret) {
		log.Fatal("secret is not a Helm release secret")
	}
	fmt.Println("Secret is a valid Helm release secret")

	// 2. Extract user-supplied values.
	values, err := helmprovider.ExtractValues(secret)
	if err != nil {
		log.Fatalf("failed to extract values: %v", err)
	}

	fmt.Printf("Path:   %s\n", values.Path)
	fmt.Printf("Name:   %s\n", values.Name)
	fmt.Printf("Values:\n%s\n", values.RawValues)
}

// buildFakeHelmSecret creates a corev1.Secret that mimics what Helm 3 stores
// in a cluster after "helm install myapp ./mychart -n production --set replicaCount=3".
func buildFakeHelmSecret() *corev1.Secret {
	// This is the release JSON that Helm stores internally.
	// The "config" field holds only user-supplied values (--set / -f).
	release := map[string]interface{}{
		"name":      "myapp",
		"namespace": "production",
		"config": map[string]interface{}{
			"replicaCount": 3,
			"image": map[string]interface{}{
				"repository": "nginx",
				"tag":        "1.25.0",
			},
			"service": map[string]interface{}{
				"type": "ClusterIP",
				"port": 8080,
			},
		},
	}

	// Encode: JSON -> gzip -> base64  (what Helm does before storing the secret)
	jsonData, _ := json.Marshal(release)

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	gz.Write(jsonData) // nolint:errcheck
	gz.Close()         // nolint:errcheck

	encoded := base64.StdEncoding.EncodeToString(buf.Bytes())

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sh.helm.release.v1.myapp.v1",
			Namespace: "production",
			Labels: map[string]string{
				"owner":   "helm",
				"name":    "myapp",
				"status":  "deployed",
				"version": "1",
			},
		},
		Type: corev1.SecretType("helm.sh/release.v1"),
		Data: map[string][]byte{
			"release": []byte(encoded),
		},
	}
}
