# E2E Test for Served API Inventory Validation

## Context

Ben wants a statically defined list of all GVKs expected to be served by an OpenShift cluster. An e2e test compares this list against what the cluster actually serves — failing on both unexpected and missing APIs. The test runs as part of the conformance suite on both standalone OpenShift and HyperShift clusters.

The list must be generated using tooling so it can be regenerated during Kubernetes rebases. Sources: Kubernetes built-in APIs + OpenShift CRDs (from openshift/api) + aggregated API servers (openshift-apiserver, oauth-apiserver).

No such test or tooling exists today. The closest patterns are:
- `origin/test/extended/etcd/etcd_storage_path.go` — static map of persisted resources with bidirectional validation against discovery
- `origin/test/extended/cli/explain.go` — static lists of expected API resources

## API Versioning and Feature Gates — Background

### API Versions (Stability Contract)

Kubernetes API versions indicate stability, not just iteration:

- **`v1`** — Stable, backward compatible, **always enabled**
- **`v1beta1`** — Beta quality, may change, **enabled by default** in modern Kubernetes
- **`v1alpha1`** — Alpha/experimental, **always behind a feature gate, disabled by default**

### Feature Gates (Functionality Control)

Feature gates can control:
- **Entire new APIs** (always for alpha APIs, sometimes for beta)
- **Specific fields** within an existing API (e.g., new field in `Pod.spec`)
- **Behavior changes** (e.g., how scheduling works)

### API Graduation and Deprecation

When an API graduates from `v1beta1` → `v1`:

1. **At v1 GA release**: Both versions are served (v1 becomes storage version, v1beta1 deprecated but still works)
2. **Deprecation window**: v1beta1 remains served for **9 months OR 3 releases** (whichever is longer)
3. **After window**: v1beta1 removed from serving, only v1 available

During the transition, the API server handles version conversion:
- **Reads**: Fetch from etcd (stored as v1) → convert to requested version → return
- **Writes**: Accept in any served version → convert to storage version (v1) → write to etcd

### Storage vs Served vs Scheme

**Storage version**: Which version is written to etcd (only ONE per resource)
**Served versions**: Which versions clients can use via API (can be MULTIPLE during deprecation)
**Scheme**: Registers types at ALL historical versions for serialization/conversion (even removed ones)

**Key insight**: The scheme is for serialization backward compatibility, not "what's served". 
After `apps/v1beta1` Deployment graduates to `apps/v1` and is removed from serving:
- Scheme still has `apps/v1beta1` Deployment types (for conversion)
- API server rejects `apps/v1beta1` API calls (not in served list)

### What DefaultAPIResourceConfigSource() Tells Us

`k8s.io/kubernetes/pkg/controlplane.DefaultAPIResourceConfigSource()` returns the list of 
GroupVersions **enabled by default** in the main kube-apiserver:

**Enabled**:
- All `v1` APIs (stable, always on)
- Beta APIs in deprecation window (still served)
- Beta APIs for new groups (no v1 yet)

**Disabled**:
- Superseded betas (graduated to v1, removal window expired)
- Alpha APIs (always disabled by default)

**Note**: There are three different `DefaultAPIResourceConfigSource()` functions:
1. **`k8s.io/kubernetes/pkg/controlplane`** — Main kube-apiserver, **this is the one we need**
2. `k8s.io/apiextensions-apiserver/pkg/apiserver` — Only `apiextensions.k8s.io` APIs (the CRD resource itself)
3. `k8s.io/kube-aggregator/pkg/apiserver` — Only `apiregistration.k8s.io` APIs (the APIService resource)

### OpenShift Feature Gates and Kubernetes APIs

OpenShift can enable **additional Kubernetes feature gates** beyond the defaults. When it does,
those gates may enable alpha/beta APIs that aren't in `DefaultAPIResourceConfigSource()`.

