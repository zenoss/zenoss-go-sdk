package tag

import (
	"os"
)

const (
	// K8sClusterEnv is an environment variable that may contain our Kubernetes cluster.
	K8sClusterEnv = "K8S_CLUSTER"

	// K8sClusterTagKey is the tag where Zenoss expects a Kubernetes cluster.
	K8sClusterTagKey = "k8s.cluster"

	// K8sNamespaceEnv is an environment variable that may contain our Kubernetes namespace.
	K8sNamespaceEnv = "K8S_NAMESPACE"

	// K8sNamespaceTagKey is the tag where Zenoss expects a Kubernetes namespace.
	K8sNamespaceTagKey = "k8s.namespace"

	// K8sPodEnv is an environment variable that may contain our Kubernetes pod.
	K8sPodEnv = "K8S_POD"

	// K8sPodTagKey is the tag where Zenoss expects a Kubernetes pod.
	K8sPodTagKey = "k8s.pod"
)

// Tags is a string-to-string map used for tags, dimensions, and metadata.
type Tags map[string]string

// DiscoverAllTags returns a tags map containing all of the tags we could discover.
func DiscoverAllTags() Tags {
	return DiscoverKubernetesTags()
}

// DiscoverKubernetesTags returns a tags map containing all Kubernetes tags we could discover.
func DiscoverKubernetesTags() Tags {
	tags := make(Tags)

	if k8sCluster := os.Getenv(K8sClusterEnv); k8sCluster != "" {
		tags[K8sClusterTagKey] = k8sCluster
	}

	if k8sNamespace := os.Getenv(K8sNamespaceEnv); k8sNamespace != "" {
		tags[K8sNamespaceTagKey] = k8sNamespace
	}

	if k8sPod := os.Getenv(K8sPodEnv); k8sPod != "" {
		tags[K8sPodTagKey] = k8sPod
	}

	return tags
}
