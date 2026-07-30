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
	"time"

	"github.com/nuclio/nuclio/pkg/platform/kube/clients"

	"github.com/nuclio/errors"
	appsv1 "k8s.io/api/apps/v1"
	autosv2 "k8s.io/api/autoscaling/v2"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	corev1apply "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// fieldManager identifies nuclio as the owner of fields written via server-side apply.
const fieldManager = "nuclio"

type clientWithRetry struct {
	kubernetes.Interface
	retries int
	delay   time.Duration
}

func NewClientWithRetryFromConfig(config *rest.Config) (Client, error) {
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create Kubernetes client")
	}
	return &clientWithRetry{
		Interface: client,
		retries:   clients.MaxRetries,
		delay:     clients.Delay,
	}, nil
}

func NewClientWithRetryFromClient(client kubernetes.Interface) Client {
	return &clientWithRetry{
		Interface: client,
		retries:   clients.MaxRetries,
		delay:     clients.Delay,
	}
}

func (r *clientWithRetry) ListSecrets(ctx context.Context, namespace string, listOptions metav1.ListOptions) (*corev1.SecretList, error) {
	return clients.RequestWithRetry[*corev1.SecretList](func() (*corev1.SecretList, error) {
		return r.CoreV1().Secrets(namespace).List(ctx, listOptions)
	}, r.retries, r.delay)
}

func (r *clientWithRetry) GetSecret(ctx context.Context, namespace, name string) (*corev1.Secret, error) {
	return clients.RequestWithRetry[*corev1.Secret](func() (*corev1.Secret, error) {
		return r.CoreV1().Secrets(namespace).Get(ctx,
			name,
			metav1.GetOptions{})
	}, r.retries, r.delay)
}

func (r *clientWithRetry) CreateSecret(ctx context.Context, namespace string, secret *corev1.Secret) (*corev1.Secret, error) {
	return clients.RequestWithRetry[*corev1.Secret](func() (*corev1.Secret, error) {
		return r.CoreV1().Secrets(namespace).Create(ctx,
			secret,
			metav1.CreateOptions{})
	}, r.retries, r.delay)
}

func (r *clientWithRetry) UpdateSecret(ctx context.Context, namespace string, secret *corev1.Secret) (*corev1.Secret, error) {
	return clients.RequestWithRetry[*corev1.Secret](func() (*corev1.Secret, error) {
		return r.CoreV1().Secrets(namespace).Update(ctx,
			secret,
			metav1.UpdateOptions{})
	}, r.retries, r.delay)
}

func (r *clientWithRetry) DeleteSecret(ctx context.Context, namespace, name string) (err error) {
	_, err = clients.RequestWithRetry(func() (any, error) {
		return nil, r.CoreV1().Secrets(namespace).Delete(ctx,
			name,
			metav1.DeleteOptions{})
	}, r.retries, r.delay)
	return
}

func (r *clientWithRetry) DeleteCollectionSecrets(ctx context.Context, namespace string, deleteOptions metav1.DeleteOptions, listOptions metav1.ListOptions) (err error) {
	_, err = clients.RequestWithRetry(func() (any, error) {
		return nil, r.CoreV1().
			Secrets(namespace).
			DeleteCollection(ctx, deleteOptions, listOptions)
	}, r.retries, r.delay)
	return
}

func (r *clientWithRetry) GetConfigMap(ctx context.Context, namespace string, name string) (*corev1.ConfigMap, error) {
	return clients.RequestWithRetry[*corev1.ConfigMap](func() (*corev1.ConfigMap, error) {
		return r.CoreV1().
			ConfigMaps(namespace).
			Get(ctx, name, metav1.GetOptions{})
	}, r.retries, r.delay)
}

func (r *clientWithRetry) CreateConfigMap(ctx context.Context, namespace string, configMap *corev1.ConfigMap) (*corev1.ConfigMap, error) {
	return clients.RequestWithRetry[*corev1.ConfigMap](func() (*corev1.ConfigMap, error) {
		return r.CoreV1().
			ConfigMaps(namespace).
			Create(ctx, configMap, metav1.CreateOptions{})
	}, r.retries, r.delay)
}

