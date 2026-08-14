package servedapis

import (
	"slices"

	"github.com/blang/semver/v4"
)

// KubeAPIOverride defines additional Kubernetes APIs that should be expected when a specific
// OpenShift feature gate is enabled, beyond what upstream DefaultAPIResourceConfigSource serves.
type KubeAPIOverride struct {
	// GroupVersion is the API group and version, e.g. "admissionregistration.k8s.io/v1beta1".
	GroupVersion string
	// Kinds lists the resource Kinds served at this GroupVersion under this gate.
	// Explicit Kinds are required because the scheme registers historical types at old GVs
	// for backward compatibility, making it unreliable for alpha/beta GVs not in
	// DefaultAPIResourceConfigSource.
	Kinds []string
	// KubeVersionRange restricts which Kubernetes minor versions this override applies to.
	// nil means all versions.
	KubeVersionRange semver.Range
}

// kubeAPIOverridesByFeatureGate maps OpenShift feature gate names to additional Kubernetes
// APIs they enable beyond upstream DefaultAPIResourceConfigSource.
var kubeAPIOverridesByFeatureGate = map[string][]KubeAPIOverride{
	// MutatingAdmissionPolicy was v1alpha1 in k8s 1.33, promoted to v1beta1 in 1.34,
	// and graduated to v1 (and removed from this list) in 1.37.
	"MutatingAdmissionPolicy": {
		{
			GroupVersion:     "admissionregistration.k8s.io/v1alpha1",
			Kinds:            []string{"MutatingAdmissionPolicy", "MutatingAdmissionPolicyBinding"},
			KubeVersionRange: semver.MustParseRange(">=1.33.0 <1.37.0"),
		},
		{
			GroupVersion:     "admissionregistration.k8s.io/v1beta1",
			Kinds:            []string{"MutatingAdmissionPolicy", "MutatingAdmissionPolicyBinding"},
			KubeVersionRange: semver.MustParseRange(">=1.34.0 <1.37.0"),
		},
	},
	"VolumeGroupSnapshot": {
		{
			GroupVersion:     "groupsnapshot.storage.k8s.io/v1",
			Kinds:            []string{"VolumeGroupSnapshot", "VolumeGroupSnapshotClass", "VolumeGroupSnapshotContent"},
			KubeVersionRange: nil, // all versions
		},
	},
}

// KubeAPIOverridesByFeatureGate returns a copy of the map of OpenShift feature gate names
// to additional Kubernetes APIs they enable. Callers in origin use this at inventory
// generation time to augment DefaultAPIResourceConfigSource with OpenShift-specific gates.
func KubeAPIOverridesByFeatureGate() map[string][]KubeAPIOverride {
	out := make(map[string][]KubeAPIOverride, len(kubeAPIOverridesByFeatureGate))
	for gate, overrides := range kubeAPIOverridesByFeatureGate {
		out[gate] = slices.Clone(overrides)
	}
	return out
}
