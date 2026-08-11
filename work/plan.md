# E2E Test for Served API Inventory Validation

## Context

Ben wants a statically defined list of all GVKs expected to be served by an OpenShift cluster. An e2e test compares this list against what the cluster actually serves — failing on both unexpected and missing APIs. The test runs as part of the conformance suite on both standalone OpenShift and HyperShift clusters.

The list must be generated using tooling so it can be regenerated during Kubernetes rebases. Sources: Kubernetes built-in APIs + OpenShift CRDs (from openshift/api) + aggregated API servers (openshift-apiserver, oauth-apiserver).

No such test or tooling exists today. The closest patterns are:
- `origin/test/extended/etcd/etcd_storage_path.go` — static map of persisted resources with bidirectional validation against discovery
- `origin/test/extended/cli/explain.go` — static lists of expected API resources

## Test Flow

The e2e test in origin builds the expected API set from three sources, queries the
cluster, and compares:

```
 1. Detect cluster state
    ├── profile      ← Infrastructure.Status.ControlPlaneTopology (Hypershift vs SelfManaged)
    ├── featureSet   ← FeatureGate("cluster").Spec.FeatureSet (Default, TechPreview, ...)
    └── enabledGates ← FeatureGate("cluster").Status.FeatureGates (list of active gates)

 2. Build expected API set
    │
    ├─ OpenShift APIs (from vendored openshift/api servedapis package)
    │  servedapis.ForProfile(profile, featureSet)
    │  → OpenShift CRDs (feature-set-aware: 5 CRDs are TP-only, absent on Default)
    │  → Aggregated API server resources (openshift-apiserver, oauth-apiserver)
    │  → Optional operator APIs (monitoring, OLM, machine-api, etc.)
    │
    ├─ Kubernetes stable APIs (derived from vendored k8s code, no manual list)
    │  DefaultAPIResourceConfigSource() → enabled GroupVersions
    │  → for each GV: clientgoscheme.Scheme.KnownTypes(gv)
    │  → filter (remove List, Options, subresource-only types)
    │  → UnsafeGuessKindToResource() → GVRs
    │
    └─ Kubernetes overrides (OpenShift feature gates that enable extra k8s APIs)
       for each enabledGate: look up override entries from map in openshift/api
       entries have explicit Kinds (not derived from scheme — see limitation below)
       e.g. MutatingAdmissionPolicy → MutatingAdmissionPolicy, MutatingAdmissionPolicyBinding
            at admissionregistration.k8s.io/v1beta1
       → UnsafeGuessKindToResource() on each Kind → GVRs

 3. Query cluster
    discovery.ServerGroupsAndResources()
    → filter out subresources (names containing "/")

 4. Compare (bidirectional)
    missing  = expected(required) - actual   → FAIL
    unknown  = actual - expected(all)        → FAIL
    optional absent                          → OK (log only)
```

## Architecture

```
openshift/api                              origin
┌──────────────────────────────┐     ┌─────────────────────────────┐
│ payload-command/cmd/         │     │ test/extended/apiserver/    │
│   write-served-api-inventory │     │   served_api_inventory.go   │
│                              │     │                             │
│ payload-command/servedapis/  │     │ Builds expected set from:   │
│   generator.go               │────>│  · vendored servedapis pkg  │
│   aggregated_apis.go         │     │  · clientgoscheme.Scheme    │
│   optional_apis.go           │     │  · DefaultAPIResourceConfig │
│                              │     │  · kube API override map    │
│ features/                    │     │                             │
│   kube_api_overrides.go      │────>│ Queries cluster discovery,  │
│                              │     │ compares bidirectionally    │
│ servedapis/                  │     └─────────────────────────────┘
│   types.go                   │
│   zz_generated.served_apis.go│  ← generated, vendored into origin
│                              │
│ payload-manifests/served-apis│
│   servedAPIs-*.yaml          │  ← generated, human-readable
└──────────────────────────────┘
```

### Key insight: no manual Kubernetes API list

Instead of maintaining a `kube_apis.go` with ~100+ Kubernetes resource entries, the
Kubernetes API inventory is derived programmatically at test time in origin using:

1. **`clientgoscheme.Scheme`** — registers all Kubernetes API types. Updated automatically
   when `k8s.io/client-go` is vendored during rebases.
2. **`DefaultAPIResourceConfigSource()`** from `k8s.io/kubernetes/pkg/controlplane/instance.go` —
   lists which GroupVersions are enabled/disabled by default. Also vendored.
3. **`meta.UnsafeGuessKindToResource()`** — converts Kind → plural resource name.
   Works correctly for all standard Kubernetes types.
4. **Type filtering** — remove List types (suffix "List"), Options types (suffix "Options"),
   and a small blocklist of subresource-only types (~5 entries: Binding, Eviction, Scale,
   TokenRequest, etc.).

This means **no manual maintenance for Kubernetes APIs during rebases**. The expected
list is always correct because it's derived from the same vendored code the cluster uses.

### Limitation: scheme is unreliable for Kubernetes-disabled beta GVs

The scheme registers types at their historical GroupVersions for serialization/conversion
backward compatibility, not just for serving. For example,
`admissionregistration.k8s.io/v1beta1` in the scheme includes 6 types, but only 2
(`MutatingAdmissionPolicy`, `MutatingAdmissionPolicyBinding`) are actually served —
the other 4 graduated to v1 long ago but remain registered so old objects can still
be decoded.

This only affects beta GVs where a **v1 already exists** in the same group — promoted
types linger at the old GV. Kubernetes puts these superseded beta GVs in its disabled
list (`betaAPIGroupVersionsDisabledByDefault`).

For GVs that `DefaultAPIResourceConfigSource()` **enables** — whether v1 or beta —
the scheme is reliable:
- **Stable GVs**: types at v1 are the final destination, they don't graduate away
- **Beta GVs of new groups** (e.g., a new API starting at `v1beta1` with its feature
  gate beta-enabled-by-default): no v1 exists yet, so no graduated types — scheme
  matches what's served

**Consequence**: Derive resources from the scheme for ALL GVs that
`DefaultAPIResourceConfigSource()` enables. Only the **override map** (for GVs that
OpenShift enables beyond Kubernetes defaults, like `admissionregistration.k8s.io/v1beta1`)
needs explicit Kinds, because those GVs are Kubernetes-disabled and have graduated types.

### Kubernetes API overrides from OpenShift feature gates

OpenShift enables certain alpha/beta Kubernetes APIs when specific OpenShift feature gates
are active (e.g., `MutatingAdmissionPolicy` gate enables `MutatingAdmissionPolicy` and
`MutatingAdmissionPolicyBinding` at `admissionregistration.k8s.io/v1beta1`).

These APIs are NOT in `DefaultAPIResourceConfigSource()` — they're added to the kube-apiserver's
`--runtime-config` by the cluster-kube-apiserver-operator. And because the scheme is
unreliable for alpha/beta GVs (see above), the override map must list **explicit Kinds**,
not just GroupVersions.

The mapping of OpenShift feature gate → additional Kubernetes API resources lives in
openshift/api, co-located with the feature gate definitions. This could be an extension
of the feature gate builder pattern in `features/features.go` or a separate registry
in `features/` or `servedapis/`.

Currently this data is maintained separately in
`cluster-kube-apiserver-operator/pkg/operator/configobservation/apienablement/observe_runtime_config.go`
(`defaultGroupVersionsByFeatureGate`). Moving it to openshift/api means:
- The e2e test in origin gets it for free via vendoring
- The kube-apiserver operator can also consume it instead of maintaining its own copy
- It's versioned alongside the feature gates themselves

At test time, the e2e test reads the cluster's enabled feature gates from
`FeatureGate("cluster").Status.FeatureGates`, looks up override GVs for each
enabled gate, and adds them to the expected Kubernetes API set.

Note: openshift/api only partially vendors `k8s.io/api` (missing resource.k8s.io,
discovery.k8s.io/v1, etc.), so the scheme-based derivation runs in origin which has
the complete vendor tree.

---

## Part A: Static API Inventory Generator (openshift/api)

### A1. Types Package — `servedapis/`