**Example**: OpenShift enables the `MutatingAdmissionPolicy` feature gate:
- Kubernetes default: gate disabled → `admissionregistration.k8s.io/v1beta1` MutatingAdmissionPolicy **not served**
- OpenShift: enables the gate → API **is served**
- This is what the override map captures: "when this OpenShift feature gate is on, these extra Kubernetes APIs become available"

## Test Flow

The e2e test in origin builds the expected API set from three sources, queries the
cluster, and compares:

```
 1. Detect cluster state
    ├── profile      ← Infrastructure.Status.ControlPlaneTopology (Hypershift vs SelfManaged)
    ├── featureSet   ← FeatureGate("cluster").Spec.FeatureSet (Default, TechPreview, ...)
    ├── enabledGates ← FeatureGate("cluster").Status.FeatureGates (list of active gates)
    └── kubeVersion  ← ClusterVersion.Status.Desired.Version → parse kube minor (e.g., "1.35")

 2. Build expected API set
    │
    ├─ Base APIs (OpenShift + Kubernetes combined)
    │  servedapis.ForProfileAndVersion(profile, featureSet, kubeVersion)
    │  → OpenShift CRDs (feature-set-aware: 5 CRDs are TP-only, absent on Default)
    │  → Aggregated API server resources (openshift-apiserver, oauth-apiserver)
    │  → Optional operator APIs (monitoring, OLM, machine-api, etc.)
    │  → Kubernetes built-in APIs (generated during rebase via scheme + DefaultAPIResourceConfigSource)
    │  → versioned to handle skew between test binary and cluster
    │
    └─ Kubernetes overrides (OpenShift feature gates that enable extra k8s APIs)
       for each enabledGate: look up override entries from map in openshift/api
       filter to KubeVersionRange matching kubeVersion
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
│ payload-command/servedapis/  │     │ Detects cluster version,    │
│   generator.go               │────>│ calls vendored function:    │
│   aggregated_apis.go         │     │  · ForProfileAndVersion()   │
│   optional_apis.go           │     │  · kube API override map    │
│   kube_api_derivation.go     │     │                             │
│   (scheme + config → GVRs)   │     │ Queries cluster discovery,  │
│                              │     │ compares bidirectionally    │
│ features/                    │     │                             │
│   kube_api_overrides.go      │────>│                             │
│                              │     └─────────────────────────────┘
│ servedapis/                  │
│   types.go                   │
│   zz_generated.served_apis.go│  ← generated Go code, vendored
│     · ForProfileAndVersion() │     into origin automatically
│     · all inventory as vars  │
└──────────────────────────────┘
```

### Key insight: Kubernetes APIs generated, not manually maintained

Instead of maintaining a `kube_apis.go` with ~100+ Kubernetes resource entries, the
Kubernetes API inventory is **generated during rebase** in openshift/api. The generator
runs once per supported Kubernetes version and outputs **versioned data as Go variables**
in `zz_generated.served_apis.go`:
- `var kubeAPIs135 = []ServedAPIEntry{ ... }`
- `var kubeAPIs136 = []ServedAPIEntry{ ... }`
- etc.

**Why per-version data:** The test runs against a live cluster whose Kubernetes version
may not match what's vendored in the origin test binary (during rebase windows, in dev
environments). The test queries the cluster's version and calls `ForProfileAndVersion()` with it,
getting the matching data for both OpenShift and Kubernetes APIs, making it robust to version skew.

### How Kubernetes API derivation works

The generation uses four components with a clear hierarchy:

**1. `DefaultAPIResourceConfigSource()` — AUTHORITATIVE (what's served)**
- From `k8s.io/kubernetes/pkg/controlplane` (not apiextensions or aggregator)
- Answers: "Which GroupVersions are served by default?"
- Source of truth for what's enabled/disabled in Kubernetes
- Contains both enabled and disabled lists
- This gates what we even look at

**2. `clientgoscheme.Scheme` — HELPER (what types exist)**
- From `k8s.io/client-go/kubernetes/scheme`
- Answers: "What Kinds exist at a given GroupVersion?"
- Type registry to enumerate resources within an enabled GV
- Contains ALL historical types (enabled and disabled)
- Only consulted for GVs that DefaultAPIResourceConfigSource() enables
- Requires openshift/api to vendor all `k8s.io/api` packages

**3. `meta.UnsafeGuessKindToResource()` — CONVERTER (Kind → resource)**
- Converts Kind names to plural resource names (e.g., Deployment → deployments)

**4. Type filtering — CLEANUP**
- Remove List types (suffix "List"), Options types (suffix "Options")
- Small blocklist of subresource-only types (~10 entries: Binding, Eviction, Scale,
  TokenRequest, NodeProxyOptions, ServiceProxyOptions, PodProxyOptions, SerializedReference, RangeAllocation)

**The complete workflow**:
```go
// 1. Get authority on what's enabled
resourceConfig := controlplane.DefaultAPIResourceConfigSource()

// 2. For each ENABLED GroupVersion
for gv := range resourceConfig.EnabledVersions() {
    
    // 3. Use scheme as lookup: what Kinds exist at this GV?
    for kind := range clientgoscheme.Scheme.KnownTypes(gv) {
        
        // 4. Convert Kind → Resource name
        plural, _ := meta.UnsafeGuessKindToResource(gv.WithKind(kind))
    }
}
```

**Without DefaultAPIResourceConfigSource()**: We'd blindly enumerate everything in the scheme,
including disabled APIs (superseded betas, disabled-by-default alphas/betas).

**Without scheme**: We'd know which GroupVersions are enabled, but not what resources exist at each one.

**Concrete example**:
```
DefaultAPIResourceConfigSource() answers: "Is apps/v1 enabled?" → YES
                    ↓
Scheme answers: "What Kinds exist at apps/v1?"
                    ↓
    Deployment, StatefulSet, DaemonSet, ReplicaSet
                    ↓
UnsafeGuessKindToResource() converts:
    Deployment → deployments
    StatefulSet → statefulsets
```

**Result**: No manual maintenance needed. The generator derives the complete Kubernetes API
inventory from vendored code during each rebase.

### Limitation: scheme is unreliable for Kubernetes-disabled beta GVs

As explained in the "API Versioning and Feature Gates" section above, the scheme registers
types at their historical GroupVersions for serialization/conversion backward compatibility
(to handle the storage version vs served versions distinction). For example,
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

As explained in the "API Versioning and Feature Gates" section, OpenShift can enable
additional Kubernetes feature gates beyond defaults. OpenShift enables certain alpha/beta
Kubernetes APIs when specific OpenShift feature gates are active (e.g., `MutatingAdmissionPolicy`
gate enables `MutatingAdmissionPolicy` and `MutatingAdmissionPolicyBinding` at
`admissionregistration.k8s.io/v1beta1`).

These APIs are NOT in `DefaultAPIResourceConfigSource()` — they're disabled by default
in Kubernetes but added to the kube-apiserver's `--runtime-config` by the
cluster-kube-apiserver-operator. And because the scheme is unreliable for these
Kubernetes-disabled GVs (see above), the override map must list **explicit Kinds**,
not just GroupVersions.

**Why explicit Kinds are required** — concrete example:

When the `MutatingAdmissionPolicy` feature gate is enabled, it enables
`admissionregistration.k8s.io/v1beta1`. If we tried to derive types from the scheme:

```go
// Without explicit Kinds - deriving from scheme
gv := schema.GroupVersion{Group: "admissionregistration.k8s.io", Version: "v1beta1"}
for kind := range clientgoscheme.Scheme.KnownTypes(gv) {
    // Scheme returns 6 types:
    // ✓ MutatingAdmissionPolicy (actually served at v1beta1)
    // ✓ MutatingAdmissionPolicyBinding (actually served at v1beta1)
    // ✗ ValidatingWebhookConfiguration (graduated to v1, NOT served at v1beta1)
    // ✗ MutatingWebhookConfiguration (graduated to v1, NOT served at v1beta1)
    // ✗ ValidatingAdmissionPolicy (graduated to v1, NOT served at v1beta1)
    // ✗ ValidatingAdmissionPolicyBinding (graduated to v1, NOT served at v1beta1)
}
```

**Result without explicit Kinds**: Test would expect all 6 types at v1beta1, but the cluster
only serves 2 at v1beta1 (the other 4 are at v1). Test fails with "missing APIs":
- `admissionregistration.k8s.io/v1beta1 ValidatingWebhookConfiguration`
- `admissionregistration.k8s.io/v1beta1 MutatingWebhookConfiguration`
- `admissionregistration.k8s.io/v1beta1 ValidatingAdmissionPolicy`
- `admissionregistration.k8s.io/v1beta1 ValidatingAdmissionPolicyBinding`

**Result with explicit Kinds**: Override map says "only these 2 Kinds", test expects only what's
actually served, test passes.

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
`FeatureGate("cluster").Status.FeatureGates`, queries the cluster's Kubernetes version,
filters the override map to entries matching that version, and adds them to the expected set.

Note: openshift/api needs to vendor the complete `k8s.io/api` (not just the partial set
it currently has) to run the scheme-based Kubernetes API derivation during generation.
The generator runs at openshift/api build time (during rebase), not at test runtime.

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
)