func (r *clientWithRetry) UpdateConfigMap(ctx context.Context, namespace string, configMap *corev1.ConfigMap) (*corev1.ConfigMap, error) {
	return clients.RequestWithRetry[*corev1.ConfigMap](func() (*corev1.ConfigMap, error) {
		return r.CoreV1().
			ConfigMaps(namespace).
			Update(ctx, configMap, metav1.UpdateOptions{})
	}, r.retries, r.delay)
}

func (r *clientWithRetry) DeleteConfigMap(ctx context.Context, namespace string, name string, deleteOptions metav1.DeleteOptions) (err error) {
	_, err = clients.RequestWithRetry(func() (any, error) {
		return nil, r.CoreV1().
			ConfigMaps(namespace).
			Delete(ctx, name, deleteOptions)
	}, r.retries, r.delay)
	return
}

func (r *clientWithRetry) ApplyConfigMap(ctx context.Context, namespace string, configMap *corev1.ConfigMap) (*corev1.ConfigMap, error) {
	applyConfig := corev1apply.ConfigMap(configMap.Name, namespace).
		WithLabels(configMap.Labels).
		WithData(configMap.Data)

	return clients.RequestWithRetry[*corev1.ConfigMap](func() (*corev1.ConfigMap, error) {
		return r.CoreV1().
			ConfigMaps(namespace).
			Apply(ctx, applyConfig, metav1.ApplyOptions{FieldManager: fieldManager, Force: true})
	}, r.retries, r.delay)
}

func (r *clientWithRetry) ListServices(ctx context.Context, namespace string, options metav1.ListOptions) (*corev1.ServiceList, error) {
	return clients.RequestWithRetry[*corev1.ServiceList](func() (*corev1.ServiceList, error) {
		return r.CoreV1().
			Services(namespace).
			List(ctx, options)
	}, r.retries, r.delay)
}

func (r *clientWithRetry) GetService(ctx context.Context, namespace string, name string) (*corev1.Service, error) {
	return clients.RequestWithRetry[*corev1.Service](func() (*corev1.Service, error) {
		return r.CoreV1().
			Services(namespace).
			Get(ctx, name, metav1.GetOptions{})
	}, r.retries, r.delay)
}

func (r *clientWithRetry) CreateService(ctx context.Context, namespace string, service *corev1.Service) (*corev1.Service, error) {
	return clients.RequestWithRetry[*corev1.Service](func() (*corev1.Service, error) {
		return r.CoreV1().
			Services(namespace).
			Create(ctx, service, metav1.CreateOptions{})
	}, r.retries, r.delay)
}

func (r *clientWithRetry) UpdateService(ctx context.Context, namespace string, service *corev1.Service) (*corev1.Service, error) {
	return clients.RequestWithRetry[*corev1.Service](func() (*corev1.Service, error) {
		return r.CoreV1().
			Services(namespace).
			Update(ctx, service, metav1.UpdateOptions{})
	}, r.retries, r.delay)
}

func (r *clientWithRetry) DeleteService(ctx context.Context, namespace string, name string, deleteOptions metav1.DeleteOptions) (err error) {
	_, err = clients.RequestWithRetry(func() (any, error) {
		return nil, r.CoreV1().
			Services(namespace).
			Delete(ctx, name, deleteOptions)
	}, r.retries, r.delay)
	return
}

func (r *clientWithRetry) PatchService(ctx context.Context, namespace string, name string, patchType types.PatchType, data []byte) (*corev1.Service, error) {
	return clients.RequestWithRetry[*corev1.Service](func() (*corev1.Service, error) {
		return r.CoreV1().
			Services(namespace).
			Patch(ctx, name, patchType, data, metav1.PatchOptions{})
	}, r.retries, r.delay)
}

func (r *clientWithRetry) ListDeployments(ctx context.Context, namespace string, listOptions metav1.ListOptions) (*appsv1.DeploymentList, error) {
	return clients.RequestWithRetry[*appsv1.DeploymentList](func() (*appsv1.DeploymentList, error) {
		return r.AppsV1().Deployments(namespace).List(ctx, listOptions)
	}, r.retries, r.delay)
}

