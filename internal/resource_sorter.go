package internal

import (
	"fmt"
	"sigs.k8s.io/kustomize/kyaml/kio/kioutil"
	"sigs.k8s.io/kustomize/kyaml/yaml"
	"strconv"
)

const APIVersionV1 = "v1"
const APIVersionV1Beta1 = "policy/v1beta1"
const APIVersionAdmissionRegistrationV1 = "admissionregistration.k8s.io/v1"
const APIVersionAPIExtensionsV1 = "apiextensions.k8s.io/v1"
const APIVersionAppsV1 = "apps/v1"
const APIVersionAutoscalingV1 = "autoscaling/v1"
const APIVersionAutoscalingV2 = "autoscaling/v2"
const APIVersionAutoscalingV2Beta2 = "autoscaling/v2beta2"
const APIVersionBatchV1 = "batch/v1"
const APIVersionNetworkingV1 = "networking.k8s.io/v1"
const APIVersionPolicyV1 = "policy/v1"
const APIVersionRBACV1 = "rbac.authorization.k8s.io/v1"
const APIVersionSchedulingV1 = "scheduling.k8s.io/v1"
const APIVersionStorageV1 = "storage.k8s.io/v1"

const KindClusterRole = "ClusterRole"
const KindClusterRoleBinding = "ClusterRoleBinding"
const KindConfigMap = "ConfigMap"
const KindControllerRevision = "ControllerRevision"
const KindCronJob = "CronJob"
const KindCSIDriver = "CSIDriver"
const KindCustomResourceDefinition = "CustomResourceDefinition"
const KindDaemonSet = "DaemonSet"
const KindDeployment = "Deployment"
const KindHorizontalPodAutoscaler = "HorizontalPodAutoscaler"
const KindIngress = "Ingress"
const KindJob = "Job"
const KindLimitRange = "LimitRange"
const KindMutatingWebhookConfiguration = "MutatingWebhookConfiguration"
const KindNamespace = "Namespace"
const KindNetworkPolicy = "NetworkPolicy"
const KindNode = "Node"
const KindPersistentVolume = "PersistentVolume"
const KindPersistentVolumeClaim = "PersistentVolumeClaim"
const KindPod = "Pod"
const KindPodDisruptionBudget = "PodDisruptionBudget"
const KindPodSecurityPolicy = "PodSecurityPolicy"
const KindPodTemplate = "PodTemplate"
const KindPriorityClass = "PriorityClass"
const KindReplicaSet = "ReplicaSet"
const KindReplicationController = "ReplicationController"
const KindResourceQuota = "ResourceQuota"
const KindRole = "Role"
const KindRoleBinding = "RoleBinding"
const KindSecret = "Secret"
const KindService = "Service"
const KindServiceAccount = "ServiceAccount"
const KindStatefulSet = "StatefulSet"
const KindStorageClass = "StorageClass"
const KindValidatingWebhookConfiguration = "ValidatingWebhookConfiguration"

type ByType []*yaml.RNode

func (a ByType) Len() int {
	return len(a)
}

func (a ByType) Swap(i, j int) {
	a[i], a[j] = a[j], a[i]
}

func (a ByType) Less(i, j int) bool {
	this := a[i]
	that := a[j]
	thisScore := a.getScoreForKind(this)
	thatScore := a.getScoreForKind(that)
	return thisScore < thatScore
}

func (a ByType) getScoreForKind(r *yaml.RNode) int {
	apiVersion := r.GetApiVersion()
	kind := r.GetKind()
	switch apiVersion + "/" + kind {
	case APIVersionV1 + "/" + KindNode:
		return -99
	case APIVersionAdmissionRegistrationV1 + "/" + KindMutatingWebhookConfiguration:
		return -96
	case APIVersionAdmissionRegistrationV1 + "/" + KindValidatingWebhookConfiguration:
		return -95
	case APIVersionAPIExtensionsV1 + "/" + KindCustomResourceDefinition:
		return -94
	case APIVersionV1 + "/" + KindNamespace:
		return -92
	case APIVersionV1 + "/" + KindServiceAccount:
		return -91
	case APIVersionRBACV1 + "/" + KindClusterRole:
		return -90
	case APIVersionRBACV1 + "/" + KindRole:
		return -89
	case APIVersionRBACV1 + "/" + KindClusterRoleBinding:
		return -88
	case APIVersionRBACV1 + "/" + KindRoleBinding:
		return -87
	default:
		indexAnnValue, ok := r.GetAnnotations()[kioutil.IndexAnnotation]
		if !ok {
			panic(fmt.Errorf("no index annotation for '%s/%s' of type '%s/%s'", r.GetNamespace(), r.GetName(), apiVersion, kind))
		}
		index, err := strconv.Atoi(indexAnnValue)
		if err != nil {
			panic(fmt.Errorf("invalid index annotation for '%s/%s' of type '%s/%s': %w", r.GetNamespace(), r.GetName(), apiVersion, kind, err))
		}
		return index
	}
}