type ServedAPIEntry struct {
    Group    string `json:"group" yaml:"group"`
    Version  string `json:"version" yaml:"version"`
    Resource string `json:"resource" yaml:"resource"`
    Kind     string `json:"kind" yaml:"kind"`
    Scope    string `json:"scope" yaml:"scope"`       // "Namespaced" or "Cluster"
    Source   Source `json:"source" yaml:"source"`     // where the API comes from
}
```

**`servedapis/zz_generated.served_apis.go`** — generated file containing the full inventory data as Go literals. Provides:
```go
// Returns required and optional APIs for a given cluster configuration
func ForProfileAndVersion(clusterProfile, featureSet, kubeVersion string) (required, optional []ServedAPIEntry, found bool)
```

Returns two separate lists:
- **required**: APIs that must be present (test fails if missing) - core Kubernetes, OpenShift CRDs, aggregated servers
- **optional**: APIs from optional operators (test logs if missing, doesn't fail) - monitoring, OLM, machine-api, etc.
- **found**: false during rebase windows when the Kubernetes version isn't generated yet

This file gets vendored into origin automatically. Contains all inventory data - both OpenShift and Kubernetes APIs.

### A2. Generator Tool — `payload-command/cmd/write-served-api-inventory/`

Standalone binary following the pattern of `write-available-featuresets`. Takes `--go-output-dir` flag for the generated Go file.

**Generator logic** in `payload-command/servedapis/`:

1. **`generator.go`** — main orchestration:
   - **OpenShift APIs** (per ClusterProfile, FeatureSet):
     - Reads CRD manifests from `payload-manifests/crds/`
     - Parses filename suffixes and `release.openshift.io/feature-set` annotations
     - Extracts served GVRs from each CRD's `spec.versions[].served`, `spec.group`, `spec.names.plural/kind`, `spec.scope`
     - Merges with hardcoded aggregated API server entries
     - Merges with optional API entries
     - Generates Go variables for each (profile, featureSet) combination
   
   - **Kubernetes APIs** (per supported kube version):
     - For each supported kube minor version (e.g., 1.35, 1.36):
       - Derives from vendored scheme + `DefaultAPIResourceConfigSource()`
       - Filters and converts to GVRs
       - Generates Go variable per version (e.g., `kubeAPIs135`, `kubeAPIs136`)
   
   - **Output**: Single `zz_generated.served_apis.go` file containing all inventory data as Go literals

2. **`kube_api_derivation.go`** — scheme-based Kubernetes API derivation. Runs once per supported kube version:
   ```go
   import (
       clientgoscheme "k8s.io/client-go/kubernetes/scheme"
       "k8s.io/apimachinery/pkg/api/meta"
       "k8s.io/kubernetes/pkg/controlplane"  // IMPORTANT: Use this one, not apiextensions or aggregator
   )
   
   func deriveKubernetesAPIs(kubeVersion string) []ServedAPIEntry {
       // Get enabled GroupVersions from main kube-apiserver config
       resourceConfig := controlplane.DefaultAPIResourceConfigSource()
       
       result := []ServedAPIEntry{}
       for gv := range resourceConfig.EnabledVersions() {
           for kind := range clientgoscheme.Scheme.KnownTypes(gv) {
               if shouldSkipType(kind) { continue }
               
               plural, _ := meta.UnsafeGuessKindToResource(gv.WithKind(kind))
               result = append(result, ServedAPIEntry{
                   Group:    plural.Group,
                   Version:  plural.Version,
                   Resource: plural.Resource,
                   Kind:     kind,
                   Scope:    inferScope(kind),  // from scheme or defaults
                   Source:   SourceCoreKube,
               })
           }
       }
       return result
   }
   
   func shouldSkipType(kind string) bool {
       if strings.HasSuffix(kind, "List") { return true }
       if strings.HasSuffix(kind, "Options") { return true }
       return subresourceOnlyTypes.Has(kind)
   }
   
   var subresourceOnlyTypes = sets.New("Binding", "Eviction", "Scale",
       "TokenRequest", "NodeProxyOptions", "ServiceProxyOptions",
       "PodProxyOptions", "SerializedReference", "RangeAllocation")
   ```

3. **`aggregated_apis.go`** — hardcoded lists for openshift-apiserver (9 groups, ~35 resources) and oauth-apiserver (2 groups, ~10 resources). Based on the exploration findings. Changes very rarely.

4. **`optional_apis.go`** — APIs from optional operators marked `Source: optional`:
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

**`servedapis/zz_generated.served_apis.go`** — single generated Go file containing all inventory data:

```go
package servedapis