func (r *clientWithRetry) GetDeployment(ctx context.Context, namespace string, name string) (*appsv1.Deployment, error) {
	return clients.RequestWithRetry[*appsv1.Deployment](func() (*appsv1.Deployment, error) {
		return r.AppsV1().
			Deployments(namespace).
			Get(ctx, name, metav1.GetOptions{})
	}, r.retries, r.delay)
}

func (r *clientWithRetry) CreateDeployment(ctx context.Context, namespace string, deployment *appsv1.Deployment) (*appsv1.Deployment, error) {
	return clients.RequestWithRetry[*appsv1.Deployment](func() (*appsv1.Deployment, error) {
		return r.AppsV1().
			Deployments(namespace).
			Create(ctx, deployment, metav1.CreateOptions{})
	}, r.retries, r.delay)
}

func (r *clientWithRetry) UpdateDeployment(ctx context.Context, namespace string, deployment *appsv1.Deployment) (*appsv1.Deployment, error) {
	return clients.RequestWithRetry[*appsv1.Deployment](func() (*appsv1.Deployment, error) {
		return r.AppsV1().
			Deployments(namespace).
			Update(ctx, deployment, metav1.UpdateOptions{})
	}, r.retries, r.delay)
}

func (r *clientWithRetry) DeleteDeployment(ctx context.Context, namespace string, name string, deleteOptions metav1.DeleteOptions) (err error) {
	_, err = clients.RequestWithRetry(func() (any, error) {
		return nil, r.AppsV1().
			Deployments(namespace).
			Delete(ctx, name, deleteOptions)
	}, r.retries, r.delay)
	return
}

func (r *clientWithRetry) DeleteCollectionReplicaSets(ctx context.Context, namespace string, deleteOptions metav1.DeleteOptions, listOptions metav1.ListOptions) (err error) {
	_, err = clients.RequestWithRetry(func() (any, error) {
		return nil, r.AppsV1().
			ReplicaSets(namespace).
			DeleteCollection(ctx, deleteOptions, listOptions)
	}, r.retries, r.delay)
	return
}

func (r *clientWithRetry) ListPods(ctx context.Context, namespace string, listOptions metav1.ListOptions) (*corev1.PodList, error) {
	return clients.RequestWithRetry[*corev1.PodList](func() (*corev1.PodList, error) {
		return r.CoreV1().Pods(namespace).List(ctx, listOptions)
	}, r.retries, r.delay)
}

func (r *clientWithRetry) GetPod(ctx context.Context, namespace string, name string) (*corev1.Pod, error) {
	return clients.RequestWithRetry[*corev1.Pod](func() (*corev1.Pod, error) {
		return r.CoreV1().
			Pods(namespace).
			Get(ctx, name, metav1.GetOptions{})
	}, r.retries, r.delay)
}

func (r *clientWithRetry) DeletePod(ctx context.Context, namespace string, name string, deleteOptions metav1.DeleteOptions) (err error) {
	_, err = clients.RequestWithRetry(func() (any, error) {
		return nil, r.CoreV1().
			Pods(namespace).
			Delete(ctx, name, deleteOptions)
	}, r.retries, r.delay)
	return
}

func (r *clientWithRetry) DeleteCollectionPods(ctx context.Context, namespace string, deleteOptions metav1.DeleteOptions, listOptions metav1.ListOptions) (err error) {
	_, err = clients.RequestWithRetry(func() (any, error) {
		return nil, r.CoreV1().
			Pods(namespace).
			DeleteCollection(ctx, deleteOptions, listOptions)
	}, r.retries, r.delay)
	return
}

func (r *clientWithRetry) StreamPodLogs(ctx context.Context, namespace, podName string, options *corev1.PodLogOptions) (io.ReadCloser, error) {
	return clients.RequestWithRetry[io.ReadCloser](func() (io.ReadCloser, error) {
		return r.CoreV1().
			Pods(namespace).
			GetLogs(podName, options).
			Stream(ctx)
	}, r.retries, r.delay)
}

