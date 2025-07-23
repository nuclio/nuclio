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

package k8s

import (
	"context"
	"io"
	"time"

	"github.com/nuclio/errors"
	appsv1 "k8s.io/api/apps/v1"
	autosv2 "k8s.io/api/autoscaling/v2"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// ClientWithRetry provides resilient methods to interact with various Kubernetes resources
type ClientWithRetry interface {

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
	PatchService(ctx context.Context, namespace string, name string, pt types.PatchType, data []byte) (*corev1.Service, error)

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

	// --- Pods ---

	// ListPods lists Pods in a namespace.
	ListPods(ctx context.Context, namespace string, listOptions metav1.ListOptions) (*corev1.PodList, error)

	// GetPod retrieves a specific Pod.
	GetPod(ctx context.Context, namespace string, name string) (*corev1.Pod, error)

	// StreamPodLogs returns a stream of logs for a Pod.
	StreamPodLogs(ctx context.Context, namespace, podName string, options *corev1.PodLogOptions) (io.ReadCloser, error)

	//DeletePod deletes a Pod by name.
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

type clientWithRetry struct {
	kubernetes.Interface
	retries int
	delay   time.Duration
}

func NewClientWithRetryFromConfig(config *rest.Config) (ClientWithRetry, error) {
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create Kubernetes client")
	}
	return &clientWithRetry{
		Interface: client,
		retries:   maxRetries,
		delay:     delay,
	}, nil
}

func NewClientWithRetryFromClient(client kubernetes.Interface) ClientWithRetry {
	return &clientWithRetry{
		Interface: client,
		retries:   maxRetries,
		delay:     delay,
	}
}

func (r *clientWithRetry) ListSecrets(ctx context.Context, namespace string, listOptions metav1.ListOptions) (*corev1.SecretList, error) {
	return requestWithRetry[*corev1.SecretList](func() (*corev1.SecretList, error) {
		return r.CoreV1().Secrets(namespace).List(ctx, listOptions)
	}, r.retries, r.delay)
}

func (r *clientWithRetry) GetSecret(ctx context.Context, namespace, name string) (*corev1.Secret, error) {
	return requestWithRetry[*corev1.Secret](func() (*corev1.Secret, error) {
		return r.CoreV1().Secrets(namespace).Get(ctx,
			name,
			metav1.GetOptions{})
	}, r.retries, r.delay)
}

func (r *clientWithRetry) CreateSecret(ctx context.Context, namespace string, secret *corev1.Secret) (*corev1.Secret, error) {
	return requestWithRetry[*corev1.Secret](func() (*corev1.Secret, error) {
		return r.CoreV1().Secrets(namespace).Create(ctx,
			secret,
			metav1.CreateOptions{})
	}, r.retries, r.delay)
}

func (r *clientWithRetry) UpdateSecret(ctx context.Context, namespace string, secret *corev1.Secret) (*corev1.Secret, error) {
	return requestWithRetry[*corev1.Secret](func() (*corev1.Secret, error) {
		return r.CoreV1().Secrets(namespace).Update(ctx,
			secret,
			metav1.UpdateOptions{})
	}, r.retries, r.delay)
}

func (r *clientWithRetry) DeleteSecret(ctx context.Context, namespace, name string) (err error) {
	_, err = requestWithRetry(func() (any, error) {
		return nil, r.CoreV1().Secrets(namespace).Delete(ctx,
			name,
			metav1.DeleteOptions{})
	}, r.retries, r.delay)
	return
}

func (r *clientWithRetry) DeleteCollectionSecrets(ctx context.Context, namespace string, deleteOptions metav1.DeleteOptions, listOptions metav1.ListOptions) (err error) {
	_, err = requestWithRetry(func() (any, error) {
		return nil, r.CoreV1().
			Secrets(namespace).
			DeleteCollection(ctx, deleteOptions, listOptions)
	}, r.retries, r.delay)
	return
}

func (r *clientWithRetry) GetConfigMap(ctx context.Context, namespace string, name string) (*corev1.ConfigMap, error) {
	return requestWithRetry[*corev1.ConfigMap](func() (*corev1.ConfigMap, error) {
		return r.CoreV1().
			ConfigMaps(namespace).
			Get(ctx, name, metav1.GetOptions{})
	}, r.retries, r.delay)
}

func (r *clientWithRetry) CreateConfigMap(ctx context.Context, namespace string, configMap *corev1.ConfigMap) (*corev1.ConfigMap, error) {
	return requestWithRetry[*corev1.ConfigMap](func() (*corev1.ConfigMap, error) {
		return r.CoreV1().
			ConfigMaps(namespace).
			Create(ctx, configMap, metav1.CreateOptions{})
	}, r.retries, r.delay)
}

func (r *clientWithRetry) UpdateConfigMap(ctx context.Context, namespace string, configMap *corev1.ConfigMap) (*corev1.ConfigMap, error) {
	return requestWithRetry[*corev1.ConfigMap](func() (*corev1.ConfigMap, error) {
		return r.CoreV1().
			ConfigMaps(namespace).
			Update(ctx, configMap, metav1.UpdateOptions{})
	}, r.retries, r.delay)
}

func (r *clientWithRetry) DeleteConfigMap(ctx context.Context, namespace string, name string, deleteOptions metav1.DeleteOptions) (err error) {
	_, err = requestWithRetry(func() (any, error) {
		return nil, r.CoreV1().
			ConfigMaps(namespace).
			Delete(ctx, name, deleteOptions)
	}, r.retries, r.delay)
	return
}

func (r *clientWithRetry) ListServices(ctx context.Context, namespace string, options metav1.ListOptions) (*corev1.ServiceList, error) {
	return requestWithRetry[*corev1.ServiceList](func() (*corev1.ServiceList, error) {
		return r.CoreV1().
			Services(namespace).
			List(ctx, options)
	}, r.retries, r.delay)
}

func (r *clientWithRetry) GetService(ctx context.Context, namespace string, name string) (*corev1.Service, error) {
	return requestWithRetry[*corev1.Service](func() (*corev1.Service, error) {
		return r.CoreV1().
			Services(namespace).
			Get(ctx, name, metav1.GetOptions{})
	}, r.retries, r.delay)
}

func (r *clientWithRetry) CreateService(ctx context.Context, namespace string, service *corev1.Service) (*corev1.Service, error) {
	return requestWithRetry[*corev1.Service](func() (*corev1.Service, error) {
		return r.CoreV1().
			Services(namespace).
			Create(ctx, service, metav1.CreateOptions{})
	}, r.retries, r.delay)
}

func (r *clientWithRetry) UpdateService(ctx context.Context, namespace string, service *corev1.Service) (*corev1.Service, error) {
	return requestWithRetry[*corev1.Service](func() (*corev1.Service, error) {
		return r.CoreV1().
			Services(namespace).
			Update(ctx, service, metav1.UpdateOptions{})
	}, r.retries, r.delay)
}

func (r *clientWithRetry) DeleteService(ctx context.Context, namespace string, name string, deleteOptions metav1.DeleteOptions) (err error) {
	_, err = requestWithRetry(func() (any, error) {
		return nil, r.CoreV1().
			Services(namespace).
			Delete(ctx, name, deleteOptions)
	}, r.retries, r.delay)
	return
}

func (r *clientWithRetry) PatchService(ctx context.Context, namespace string, name string, pt types.PatchType, data []byte) (*corev1.Service, error) {
	return requestWithRetry[*corev1.Service](func() (*corev1.Service, error) {
		return r.CoreV1().
			Services(namespace).
			Patch(ctx, name, pt, data, metav1.PatchOptions{})
	}, r.retries, r.delay)
}

func (r *clientWithRetry) ListDeployments(ctx context.Context, namespace string, listOptions metav1.ListOptions) (*appsv1.DeploymentList, error) {
	return requestWithRetry[*appsv1.DeploymentList](func() (*appsv1.DeploymentList, error) {
		return r.AppsV1().Deployments(namespace).List(ctx, listOptions)
	}, r.retries, r.delay)
}

func (r *clientWithRetry) GetDeployment(ctx context.Context, namespace string, name string) (*appsv1.Deployment, error) {
	return requestWithRetry[*appsv1.Deployment](func() (*appsv1.Deployment, error) {
		return r.AppsV1().
			Deployments(namespace).
			Get(ctx, name, metav1.GetOptions{})
	}, r.retries, r.delay)
}

func (r *clientWithRetry) CreateDeployment(ctx context.Context, namespace string, deployment *appsv1.Deployment) (*appsv1.Deployment, error) {
	return requestWithRetry[*appsv1.Deployment](func() (*appsv1.Deployment, error) {
		return r.AppsV1().
			Deployments(namespace).
			Create(ctx, deployment, metav1.CreateOptions{})
	}, r.retries, r.delay)
}

func (r *clientWithRetry) UpdateDeployment(ctx context.Context, namespace string, deployment *appsv1.Deployment) (*appsv1.Deployment, error) {
	return requestWithRetry[*appsv1.Deployment](func() (*appsv1.Deployment, error) {
		return r.AppsV1().
			Deployments(namespace).
			Update(ctx, deployment, metav1.UpdateOptions{})
	}, r.retries, r.delay)
}

func (r *clientWithRetry) DeleteDeployment(ctx context.Context, namespace string, name string, deleteOptions metav1.DeleteOptions) (err error) {
	_, err = requestWithRetry(func() (any, error) {
		return nil, r.AppsV1().
			Deployments(namespace).
			Delete(ctx, name, deleteOptions)
	}, r.retries, r.delay)
	return
}

func (r *clientWithRetry) ListPods(ctx context.Context, namespace string, listOptions metav1.ListOptions) (*corev1.PodList, error) {
	return requestWithRetry[*corev1.PodList](func() (*corev1.PodList, error) {
		return r.CoreV1().Pods(namespace).List(ctx, listOptions)
	}, r.retries, r.delay)
}

func (r *clientWithRetry) GetPod(ctx context.Context, namespace string, name string) (*corev1.Pod, error) {
	return requestWithRetry[*corev1.Pod](func() (*corev1.Pod, error) {
		return r.CoreV1().
			Pods(namespace).
			Get(ctx, name, metav1.GetOptions{})
	}, r.retries, r.delay)
}

func (r *clientWithRetry) DeletePod(ctx context.Context, namespace string, name string, deleteOptions metav1.DeleteOptions) (err error) {
	_, err = requestWithRetry(func() (any, error) {
		return nil, r.CoreV1().
			Pods(namespace).
			Delete(ctx, name, deleteOptions)
	}, r.retries, r.delay)
	return
}

func (r *clientWithRetry) DeleteCollectionPods(ctx context.Context, namespace string, deleteOptions metav1.DeleteOptions, listOptions metav1.ListOptions) (err error) {
	_, err = requestWithRetry(func() (any, error) {
		return nil, r.CoreV1().
			Pods(namespace).
			DeleteCollection(ctx, deleteOptions, listOptions)
	}, r.retries, r.delay)
	return
}

func (r *clientWithRetry) StreamPodLogs(ctx context.Context, namespace, podName string, options *corev1.PodLogOptions) (io.ReadCloser, error) {
	return requestWithRetry[io.ReadCloser](func() (io.ReadCloser, error) {
		return r.CoreV1().
			Pods(namespace).
			GetLogs(podName, options).
			Stream(ctx)
	}, r.retries, r.delay)
}

func (r *clientWithRetry) GetJob(ctx context.Context, namespace string, name string) (*batchv1.Job, error) {
	return requestWithRetry[*batchv1.Job](func() (*batchv1.Job, error) {
		return r.BatchV1().
			Jobs(namespace).
			Get(ctx, name, metav1.GetOptions{})
	}, r.retries, r.delay)
}

func (r *clientWithRetry) CreateJob(ctx context.Context, namespace string, job *batchv1.Job) (*batchv1.Job, error) {
	return requestWithRetry[*batchv1.Job](func() (*batchv1.Job, error) {
		return r.BatchV1().
			Jobs(namespace).
			Create(ctx, job, metav1.CreateOptions{})
	}, r.retries, r.delay)
}

func (r *clientWithRetry) DeleteJob(ctx context.Context, namespace string, name string, deleteOptions metav1.DeleteOptions) (err error) {
	_, err = requestWithRetry(func() (any, error) {
		return nil, r.BatchV1().
			Jobs(namespace).
			Delete(ctx, name, deleteOptions)
	}, r.retries, r.delay)
	return
}

func (r *clientWithRetry) ListJobs(ctx context.Context, namespace string, listOptions metav1.ListOptions) (*batchv1.JobList, error) {
	return requestWithRetry[*batchv1.JobList](func() (*batchv1.JobList, error) {
		return r.BatchV1().Jobs(namespace).List(ctx, listOptions)
	}, r.retries, r.delay)
}

func (r *clientWithRetry) ListCronJobs(ctx context.Context, namespace string, listOptions metav1.ListOptions) (*batchv1.CronJobList, error) {
	return requestWithRetry[*batchv1.CronJobList](func() (*batchv1.CronJobList, error) {
		return r.BatchV1().CronJobs(namespace).List(ctx, listOptions)
	}, r.retries, r.delay)
}

func (r *clientWithRetry) CreateCronJob(ctx context.Context, namespace string, job *batchv1.CronJob) (*batchv1.CronJob, error) {
	return requestWithRetry[*batchv1.CronJob](func() (*batchv1.CronJob, error) {
		return r.BatchV1().
			CronJobs(namespace).
			Create(ctx, job, metav1.CreateOptions{})
	}, r.retries, r.delay)
}

func (r *clientWithRetry) UpdateCronJob(ctx context.Context, namespace string, job *batchv1.CronJob) (*batchv1.CronJob, error) {
	return requestWithRetry[*batchv1.CronJob](func() (*batchv1.CronJob, error) {
		return r.BatchV1().
			CronJobs(namespace).
			Update(ctx, job, metav1.UpdateOptions{})
	}, r.retries, r.delay)
}

func (r *clientWithRetry) DeleteCronJob(ctx context.Context, namespace string, name string, deleteOptions metav1.DeleteOptions) (err error) {
	_, err = requestWithRetry(func() (any, error) {
		return nil, r.BatchV1().
			CronJobs(namespace).
			Delete(ctx, name, deleteOptions)
	}, r.retries, r.delay)
	return
}

func (r *clientWithRetry) DeleteCollectionCronJobs(ctx context.Context, namespace string, deleteOptions metav1.DeleteOptions, listOptions metav1.ListOptions) (err error) {
	_, err = requestWithRetry(func() (any, error) {
		return nil, r.BatchV1().
			CronJobs(namespace).
			DeleteCollection(ctx, deleteOptions, listOptions)
	}, r.retries, r.delay)
	return
}

func (r *clientWithRetry) ListIngresses(ctx context.Context, namespace string, listOptions metav1.ListOptions) (*networkingv1.IngressList, error) {
	return requestWithRetry[*networkingv1.IngressList](func() (*networkingv1.IngressList, error) {
		return r.NetworkingV1().
			Ingresses(namespace).
			List(ctx, listOptions)
	}, r.retries, r.delay)
}

func (r *clientWithRetry) GetIngress(ctx context.Context, namespace, name string) (*networkingv1.Ingress, error) {
	return requestWithRetry[*networkingv1.Ingress](func() (*networkingv1.Ingress, error) {
		return r.NetworkingV1().
			Ingresses(namespace).
			Get(ctx, name, metav1.GetOptions{})
	}, r.retries, r.delay)
}

func (r *clientWithRetry) CreateIngress(ctx context.Context, namespace string, ingress *networkingv1.Ingress) (*networkingv1.Ingress, error) {
	return requestWithRetry[*networkingv1.Ingress](func() (*networkingv1.Ingress, error) {
		return r.NetworkingV1().
			Ingresses(namespace).
			Create(ctx, ingress, metav1.CreateOptions{})
	}, r.retries, r.delay)
}

func (r *clientWithRetry) UpdateIngress(ctx context.Context, namespace string, ingress *networkingv1.Ingress) (*networkingv1.Ingress, error) {
	return requestWithRetry[*networkingv1.Ingress](func() (*networkingv1.Ingress, error) {
		return r.NetworkingV1().
			Ingresses(namespace).
			Update(ctx, ingress, metav1.UpdateOptions{})
	}, r.retries, r.delay)
}

func (r *clientWithRetry) DeleteIngress(ctx context.Context, namespace, name string, options metav1.DeleteOptions) (err error) {
	_, err = requestWithRetry(func() (any, error) {
		return nil, r.NetworkingV1().
			Ingresses(namespace).
			Delete(ctx, name, options)
	}, r.retries, r.delay)
	return
}

func (r *clientWithRetry) GetHorizontalPodAutoscaler(ctx context.Context, namespace string, name string) (*autosv2.HorizontalPodAutoscaler, error) {
	return requestWithRetry[*autosv2.HorizontalPodAutoscaler](func() (*autosv2.HorizontalPodAutoscaler, error) {
		return r.AutoscalingV2().
			HorizontalPodAutoscalers(namespace).
			Get(ctx, name, metav1.GetOptions{})
	}, r.retries, r.delay)
}

func (r *clientWithRetry) CreateHorizontalPodAutoscaler(ctx context.Context, namespace string, hpa *autosv2.HorizontalPodAutoscaler) (*autosv2.HorizontalPodAutoscaler, error) {
	return requestWithRetry[*autosv2.HorizontalPodAutoscaler](func() (*autosv2.HorizontalPodAutoscaler, error) {
		return r.AutoscalingV2().
			HorizontalPodAutoscalers(namespace).
			Create(ctx, hpa, metav1.CreateOptions{})
	}, r.retries, r.delay)
}

func (r *clientWithRetry) UpdateHorizontalPodAutoscaler(ctx context.Context, namespace string, hpa *autosv2.HorizontalPodAutoscaler) (*autosv2.HorizontalPodAutoscaler, error) {
	return requestWithRetry[*autosv2.HorizontalPodAutoscaler](func() (*autosv2.HorizontalPodAutoscaler, error) {
		return r.AutoscalingV2().
			HorizontalPodAutoscalers(namespace).
			Update(ctx, hpa, metav1.UpdateOptions{})
	}, r.retries, r.delay)
}

func (r *clientWithRetry) DeleteHorizontalPodAutoscaler(ctx context.Context, namespace string, name string, deleteOptions metav1.DeleteOptions) (err error) {
	_, err = requestWithRetry(func() (any, error) {
		return nil, r.AutoscalingV2().
			HorizontalPodAutoscalers(namespace).
			Delete(ctx, name, deleteOptions)
	}, r.retries, r.delay)
	return
}

func (r *clientWithRetry) ListEvents(ctx context.Context, namespace string, listOptions metav1.ListOptions) (*corev1.EventList, error) {
	return requestWithRetry[*corev1.EventList](func() (*corev1.EventList, error) {
		return r.CoreV1().Events(namespace).List(ctx, listOptions)
	}, r.retries, r.delay)
}

func (r *clientWithRetry) ListNamespaces(ctx context.Context, listOptions metav1.ListOptions) (*corev1.NamespaceList, error) {
	return requestWithRetry[*corev1.NamespaceList](func() (*corev1.NamespaceList, error) {
		return r.CoreV1().Namespaces().List(ctx, listOptions)
	}, r.retries, r.delay)
}