**`servedapis/types.go`** — types shared between the generator and the e2e test:

```go
package servedapis

type Source string
const (
    SourceCoreKube           Source = "core-kube"
    SourceOpenShiftCRD       Source = "openshift-crd"
    SourceOpenShiftAPIServer Source = "openshift-apiserver"
    SourceOAuthAPIServer     Source = "oauth-apiserver"
    SourceOptional           Source = "optional"
)

type ServedAPIEntry struct {
    Group    string `json:"group" yaml:"group"`
    Version  string `json:"version" yaml:"version"`
    Resource string `json:"resource" yaml:"resource"`
    Kind     string `json:"kind" yaml:"kind"`
    Scope    string `json:"scope" yaml:"scope"`       // "Namespaced" or "Cluster"
    Source   Source `json:"source" yaml:"source"`
}
```

**`servedapis/zz_generated.served_apis.go`** — generated file containing the full inventory data as Go literals. Provides:
```go
func ForProfile(clusterProfile, featureSet string) []ServedAPIEntry
```

This file gets vendored into origin automatically.

### A2. Generator Tool — `payload-command/cmd/write-served-api-inventory/`

Standalone binary following the pattern of `write-available-featuresets`. Takes `--asset-output-dir` flag for YAML output and `--go-output-dir` for the generated Go file.

**Generator logic** in `payload-command/servedapis/`:

1. **`generator.go`** — main orchestration:
   - Reads CRD manifests from `payload-manifests/crds/`
   - Parses filename suffixes to determine (ClusterProfile, FeatureSet) applicability
   - Extracts served GVRs from each CRD's `spec.versions[].served`, `spec.group`, `spec.names.plural/kind`, `spec.scope`
   - Merges with hardcoded aggregated API server entries
   - Merges with optional API entries
   - Outputs one YAML file per (ClusterProfile, FeatureSet) combination
   - Outputs `zz_generated.served_apis.go`
   - **Does NOT include Kubernetes built-in APIs** — those are derived at test time in origin

2. **`aggregated_apis.go`** — hardcoded lists for openshift-apiserver (9 groups, ~35 resources) and oauth-apiserver (2 groups, ~10 resources). Based on the exploration findings. Changes very rarely.

3. **`optional_apis.go`** — APIs from optional operators marked `Source: optional`:
   - monitoring.coreos.com (alertmanagers, prometheuses, servicemonitors, etc.)
   - operators.coreos.com (clusterserviceversions, subscriptions, etc.)
   - packages.operators.coreos.com (packagemanifests)
   - machine.openshift.io (machines, machinesets, machinehealthchecks)
   - autoscaling.openshift.io (clusterautoscalers, machineautoscalers)
   - metal3.io (baremetalhosts, provisionings, etc.)
   - tuned.openshift.io, performance.openshift.io
   - helm.openshift.io
   - cloudcredential.openshift.io

### A3. CRD Variant Detection

Two mechanisms determine which (ClusterProfile, FeatureSet) a CRD applies to:

**1. Filename suffix** — determines which CRD schema variant to use:
- No suffix: same schema for all variants
- `-Default`, `-TechPreviewNoUpgrade`: feature-set-specific schema
- `-Hypershift`, `-SelfManagedHA`: profile-specific
- `-SelfManagedHA-TechPreviewNoUpgrade`: compound

**2. `release.openshift.io/feature-set` annotation** — determines on which clusters
the CRD is deployed. This is the critical one for inventory:
- Absent or empty: CRD is deployed on ALL feature sets
- `TechPreviewNoUpgrade,DevPreviewNoUpgrade,CustomNoUpgrade`: CRD is **NOT** deployed on Default/production clusters

Currently **5 CRD resources** are TechPreview-only (absent from Default):
- `backups.config.openshift.io`
- `clustermonitorings.config.openshift.io`
- `etcdbackups.operator.openshift.io`
- `ingresses.operator.openshift.io`
- `pkis.config.openshift.io`

The generator must parse the annotation to determine CRD availability per feature set.
The `include.release.openshift.io/*` annotations determine profile availability
(`ibm-cloud-managed` = Hypershift, `self-managed-high-availability` = SelfManaged).

