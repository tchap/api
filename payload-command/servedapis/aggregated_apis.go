package servedapis

import "github.com/openshift/api/servedapis"

// aggregatedAPIServerEntries returns the hardcoded list of resources served by the
// openshift-apiserver and oauth-apiserver aggregated API servers.
// These entries apply to both cluster profiles.
func aggregatedAPIServerEntries() []servedapis.ServedAPIEntry {
	entries := []servedapis.ServedAPIEntry{}
	entries = append(entries, openshiftAPIServerEntries()...)
	entries = append(entries, oauthAPIServerEntries()...)
	return entries
}

func openshiftAPIServerEntries() []servedapis.ServedAPIEntry {
	return []servedapis.ServedAPIEntry{
		// apps.openshift.io/v1
		{Group: "apps.openshift.io", Version: "v1", Resource: "deploymentconfigs", Kind: "DeploymentConfig", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftAPIServer},

		// authorization.openshift.io/v1
		{Group: "authorization.openshift.io", Version: "v1", Resource: "clusterrolebindings", Kind: "ClusterRoleBinding", Scope: servedapis.ScopeCluster, Source: servedapis.SourceOpenShiftAPIServer},
		{Group: "authorization.openshift.io", Version: "v1", Resource: "clusterroles", Kind: "ClusterRole", Scope: servedapis.ScopeCluster, Source: servedapis.SourceOpenShiftAPIServer},
		{Group: "authorization.openshift.io", Version: "v1", Resource: "localresourceaccessreviews", Kind: "LocalResourceAccessReview", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftAPIServer},
		{Group: "authorization.openshift.io", Version: "v1", Resource: "localsubjectaccessreviews", Kind: "LocalSubjectAccessReview", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftAPIServer},
		{Group: "authorization.openshift.io", Version: "v1", Resource: "resourceaccessreviews", Kind: "ResourceAccessReview", Scope: servedapis.ScopeCluster, Source: servedapis.SourceOpenShiftAPIServer},
		{Group: "authorization.openshift.io", Version: "v1", Resource: "rolebindingrestrictions", Kind: "RoleBindingRestriction", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftAPIServer},
		{Group: "authorization.openshift.io", Version: "v1", Resource: "rolebindings", Kind: "RoleBinding", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftAPIServer},
		{Group: "authorization.openshift.io", Version: "v1", Resource: "roles", Kind: "Role", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftAPIServer},
		{Group: "authorization.openshift.io", Version: "v1", Resource: "selfsubjectrulesreviews", Kind: "SelfSubjectRulesReview", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftAPIServer},
		{Group: "authorization.openshift.io", Version: "v1", Resource: "subjectaccessreviews", Kind: "SubjectAccessReview", Scope: servedapis.ScopeCluster, Source: servedapis.SourceOpenShiftAPIServer},
		{Group: "authorization.openshift.io", Version: "v1", Resource: "subjectrulesreviews", Kind: "SubjectRulesReview", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftAPIServer},

		// build.openshift.io/v1
		{Group: "build.openshift.io", Version: "v1", Resource: "buildconfigs", Kind: "BuildConfig", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftAPIServer},
		{Group: "build.openshift.io", Version: "v1", Resource: "builds", Kind: "Build", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftAPIServer},

		// image.openshift.io/v1
		{Group: "image.openshift.io", Version: "v1", Resource: "imagestreamimages", Kind: "ImageStreamImage", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftAPIServer},
		{Group: "image.openshift.io", Version: "v1", Resource: "imagestreamimports", Kind: "ImageStreamImport", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftAPIServer},
		{Group: "image.openshift.io", Version: "v1", Resource: "imagestreammappings", Kind: "ImageStreamMapping", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftAPIServer},
		{Group: "image.openshift.io", Version: "v1", Resource: "imagestreams", Kind: "ImageStream", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftAPIServer},
		{Group: "image.openshift.io", Version: "v1", Resource: "imagestreamtags", Kind: "ImageStreamTag", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftAPIServer},
		{Group: "image.openshift.io", Version: "v1", Resource: "imagetags", Kind: "ImageTag", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftAPIServer},
		{Group: "image.openshift.io", Version: "v1", Resource: "images", Kind: "Image", Scope: servedapis.ScopeCluster, Source: servedapis.SourceOpenShiftAPIServer},
		{Group: "image.openshift.io", Version: "v1", Resource: "imagesignatures", Kind: "ImageSignature", Scope: servedapis.ScopeCluster, Source: servedapis.SourceOpenShiftAPIServer},

		// project.openshift.io/v1
		{Group: "project.openshift.io", Version: "v1", Resource: "projectrequests", Kind: "ProjectRequest", Scope: servedapis.ScopeCluster, Source: servedapis.SourceOpenShiftAPIServer},
		{Group: "project.openshift.io", Version: "v1", Resource: "projects", Kind: "Project", Scope: servedapis.ScopeCluster, Source: servedapis.SourceOpenShiftAPIServer},

		// quota.openshift.io/v1
		{Group: "quota.openshift.io", Version: "v1", Resource: "appliedclusterresourcequotas", Kind: "AppliedClusterResourceQuota", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftAPIServer},
		{Group: "quota.openshift.io", Version: "v1", Resource: "clusterresourcequotas", Kind: "ClusterResourceQuota", Scope: servedapis.ScopeCluster, Source: servedapis.SourceOpenShiftAPIServer},

		// route.openshift.io/v1
		{Group: "route.openshift.io", Version: "v1", Resource: "routes", Kind: "Route", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftAPIServer},

		// security.openshift.io/v1
		{Group: "security.openshift.io", Version: "v1", Resource: "podsecuritypolicyselfsubjectreviews", Kind: "PodSecurityPolicySelfSubjectReview", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftAPIServer},
		{Group: "security.openshift.io", Version: "v1", Resource: "podsecuritypolicyreviews", Kind: "PodSecurityPolicyReview", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftAPIServer},
		{Group: "security.openshift.io", Version: "v1", Resource: "podsecuritypolicysubjectreviews", Kind: "PodSecurityPolicySubjectReview", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftAPIServer},
		{Group: "security.openshift.io", Version: "v1", Resource: "rangeallocations", Kind: "RangeAllocation", Scope: servedapis.ScopeCluster, Source: servedapis.SourceOpenShiftAPIServer},
		{Group: "security.openshift.io", Version: "v1", Resource: "securitycontextconstraints", Kind: "SecurityContextConstraints", Scope: servedapis.ScopeCluster, Source: servedapis.SourceOpenShiftAPIServer},

		// template.openshift.io/v1
		{Group: "template.openshift.io", Version: "v1", Resource: "brokertemplateinstances", Kind: "BrokerTemplateInstance", Scope: servedapis.ScopeCluster, Source: servedapis.SourceOpenShiftAPIServer},
		{Group: "template.openshift.io", Version: "v1", Resource: "processedtemplates", Kind: "Template", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftAPIServer},
		{Group: "template.openshift.io", Version: "v1", Resource: "templateinstances", Kind: "TemplateInstance", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftAPIServer},
		{Group: "template.openshift.io", Version: "v1", Resource: "templates", Kind: "Template", Scope: servedapis.ScopeNamespaced, Source: servedapis.SourceOpenShiftAPIServer},
	}
}