// Required OpenShift API variables (core CRDs, aggregated servers - per profile/featureSet)
var requiredSelfManagedHADefault = []ServedAPIEntry{ ... }
var requiredHypershiftDefault = []ServedAPIEntry{ ... }
// ... etc

// Optional OpenShift API variables (optional operators - per profile/featureSet)
var optionalSelfManagedHADefault = []ServedAPIEntry{
    {Group: "monitoring.coreos.com", Version: "v1", Resource: "prometheuses", Kind: "Prometheus", Scope: "Namespaced", Source: SourceOpenShiftCRD},
    {Group: "operators.coreos.com", Version: "v1alpha1", Resource: "subscriptions", Kind: "Subscription", Scope: "Namespaced", Source: SourceOpenShiftCRD},
    // ... monitoring, OLM, machine-api, metal3, etc.
}
var optionalHypershiftDefault = []ServedAPIEntry{ ... }
// ... etc

// Kubernetes API variables (one per supported version)
var kubeAPIs135 = []ServedAPIEntry{
    {Group: "apps", Version: "v1", Resource: "deployments", Kind: "Deployment", Scope: "Namespaced", Source: SourceCoreKube},
    {Group: "apps", Version: "v1", Resource: "statefulsets", Kind: "StatefulSet", Scope: "Namespaced", Source: SourceCoreKube},
    // ... ~100+ entries per version
}
var kubeAPIs136 = []ServedAPIEntry{ ... }
var kubeAPIs137 = []ServedAPIEntry{ ... }

