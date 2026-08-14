package servedapis

import (
	"bytes"
	"flag"
	"fmt"
	"go/format"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	configv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/api/features"
	"github.com/openshift/api/servedapis"
	kyaml "sigs.k8s.io/yaml"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	annotationIBMCloudManaged = "include.release.openshift.io/ibm-cloud-managed"
	annotationSelfManagedHA   = "include.release.openshift.io/self-managed-high-availability"
	annotationFeatureSet      = "release.openshift.io/feature-set"
	annotationFeatureGate     = "release.openshift.io/feature-gate"
	featureSetDefault         = "Default"
)

// defaultEnabledFeatureGates returns the set of feature gate names that are enabled
// in the Default feature set for any cluster profile and any supported OCP version.
func defaultEnabledFeatureGates() map[string]bool {
	enabled := map[string]bool{}
	for _, byProfile := range features.AllFeatureSets() {
		for _, byFeatureSet := range byProfile {
			fged, ok := byFeatureSet[configv1.Default]
			if !ok {
				continue
			}
			for _, fg := range fged.Enabled {
				enabled[string(fg.FeatureGateAttributes.Name)] = true
			}
		}
	}
	return enabled
}

// WriteServedAPIInventory generates servedapis/zz_generated_openshift.go from CRD manifests.
type WriteServedAPIInventory struct {
	// SourceDir is the root of the openshift/api repository. The generator discovers all
	// zz_generated.crd-manifests/ subdirectories under it automatically.
	SourceDir   string
	GoOutputDir string
}

func (o *WriteServedAPIInventory) AddFlags(fs *flag.FlagSet) {
	fs.StringVar(&o.SourceDir, "source-dir", o.SourceDir, "Root of the openshift/api repository. All zz_generated.crd-manifests/ subdirectories are scanned for CRD YAML files.")
	fs.StringVar(&o.GoOutputDir, "go-output-dir", o.GoOutputDir, "Directory to write zz_generated_openshift.go into.")
}

func (o *WriteServedAPIInventory) Run() error {
	required, optional, err := o.buildInventory()
	if err != nil {
		return err
	}
	return o.writeGoFile(required, optional)
}

// buildInventory reads CRD manifests and assembles the per-profile API lists.
func (o *WriteServedAPIInventory) buildInventory() (
	required map[servedapis.ClusterProfile][]servedapis.ServedAPIEntry,
	optional map[servedapis.ClusterProfile][]servedapis.ServedAPIEntry,
	err error,
) {
	profiles := []servedapis.ClusterProfile{
		servedapis.ClusterProfileSelfManagedHA,
		servedapis.ClusterProfileHyperShift,
	}

	// seen tracks (profile, group/version/resource) tuples already added to avoid duplicates.
	seen := map[servedapis.ClusterProfile]map[string]bool{}
	required = map[servedapis.ClusterProfile][]servedapis.ServedAPIEntry{}
	optional = map[servedapis.ClusterProfile][]servedapis.ServedAPIEntry{}
	for _, p := range profiles {
		seen[p] = map[string]bool{}
	}

	// Parse CRD manifests.
	entries, err := o.loadCRDEntries()
	if err != nil {
		return nil, nil, err
	}
	for _, e := range entries {
		for _, p := range profilesForEntry(e) {
			key := e.entry.Group + "/" + e.entry.Version + "/" + e.entry.Resource
			if seen[p][key] {
				continue
			}
			seen[p][key] = true
			required[p] = append(required[p], e.entry)
		}
	}

	// Merge hardcoded aggregated API server entries (same for all profiles).
	for _, e := range aggregatedAPIServerEntries() {
		for _, p := range profiles {
			key := e.Group + "/" + e.Version + "/" + e.Resource
			if seen[p][key] {
				continue
			}
			seen[p][key] = true
			required[p] = append(required[p], e)
		}
	}

	// Optional entries are the same for all profiles.
	for _, e := range optionalAPIEntries() {
		for _, p := range profiles {
			optional[p] = append(optional[p], e)
		}
	}

	// Sort for stable output.
	for _, p := range profiles {
		sort.Slice(required[p], byGVR(required[p]))
		sort.Slice(optional[p], byGVR(optional[p]))
	}

	return required, optional, nil
}