### A4. Output Files

**YAML** (in `payload-manifests/served-apis/`):
- `servedAPIs-SelfManagedHA-Default.yaml`
- `servedAPIs-SelfManagedHA-TechPreviewNoUpgrade.yaml`
- `servedAPIs-SelfManagedHA-DevPreviewNoUpgrade.yaml`
- `servedAPIs-SelfManagedHA-OKD.yaml`
- `servedAPIs-Hypershift-Default.yaml`
- `servedAPIs-Hypershift-TechPreviewNoUpgrade.yaml`
- `servedAPIs-Hypershift-DevPreviewNoUpgrade.yaml`
- `servedAPIs-Hypershift-OKD.yaml`

Each YAML file is a sorted list of `ServedAPIEntry` records. Sorted by (group, version, resource) for stable diffs.

**Go** (`servedapis/zz_generated.served_apis.go`): Same data as Go literals with a lookup function.

### A5. Build System

New scripts following the `update-payload-featuregates.sh` pattern:

**`hack/update-served-api-inventory.sh`**:
```bash
rm -f ./payload-manifests/served-apis/*
go run --mod=vendor github.com/openshift/api/payload-command/cmd/write-served-api-inventory \
  --crd-dir=./payload-manifests/crds \
  --yaml-output-dir=./payload-manifests/served-apis \
  --go-output-dir=./servedapis
```

**`hack/verify-served-api-inventory.sh`**: Runs the generator to a temp dir and diffs against checked-in files.

**Makefile changes**:
- Add `update-served-api-inventory` to the `update-codegen` target (after `update-payload-crds`)
- Add `hack/verify-served-api-inventory.sh` to `verify-non-codegen` target
- Add `write-served-api-inventory` to `make build`

---

## Part B: E2E Test (origin)

### B1. Test File

**`test/extended/apiserver/served_api_inventory.go`**

### B2. Test Logic

```
[sig-api-machinery] Served API inventory should match the expected list
  [Suite:openshift/conformance/parallel]
```

Steps:
1. **Detect cluster state**:
   - `profile` = `exutil.GetControlPlaneTopology(oc)` — `External` → Hypershift, otherwise SelfManaged
   - `featureSet` = `FeatureGates("cluster").Spec.FeatureSet`
   - `enabledGates` = `FeatureGates("cluster").Status.FeatureGates` — list of enabled feature gates
2. **Build OpenShift expected set**: `servedapis.ForProfile(profile, featureSet)` (from vendored openshift/api) → CRDs + aggregated API server resources + optional
3. **Build Kubernetes stable expected set**: `DefaultAPIResourceConfigSource()` → enabled GVs → scheme types → filter → `UnsafeGuessKindToResource()` (see B5)
4. **Apply Kubernetes overrides**: for each `enabledGate`, look up additional Kubernetes GVs from the override map in openshift/api → derive resources from scheme the same way
5. **Merge** steps 2 + 3 + 4 into one expected set
6. **Query actual APIs**: `kubeClient.Discovery().ServerGroupsAndResources()` — filter out subresources (resource names containing `/`)
7. **Bidirectional comparison**:
   - Every required API (source != optional) not served → **FAIL** with clear message listing missing GVRs
   - Every served API not in expected (required or optional) → **FAIL** with clear message listing unexpected GVRs
   - Optional API not served → **OK** (logged for visibility)

### B5. Deriving Kubernetes APIs from the Scheme

The e2e test derives the expected Kubernetes API resources programmatically:

```go
import (
    clientgoscheme "k8s.io/client-go/kubernetes/scheme"
    "k8s.io/apimachinery/pkg/api/meta"
    "k8s.io/kubernetes/pkg/controlplane"
)

func expectedKubeResources() sets.Set[schema.GroupVersionResource] {
    // Get enabled GroupVersions — includes both stable (v1) and beta GVs
    // of new groups whose feature gate is beta-enabled-by-default.
    // Superseded beta GVs (where v1 exists) are in the disabled list,
    // so they won't appear here — no graduated-type problem.
    resourceConfig := controlplane.DefaultAPIResourceConfigSource()

    result := sets.New[schema.GroupVersionResource]()
    for gv := range resourceConfig.EnabledVersions() {
        for kind := range clientgoscheme.Scheme.KnownTypes(gv) {
            if shouldSkipType(kind) {
                continue
            }
            plural, _ := meta.UnsafeGuessKindToResource(gv.WithKind(kind))
            result.Insert(plural)
        }
    }
    return result
}

// shouldSkipType filters out types that aren't top-level API resources
func shouldSkipType(kind string) bool {
    if strings.HasSuffix(kind, "List") { return true }
    if strings.HasSuffix(kind, "Options") { return true }
    return subresourceOnlyTypes.Has(kind)
}

var subresourceOnlyTypes = sets.New("Binding", "Eviction", "Scale",
    "TokenRequest", "NodeProxyOptions", "ServiceProxyOptions",
    "PodProxyOptions", "SerializedReference", "RangeAllocation")
```

**Why this works for both stable and beta GVs:**
- `DefaultAPIResourceConfigSource()` enables stable GVs and beta GVs of new API groups
  (where the Kubernetes feature gate is beta-enabled-by-default)
- Superseded beta GVs (where types graduated to v1) are in the disabled list — they're
  never in the enabled set, so the graduated-type problem doesn't arise
- The scheme is reliable for all enabled GVs because enabled betas are always for new
  groups without a v1 to graduate types away from
- `UnsafeGuessKindToResource()` handles standard Kubernetes pluralization correctly
- The `subresourceOnlyTypes` blocklist is ~10 entries and extremely stable across releases

**Reference**: PR openshift/cluster-kube-apiserver-operator#2179 uses the same
`clientgoscheme.Scheme` approach for staleness detection.

### B3. Helpers to Reuse

- `exutil.GetControlPlaneTopology(oc)` — `test/extended/util/framework.go:2125`
- Feature set detection — `test/extended/util/framework.go:2197` (`IsTechPreviewNoUpgrade`)
- Discovery client error handling pattern from `DoesApiResourceExist` — `test/extended/util/framework.go:2224`
- `diffMaps()` pattern from `test/extended/etcd/etcd_storage_path.go:457`

### B4. Profile Mapping

```go
func clusterProfileName(topology configv1.TopologyMode) string {
    if topology == configv1.ExternalTopologyMode {
        return "Hypershift"
    }
    return "SelfManagedHA"
}
```

---

## Adding New OpenShift CRDs

When a new CRD is added to openshift/api, the e2e test would fail because the cluster
now serves an API that isn't in the expected list yet. The release timeline means the
CRD lands in openshift/api before the origin vendor bump picks it up.

**Approach**: Mark new CRDs as temporarily optional in origin's test until the vendor
bump catches up.

The e2e test in origin maintains a `newCRDAllowlist` — a set of GVRs that are allowed
to be present or absent without failing the test. When origin vendors the updated
openshift/api (which includes the new CRD in its generated inventory), the GVR moves
from the allowlist to the required set, and the allowlist entry is removed.

```go
// test/extended/apiserver/served_api_inventory.go
//
// New CRDs added to openshift/api but not yet in the vendored servedapis
// inventory. Remove entries as origin vendors the updated openshift/api.
var newCRDAllowlist = sets.New[schema.GroupVersionResource](
    // Added in openshift/api PR #NNNN, pending vendor bump
    // schema.GroupVersionResource{Group: "example.openshift.io", Version: "v1alpha1", Resource: "examples"},
)
```

This keeps the test strict (unknown APIs still fail) while giving a clear path for
new CRDs during the time between the openshift/api merge and the origin vendor bump.

---

## Kubernetes Rebase

During a Kubernetes rebase, the vendored `k8s.io/client-go` and `k8s.io/kubernetes`
are updated. This changes the scheme and `DefaultAPIResourceConfigSource()`, which
changes the derived expected Kubernetes API set.

**The test will fail** if the new Kubernetes version adds, removes, or moves APIs.
This is intentional — the failure surfaces exactly which APIs changed and forces
explicit acknowledgment.