func (r *clientWithRetry) GetJob(ctx context.Context, namespace string, name string) (*batchv1.Job, error) {
	return clients.RequestWithRetry[*batchv1.Job](func() (*batchv1.Job, error) {
		return r.BatchV1().
			Jobs(namespace).
			Get(ctx, name, metav1.GetOptions{})
	}, r.retries, r.delay)
}

func (r *clientWithRetry) CreateJob(ctx context.Context, namespace string, job *batchv1.Job) (*batchv1.Job, error) {
	return clients.RequestWithRetry[*batchv1.Job](func() (*batchv1.Job, error) {
		return r.BatchV1().
			Jobs(namespace).
			Create(ctx, job, metav1.CreateOptions{})
	}, r.retries, r.delay)
}

func (r *clientWithRetry) DeleteJob(ctx context.Context, namespace string, name string, deleteOptions metav1.DeleteOptions) (err error) {
	_, err = clients.RequestWithRetry(func() (any, error) {
		return nil, r.BatchV1().
			Jobs(namespace).
			Delete(ctx, name, deleteOptions)
	}, r.retries, r.delay)
	return
}

func (r *clientWithRetry) ListJobs(ctx context.Context, namespace string, listOptions metav1.ListOptions) (*batchv1.JobList, error) {
	return clients.RequestWithRetry[*batchv1.JobList](func() (*batchv1.JobList, error) {
		return r.BatchV1().Jobs(namespace).List(ctx, listOptions)
	}, r.retries, r.delay)
}

func (r *clientWithRetry) ListCronJobs(ctx context.Context, namespace string, listOptions metav1.ListOptions) (*batchv1.CronJobList, error) {
	return clients.RequestWithRetry[*batchv1.CronJobList](func() (*batchv1.CronJobList, error) {
		return r.BatchV1().CronJobs(namespace).List(ctx, listOptions)
	}, r.retries, r.delay)
}

func (r *clientWithRetry) CreateCronJob(ctx context.Context, namespace string, job *batchv1.CronJob) (*batchv1.CronJob, error) {
	return clients.RequestWithRetry[*batchv1.CronJob](func() (*batchv1.CronJob, error) {
		return r.BatchV1().
			CronJobs(namespace).
			Create(ctx, job, metav1.CreateOptions{})
	}, r.retries, r.delay)
}

func (r *clientWithRetry) UpdateCronJob(ctx context.Context, namespace string, job *batchv1.CronJob) (*batchv1.CronJob, error) {
	return clients.RequestWithRetry[*batchv1.CronJob](func() (*batchv1.CronJob, error) {
		return r.BatchV1().
			CronJobs(namespace).
			Update(ctx, job, metav1.UpdateOptions{})
	}, r.retries, r.delay)
}

func (r *clientWithRetry) DeleteCronJob(ctx context.Context, namespace string, name string, deleteOptions metav1.DeleteOptions) (err error) {
	_, err = clients.RequestWithRetry(func() (any, error) {
		return nil, r.BatchV1().
			CronJobs(namespace).
			Delete(ctx, name, deleteOptions)
	}, r.retries, r.delay)
	return
}

func (r *clientWithRetry) DeleteCollectionCronJobs(ctx context.Context, namespace string, deleteOptions metav1.DeleteOptions, listOptions metav1.ListOptions) (err error) {
	_, err = clients.RequestWithRetry(func() (any, error) {
		return nil, r.BatchV1().
			CronJobs(namespace).
			DeleteCollection(ctx, deleteOptions, listOptions)
	}, r.retries, r.delay)
	return
}

func (r *clientWithRetry) ListIngresses(ctx context.Context, namespace string, listOptions metav1.ListOptions) (*networkingv1.IngressList, error) {
	return clients.RequestWithRetry[*networkingv1.IngressList](func() (*networkingv1.IngressList, error) {
		return r.NetworkingV1().
			Ingresses(namespace).
			List(ctx, listOptions)
	}, r.retries, r.delay)
}

func (r *clientWithRetry) GetIngress(ctx context.Context, namespace, name string) (*networkingv1.Ingress, error) {
	return clients.RequestWithRetry[*networkingv1.Ingress](func() (*networkingv1.Ingress, error) {
		return r.NetworkingV1().
			Ingresses(namespace).
			Get(ctx, name, metav1.GetOptions{})
	}, r.retries, r.delay)
}