// Lookup function - returns required and optional APIs separately
func ForProfileAndVersion(clusterProfile, featureSet, kubeVersion string) (required, optional []ServedAPIEntry, found bool) {
    // Get OpenShift APIs (required and optional)
    key := clusterProfile + "-" + featureSet
    var requiredOpenShift, optionalOpenShift []ServedAPIEntry
    switch key {
    case "SelfManagedHA-Default":
        requiredOpenShift = requiredSelfManagedHADefault
        optionalOpenShift = optionalSelfManagedHADefault
    case "Hypershift-Default":
        requiredOpenShift = requiredHypershiftDefault
        optionalOpenShift = optionalHypershiftDefault
    // ... etc
    default:
        return nil, nil, false
    }
    
    // Get Kubernetes APIs (always required)
    var kubeAPIs []ServedAPIEntry
    switch kubeVersion {
    case "1.35":
        kubeAPIs = kubeAPIs135
    case "1.36":
        kubeAPIs = kubeAPIs136
    case "1.37":
        kubeAPIs = kubeAPIs137
    default:
        return nil, nil, false  // kubeVersion not found
    }
    
    // Combine and return
    required = append(requiredOpenShift, kubeAPIs...)
    optional = optionalOpenShift
    return required, optional, true
}
```

All data is Go code - no YAML parsing needed. Entries are sorted by (group, version, resource) for stable diffs in code review.

### A5. Build System

New scripts following the `update-payload-featuregates.sh` pattern:

**`hack/update-served-api-inventory.sh`**:
```bash
go run --mod=vendor github.com/openshift/api/payload-command/cmd/write-served-api-inventory \
  --crd-dir=./payload-manifests/crds \
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
   - **Skip if not Default**: `if featureSet != "Default" { e2eskipper.Skipf("Test only runs on Default feature set, got %s", featureSet) }`
   - `enabledGates` = `FeatureGates("cluster").Status.FeatureGates` — list of enabled feature gates
   - `kubeVersion` = parse minor version from `ClusterVersion("version").Status.Desired.Version` (e.g., "4.19.0-0.nightly-2026-08-11-225619" → "1.35")

2. **Build expected sets**:
   - **Base APIs**: `required, optional, found := servedapis.ForProfileAndVersion(profile, featureSet, kubeVersion)` from vendored openshift/api
     - Returns two separate lists: required APIs and optional APIs
     - If `found == false` (during rebase window when kubeVersion not generated): skip entire test
   - **Kubernetes overrides**: For each `enabledGate`, look up override entries from `KubeAPIOverridesByFeatureGate` map in openshift/api, filter to those matching `kubeVersion` range, add their GVRs to required list
   - **Convert to sets**: `expectedRequired`, `expectedOptional`

3. **Query actual APIs**: `kubeClient.Discovery().ServerGroupsAndResources()` — filter out subresources (resource names containing `/`)

4. **Bidirectional comparison**:
   - Every required API not served → **FAIL** with clear message listing missing GVRs
   - Every served API not in `expectedRequired` or `expectedOptional` → **FAIL** with clear message listing unexpected GVRs
   - Optional API not served → **OK** (logged for visibility)

### B5. Loading API Inventory

The test calls a single function from vendored openshift/api:

```go
import (
    "github.com/openshift/api/servedapis"
)

func parseKubeVersion(clusterVersion string) string {
    // ClusterVersion.Status.Desired.Version format: "4.19.0-0.nightly-2026-08-11-225619"
    // Contains embedded kube version somewhere in metadata/tags
    // Actual implementation will query kube-apiserver version or parse from
    // ClusterVersion payload metadata
    return "1.35"  // placeholder
}

// In test body
profile := clusterProfileName(exutil.GetControlPlaneTopology(oc))
featureSet := "Default"  // already checked and skipped if not Default
kubeVersion := parseKubeVersion(clusterVersion)

required, optional, found := servedapis.ForProfileAndVersion(profile, featureSet, kubeVersion)
if !found {
    e2eskipper.Skipf("API inventory for profile=%s featureSet=%s kubeVersion=%s not found. This is expected during Kubernetes rebase.", profile, featureSet, kubeVersion)
}

// Add override APIs for enabled feature gates (to required list)
for _, gate := range enabledGates {
    overrides := getOverridesForGate(gate, kubeVersion)
    required = append(required, overrides...)
}

// Convert to GVR sets
expectedRequired := toGVRSet(required)
expectedOptional := toGVRSet(optional)

// Query actual APIs
actual := getActualAPIs(discovery)  // returns sets.Set[schema.GroupVersionResource]

// Compare
for gvr := range actual {
    if !expectedRequired.Has(gvr) && !expectedOptional.Has(gvr) {
        framework.Failf("Unexpected API: %v", gvr)
    }
}

for gvr := range expectedRequired {
    if !actual.Has(gvr) {
        framework.Failf("Missing required API: %v", gvr)
    }
}

for gvr := range expectedOptional {
    if !actual.Has(gvr) {
        framework.Logf("Optional API not present (ok): %v", gvr)
    }
}
```