### What changes automatically (no action needed)
- New resources added to existing stable GroupVersions (e.g., new Kind in `apps/v1`)
  → the scheme picks them up
- Resources removed from the scheme → automatically excluded
- GroupVersions moving from disabled-by-default to enabled-by-default (or vice versa)
  → `DefaultAPIResourceConfigSource()` reflects this

### What needs manual attention
- **Kubernetes API override map**: If the rebase changes which alpha/beta APIs exist
  for a feature-gate-enabled GroupVersion (e.g., `v1alpha1` → `v1beta1` for
  MutatingAdmissionPolicy), update the override mapping in `features/kube_api_overrides.go`.
  This is the same update currently done in `cluster-kube-apiserver-operator`'s
  `defaultGroupVersionsByFeatureGate`.
- **Subresource-only blocklist**: If Kubernetes adds a new type that is only a
  subresource (rare), add it to `subresourceOnlyTypes`.
- **OpenShift CRD changes**: If the rebase changes OpenShift CRDs, run `make update`
  in openshift/api to regenerate the inventory.

### Per-version override map

The Kubernetes API override map should be version-aware, since different Kubernetes
versions may need different overrides for the same feature gate (e.g.,
`MutatingAdmissionPolicy` enables `v1alpha1` on kube 1.33 but `v1beta1` on kube 1.34+).

This already exists in `cluster-kube-apiserver-operator` as the `KubeVersionRange`
field on `groupVersionKindsByOpenshiftVersion`. The same pattern applies here:

```go
// features/kube_api_overrides.go
type KubeAPIOverride struct {
    GroupVersion     schema.GroupVersion
    Kinds            []string      // explicit — scheme is unreliable for alpha/beta GVs
    KubeVersionRange semver.Range  // nil means all versions
}

var KubeAPIOverridesByFeatureGate = map[FeatureGateName][]KubeAPIOverride{
    "MutatingAdmissionPolicy": {
        {KubeVersionRange: semver.MustParseRange(">=1.33.0 <1.34.0"),
         GroupVersion: schema.GroupVersion{Group: "admissionregistration.k8s.io", Version: "v1alpha1"},
         Kinds: []string{"MutatingAdmissionPolicy", "MutatingAdmissionPolicyBinding"}},
        {KubeVersionRange: semver.MustParseRange(">=1.34.0 <1.37.0"),
         GroupVersion: schema.GroupVersion{Group: "admissionregistration.k8s.io", Version: "v1beta1"},
         Kinds: []string{"MutatingAdmissionPolicy", "MutatingAdmissionPolicyBinding"}},
    },
}
```

**Why explicit Kinds**: The scheme registers types at old GVs for serialization
backward compatibility even after graduation. `admissionregistration.k8s.io/v1beta1`
has 6 types in the scheme, but only 2 are actually served — the other 4 graduated
to v1. We can't derive resources from the scheme for alpha/beta GVs.

At test time, Kinds are converted to GVRs via `UnsafeGuessKindToResource()`. The kube
version is available from `componentbaseversion.DefaultKubeBinaryVersion` (vendored),
and the test filters the override map to only entries matching the current kube version.

---

## Part C: Edge Cases

### Optional vs Required Boundary
- **Required**: All Kubernetes stable APIs, all OpenShift CRDs in `payload-manifests/crds/`, all aggregated API server resources
- **Optional**: APIs from operators outside the core payload (OLM, monitoring, machine-api, metal3, node-tuning, autoscaling, helm, cloudcredential)

### HyperShift Differences
- oauth-apiserver may not serve APIs when external OIDC is used → consider marking oauth-apiserver APIs as optional specifically for Hypershift, or detecting OIDC mode
- Hypershift-specific CRDs (e.g., `criocredentialproviderconfigs-Hypershift.crd.yaml`) are handled by the separate Hypershift inventory file

### Discovery API Reliability
- `ServerGroupsAndResources()` can return partial results if an aggregated API server is restarting — the test should use `discovery.IsGroupDiscoveryFailedError` and retry or fail clearly

---

## Verification

