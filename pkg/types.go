package helmprovider

// HelmValues contains the extracted user-supplied values from a Helm release secret.
type HelmValues struct {
	Namespace        string
	Name             string
	RawValues        string
	StructuredValues map[string]interface{}
}

const HelmValuesAnnotation = "helm.syngit.io/helm-values"
