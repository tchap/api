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

OpenShift may enable **additional Kubernetes feature gates** beyond upstream defaults. When it does,
those gates may enable alpha/beta APIs that aren't in upstream `DefaultAPIResourceConfigSource()`.

**Handling in this design:**
- The Kubernetes inventory is generated from origin's vendored k8s (not upstream k8s)
- If OpenShift patches the kube-apiserver to enable extra gates by default, those APIs will be in `DefaultAPIResourceConfigSource()` in origin's vendor
- Generator captures whatever is actually enabled in origin's build → static inventory is complete
- No runtime gate consultation needed - Default feature set has deterministic API surface

**Example**: If OpenShift 4.19 enables the `MutatingAdmissionPolicy` feature gate by default:
- origin's k8s vendor (potentially patched) has this gate enabled
- Generator runs against origin's vendor → sees `admissionregistration.k8s.io/v1beta1` MutatingAdmissionPolicy enabled
- Static inventory includes it → test validates it's served

## Test Flow

The e2e test in origin builds the expected API set from static inventory, queries the
cluster, and compares:

```
 1. Detect cluster state
    ├── profile      ← Infrastructure.Status.ControlPlaneTopology (Hypershift vs SelfManaged)
    ├── featureSet   ← FeatureGate("cluster").Spec.FeatureSet → skip test if not "Default"
    └── kubeVersion  ← ClusterVersion.Status.Desired.Version → parse kube minor (e.g., "1.35")

 2. Build expected API set (purely static, no runtime gate consultation)
    │
    ├─ OpenShift APIs from vendored o/api
    │  servedapis.ForProfile(profile)
    │  → OpenShift CRDs (Default feature set only)
    │  → Aggregated API server resources (openshift-apiserver, oauth-apiserver)
    │  → Optional operator APIs (monitoring, OLM, machine-api, etc.)
    │  → Returns: required, optional lists
    │
    └─ Kubernetes APIs from local origin generation
       inventory.ForKubeVersion(kubeVersion)
       → Kubernetes built-in APIs for this k8s version
       → Generated using DefaultAPIResourceConfigSource() at build time
       → Returns: kubeAPIs list (all required)
       → Returns found=false if kubeVersion not in generated inventory

    Combine: required = osRequired + kubeAPIs
             optional = osOptional

 3. Query cluster
    discovery.ServerGroupsAndResources()
    → filter out subresources (names containing "/")
    → convert to GVR set

 4. Compare (bidirectional)
    missing  = expectedRequired - actual  → FAIL (with clear list of missing GVRs)
    unknown  = actual - (required ∪ optional) → FAIL (with clear list of unexpected GVRs)
    optional absent = expectedOptional - actual → OK (log only for visibility)
```

