package helmprovider

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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