// crdFileEntry pairs a parsed ServedAPIEntry with the profiles it applies to.
type crdFileEntry struct {
	entry    servedapis.ServedAPIEntry
	profiles []servedapis.ClusterProfile
}

// crdDirSkipPrefixes lists repository-relative path prefixes (using the OS path separator)
// of zz_generated.crd-manifests parent directories to skip during the walk.
// These directories either contain non-deployed CRDs, platform-specific CRDs, or
// alpha versions that have been superseded by stable versions in sibling packages.
var crdDirSkipPrefixes = func() []string {
	join := filepath.Join
	return []string{
		// config/v1alpha1 and v1alpha2: alpha versions superseded by config/v1
		join("config", "v1alpha1"),
		join("config", "v1alpha2"),
		// etcd: pacemakerclusters CRDs are only deployed on pacemaker-based HA etcd clusters,
		// not on standard Default clusters. The CRDs lack a feature-gate annotation that would
		// normally cause the generator to filter them; they are listed explicitly in optional_apis.go.
		"etcd",
		// example: documentation/example CRDs, never deployed in production
		"example",
		// insights/v1alpha1 and v1alpha2: superseded by insights/v1
		join("insights", "v1alpha1"),
		join("insights", "v1alpha2"),
		// machineconfiguration/v1alpha1: alpha resources not yet deployed on Default clusters
		join("machineconfiguration", "v1alpha1"),
		// network: OpenShift SDN virtual resources (clusternetworks, hostsubnets, etc.) are only
		// served on clusters using the OpenShift SDN network plugin; OVN-Kubernetes clusters do
		// not serve them. The CRDs lack a feature-gate annotation; they are listed in optional_apis.go.
		"network",
		// operator/v1alpha1: alpha versions superseded by operator/v1
		join("operator", "v1alpha1"),
		// sharedresource: TechPreview SharedResource CSI Driver CRDs are missing the
		// release.openshift.io/feature-gate annotation that would normally filter them.
		// Listed explicitly in optional_apis.go.
		"sharedresource",
	}
}()

// loadCRDEntries walks SourceDir for all zz_generated.crd-manifests/ directories and returns
// entries for every CRD file that belongs to the Default feature set.
func (o *WriteServedAPIInventory) loadCRDEntries() ([]crdFileEntry, error) {
	defaultGates := defaultEnabledFeatureGates()

	var result []crdFileEntry
	err := filepath.WalkDir(o.SourceDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			if !strings.HasSuffix(d.Name(), ".yaml") {
				return nil
			}
			// Only process files directly inside a zz_generated.crd-manifests/ directory.
			if filepath.Base(filepath.Dir(path)) != "zz_generated.crd-manifests" {
				return nil
			}
			entries, err := parseCRDFile(path, defaultGates)
			if err != nil {
				return fmt.Errorf("parsing %q: %w", path, err)
			}
			result = append(result, entries...)
			return nil
		}

		// Skip well-known non-CRD directories.
		switch d.Name() {
		case "vendor", "tests", ".git":
			return filepath.SkipDir
		}

		// Skip package directories on the skip list.
		// Note: the skip list only covers packages whose CRDs are NOT deployed on Default
		// clusters under any configuration. CRDs that ARE deployed on some configurations
		// (e.g. SDN-only, pacemaker etcd) go in the optional list in optional_apis.go.
		rel, err := filepath.Rel(o.SourceDir, path)
		if err != nil {
			return err
		}
		for _, prefix := range crdDirSkipPrefixes {
			if rel == prefix || strings.HasPrefix(rel, prefix+string(filepath.Separator)) {
				return filepath.SkipDir
			}
		}

		return nil
	})
	return result, err
}

