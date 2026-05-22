package helmprovider

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	chartcommon "helm.sh/helm/v4/pkg/chart/common"
	chart "helm.sh/helm/v4/pkg/chart/v2"
	releasecommon "helm.sh/helm/v4/pkg/release/common"
	release "helm.sh/helm/v4/pkg/release/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// encodeRelease encodes a release.Release the same way Helm does:
// JSON -> gzip -> base64
func encodeRelease(rel *release.Release) ([]byte, error) {
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
	rel := &release.Release{
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

func TestExtractRelease(t *testing.T) {
	t.Run("full release with chart metadata", func(t *testing.T) {
		firstDeployed, _ := time.Parse(time.RFC3339, "2026-05-01T10:00:00Z")
		lastDeployed, _ := time.Parse(time.RFC3339, "2026-05-20T12:00:00Z")

		rel := &release.Release{
			Name:      "myapp",
			Namespace: "production",
			Version:   3,
			Manifest:  "apiVersion: v1\nkind: ConfigMap\n",
			Config: map[string]interface{}{
				"replicaCount": float64(2),
			},
			Info: &release.Info{
				FirstDeployed: firstDeployed,
				LastDeployed:  lastDeployed,
				Description:   "Install complete",
				Status:        releasecommon.Status("deployed"),
				Notes:         "Release notes",
			},
			Chart: &chart.Chart{
				Metadata: &chart.Metadata{
					Name:        "mychart",
					Version:     "1.2.3",
					AppVersion:  "4.5.6",
					Description: "A chart",
					APIVersion:  "v2",
					Type:        "application",
					Home:        "https://example.com",
					Sources:     []string{"https://github.com/example/mychart"},
					Keywords:    []string{"web", "nginx"},
					Maintainers: []*chart.Maintainer{
						{Name: "Alice", Email: "alice@example.com"},
					},
					Dependencies: []*chart.Dependency{
						{Name: "redis", Version: "1.0.0", Repository: "https://charts.example.com"},
					},
					Annotations: map[string]string{"category": "Infrastructure"},
				},
				Values: map[string]interface{}{
					"replicaCount": float64(1),
					"image":        "nginx:latest",
				},
				Schema: []byte(`{"type":"object"}`),
				Files: []*chartcommon.File{
					{Name: "README.md", Data: []byte("# Readme")},
				},
			},
		}

		data, err := encodeRelease(rel)
		if err != nil {
			t.Fatalf("encodeRelease() error = %v", err)
		}

		secretLabels := validHelmLabels("myapp")
		secretLabels["env"] = "prod"
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "sh.helm.release.v1.myapp.v3",
				Namespace: "production",
				Labels:    secretLabels,
			},
			Type: corev1.SecretType(helmSecretType),
			Data: map[string][]byte{"release": data},
		}

		got, err := ExtractRelease(secret)
		if err != nil {
			t.Fatalf("ExtractRelease() error = %v", err)
		}

		if got.Name != "myapp" || got.Namespace != "production" || got.Version != 3 {
			t.Errorf("unexpected release identity: %+v", got)
		}
		if got.Manifest == "" {
			t.Error("Manifest should not be empty")
		}
		if got.Labels["env"] != "prod" {
			t.Errorf("Labels[env] = %q, want %q", got.Labels["env"], "prod")
		}
		if got.Config["replicaCount"] != float64(2) {
			t.Errorf("Config[replicaCount] = %v, want 2", got.Config["replicaCount"])
		}
		if got.Info == nil || got.Info.Status != releasecommon.Status("deployed") {
			t.Errorf("Info = %+v, want status=deployed", got.Info)
		}
		if got.Chart == nil || got.Chart.Metadata == nil {
			t.Fatalf("Chart metadata missing")
		}
		meta := got.Chart.Metadata
		if meta.Name != "mychart" || meta.Version != "1.2.3" || meta.AppVersion != "4.5.6" {
			t.Errorf("chart metadata mismatch: %+v", meta)
		}
		if len(meta.Maintainers) != 1 || meta.Maintainers[0].Email != "alice@example.com" {
			t.Errorf("Maintainers mismatch: %+v", meta.Maintainers)
		}
		if len(meta.Dependencies) != 1 || meta.Dependencies[0].Name != "redis" {
			t.Errorf("Dependencies mismatch: %+v", meta.Dependencies)
		}
		if got.Chart.Values["replicaCount"] != float64(1) {
			t.Errorf("Chart.Values[replicaCount] = %v, want 1", got.Chart.Values["replicaCount"])
		}
		if len(got.Chart.Files) != 1 || got.Chart.Files[0].Name != "README.md" || string(got.Chart.Files[0].Data) != "# Readme" {
			t.Errorf("Chart.Files mismatch: %+v", got.Chart.Files)
		}
	})

	t.Run("invalid secret rejected", func(t *testing.T) {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "not-a-helm-secret"},
			Type:       corev1.SecretTypeOpaque,
		}

		if _, err := ExtractRelease(secret); err == nil {
			t.Fatal("ExtractRelease() should return error for non-helm secret")
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

		if _, err := ExtractRelease(secret); err == nil {
			t.Fatal("ExtractRelease() should return error for missing release key")
		}
	})
}