**Why no runtime feature gate consultation:**
- Default feature set has a deterministic API surface
- Static inventory captures the complete expected state for Default
- Any APIs gated by feature flags are either:
  - Included in Default → already in static inventory
  - Not in Default → test skips (we don't validate TechPreview)
- Simpler, more maintainable, no need for gate-to-API mapping

## Architecture

**Split generation between openshift/api (OpenShift APIs) and origin (Kubernetes APIs)**

```
openshift/api                              origin
┌──────────────────────────────┐     ┌──────────────────────────────────┐
│ payload-command/cmd/         │     │ test/extended/apiserver/         │
│   write-served-api-inventory │     │   inventory/                     │
│                              │     │                                  │
│ payload-command/servedapis/  │     │   write-kube-api-inventory/      │
│   generator.go               │     │     main.go                      │
│   aggregated_apis.go         │     │     generator.go                 │
│   optional_apis.go           │     │     (DefaultAPIResourceConfig    │
│   (reads CRDs, no k8s vendor)│     │      + scheme → GVRs)            │
│                              │     │                                  │
│ servedapis/                  │     │   zz_generated_kubernetes.go     │
│   types.go                   │     │     · kubeAPIs135 = []...        │
│   zz_generated_openshift.go  │──┐  │     · kubeAPIs136 = []...        │
│     · osAPIs{Profile} = []...│  │  │     · ForKubeVersion()           │
│     · ForProfile()           │  │  │                                  │
│ (vendored into origin)───────┘  └─>│   served_api_inventory_test.go   │
│                              │     │     1. Import o/api OpenShift    │
│ payload-manifests/crds/      │     │     2. Import local Kube         │
│   (source for OpenShift APIs)│     │     3. Combine + compare         │
│                              │     │                                  │
│ hack/                        │     │   hack/                          │
│   update-served-api-inv.sh   │     │     update-kube-api-inv.sh       │
│   verify-served-api-inv.sh   │     │     verify-kube-api-inv.sh       │
└──────────────────────────────┘     └──────────────────────────────────┘
     ↑ make verify fails if               ↑ make verify fails if
       CRD added w/o regen                   kube vendor bump w/o regen
```

### Key insight: Split generation to avoid kubernetes vendor in openshift/api

**Why split:**
- openshift/api is a lightweight API definition repository
- Vendoring kubernetes would add massive bloat (~100MB+)
- origin already vendors kubernetes for its own needs

**Generation split:**
1. **openshift/api**: Generates OpenShift-only inventory (CRDs + aggregated servers + optional APIs)
   - No kubernetes vendor required
   - CI verifies regeneration on CRD changes
   - Exported via `servedapis.ForProfile(profile)`

2. **origin**: Generates Kubernetes-only inventory using `DefaultAPIResourceConfigSource()`
   - Already has kubernetes vendored
   - Generates versioned data per Kubernetes version: `kubeAPIs135`, `kubeAPIs136`, etc.
   - Exported via `inventory.ForKubeVersion(kubeVersion)`

3. **Test**: Combines both at runtime
   ```go
   osRequired, osOptional := servedapis.ForProfile(profile)  // from vendored o/api
   kubeAPIs := inventory.ForKubeVersion(kubeVersion)         // from local origin
   required = append(osRequired, kubeAPIs...)
   ```

**Why per-version Kubernetes data:** The test runs against a live cluster whose Kubernetes version
may not match what's vendored in the origin test binary (during rebase windows, in dev
environments). Versioned data makes the test robust to version skew.

### Why OpenShift APIs aren't versioned by Kubernetes version

**Key insight:** OpenShift API inventory doesn't need explicit Kubernetes versioning because:

1. **OpenShift APIs are defined by OpenShift release, not Kubernetes version**
   - A `ClusterVersion` CRD is the same whether running on k8s 1.35 or 1.36
   - OpenShift CRDs don't vary based on the underlying Kubernetes version
   - What matters is "which OpenShift release" not "which Kubernetes version"

2. **Vendor mechanism handles OpenShift versioning implicitly**
   - origin vendors a specific commit of openshift/api
   - That vendored snapshot contains the OpenShift APIs for that origin build
   - Test uses whatever OpenShift APIs were vendored when the test binary was built
   - This naturally matches what the cluster is serving (same origin build)

3. **Kubernetes APIs DO vary by version**
   - New APIs added: e.g., k8s 1.36 adds new resources not in 1.35
   - APIs removed: beta APIs graduate and old versions stop being served
   - Test binary built against k8s 1.35 needs data for k8s 1.36 clusters
   - Hence explicit versioning: `kubeAPIs135`, `kubeAPIs136`, etc.

**Example rebase scenario:**
```
OpenShift 4.19 ships with k8s 1.35:
  - origin vendors o/api commit abc123 (contains 150 OpenShift CRDs)
  - origin vendors k8s 1.35
  - Test uses: OpenShift APIs from abc123 + k8s 1.35 data

OpenShift 4.20 ships with k8s 1.36:
  - origin vendors o/api commit def456 (contains 152 OpenShift CRDs - 2 added)
  - origin vendors k8s 1.36
  - Regenerate: adds kubeAPIs136 data
  - Test uses: OpenShift APIs from def456 + k8s 1.36 data

During rebase window (4.19 test binary, 4.20 cluster):
  - Test queries cluster version → k8s 1.36
  - OpenShift APIs: uses vendored abc123 (slightly stale, but close enough)
  - Kubernetes APIs: uses kubeAPIs136 data (correct for cluster)
  - Test might fail on 2 new OpenShift CRDs → expected, skip test
```

