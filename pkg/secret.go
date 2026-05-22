package helmprovider

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"

	release "helm.sh/helm/v4/pkg/release/v1"
)

// decodeRelease decodes the Helm release data from its encoded form.
// Decoding chain: base64 (Helm's encoding) -> gzip decompress -> JSON unmarshal.
// Note: Kubernetes already handles its own base64 layer when populating secret.Data.
func decodeRelease(data []byte) (*release.Release, error) {
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

	var rel release.Release
	if err := json.Unmarshal(decompressed, &rel); err != nil {
		return nil, fmt.Errorf("JSON unmarshal failed: %w", err)
	}

	return &rel, nil
}