// parseCRDFile reads a single CRD YAML file and returns crdFileEntries for every
// served version that belongs to the Default feature set.
// defaultGates is the set of feature gate names enabled in Default, used to filter
// CRDs annotated with release.openshift.io/feature-gate.
func parseCRDFile(path string, defaultGates map[string]bool) ([]crdFileEntry, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	obj := map[string]interface{}{}
	if err := kyaml.Unmarshal(raw, &obj); err != nil {
		return nil, err
	}

	annotations, _, _ := unstructured.NestedStringMap(obj, "metadata", "annotations")

	// If the CRD is gated by a specific feature gate, only include it when that
	// gate is enabled in the Default feature set.
	if gate, ok := annotations[annotationFeatureGate]; ok && gate != "" {
		if !defaultGates[gate] {
			return nil, nil
		}
	}

	// Only include CRDs that are deployed on the Default feature set.
	if !isDefaultFeatureSet(annotations) {
		return nil, nil
	}

	profiles := profilesFromAnnotations(annotations)
	if len(profiles) == 0 {
		return nil, nil
	}

	group, _, _ := unstructured.NestedString(obj, "spec", "group")
	plural, _, _ := unstructured.NestedString(obj, "spec", "names", "plural")
	kind, _, _ := unstructured.NestedString(obj, "spec", "names", "kind")
	scopeStr, _, _ := unstructured.NestedString(obj, "spec", "scope")
	scope := servedapis.Scope(scopeStr)

	versions, _, _ := unstructured.NestedSlice(obj, "spec", "versions")

	var entries []crdFileEntry
	for _, v := range versions {
		versionMap, ok := v.(map[string]interface{})
		if !ok {
			continue
		}
		served, _, _ := unstructured.NestedBool(versionMap, "served")
		if !served {
			continue
		}
		versionName, _, _ := unstructured.NestedString(versionMap, "name")
		entries = append(entries, crdFileEntry{
			entry: servedapis.ServedAPIEntry{
				Group:    group,
				Version:  versionName,
				Resource: plural,
				Kind:     kind,
				Scope:    scope,
				Source:   servedapis.SourceOpenShiftCRD,
			},
			profiles: profiles,
		})
	}
	return entries, nil
}

// isDefaultFeatureSet returns true when the CRD should be deployed on the Default feature set.
//
// Rules:
//   - Annotation absent → applies to all feature sets, including Default.
//   - Annotation present and contains "Default" → Default feature set.
//   - Annotation present, does not contain "Default" → not a Default CRD, skip.
func isDefaultFeatureSet(annotations map[string]string) bool {
	v, ok := annotations[annotationFeatureSet]
	if !ok || v == "" {
		return true
	}
	for _, fs := range strings.Split(v, ",") {
		if strings.TrimSpace(fs) == featureSetDefault {
			return true
		}
	}
	return false
}

// profilesFromAnnotations returns the cluster profiles a CRD applies to,
// derived from include.release.openshift.io/* annotations.
func profilesFromAnnotations(annotations map[string]string) []servedapis.ClusterProfile {
	var profiles []servedapis.ClusterProfile
	if annotations[annotationIBMCloudManaged] == "true" {
		profiles = append(profiles, servedapis.ClusterProfileHyperShift)
	}
	if annotations[annotationSelfManagedHA] == "true" {
		profiles = append(profiles, servedapis.ClusterProfileSelfManagedHA)
	}
	return profiles
}

// profilesForEntry returns the profiles a crdFileEntry applies to.
func profilesForEntry(e crdFileEntry) []servedapis.ClusterProfile {
	return e.profiles
}

func byGVR(entries []servedapis.ServedAPIEntry) func(i, j int) bool {
	return func(i, j int) bool {
		a, b := entries[i], entries[j]
		if a.Group != b.Group {
			return a.Group < b.Group
		}
		if a.Version != b.Version {
			return a.Version < b.Version
		}
		return a.Resource < b.Resource
	}
}

// writeGoFile renders the generated Go source file.
func (o *WriteServedAPIInventory) writeGoFile(
	required map[servedapis.ClusterProfile][]servedapis.ServedAPIEntry,
	optional map[servedapis.ClusterProfile][]servedapis.ServedAPIEntry,
) error {
	if err := os.MkdirAll(o.GoOutputDir, 0755); err != nil {
		return err
	}

	var buf bytes.Buffer
	if err := outputTemplate.Execute(&buf, templateData{
		RequiredSelfManagedHA: toTmplEntries(required[servedapis.ClusterProfileSelfManagedHA]),
		RequiredHyperShift:    toTmplEntries(required[servedapis.ClusterProfileHyperShift]),
		OptionalSelfManagedHA: toTmplEntries(optional[servedapis.ClusterProfileSelfManagedHA]),
		OptionalHyperShift:    toTmplEntries(optional[servedapis.ClusterProfileHyperShift]),
	}); err != nil {
		return fmt.Errorf("rendering template: %w", err)
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return fmt.Errorf("formatting generated source: %w\n\n%s", err, buf.String())
	}

	outPath := filepath.Join(o.GoOutputDir, "zz_generated_openshift.go")
	return os.WriteFile(outPath, formatted, 0644)
}