func (r *clientWithRetry) CreateIngress(ctx context.Context, namespace string, ingress *networkingv1.Ingress) (*networkingv1.Ingress, error) {
	return clients.RequestWithRetry[*networkingv1.Ingress](func() (*networkingv1.Ingress, error) {
		return r.NetworkingV1().
			Ingresses(namespace).
			Create(ctx, ingress, metav1.CreateOptions{})
	}, r.retries, r.delay)
}

func (r *clientWithRetry) UpdateIngress(ctx context.Context, namespace string, ingress *networkingv1.Ingress) (*networkingv1.Ingress, error) {
	return clients.RequestWithRetry[*networkingv1.Ingress](func() (*networkingv1.Ingress, error) {
		return r.NetworkingV1().
			Ingresses(namespace).
			Update(ctx, ingress, metav1.UpdateOptions{})
	}, r.retries, r.delay)
}

func (r *clientWithRetry) DeleteIngress(ctx context.Context, namespace, name string, options metav1.DeleteOptions) (err error) {
	_, err = clients.RequestWithRetry(func() (any, error) {
		return nil, r.NetworkingV1().
			Ingresses(namespace).
			Delete(ctx, name, options)
	}, r.retries, r.delay)
	return
}

func (r *clientWithRetry) GetHorizontalPodAutoscaler(ctx context.Context, namespace string, name string) (*autosv2.HorizontalPodAutoscaler, error) {
	return clients.RequestWithRetry[*autosv2.HorizontalPodAutoscaler](func() (*autosv2.HorizontalPodAutoscaler, error) {
		return r.AutoscalingV2().
			HorizontalPodAutoscalers(namespace).
			Get(ctx, name, metav1.GetOptions{})
	}, r.retries, r.delay)
}

func (r *clientWithRetry) CreateHorizontalPodAutoscaler(ctx context.Context, namespace string, hpa *autosv2.HorizontalPodAutoscaler) (*autosv2.HorizontalPodAutoscaler, error) {
	return clients.RequestWithRetry[*autosv2.HorizontalPodAutoscaler](func() (*autosv2.HorizontalPodAutoscaler, error) {
		return r.AutoscalingV2().
			HorizontalPodAutoscalers(namespace).
			Create(ctx, hpa, metav1.CreateOptions{})
	}, r.retries, r.delay)
}

func (r *clientWithRetry) UpdateHorizontalPodAutoscaler(ctx context.Context, namespace string, hpa *autosv2.HorizontalPodAutoscaler) (*autosv2.HorizontalPodAutoscaler, error) {
	return clients.RequestWithRetry[*autosv2.HorizontalPodAutoscaler](func() (*autosv2.HorizontalPodAutoscaler, error) {
		return r.AutoscalingV2().
			HorizontalPodAutoscalers(namespace).
			Update(ctx, hpa, metav1.UpdateOptions{})
	}, r.retries, r.delay)
}

func (r *clientWithRetry) DeleteHorizontalPodAutoscaler(ctx context.Context, namespace string, name string, deleteOptions metav1.DeleteOptions) (err error) {
	_, err = clients.RequestWithRetry(func() (any, error) {
		return nil, r.AutoscalingV2().
			HorizontalPodAutoscalers(namespace).
			Delete(ctx, name, deleteOptions)
	}, r.retries, r.delay)
	return
}

func (r *clientWithRetry) ListEvents(ctx context.Context, namespace string, listOptions metav1.ListOptions) (*corev1.EventList, error) {
	return clients.RequestWithRetry[*corev1.EventList](func() (*corev1.EventList, error) {
		return r.CoreV1().Events(namespace).List(ctx, listOptions)
	}, r.retries, r.delay)
}

func (r *clientWithRetry) ListNamespaces(ctx context.Context, listOptions metav1.ListOptions) (*corev1.NamespaceList, error) {
	return clients.RequestWithRetry[*corev1.NamespaceList](func() (*corev1.NamespaceList, error) {
		return r.CoreV1().Namespaces().List(ctx, listOptions)
	}, r.retries, r.delay)
}
