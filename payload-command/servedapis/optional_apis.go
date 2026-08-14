package servedapis

import "github.com/openshift/api/servedapis"

// optionalAPIEntries returns the list of optional operator APIs that may or may not be
// present depending on the cluster configuration. These APIs are not failed on if absent.
// The same set applies to all cluster profiles.
func optionalAPIEntries() []servedapis.ServedAPIEntry {
	return []servedapis.ServedAPIEntry{
		// autoscaling.openshift.io
		{Group: "autoscaling.openshift.io", Version: "v1", Resource: "clusterautoscalers", Kind: "ClusterAutoscaler", Scope: servedapis.ScopeCluster, Source: servedapis.SourceOpenShiftCRD},
		{Group: "autoscaling.openshift.io", Version: "v1beta1", Resource: "machineautoscalers", Kind: "MachineAutoscaler", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftCRD},

		// cloudcredential.openshift.io
		{Group: "cloudcredential.openshift.io", Version: "v1", Resource: "credentialsrequests", Kind: "CredentialsRequest", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftCRD},

		// helm.openshift.io
		{Group: "helm.openshift.io", Version: "v1beta1", Resource: "helmchartrepositories", Kind: "HelmChartRepository", Scope: servedapis.ScopeCluster, Source: servedapis.SourceOpenShiftCRD},
		{Group: "helm.openshift.io", Version: "v1beta1", Resource: "projecthelmchartrepositories", Kind: "ProjectHelmChartRepository", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftCRD},

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
		{Group: "metal3.io", Version: "v1alpha1", Resource: "preprovisioningimages", Kind: "PreprovisioningImage", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftCRD},
		{Group: "metal3.io", Version: "v1alpha1", Resource: "provisionings", Kind: "Provisioning", Scope: servedapis.ScopeCluster, Source: servedapis.SourceOpenShiftCRD},

		// monitoring.coreos.com
		{Group: "monitoring.coreos.com", Version: "v1", Resource: "alertmanagers", Kind: "Alertmanager", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftCRD},
		{Group: "monitoring.coreos.com", Version: "v1alpha1", Resource: "alertmanagerconfigs", Kind: "AlertmanagerConfig", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftCRD},
		{Group: "monitoring.coreos.com", Version: "v1", Resource: "podmonitors", Kind: "PodMonitor", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftCRD},
		{Group: "monitoring.coreos.com", Version: "v1", Resource: "probes", Kind: "Probe", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftCRD},
		{Group: "monitoring.coreos.com", Version: "v1alpha1", Resource: "prometheusagents", Kind: "PrometheusAgent", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftCRD},
		{Group: "monitoring.coreos.com", Version: "v1", Resource: "prometheuses", Kind: "Prometheus", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftCRD},
		{Group: "monitoring.coreos.com", Version: "v1", Resource: "prometheusrules", Kind: "PrometheusRule", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftCRD},
		{Group: "monitoring.coreos.com", Version: "v1alpha1", Resource: "scrapeconfigs", Kind: "ScrapeConfig", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftCRD},
		{Group: "monitoring.coreos.com", Version: "v1", Resource: "servicemonitors", Kind: "ServiceMonitor", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftCRD},
		{Group: "monitoring.coreos.com", Version: "v1", Resource: "thanosrulers", Kind: "ThanosRuler", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftCRD},

		// operators.coreos.com
		{Group: "operators.coreos.com", Version: "v1alpha1", Resource: "catalogsources", Kind: "CatalogSource", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftCRD},
		{Group: "operators.coreos.com", Version: "v1alpha1", Resource: "clusterserviceversions", Kind: "ClusterServiceVersion", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftCRD},
		{Group: "operators.coreos.com", Version: "v1alpha1", Resource: "installplans", Kind: "InstallPlan", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftCRD},
		{Group: "operators.coreos.com", Version: "v1", Resource: "operatorgroups", Kind: "OperatorGroup", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftCRD},
		{Group: "operators.coreos.com", Version: "v1", Resource: "operators", Kind: "Operator", Scope: servedapis.ScopeCluster, Source: servedapis.SourceOpenShiftCRD},
		{Group: "operators.coreos.com", Version: "v1alpha1", Resource: "subscriptions", Kind: "Subscription", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftCRD},

		// packages.operators.coreos.com
		{Group: "packages.operators.coreos.com", Version: "v1", Resource: "packagemanifests", Kind: "PackageManifest", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftCRD},

		// performance.openshift.io
		{Group: "performance.openshift.io", Version: "v2", Resource: "performanceprofiles", Kind: "PerformanceProfile", Scope: servedapis.ScopeCluster, Source: servedapis.SourceOpenShiftCRD},

		// tuned.openshift.io
		{Group: "tuned.openshift.io", Version: "v1", Resource: "profiles", Kind: "Profile", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftCRD},
		{Group: "tuned.openshift.io", Version: "v1", Resource: "tuneds", Kind: "Tuned", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftCRD},
	}
}