// tmplEntry holds all string fields so the template can use them with a plain quote func.
type tmplEntry struct {
	Group, Version, Resource, Kind, Scope, Source string
}

type templateData struct {
	RequiredSelfManagedHA []tmplEntry
	RequiredHyperShift    []tmplEntry
	OptionalSelfManagedHA []tmplEntry
	OptionalHyperShift    []tmplEntry
}

func toTmplEntries(in []servedapis.ServedAPIEntry) []tmplEntry {
	out := make([]tmplEntry, len(in))
	for i, e := range in {
		out[i] = tmplEntry{
			Group:    e.Group,
			Version:  e.Version,
			Resource: e.Resource,
			Kind:     e.Kind,
			Scope:    string(e.Scope),
			Source:   string(e.Source),
		}
	}
	return out
}

// scopeConst maps a scope value to its constant name.
var scopeConst = map[string]string{
	string(servedapis.ScopeCluster):    "ScopeCluster",
	string(servedapis.ScopeNamespaced): "ScopeNamespaced",
}

// sourceConst maps a source value to its constant name.
var sourceConst = map[string]string{
	string(servedapis.SourceOpenShiftCRD):       "SourceOpenShiftCRD",
	string(servedapis.SourceOpenShiftAPIServer):  "SourceOpenShiftAPIServer",
	string(servedapis.SourceOAuthAPIServer):      "SourceOAuthAPIServer",
}

var outputTemplate = template.Must(template.New("inventory").Funcs(template.FuncMap{
	"quote":      func(s string) string { return fmt.Sprintf("%q", s) },
	"scopeConst": func(s string) string { return scopeConst[s] },
	"srcConst":   func(s string) string { return sourceConst[s] },
}).Parse(`// Code generated by write-served-api-inventory. DO NOT EDIT.

package servedapis

var requiredSelfManagedHA = []ServedAPIEntry{
{{- range .RequiredSelfManagedHA}}
	{Group: {{quote .Group}}, Version: {{quote .Version}}, Resource: {{quote .Resource}}, Kind: {{quote .Kind}}, Scope: {{scopeConst .Scope}}, Source: {{srcConst .Source}}},
{{- end}}
}

var optionalSelfManagedHA = []ServedAPIEntry{
{{- range .OptionalSelfManagedHA}}
	{Group: {{quote .Group}}, Version: {{quote .Version}}, Resource: {{quote .Resource}}, Kind: {{quote .Kind}}, Scope: {{scopeConst .Scope}}, Source: {{srcConst .Source}}},
{{- end}}
}

var requiredHyperShift = []ServedAPIEntry{
{{- range .RequiredHyperShift}}
	{Group: {{quote .Group}}, Version: {{quote .Version}}, Resource: {{quote .Resource}}, Kind: {{quote .Kind}}, Scope: {{scopeConst .Scope}}, Source: {{srcConst .Source}}},
{{- end}}
}

var optionalHyperShift = []ServedAPIEntry{
{{- range .OptionalHyperShift}}
	{Group: {{quote .Group}}, Version: {{quote .Version}}, Resource: {{quote .Resource}}, Kind: {{quote .Kind}}, Scope: {{scopeConst .Scope}}, Source: {{srcConst .Source}}},
{{- end}}
}

// ForProfile returns the required and optional OpenShift API entries for a given cluster profile.
// Only supports the Default feature set — tests should skip on TechPreview/DevPreview.
// Does not include Kubernetes built-in APIs; those are generated separately in origin.
func ForProfile(clusterProfile ClusterProfile) (required, optional []ServedAPIEntry, found bool) {
	switch clusterProfile {
	case ClusterProfileSelfManagedHA:
		return requiredSelfManagedHA, optionalSelfManagedHA, true
	case ClusterProfileHyperShift:
		return requiredHyperShift, optionalHyperShift, true
	default:
		return nil, nil, false
	}
}
`))