**The split is:**
- **Explicit** Kubernetes versioning in origin (multiple versions supported)
- **Implicit** OpenShift versioning via vendor (one version per origin build)

### How Kubernetes API derivation works (in origin)

**Note:** This happens in origin's generator, not openshift/api, because it requires
vendoring kubernetes (which origin already has).

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

---

## Part A: OpenShift API Inventory Generator (openshift/api)

**Scope:** Generates OpenShift-only inventory (CRDs + aggregated servers + optional APIs).
Does NOT include Kubernetes APIs (to avoid vendoring kubernetes dependency).

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

type ClusterProfile string
const (
    ClusterProfileSelfManagedHA ClusterProfile = "SelfManagedHA"
    ClusterProfileHypershift    ClusterProfile = "Hypershift"
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

**`servedapis/zz_generated_openshift.go`** — generated file containing OpenShift-only inventory as Go literals. Provides:
```go
package servedapis

// Returns required and optional OpenShift APIs for a given cluster profile
// Only supports Default feature set - test skips on TechPreview/DevPreview
// Does NOT include Kubernetes APIs - those are generated separately in origin
func ForProfile(clusterProfile ClusterProfile) (required, optional []ServedAPIEntry)
```

Returns two separate lists:
- **required**: OpenShift APIs that must be present (test fails if missing) - core CRDs, aggregated servers
- **optional**: APIs from optional operators (test logs if missing, doesn't fail) - monitoring, OLM, machine-api, etc.

This file gets vendored into origin automatically. Contains OpenShift APIs only.

### A2. Generator Tool — `payload-command/cmd/write-served-api-inventory/`

Standalone binary following the pattern of `write-available-featuresets`. Takes `--go-output-dir` flag for the generated Go file.

**Generator logic** in `payload-command/servedapis/`:

1. **`generator.go`** — main orchestration (OpenShift APIs only):
     - Reads CRD manifests from `payload-manifests/crds/`
     - Parses filename suffixes for profile detection (only processes Default feature set CRDs)
     - Extracts served GVRs from each CRD's `spec.versions[].served`, `spec.group`, `spec.names.plural/kind`, `spec.scope`
     - Merges with hardcoded aggregated API server entries (from `aggregated_apis.go`)
     - Merges with optional API entries (from `optional_apis.go`)
     - Generates Go variables for each profile (SelfManagedHA, Hypershift) - Default feature set only
   
   - **Output**: Single `zz_generated_openshift.go` file containing OpenShift inventory as Go literals

2. **`aggregated_apis.go`** — hardcoded lists for openshift-apiserver (9 groups, ~35 resources) and oauth-apiserver (2 groups, ~10 resources). Based on the exploration findings. Changes very rarely.

3. **`optional_apis.go`** — APIs from optional operators (returned in the optional list, not required):
   - monitoring.coreos.com (alertmanagers, prometheuses, servicemonitors, etc.) - `Source: SourceOpenShiftCRD`
   - operators.coreos.com (clusterserviceversions, subscriptions, etc.) - `Source: SourceOpenShiftCRD`
   - packages.operators.coreos.com (packagemanifests) - `Source: SourceOpenShiftCRD`
   - machine.openshift.io (machines, machinesets, machinehealthchecks) - `Source: SourceOpenShiftCRD`
   - autoscaling.openshift.io (clusterautoscalers, machineautoscalers) - `Source: SourceOpenShiftCRD`
   - metal3.io (baremetalhosts, provisionings, etc.) - `Source: SourceOpenShiftCRD`
   - tuned.openshift.io, performance.openshift.io - `Source: SourceOpenShiftCRD`
   - helm.openshift.io - `Source: SourceOpenShiftCRD`
   - cloudcredential.openshift.io - `Source: SourceOpenShiftCRD`

### A3. CRD Variant Detection

The generator determines which ClusterProfile a CRD applies to using two mechanisms:

**1. Filename suffix** — determines which CRD schema variant to use:
- No suffix: same schema for all profiles
- `-Hypershift`, `-SelfManagedHA`: profile-specific schema
- `-Default`: schema applies to Default feature set (the only one we support)

**2. Annotations** — determine deployment targets:
- `include.release.openshift.io/*` annotations determine profile availability:
  - `ibm-cloud-managed` = Hypershift
  - `self-managed-high-availability` = SelfManagedHA
- `release.openshift.io/feature-set` annotation determines feature set deployment:
  - Absent or empty: CRD is deployed on Default (included in inventory)
  - `TechPreviewNoUpgrade,DevPreviewNoUpgrade,CustomNoUpgrade`: CRD is **NOT** deployed on Default → **skip this CRD**

**TechPreview-only CRDs excluded from inventory** (currently 5 resources):
- `backups.config.openshift.io`
- `clustermonitorings.config.openshift.io`
- `etcdbackups.operator.openshift.io`
- `ingresses.operator.openshift.io`
- `pkis.config.openshift.io`

The generator only processes CRDs available on the Default feature set, creating separate inventories for SelfManagedHA and Hypershift profiles.

### A4. Output Files

**`servedapis/zz_generated.served_apis.go`** — single generated Go file containing all inventory data:

```go
package servedapis

// Required OpenShift API variables (core CRDs, aggregated servers - per profile, Default feature set only)
var requiredSelfManagedHA = []ServedAPIEntry{ ... }
var requiredHypershift = []ServedAPIEntry{ ... }

// Optional OpenShift API variables (optional operators - per profile, Default feature set only)
var optionalSelfManagedHA = []ServedAPIEntry{
    {Group: "monitoring.coreos.com", Version: "v1", Resource: "prometheuses", Kind: "Prometheus", Scope: "Namespaced", Source: SourceOpenShiftCRD},
    {Group: "operators.coreos.com", Version: "v1alpha1", Resource: "subscriptions", Kind: "Subscription", Scope: "Namespaced", Source: SourceOpenShiftCRD},
    // ... monitoring, OLM, machine-api, metal3, etc.
}
var optionalHypershift = []ServedAPIEntry{ ... }

// Kubernetes API variables (one per supported version)
var kubeAPIs135 = []ServedAPIEntry{
    {Group: "apps", Version: "v1", Resource: "deployments", Kind: "Deployment", Scope: "Namespaced", Source: SourceCoreKube},
    {Group: "apps", Version: "v1", Resource: "statefulsets", Kind: "StatefulSet", Scope: "Namespaced", Source: SourceCoreKube},
    // ... ~100+ entries per version
}
var kubeAPIs136 = []ServedAPIEntry{ ... }
var kubeAPIs137 = []ServedAPIEntry{ ... }

// Lookup function - returns required and optional APIs separately
func ForProfileAndVersion(clusterProfile ClusterProfile, kubeVersion *version.Version) (required, optional []ServedAPIEntry, found bool) {
    // Get OpenShift APIs (required and optional)
    var requiredOpenShift, optionalOpenShift []ServedAPIEntry
    switch clusterProfile {
    case ClusterProfileSelfManagedHA:
        requiredOpenShift = requiredSelfManagedHA
        optionalOpenShift = optionalSelfManagedHA
    case ClusterProfileHypershift:
        requiredOpenShift = requiredHypershift
        optionalOpenShift = optionalHypershift
    default:
        return nil, nil, false
    }
    
    // Get Kubernetes APIs (always required) - uses major.minor only, ignores patch
    var kubeAPIs []ServedAPIEntry
    if kubeVersion.Major() == 1 {
        switch kubeVersion.Minor() {
        case 35:
            kubeAPIs = kubeAPIs135
        case 36:
            kubeAPIs = kubeAPIs136
        case 37:
            kubeAPIs = kubeAPIs137
        default:
            return nil, nil, false  // minor version not found
        }
    } else {
        return nil, nil, false  // unexpected major version
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
   - `kubeVersion` = parse minor version from `ClusterVersion("version").Status.Desired.Version` (e.g., "4.19.0-0.nightly-2026-08-11-225619" → "1.35")

2. **Build expected sets** (purely static):
   - **OpenShift APIs**: `osRequired, osOptional := servedapis.ForProfile(profile)` from vendored openshift/api
   - **Kubernetes APIs**: `kubeAPIs, found := inventory.ForKubeVersion(kubeVersion)` from local origin generation
     - If `found == false` (during rebase window when kubeVersion not generated): skip entire test
   - **Combine**: `required = append(osRequired, kubeAPIs...)`, `optional = osOptional`
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
    "k8s.io/apimachinery/pkg/util/version"
)

func parseKubeVersion(clusterVersionStr string) (*version.Version, error) {
    // ClusterVersion.Status.Desired.Version format: "4.19.0-0.nightly-2026-08-11-225619"
    // Actual implementation should query kube-apiserver version or parse from ClusterVersion payload metadata
    // For now, extract kube version from the cluster version (mapping TBD)
    // Example: map OCP 4.19 → Kubernetes 1.35
    
    // Parse as semver - this handles "1.35.0" or "1.35.2" equally (both → major=1, minor=35)
    return version.ParseSemantic(kubeVersionString)
}

// In test body
profile := clusterProfileName(exutil.GetControlPlaneTopology(oc))  // returns servedapis.ClusterProfile constant
kubeVersion, err := parseKubeVersion(clusterVersionStr)
if err != nil {
    framework.Failf("Failed to parse Kubernetes version: %v", err)
}

// Get OpenShift APIs from vendored o/api
osRequired, osOptional := servedapis.ForProfile(profile)

// Get Kubernetes APIs from local origin generation
kubeAPIs, found := inventory.ForKubeVersion(kubeVersion)
if !found {
    e2eskipper.Skipf("API inventory for kubeVersion=%d.%d not found. This is expected during Kubernetes rebase.", kubeVersion.Major(), kubeVersion.Minor())
}

// Combine (no runtime feature gate consultation needed - static inventory is complete)
required := append(osRequired, kubeAPIs...)
optional := osOptional

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
required, optional, found := servedapis.ForProfileAndVersion(profile, kubeVersion)
if !found {
    e2eskipper.Skipf("API inventory for profile=%s kubeVersion=%d.%d not found. This is expected during Kubernetes rebase. Update openshift/api and regenerate servedapis/zz_generated.served_apis.go", profile, kubeVersion.Major(), kubeVersion.Minor())
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
func clusterProfileName(topology configv1.TopologyMode) servedapis.ClusterProfile {
    if topology == configv1.ExternalTopologyMode {
        return servedapis.ClusterProfileHypershift
    }
    return servedapis.ClusterProfileSelfManagedHA
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

## Rebase Workflow

### When rebasing to a new Kubernetes version (e.g., k8s 1.35 → 1.36)

**Step 1: Update origin kubernetes vendor**
```bash
# In origin repo
# Update go.mod to new k8s version
# go mod vendor
```

**Step 2: Regenerate Kubernetes API inventory in origin**
```bash
# In origin repo
make update-kube-api-inventory

# This generates/updates:
# - test/extended/apiserver/inventory/zz_generated_kubernetes.go
# - Adds: var kubeAPIs136 = []ServedAPIEntry{ ... }
# - Updates: ForKubeVersion() switch statement
```

**Step 3: Verify and commit in origin**
```bash
git diff test/extended/apiserver/inventory/zz_generated_kubernetes.go
# Review: new APIs added, old APIs removed, etc.

make verify-kube-api-inventory  # ensures no drift
git add test/extended/apiserver/inventory/zz_generated_kubernetes.go
git commit -m "Update Kubernetes API inventory for 1.36"
```

**Step 4: Update openshift/api vendor in origin (later)**
```bash
# After openshift/api merges any new CRDs for this release
# Update origin's vendor of openshift/api
# The test automatically picks up new OpenShift APIs via vendor
```

### When adding/modifying OpenShift CRDs

**In openshift/api:**
```bash
# Edit CRD files in payload-manifests/crds/
make update-served-api-inventory

# This regenerates:
# - servedapis/zz_generated_openshift.go (OpenShift APIs only)

make verify  # CI fails if you forget this
git commit -am "Add new MyCRD resource"
```

**In origin (after vendor bump):**
```bash
# Update vendor to pick up openshift/api changes
# No regeneration needed - vendor bump is enough
# Test automatically uses updated OpenShift APIs
```

### Key insight: Different update triggers

**Kubernetes inventory (origin):**
- **Trigger**: kubernetes vendor bump in origin
- **Action**: Regenerate Kubernetes API inventory
- **Adds**: New versioned data (kubeAPIsXXX)

**OpenShift inventory (openshift/api):**
- **Trigger**: CRD manifest changes in openshift/api
- **Action**: Regenerate OpenShift API inventory
- **Updates**: Same variables (requiredSelfManagedHA, etc.)

**Test (origin):**
- **Trigger**: vendor bump of openshift/api in origin
- **Action**: None - automatically uses new vendored data
- **Result**: Test validates updated OpenShift + versioned Kubernetes APIs

---

## Verification

### openshift/api CI (OpenShift APIs)
1. **make verify**: includes `verify-served-api-inventory` — ensures `zz_generated_openshift.go` is current
2. **CRD changes**: Adding/modifying CRDs without regenerating fails CI
3. **Review diffs**: Changes to `zz_generated_openshift.go` show exactly which OpenShift APIs changed

### origin CI (Kubernetes APIs + integration)
1. **make verify**: includes `verify-kube-api-inventory` — ensures `zz_generated_kubernetes.go` is current
2. **Kubernetes vendor bumps**: Updating k8s without regenerating fails CI
3. **e2e test**: Runs as `[Suite:openshift/conformance/parallel]` on SelfManaged and HyperShift clusters
   - Skips on TechPreview/DevPreview feature sets (only validates Default)
   - Queries cluster for actual served APIs
   - Compares against combined inventory (OpenShift from vendor + Kubernetes from local gen)

### Version skew handling
- **Kubernetes**: Test queries cluster version → uses matching `kubeAPIsXXX` data → handles test binary vs cluster mismatch
- **OpenShift**: Test uses vendored o/api snapshot → matches what the origin build ships → no mismatch in practice
- **Rebase window**: If cluster runs newer k8s version than test has data for → test skips (expected, not a failure)

---

## Implementation Order

### In openshift/api:

1. Create `servedapis/types.go` — shared types (ClusterProfile, Source, ServedAPIEntry)
2. Create generator package `payload-command/servedapis/`:
   - `generator.go` — reads CRDs, orchestrates generation (OpenShift APIs only, no k8s vendor)
   - `aggregated_apis.go` — hardcoded aggregated API server lists
   - `optional_apis.go` — optional operator API lists
3. Create `payload-command/cmd/write-served-api-inventory/main.go`
4. Create `hack/update-served-api-inventory.sh` and `hack/verify-served-api-inventory.sh`
5. Wire into Makefile: `update-served-api-inventory`, `verify-served-api-inventory`, `build`
6. Run generator → produces:
   - `servedapis/zz_generated_openshift.go` (OpenShift-only inventory as Go code)
   - Exported via `ForProfile(clusterProfile)`

### In origin:

7. Create `test/extended/apiserver/inventory/` package:
   - `types.go` — same ServedAPIEntry type (or import from o/api)
   - `generator.go` — Kubernetes API derivation using `DefaultAPIResourceConfigSource()` + scheme
   - `zz_generated_kubernetes.go` — generated Kubernetes-only inventory (versioned)
8. Create `test/extended/apiserver/inventory/write-kube-api-inventory/main.go`
9. Create `hack/update-kube-api-inventory.sh` and `hack/verify-kube-api-inventory.sh`
10. Wire into Makefile
11. Vendor updated openshift/api
12. Run Kubernetes generator → produces versioned data: `kubeAPIs135`, `kubeAPIs136`, etc.
13. Create `test/extended/apiserver/inventory/served_api_inventory_test.go`:
   - Import `servedapis.ForProfile()` from vendored openshift/api
   - Import `inventory.ForKubeVersion()` from local package
   - Detect cluster profile, feature set, kube version
   - Combine OpenShift + Kubernetes inventories
   - Query discovery, compare
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
