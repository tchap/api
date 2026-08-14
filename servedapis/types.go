package servedapis

// Source identifies where a served API comes from.
type Source string

const (
	SourceOpenShiftCRD       Source = "openshift-crd"
	SourceOpenShiftAPIServer Source = "openshift-apiserver"
	SourceOAuthAPIServer     Source = "oauth-apiserver"
)

// ClusterProfile identifies the deployment profile of an OpenShift cluster.
type ClusterProfile string

const (
	ClusterProfileSelfManagedHA ClusterProfile = "SelfManagedHA"
	ClusterProfileHyperShift    ClusterProfile = "HyperShift"
)

// Scope identifies whether a resource is cluster-scoped or namespace-scoped.
type Scope string

const (
	ScopeCluster    Scope = "Cluster"
	ScopeNamespaced Scope = "Namespaced"
)

// ServedAPIEntry describes a single API resource expected to be served by an OpenShift cluster.
type ServedAPIEntry struct {
	Group    string `json:"group"`
	Version  string `json:"version"`
	Resource string `json:"resource"`
	Kind     string `json:"kind"`
	Scope    Scope  `json:"scope"`
	Source   Source `json:"source"`
}
