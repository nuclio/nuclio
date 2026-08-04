/*
Copyright 2025 The Nuclio Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package kube

import (
	"context"
	"io"

	appsv1 "k8s.io/api/apps/v1"
	autosv2 "k8s.io/api/autoscaling/v2"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// Client provides resilient methods to interact with various Kubernetes resources
type Client interface {

	// --- Secrets ---

	// ListSecrets retrieves the list of Secrets in a given namespace.
	ListSecrets(ctx context.Context, namespace string, listOptions metav1.ListOptions) (*corev1.SecretList, error)

	// GetSecret fetches a specific Secret by name from a given namespace.
	GetSecret(ctx context.Context, namespace, name string) (*corev1.Secret, error)

	// CreateSecret creates a new Secret in the specified namespace.
	CreateSecret(ctx context.Context, namespace string, secret *corev1.Secret) (*corev1.Secret, error)

	// UpdateSecret updates an existing Secret in the specified namespace.
	UpdateSecret(ctx context.Context, namespace string, secret *corev1.Secret) (*corev1.Secret, error)

	// DeleteSecret removes a Secret from the specified namespace.
	DeleteSecret(ctx context.Context, namespace, name string) error

	// DeleteCollectionSecrets deletes a collection of Secrets in the specified namespace.
	DeleteCollectionSecrets(ctx context.Context, namespace string, deleteOptions metav1.DeleteOptions, listOptions metav1.ListOptions) error

	// --- ConfigMaps ---

	// GetConfigMap retrieves a ConfigMap by name from a namespace.
	GetConfigMap(ctx context.Context, namespace string, name string) (*corev1.ConfigMap, error)

	// CreateConfigMap creates a new ConfigMap in the specified namespace.
	CreateConfigMap(ctx context.Context, namespace string, configMap *corev1.ConfigMap) (*corev1.ConfigMap, error)

	// UpdateConfigMap updates an existing ConfigMap.
	UpdateConfigMap(ctx context.Context, namespace string, configMap *corev1.ConfigMap) (*corev1.ConfigMap, error)

	// DeleteConfigMap deletes a ConfigMap by name.
	DeleteConfigMap(ctx context.Context, namespace string, name string, deleteOptions metav1.DeleteOptions) error

	// ApplyConfigMap creates or updates a ConfigMap in a single server-side-apply call by name.
	ApplyConfigMap(ctx context.Context, namespace string, configMap *corev1.ConfigMap) (*corev1.ConfigMap, error)

	// --- Services ---

	// GetService fetches a Service by name in the given namespace.
	GetService(ctx context.Context, namespace string, name string) (*corev1.Service, error)

	// ListServices retrieves the list of Services in a given namespace.
	ListServices(ctx context.Context, namespace string, options metav1.ListOptions) (*corev1.ServiceList, error)

	// CreateService creates a new Service.
	CreateService(ctx context.Context, namespace string, service *corev1.Service) (*corev1.Service, error)

	// UpdateService updates an existing Service.
	UpdateService(ctx context.Context, namespace string, service *corev1.Service) (*corev1.Service, error)

	// DeleteService deletes a Service.
	DeleteService(ctx context.Context, namespace string, name string, deleteOptions metav1.DeleteOptions) error

	// PatchService applies a patch to a Service.
	PatchService(ctx context.Context, namespace string, name string, patchType types.PatchType, data []byte) (*corev1.Service, error)

	// --- Deployments ---

	// ListDeployments lists Deployments in a namespace.
	ListDeployments(ctx context.Context, namespace string, listOptions metav1.ListOptions) (*appsv1.DeploymentList, error)

	// GetDeployment retrieves a Deployment by name.
	GetDeployment(ctx context.Context, namespace string, name string) (*appsv1.Deployment, error)

	// CreateDeployment creates a new Deployment.
	CreateDeployment(ctx context.Context, namespace string, deployment *appsv1.Deployment) (*appsv1.Deployment, error)

	// UpdateDeployment updates an existing Deployment.
	UpdateDeployment(ctx context.Context, namespace string, deployment *appsv1.Deployment) (*appsv1.Deployment, error)

	// DeleteDeployment deletes a Deployment.
	DeleteDeployment(ctx context.Context, namespace string, name string, deleteOptions metav1.DeleteOptions) error

	// --- ReplicaSets ---

	// DeleteCollectionReplicaSets deletes a collection of ReplicaSets in a namespace.
	DeleteCollectionReplicaSets(ctx context.Context, namespace string, deleteOptions metav1.DeleteOptions, listOptions metav1.ListOptions) error

	// --- Pods ---

	// ListPods lists Pods in a namespace.
	ListPods(ctx context.Context, namespace string, listOptions metav1.ListOptions) (*corev1.PodList, error)

	// GetPod retrieves a specific Pod.
	GetPod(ctx context.Context, namespace string, name string) (*corev1.Pod, error)

	// StreamPodLogs returns a stream of logs for a Pod.
	StreamPodLogs(ctx context.Context, namespace, podName string, options *corev1.PodLogOptions) (io.ReadCloser, error)

	// DeletePod deletes a Pod by name.
	DeletePod(ctx context.Context, namespace string, name string, deleteOptions metav1.DeleteOptions) (err error)

	// DeleteCollectionPods deletes a collection of Pods in a namespace.
	DeleteCollectionPods(ctx context.Context, namespace string, deleteOptions metav1.DeleteOptions, listOptions metav1.ListOptions) (err error)

	// --- Jobs ---

	// GetJob retrieves a Job by name.
	GetJob(ctx context.Context, namespace string, name string) (*batchv1.Job, error)

	// CreateJob creates a new Job.
	CreateJob(ctx context.Context, namespace string, job *batchv1.Job) (*batchv1.Job, error)

	// DeleteJob deletes a Job.
	DeleteJob(ctx context.Context, namespace string, name string, deleteOptions metav1.DeleteOptions) error

	// ListJobs lists Jobs in a namespace.
	ListJobs(ctx context.Context, namespace string, listOptions metav1.ListOptions) (*batchv1.JobList, error)

	// --- CronJobs ---

	// ListCronJobs lists CronJobs in a namespace.
	ListCronJobs(ctx context.Context, namespace string, listOptions metav1.ListOptions) (*batchv1.CronJobList, error)

	// CreateCronJob creates a new CronJob.
	CreateCronJob(ctx context.Context, namespace string, job *batchv1.CronJob) (*batchv1.CronJob, error)

	// UpdateCronJob updates an existing CronJob.
	UpdateCronJob(ctx context.Context, namespace string, job *batchv1.CronJob) (*batchv1.CronJob, error)

	// DeleteCronJob deletes a CronJob.
	DeleteCronJob(ctx context.Context, namespace string, name string, deleteOptions metav1.DeleteOptions) error

	// DeleteCollectionCronJobs deletes a collection of CronJobs.
	DeleteCollectionCronJobs(ctx context.Context, namespace string, deleteOptions metav1.DeleteOptions, listOptions metav1.ListOptions) error

	// --- Ingresses ---

	// ListIngresses lists Ingresses in a namespace.
	ListIngresses(ctx context.Context, namespace string, listOptions metav1.ListOptions) (*networkingv1.IngressList, error)

	// GetIngress retrieves an Ingress.
	GetIngress(ctx context.Context, namespace, name string) (*networkingv1.Ingress, error)

	// CreateIngress creates a new Ingress.
	CreateIngress(ctx context.Context, namespace string, ingress *networkingv1.Ingress) (*networkingv1.Ingress, error)

	// UpdateIngress updates an Ingress.
	UpdateIngress(ctx context.Context, namespace string, ingress *networkingv1.Ingress) (*networkingv1.Ingress, error)

	// DeleteIngress deletes an Ingress.
	DeleteIngress(ctx context.Context, namespace, name string, deleteOptions metav1.DeleteOptions) error

	// --- HorizontalPodAutoscalers (HPA) ---

	// GetHorizontalPodAutoscaler retrieves an HPA by name.
	GetHorizontalPodAutoscaler(ctx context.Context, namespace string, name string) (*autosv2.HorizontalPodAutoscaler, error)

	// CreateHorizontalPodAutoscaler creates a new HPA.
	CreateHorizontalPodAutoscaler(ctx context.Context, namespace string, hpa *autosv2.HorizontalPodAutoscaler) (*autosv2.HorizontalPodAutoscaler, error)

	// UpdateHorizontalPodAutoscaler updates an HPA.
	UpdateHorizontalPodAutoscaler(ctx context.Context, namespace string, hpa *autosv2.HorizontalPodAutoscaler) (*autosv2.HorizontalPodAutoscaler, error)

	// DeleteHorizontalPodAutoscaler deletes an HPA.
	DeleteHorizontalPodAutoscaler(ctx context.Context, namespace string, name string, deleteOptions metav1.DeleteOptions) error

	// --- Events ---

	// ListEvents lists Events in a namespace.
	ListEvents(ctx context.Context, namespace string, listOptions metav1.ListOptions) (*corev1.EventList, error)

	// --- Namespaces ---

	// ListNamespaces lists all Namespaces in the cluster.
	ListNamespaces(ctx context.Context, listOptions metav1.ListOptions) (*corev1.NamespaceList, error)
}