**Why version-specific data:**
- Test binary may have different vendored k8s.io dependencies than the live cluster
- Cluster may be n-1 or n+1 from what the test binary was built against
- Static data is generated once per supported kube version during openshift/api rebase
- Test runtime queries cluster version and loads matching data — no scheme needed
- Eliminates version skew problems entirely
- All in Go code, no YAML parsing needed

**Handling missing version data (during rebase):**

When a cluster is running Kubernetes 1.37 but openshift/api hasn't been updated yet to include
version 1.37 in the generated code, `ForProfileAndVersion()` returns `(nil, nil, false)` and the test skips:

```go
// In test body
required, optional, found := servedapis.ForProfileAndVersion(profile, featureSet, kubeVersion)
if !found {
    e2eskipper.Skipf("API inventory for profile=%s featureSet=%s kubeVersion=%s not found. This is expected during Kubernetes rebase. Update openshift/api and regenerate servedapis/zz_generated.served_apis.go", profile, featureSet, kubeVersion)
}

// Normal test flow - validate everything
// required contains: Kubernetes APIs + core OpenShift CRDs + aggregated servers
// optional contains: APIs from optional operators (monitoring, OLM, machine-api, etc.)
// ... add overrides, compare against discovery
```

**Why skip the entire test:**
- Simple - no complex logic to filter Kubernetes APIs from comparison
- Ginkgo marks as skipped (yellow in CI), not passed (green) - clear signal validation is incomplete
- Forces attention during rebase - can't be ignored like a warning
- During rebase, repos update in sequence (kubernetes → openshift/api → origin)
- Once openshift/api is updated with new version file, test automatically starts running again
- Prevents test from blocking CI during rebase window

**Why not validate OpenShift APIs only:**
- Would need complex group filtering to avoid "unexpected API" failures on Kubernetes APIs
- Partial validation is misleading - better to be explicit that validation is incomplete
- Simpler to skip and wait for openshift/api update

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

During a Kubernetes rebase in openshift/api:

1. **Update vendored k8s.io dependencies** in openshift/api (update `k8s.io/api`, `k8s.io/client-go`, `k8s.io/kubernetes`)
2. **Regenerate Kubernetes API data**: Run the generator (via `make update`) to update `zz_generated.served_apis.go` with new version data
3. **Review the diff** in `servedapis/zz_generated.served_apis.go` — check the new `kubeAPIs{version}` variable to see which APIs changed
4. **Update override map** if needed (see below)
5. **Vendor bump in origin**: When origin vendors the updated openshift/api, the test automatically picks up the new Kubernetes API data

**What changes automatically**:
- New resources added to existing stable GroupVersions → scheme-based generation picks them up
- Resources removed from Kubernetes → absent from generated data
- GroupVersions moving from disabled to enabled (or vice versa) → reflected in `DefaultAPIResourceConfigSource()`
- The generated `kubeAPIs{version}` variables capture the complete state for that kube version

**What needs manual attention**:
- **Kubernetes API override map**: If the rebase changes which alpha/beta APIs exist for a feature-gate-enabled GroupVersion (e.g., `v1alpha1` → `v1beta1` for MutatingAdmissionPolicy), update `features/kube_api_overrides.go`
- **New kube version**: Add new case to `ForProfileAndVersion()` function and generate new `kubeAPIs{newVersion}` variable
- **OpenShift CRD changes**: If the rebase changes OpenShift CRDs, `make update` regenerates both kube and OpenShift inventories

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

