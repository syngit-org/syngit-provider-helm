package helmprovider

import "testing"

func TestReleaseNameFromSecretName(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"sh.helm.release.v1.podinfo.v1", "podinfo"},
		{"sh.helm.release.v1.podinfo.v42", "podinfo"},
		{"sh.helm.release.v1.my-app.v3", "my-app"},
		// Not a Helm secret name: returned unchanged.
		{"my-app-config", "my-app-config"},
		{"", ""},
	}
	for _, c := range cases {
		if got := GetReleaseNameFromSecretName(c.in); got != c.want {
			t.Errorf("releaseNameFromSecretName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
