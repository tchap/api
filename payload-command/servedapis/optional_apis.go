package servedapis

import "github.com/openshift/api/servedapis"

// optionalAPIEntries returns the list of optional operator APIs that may or may not be
// present depending on the cluster configuration. These APIs are not failed on if absent.
// The same set applies to all cluster profiles.
func optionalAPIEntries() []servedapis.ServedAPIEntry {
	return []servedapis.ServedAPIEntry{
		// etcd.openshift.io — pacemaker-based HA etcd clusters only
		{Group: "etcd.openshift.io", Version: "v1", Resource: "pacemakerclusters", Kind: "PacemakerCluster", Scope: servedapis.ScopeCluster, Source: servedapis.SourceOpenShiftCRD},
		{Group: "etcd.openshift.io", Version: "v1alpha1", Resource: "pacemakerclusters", Kind: "PacemakerCluster", Scope: servedapis.ScopeCluster, Source: servedapis.SourceOpenShiftCRD},

		// autoscaling.openshift.io
		{Group: "autoscaling.openshift.io", Version: "v1", Resource: "clusterautoscalers", Kind: "ClusterAutoscaler", Scope: servedapis.ScopeCluster, Source: servedapis.SourceOpenShiftCRD},
		{Group: "autoscaling.openshift.io", Version: "v1beta1", Resource: "machineautoscalers", Kind: "MachineAutoscaler", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftCRD},

		// cloudcredential.openshift.io
		{Group: "cloudcredential.openshift.io", Version: "v1", Resource: "credentialsrequests", Kind: "CredentialsRequest", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftCRD},

		// gateway.networking.k8s.io
		{Group: "gateway.networking.k8s.io", Version: "v1", Resource: "backendtlspolicies", Kind: "BackendTLSPolicy", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftCRD},
		{Group: "gateway.networking.k8s.io", Version: "v1", Resource: "gatewayclasses", Kind: "GatewayClass", Scope: servedapis.ScopeCluster, Source: servedapis.SourceOpenShiftCRD},
		{Group: "gateway.networking.k8s.io", Version: "v1", Resource: "gateways", Kind: "Gateway", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftCRD},
		{Group: "gateway.networking.k8s.io", Version: "v1", Resource: "grpcroutes", Kind: "GRPCRoute", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftCRD},
		{Group: "gateway.networking.k8s.io", Version: "v1", Resource: "httproutes", Kind: "HTTPRoute", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftCRD},
		{Group: "gateway.networking.k8s.io", Version: "v1", Resource: "listenersets", Kind: "ListenerSet", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftCRD},
		{Group: "gateway.networking.k8s.io", Version: "v1", Resource: "referencegrants", Kind: "ReferenceGrant", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftCRD},
		{Group: "gateway.networking.k8s.io", Version: "v1", Resource: "tlsroutes", Kind: "TLSRoute", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftCRD},
		{Group: "gateway.networking.k8s.io", Version: "v1beta1", Resource: "gatewayclasses", Kind: "GatewayClass", Scope: servedapis.ScopeCluster, Source: servedapis.SourceOpenShiftCRD},
		{Group: "gateway.networking.k8s.io", Version: "v1beta1", Resource: "gateways", Kind: "Gateway", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftCRD},
		{Group: "gateway.networking.k8s.io", Version: "v1beta1", Resource: "httproutes", Kind: "HTTPRoute", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftCRD},
		{Group: "gateway.networking.k8s.io", Version: "v1beta1", Resource: "referencegrants", Kind: "ReferenceGrant", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftCRD},

		// helm.openshift.io
		{Group: "helm.openshift.io", Version: "v1beta1", Resource: "helmchartrepositories", Kind: "HelmChartRepository", Scope: servedapis.ScopeCluster, Source: servedapis.SourceOpenShiftCRD},
		{Group: "helm.openshift.io", Version: "v1beta1", Resource: "projecthelmchartrepositories", Kind: "ProjectHelmChartRepository", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftCRD},

		// infrastructure.cluster.x-k8s.io — Cluster API infrastructure providers (bare metal)
		{Group: "infrastructure.cluster.x-k8s.io", Version: "v1beta1", Resource: "metal3remediations", Kind: "Metal3Remediation", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftCRD},
		{Group: "infrastructure.cluster.x-k8s.io", Version: "v1beta1", Resource: "metal3remediationtemplates", Kind: "Metal3RemediationTemplate", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftCRD},

		// ipam.cluster.x-k8s.io — Cluster API IPAM
		{Group: "ipam.cluster.x-k8s.io", Version: "v1alpha1", Resource: "ipaddressclaims", Kind: "IPAddressClaim", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftCRD},
		{Group: "ipam.cluster.x-k8s.io", Version: "v1alpha1", Resource: "ipaddresses", Kind: "IPAddress", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftCRD},
		{Group: "ipam.cluster.x-k8s.io", Version: "v1beta1", Resource: "ipaddressclaims", Kind: "IPAddressClaim", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftCRD},
		{Group: "ipam.cluster.x-k8s.io", Version: "v1beta1", Resource: "ipaddresses", Kind: "IPAddress", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftCRD},

		// k8s.cni.cncf.io — Multus CNI
		{Group: "k8s.cni.cncf.io", Version: "v1", Resource: "network-attachment-definitions", Kind: "NetworkAttachmentDefinition", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftCRD},
		{Group: "k8s.cni.cncf.io", Version: "v1alpha1", Resource: "ipamclaims", Kind: "IPAMClaim", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftCRD},

		// k8s.ovn.org — OVN-Kubernetes
		{Group: "k8s.ovn.org", Version: "v1", Resource: "adminpolicybasedexternalroutes", Kind: "AdminPolicyBasedExternalRoute", Scope: servedapis.ScopeCluster, Source: servedapis.SourceOpenShiftCRD},
		{Group: "k8s.ovn.org", Version: "v1", Resource: "clusteruserdefinednetworks", Kind: "ClusterUserDefinedNetwork", Scope: servedapis.ScopeCluster, Source: servedapis.SourceOpenShiftCRD},
		{Group: "k8s.ovn.org", Version: "v1", Resource: "egressfirewalls", Kind: "EgressFirewall", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftCRD},
		{Group: "k8s.ovn.org", Version: "v1", Resource: "egressips", Kind: "EgressIP", Scope: servedapis.ScopeCluster, Source: servedapis.SourceOpenShiftCRD},
		{Group: "k8s.ovn.org", Version: "v1", Resource: "egressqoses", Kind: "EgressQoS", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftCRD},
		{Group: "k8s.ovn.org", Version: "v1", Resource: "egressservices", Kind: "EgressService", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftCRD},
		{Group: "k8s.ovn.org", Version: "v1", Resource: "userdefinednetworks", Kind: "UserDefinedNetwork", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftCRD},

		// machine.openshift.io
		{Group: "machine.openshift.io", Version: "v1beta1", Resource: "machinehealthchecks", Kind: "MachineHealthCheck", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftCRD},
		{Group: "machine.openshift.io", Version: "v1beta1", Resource: "machines", Kind: "Machine", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftCRD},
		{Group: "machine.openshift.io", Version: "v1beta1", Resource: "machinesets", Kind: "MachineSet", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftCRD},

		// metal3.io
		{Group: "metal3.io", Version: "v1alpha1", Resource: "baremetalhosts", Kind: "BareMetalHost", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftCRD},
		{Group: "metal3.io", Version: "v1alpha1", Resource: "bmceventsubscriptions", Kind: "BMCEventSubscription", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftCRD},
		{Group: "metal3.io", Version: "v1alpha1", Resource: "dataimages", Kind: "DataImage", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftCRD},
		{Group: "metal3.io", Version: "v1alpha1", Resource: "firmwareschemas", Kind: "FirmwareSchema", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftCRD},
		{Group: "metal3.io", Version: "v1alpha1", Resource: "hardwaredata", Kind: "HardwareData", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftCRD},
		{Group: "metal3.io", Version: "v1alpha1", Resource: "hostbmcsecrets", Kind: "HostBMCSecret", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftCRD},
		{Group: "metal3.io", Version: "v1alpha1", Resource: "hostfirmwarecomponents", Kind: "HostFirmwareComponents", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftCRD},
		{Group: "metal3.io", Version: "v1alpha1", Resource: "hostfirmwaresettings", Kind: "HostFirmwareSettings", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftCRD},
		{Group: "metal3.io", Version: "v1alpha1", Resource: "hostupdatepolicies", Kind: "HostUpdatePolicy", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftCRD},
		{Group: "metal3.io", Version: "v1alpha1", Resource: "preprovisioningimages", Kind: "PreprovisioningImage", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftCRD},
		{Group: "metal3.io", Version: "v1alpha1", Resource: "provisionings", Kind: "Provisioning", Scope: servedapis.ScopeCluster, Source: servedapis.SourceOpenShiftCRD},

		// metrics.k8s.io — served by kube-aggregator / metrics-server
		{Group: "metrics.k8s.io", Version: "v1beta1", Resource: "nodes", Kind: "NodeMetrics", Scope: servedapis.ScopeCluster, Source: servedapis.SourceOpenShiftCRD},
		{Group: "metrics.k8s.io", Version: "v1beta1", Resource: "pods", Kind: "PodMetrics", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftCRD},

		// migration.k8s.io — kube-storage-version-migrator
		{Group: "migration.k8s.io", Version: "v1alpha1", Resource: "storagestates", Kind: "StorageState", Scope: servedapis.ScopeCluster, Source: servedapis.SourceOpenShiftCRD},
		{Group: "migration.k8s.io", Version: "v1alpha1", Resource: "storageversionmigrations", Kind: "StorageVersionMigration", Scope: servedapis.ScopeCluster, Source: servedapis.SourceOpenShiftCRD},

		// monitoring.coreos.com
		{Group: "monitoring.coreos.com", Version: "v1", Resource: "alertmanagers", Kind: "Alertmanager", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftCRD},
		{Group: "monitoring.coreos.com", Version: "v1alpha1", Resource: "alertmanagerconfigs", Kind: "AlertmanagerConfig", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftCRD},
		{Group: "monitoring.coreos.com", Version: "v1beta1", Resource: "alertmanagerconfigs", Kind: "AlertmanagerConfig", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftCRD},
		{Group: "monitoring.coreos.com", Version: "v1", Resource: "podmonitors", Kind: "PodMonitor", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftCRD},
		{Group: "monitoring.coreos.com", Version: "v1", Resource: "probes", Kind: "Probe", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftCRD},
		{Group: "monitoring.coreos.com", Version: "v1alpha1", Resource: "prometheusagents", Kind: "PrometheusAgent", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftCRD},
		{Group: "monitoring.coreos.com", Version: "v1", Resource: "prometheuses", Kind: "Prometheus", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftCRD},
		{Group: "monitoring.coreos.com", Version: "v1", Resource: "prometheusrules", Kind: "PrometheusRule", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftCRD},
		{Group: "monitoring.coreos.com", Version: "v1alpha1", Resource: "scrapeconfigs", Kind: "ScrapeConfig", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftCRD},
		{Group: "monitoring.coreos.com", Version: "v1", Resource: "servicemonitors", Kind: "ServiceMonitor", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftCRD},
		{Group: "monitoring.coreos.com", Version: "v1", Resource: "thanosrulers", Kind: "ThanosRuler", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftCRD},

		// operator.openshift.io/v1alpha1 — deprecated APIs still served for backward compatibility.
		// The operator/v1alpha1 directory is excluded from the CRD walk (it contains alpha versions
		// superseded by operator/v1), but some resources in it are still actively served.
		{Group: "operator.openshift.io", Version: "v1alpha1", Resource: "imagecontentsourcepolicies", Kind: "ImageContentSourcePolicy", Scope: servedapis.ScopeCluster, Source: servedapis.SourceOpenShiftCRD},

		// olm.operatorframework.io — OLM v1
		{Group: "olm.operatorframework.io", Version: "v1", Resource: "clustercatalogs", Kind: "ClusterCatalog", Scope: servedapis.ScopeCluster, Source: servedapis.SourceOpenShiftCRD},
		{Group: "olm.operatorframework.io", Version: "v1", Resource: "clusterextensions", Kind: "ClusterExtension", Scope: servedapis.ScopeCluster, Source: servedapis.SourceOpenShiftCRD},

		// operators.coreos.com — OLM v0
		{Group: "operators.coreos.com", Version: "v1alpha1", Resource: "catalogsources", Kind: "CatalogSource", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftCRD},
		{Group: "operators.coreos.com", Version: "v1alpha1", Resource: "clusterserviceversions", Kind: "ClusterServiceVersion", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftCRD},
		{Group: "operators.coreos.com", Version: "v1alpha1", Resource: "installplans", Kind: "InstallPlan", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftCRD},
		{Group: "operators.coreos.com", Version: "v1", Resource: "olmconfigs", Kind: "OLMConfig", Scope: servedapis.ScopeCluster, Source: servedapis.SourceOpenShiftCRD},
		{Group: "operators.coreos.com", Version: "v1", Resource: "operatorconditions", Kind: "OperatorCondition", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftCRD},
		{Group: "operators.coreos.com", Version: "v1", Resource: "operatorgroups", Kind: "OperatorGroup", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftCRD},
		{Group: "operators.coreos.com", Version: "v1alpha2", Resource: "operatorgroups", Kind: "OperatorGroup", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftCRD},
		{Group: "operators.coreos.com", Version: "v1", Resource: "operators", Kind: "Operator", Scope: servedapis.ScopeCluster, Source: servedapis.SourceOpenShiftCRD},
		{Group: "operators.coreos.com", Version: "v1alpha1", Resource: "subscriptions", Kind: "Subscription", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftCRD},
		{Group: "operators.coreos.com", Version: "v2", Resource: "operatorconditions", Kind: "OperatorCondition", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftCRD},

		// packages.operators.coreos.com
		{Group: "packages.operators.coreos.com", Version: "v1", Resource: "packagemanifests", Kind: "PackageManifest", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftCRD},

		// performance.openshift.io
		{Group: "performance.openshift.io", Version: "v1alpha1", Resource: "performanceprofiles", Kind: "PerformanceProfile", Scope: servedapis.ScopeCluster, Source: servedapis.SourceOpenShiftCRD},
		{Group: "performance.openshift.io", Version: "v1", Resource: "performanceprofiles", Kind: "PerformanceProfile", Scope: servedapis.ScopeCluster, Source: servedapis.SourceOpenShiftCRD},
		{Group: "performance.openshift.io", Version: "v2", Resource: "performanceprofiles", Kind: "PerformanceProfile", Scope: servedapis.ScopeCluster, Source: servedapis.SourceOpenShiftCRD},

		// policy.networking.k8s.io — Admin Network Policy
		{Group: "policy.networking.k8s.io", Version: "v1alpha1", Resource: "adminnetworkpolicies", Kind: "AdminNetworkPolicy", Scope: servedapis.ScopeCluster, Source: servedapis.SourceOpenShiftCRD},
		{Group: "policy.networking.k8s.io", Version: "v1alpha1", Resource: "baselineadminnetworkpolicies", Kind: "BaselineAdminNetworkPolicy", Scope: servedapis.ScopeCluster, Source: servedapis.SourceOpenShiftCRD},

		// populator.storage.k8s.io
		{Group: "populator.storage.k8s.io", Version: "v1beta1", Resource: "volumepopulators", Kind: "VolumePopulator", Scope: servedapis.ScopeCluster, Source: servedapis.SourceOpenShiftCRD},

		// samples.operator.openshift.io — handled by zz_generated.crd-manifests walk, listed here
		// only if it needs to be optional rather than required.

		// snapshot.storage.k8s.io — CSI Volume Snapshots
		{Group: "snapshot.storage.k8s.io", Version: "v1", Resource: "volumesnapshotclasses", Kind: "VolumeSnapshotClass", Scope: servedapis.ScopeCluster, Source: servedapis.SourceOpenShiftCRD},
		{Group: "snapshot.storage.k8s.io", Version: "v1", Resource: "volumesnapshotcontents", Kind: "VolumeSnapshotContent", Scope: servedapis.ScopeCluster, Source: servedapis.SourceOpenShiftCRD},
		{Group: "snapshot.storage.k8s.io", Version: "v1", Resource: "volumesnapshots", Kind: "VolumeSnapshot", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftCRD},

		// testextension.redhat.io — test-only, present on CI clusters
		{Group: "testextension.redhat.io", Version: "v1", Resource: "testextensionadmissions", Kind: "TestExtensionAdmission", Scope: servedapis.ScopeCluster, Source: servedapis.SourceOpenShiftCRD},

		// network.openshift.io — OpenShift SDN; absent on OVN-Kubernetes clusters
		{Group: "network.openshift.io", Version: "v1", Resource: "clusternetworks", Kind: "ClusterNetwork", Scope: servedapis.ScopeCluster, Source: servedapis.SourceOpenShiftCRD},
		{Group: "network.openshift.io", Version: "v1", Resource: "egressnetworkpolicies", Kind: "EgressNetworkPolicy", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftCRD},
		{Group: "network.openshift.io", Version: "v1", Resource: "hostsubnets", Kind: "HostSubnet", Scope: servedapis.ScopeCluster, Source: servedapis.SourceOpenShiftCRD},
		{Group: "network.openshift.io", Version: "v1", Resource: "netnamespaces", Kind: "NetNamespace", Scope: servedapis.ScopeCluster, Source: servedapis.SourceOpenShiftCRD},

		// sharedresource.openshift.io — TechPreview SharedResource CSI Driver
		// Note: these CRDs are missing the release.openshift.io/feature-gate annotation
		// that would normally cause the generator to filter them; listed here explicitly.
		{Group: "sharedresource.openshift.io", Version: "v1alpha1", Resource: "sharedconfigmaps", Kind: "SharedConfigMap", Scope: servedapis.ScopeCluster, Source: servedapis.SourceOpenShiftCRD},
		{Group: "sharedresource.openshift.io", Version: "v1alpha1", Resource: "sharedsecrets", Kind: "SharedSecret", Scope: servedapis.ScopeCluster, Source: servedapis.SourceOpenShiftCRD},

		// tuned.openshift.io
		{Group: "tuned.openshift.io", Version: "v1", Resource: "profiles", Kind: "Profile", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftCRD},
		{Group: "tuned.openshift.io", Version: "v1", Resource: "tuneds", Kind: "Tuned", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftCRD},

		// whereabouts.cni.cncf.io — Whereabouts IPAM CNI
		{Group: "whereabouts.cni.cncf.io", Version: "v1alpha1", Resource: "ippools", Kind: "IPPool", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftCRD},
		{Group: "whereabouts.cni.cncf.io", Version: "v1alpha1", Resource: "nodeslicepools", Kind: "NodeSlicePool", Scope: servedapis.ScopeCluster, Source: servedapis.SourceOpenShiftCRD},
		{Group: "whereabouts.cni.cncf.io", Version: "v1alpha1", Resource: "overlappingrangeipreservations", Kind: "OverlappingRangeIPReservation", Scope: servedapis.ScopeCluster, Source: servedapis.SourceOpenShiftCRD},
	}
}