func oauthAPIServerEntries() []servedapis.ServedAPIEntry {
	return []servedapis.ServedAPIEntry{
		// oauth.openshift.io/v1
		{Group: "oauth.openshift.io", Version: "v1", Resource: "oauthaccesstokens", Kind: "OAuthAccessToken", Scope: servedapis.ScopeCluster, Source: servedapis.SourceOAuthAPIServer},
		{Group: "oauth.openshift.io", Version: "v1", Resource: "oauthauthorizetokens", Kind: "OAuthAuthorizeToken", Scope: servedapis.ScopeCluster, Source: servedapis.SourceOAuthAPIServer},
		{Group: "oauth.openshift.io", Version: "v1", Resource: "oauthclientauthorizations", Kind: "OAuthClientAuthorization", Scope: servedapis.ScopeCluster, Source: servedapis.SourceOAuthAPIServer},
		{Group: "oauth.openshift.io", Version: "v1", Resource: "oauthclients", Kind: "OAuthClient", Scope: servedapis.ScopeCluster, Source: servedapis.SourceOAuthAPIServer},
		{Group: "oauth.openshift.io", Version: "v1", Resource: "useroauthaccesstokens", Kind: "UserOAuthAccessToken", Scope: servedapis.ScopeCluster, Source: servedapis.SourceOAuthAPIServer},

		// user.openshift.io/v1
		{Group: "user.openshift.io", Version: "v1", Resource: "groups", Kind: "Group", Scope: servedapis.ScopeCluster, Source: servedapis.SourceOAuthAPIServer},
		{Group: "user.openshift.io", Version: "v1", Resource: "identities", Kind: "Identity", Scope: servedapis.ScopeCluster, Source: servedapis.SourceOAuthAPIServer},
		{Group: "user.openshift.io", Version: "v1", Resource: "useridentitymappings", Kind: "UserIdentityMapping", Scope: servedapis.ScopeCluster, Source: servedapis.SourceOAuthAPIServer},
		{Group: "user.openshift.io", Version: "v1", Resource: "users", Kind: "User", Scope: servedapis.ScopeCluster, Source: servedapis.SourceOAuthAPIServer},
	}
}