1. **openshift/api CI**: `make verify` includes `verify-served-api-inventory`
2. **origin e2e**: Test runs as `[Suite:openshift/conformance/parallel]` on SelfManaged and HyperShift clusters
3. **During development**: After modifying CRDs or feature gates → `make update` → review diff
4. **During rebase**: Test failures surface Kubernetes API changes; update override map + blocklist as needed

---

## Implementation Order

1. Create `servedapis/types.go` in openshift/api
2. Create generator package `payload-command/servedapis/` with generator.go, aggregated_apis.go, optional_apis.go
3. Create `payload-command/cmd/write-served-api-inventory/main.go`
4. Create `hack/update-served-api-inventory.sh` and `hack/verify-served-api-inventory.sh`
5. Wire into Makefile
6. Run generator → produces `payload-manifests/served-apis/*.yaml` and `servedapis/zz_generated.served_apis.go`
7. Vendor updated openshift/api into origin
8. Create `test/extended/apiserver/served_api_inventory.go` in origin, including the
   scheme-based Kubernetes API derivation using `clientgoscheme.Scheme` +
   `DefaultAPIResourceConfigSource()` + `UnsafeGuessKindToResource()`
9. Test against a real cluster

---

## Research Notes

### Aggregated API Server Resources (complete)

**openshift-apiserver** (9 groups, all v1):

| Group | Resources |
|---|---|
| apps.openshift.io/v1 | deploymentconfigs |
| authorization.openshift.io/v1 | resourceaccessreviews, subjectaccessreviews, localsubjectaccessreviews, localresourceaccessreviews, selfsubjectrulesreviews, subjectrulesreviews, roles, rolebindings, clusterroles, clusterrolebindings, rolebindingrestrictions |
| build.openshift.io/v1 | builds, buildconfigs |
| image.openshift.io/v1 | images, imagesignatures, imagestreams, imagestreamimports, imagestreamimages, imagestreammappings, imagestreamtags, imagetags |
| project.openshift.io/v1 | projects, projectrequests |
| quota.openshift.io/v1 | clusterresourcequotas, appliedclusterresourcequotas |
| route.openshift.io/v1 | routes |
| security.openshift.io/v1 | securitycontextconstraints, podsecuritypolicyreviews, podsecuritypolicysubjectreviews, podsecuritypolicyselfsubjectreviews, rangeallocations |
| template.openshift.io/v1 | processedtemplates, templates, templateinstances, brokertemplateinstances |

Only apps and build can be disabled via config (`openshiftcontrolplanev1.APIServers.PerGroupOptions`).

**oauth-apiserver** (2 groups, all v1):

| Group | Resources |
|---|---|
| oauth.openshift.io/v1 | oauthauthorizetokens, oauthaccesstokens, oauthclients, oauthclientauthorizations, useroauthaccesstokens, tokenreviews |
| user.openshift.io/v1 | users, groups, identities, useridentitymappings |

Always enabled (no per-group disable).

### Key Source Files

- `openshift/api/features/features.go` — central feature gate registry
- `openshift/api/payload-manifests/crds/` — ~144 CRD manifest files
- `openshift/api/payload-manifests/featuregates/` — featuregate manifests per variant
- `openshift/api/payload-command/render/write_featureset.go` — reference for featureset iteration
- `openshift/api/hack/update-payload-crds.sh` — CRD copy script with glob patterns
- `openshift/api/hack/update-payload-featuregates.sh` — featuregate generation script
- `origin/test/extended/etcd/etcd_storage_path.go` — bidirectional validation pattern
- `origin/test/extended/util/framework.go:2125` — GetControlPlaneTopology
- `origin/test/extended/util/framework.go:2197` — IsTechPreviewNoUpgrade
- `origin/test/extended/util/framework.go:2224` — DoesApiResourceExist
- `origin/vendor/k8s.io/kubernetes/pkg/controlplane/instance.go:449-510` — Kubernetes enabled/disabled API lists
- `openshift-apiserver/pkg/cmd/openshift-apiserver/openshiftapiserver/openshift_apiserver.go` — API group registration
- `oauth-apiserver/pkg/apiserver/apiserver.go` — OAuth API group registration
