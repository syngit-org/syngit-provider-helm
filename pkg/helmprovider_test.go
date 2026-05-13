package helmprovider

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// encodeRelease encodes a helmRelease the same way Helm does:
// JSON -> gzip -> base64
func encodeRelease(rel *helmRelease) ([]byte, error) {
	jsonData, err := json.Marshal(rel)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write(jsonData); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}

	encoded := base64.StdEncoding.EncodeToString(buf.Bytes())
	return []byte(encoded), nil
}

func validHelmLabels(name string) map[string]string {
	return map[string]string{
		"owner":   "helm",
		"name":    name,
		"status":  "deployed",
		"version": "1",
	}
}

func validHelmSecret(releaseName, namespace string, config map[string]interface{}) *corev1.Secret {
	rel := &helmRelease{
		Name:      releaseName,
		Namespace: namespace,
		Config:    config,
	}
	data, _ := encodeRelease(rel)

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sh.helm.release.v1." + releaseName + ".v1",
			Namespace: namespace,
			Labels:    validHelmLabels(releaseName),
		},
		Type: corev1.SecretType(helmSecretType),
		Data: map[string][]byte{
			"release": data,
		},
	}
}

func TestIsHelmSecret(t *testing.T) {
	tests := []struct {
		name   string
		secret *corev1.Secret
		want   bool
	}{
		{
			name:   "nil secret",
			secret: nil,
			want:   false,
		},
		{
			name: "valid helm secret",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "sh.helm.release.v1.myapp.v1",
					Labels: validHelmLabels("myapp"),
				},
				Type: corev1.SecretType(helmSecretType),
			},
			want: true,
		},
		{
			name: "wrong secret type",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "sh.helm.release.v1.myapp.v1",
					Labels: validHelmLabels("myapp"),
				},
				Type: corev1.SecretTypeOpaque,
			},
			want: false,
		},
		{
			name: "wrong name pattern",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "my-random-secret",
					Labels: validHelmLabels("myapp"),
				},
				Type: corev1.SecretType(helmSecretType),
			},
			want: false,
		},
		{
			name: "missing labels",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: "sh.helm.release.v1.myapp.v1",
				},
				Type: corev1.SecretType(helmSecretType),
			},
			want: false,
		},
		{
			name: "missing owner label",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: "sh.helm.release.v1.myapp.v1",
					Labels: map[string]string{
						"name":    "myapp",
						"status":  "deployed",
						"version": "1",
					},
				},
				Type: corev1.SecretType(helmSecretType),
			},
			want: false,
		},
		{
			name: "wrong owner label value",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: "sh.helm.release.v1.myapp.v1",
					Labels: map[string]string{
						"owner":   "tiller",
						"name":    "myapp",
						"status":  "deployed",
						"version": "1",
					},
				},
				Type: corev1.SecretType(helmSecretType),
			},
			want: false,
		},
		{
			name: "higher version number",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "sh.helm.release.v1.myapp.v42",
					Labels: validHelmLabels("myapp"),
				},
				Type: corev1.SecretType(helmSecretType),
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsHelmSecret(tt.secret)
			if got != tt.want {
				t.Errorf("IsHelmSecret() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractValues(t *testing.T) {
	t.Run("valid secret with values", func(t *testing.T) {
		config := map[string]interface{}{
			"replicaCount": float64(3),
			"image": map[string]interface{}{
				"repository": "nginx",
				"tag":        "latest",
			},
		}

		secret := validHelmSecret("myapp", "production", config)
		result, err := ExtractValues(secret)
		if err != nil {
			t.Fatalf("ExtractValues() error = %v", err)
		}

		if result.Namespace != "production" {
			t.Errorf("Namespace = %q, want %q", result.Namespace, "production")
		}
		if result.Name != "myapp" {
			t.Errorf("Name = %q, want %q", result.Name, "myapp")
		}
		if result.StructuredValues["replicaCount"] != float64(3) {
			t.Errorf("StructuredValues[replicaCount] = %v, want 3", result.StructuredValues["replicaCount"])
		}
		imageMap, ok := result.StructuredValues["image"].(map[string]interface{})
		if !ok {
			t.Fatal("StructuredValues[image] is not a map")
		}
		if imageMap["repository"] != "nginx" {
			t.Errorf("image.repository = %v, want nginx", imageMap["repository"])
		}
		if result.RawValues == "" {
			t.Error("RawValues should not be empty")
		}
		wantHeader := "# " + ResourceFinderCommentPrefix + "production/sh.helm.release.v1.myapp.v1\n"
		if !strings.HasPrefix(result.RawValues, wantHeader) {
			t.Errorf("RawValues should start with %q, got %q", wantHeader, result.RawValues)
		}
	})

	t.Run("valid secret with no user values", func(t *testing.T) {
		secret := validHelmSecret("emptyapp", "default", nil)
		result, err := ExtractValues(secret)
		if err != nil {
			t.Fatalf("ExtractValues() error = %v", err)
		}

		if result.Namespace != "default" {
			t.Errorf("Namespace = %q, want %q", result.Namespace, "default")
		}
		if result.Name != "emptyapp" {
			t.Errorf("Name = %q, want %q", result.Name, "emptyapp")
		}
		if len(result.StructuredValues) != 0 {
			t.Errorf("StructuredValues should be empty, got %v", result.StructuredValues)
		}
	})

	t.Run("invalid secret rejected", func(t *testing.T) {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: "not-a-helm-secret",
			},
			Type: corev1.SecretTypeOpaque,
		}

		_, err := ExtractValues(secret)
		if err == nil {
			t.Fatal("ExtractValues() should return error for non-helm secret")
		}
	})

	t.Run("missing release key", func(t *testing.T) {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "sh.helm.release.v1.myapp.v1",
				Labels: validHelmLabels("myapp"),
			},
			Type: corev1.SecretType(helmSecretType),
			Data: map[string][]byte{},
		}

		_, err := ExtractValues(secret)
		if err == nil {
			t.Fatal("ExtractValues() should return error for missing release key")
		}
	})

	t.Run("corrupted release data", func(t *testing.T) {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "sh.helm.release.v1.myapp.v1",
				Labels: validHelmLabels("myapp"),
			},
			Type: corev1.SecretType(helmSecretType),
			Data: map[string][]byte{
				"release": []byte("not-valid-base64!@#$"),
			},
		}

		_, err := ExtractValues(secret)
		if err == nil {
			t.Fatal("ExtractValues() should return error for corrupted data")
		}
	})
}