At test time, Kinds are converted to GVRs via `UnsafeGuessKindToResource()`. The test
queries the cluster's actual Kubernetes version (from `ClusterVersion.Status.Desired.Version`
or kube-apiserver version endpoint), and filters the override map to only entries whose
`KubeVersionRange` matches that version.

---

## Part C: Edge Cases

### Optional vs Required Boundary
- **Required**: All Kubernetes APIs, core OpenShift CRDs (in `payload-manifests/crds/`), aggregated API server resources (openshift-apiserver, oauth-apiserver)
- **Optional**: APIs from optional operators outside the core payload:
  - monitoring.coreos.com (Prometheus operator)
  - operators.coreos.com, packages.operators.coreos.com (OLM)
  - machine.openshift.io, autoscaling.openshift.io, metal3.io (machine management)
  - tuned.openshift.io, performance.openshift.io (node tuning)
  - helm.openshift.io, cloudcredential.openshift.io

The distinction is made at generation time - the generator categorizes each API as required or optional based on its source.

### HyperShift Differences
- oauth-apiserver may not serve APIs when external OIDC is used → consider marking oauth-apiserver APIs as optional specifically for Hypershift, or detecting OIDC mode
- Hypershift-specific CRDs (e.g., `criocredentialproviderconfigs-Hypershift.crd.yaml`) are handled by the separate Hypershift inventory file

### Discovery API Reliability
- `ServerGroupsAndResources()` can return partial results if an aggregated API server is restarting — the test should use `discovery.IsGroupDiscoveryFailedError` and retry or fail clearly

### TechPreview/DevPreview Feature Sets
The test **skips** on TechPreview and DevPreview feature sets:
- TechPreview CRDs change frequently (5 CRDs are TP-only currently: backups, clustermonitorings, etcdbackups, ingresses, pkis)
- Would add significant volatility to the test
- Hard to keep inventory up-to-date with frequent TP changes
- Test focuses on production/Default configurations which are stable
- TP functionality is validated through other test suites specific to those features

**Rationale**: The value of this test is catching unexpected API changes in production configurations. TechPreview APIs are expected to change frequently by definition, so validating them here adds noise without much benefit.

---

## Verification

1. **openshift/api CI**: `make verify` includes `verify-served-api-inventory` — ensures generated file is in sync
2. **openshift/api rebase**: After k8s vendor bump → `make update` → review diff in `zz_generated.served_apis.go`
3. **origin e2e**: Test runs as `[Suite:openshift/conformance/parallel]` on SelfManaged and HyperShift clusters with Default feature set (skips TechPreview/DevPreview to avoid volatility)
4. **During development**: After modifying CRDs or feature gates → `make update` in openshift/api → review diff
5. **Version skew handled**: Test queries cluster version and calls `ForProfileAndVersion()` with it, eliminating test-binary vs cluster-version mismatch

---

## Implementation Order

### In openshift/api:

1. Create `servedapis/types.go`
2. Create generator package `payload-command/servedapis/`:
   - `generator.go` — main orchestration for OpenShift APIs
   - `kube_api_derivation.go` — scheme-based Kubernetes API derivation per version
   - `aggregated_apis.go` — hardcoded aggregated API server lists
   - `optional_apis.go` — optional operator API lists
3. Create `features/kube_api_overrides.go` — feature-gate → Kubernetes API mapping with version ranges
4. Create `payload-command/cmd/write-served-api-inventory/main.go`
5. Create `hack/update-served-api-inventory.sh` and `hack/verify-served-api-inventory.sh`
6. Wire into Makefile: `update-served-api-inventory`, `verify-served-api-inventory`, `build`
7. Run generator → produces:
   - `servedapis/zz_generated.served_apis.go` (all inventory data as Go code)

### In origin:

8. Vendor updated openshift/api
9. Create `test/extended/apiserver/served_api_inventory.go`:
   - Call `servedapis.ForProfileAndVersion()` from vendored openshift/api
   - Detect cluster profile, feature set, kube version
   - Build expected sets from three sources
   - Query discovery
   - Bidirectional compare
10. Test against real cluster

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
