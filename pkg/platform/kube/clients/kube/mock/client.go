/*
Copyright 2026 The Nuclio Authors.

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

package mock

import (
	"context"
	"io"

	"github.com/stretchr/testify/mock"
	appsv1 "k8s.io/api/apps/v1"
	autosv2 "k8s.io/api/autoscaling/v2"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// Client mocks kube.Client.
type Client struct {
	mock.Mock
}

// --- Secrets ---

func (c *Client) ListSecrets(ctx context.Context, namespace string, listOptions metav1.ListOptions) (*corev1.SecretList, error) {
	args := c.Called(ctx, namespace, listOptions)
	list, _ := args.Get(0).(*corev1.SecretList)
	return list, args.Error(1)
}

func (c *Client) GetSecret(ctx context.Context, namespace, name string) (*corev1.Secret, error) {
	args := c.Called(ctx, namespace, name)
	secret, _ := args.Get(0).(*corev1.Secret)
	return secret, args.Error(1)
}

func (c *Client) CreateSecret(ctx context.Context, namespace string, secret *corev1.Secret) (*corev1.Secret, error) {
	args := c.Called(ctx, namespace, secret)
	created, _ := args.Get(0).(*corev1.Secret)
	return created, args.Error(1)
}

func (c *Client) UpdateSecret(ctx context.Context, namespace string, secret *corev1.Secret) (*corev1.Secret, error) {
	args := c.Called(ctx, namespace, secret)
	updated, _ := args.Get(0).(*corev1.Secret)
	return updated, args.Error(1)
}

func (c *Client) DeleteSecret(ctx context.Context, namespace, name string) error {
	args := c.Called(ctx, namespace, name)
	return args.Error(0)
}

func (c *Client) DeleteCollectionSecrets(ctx context.Context, namespace string, deleteOptions metav1.DeleteOptions, listOptions metav1.ListOptions) error {
	args := c.Called(ctx, namespace, deleteOptions, listOptions)
	return args.Error(0)
}

// --- ConfigMaps ---

func (c *Client) GetConfigMap(ctx context.Context, namespace string, name string) (*corev1.ConfigMap, error) {
	args := c.Called(ctx, namespace, name)
	configMap, _ := args.Get(0).(*corev1.ConfigMap)
	return configMap, args.Error(1)
}

func (c *Client) CreateConfigMap(ctx context.Context, namespace string, configMap *corev1.ConfigMap) (*corev1.ConfigMap, error) {
	args := c.Called(ctx, namespace, configMap)
	created, _ := args.Get(0).(*corev1.ConfigMap)
	return created, args.Error(1)
}

func (c *Client) UpdateConfigMap(ctx context.Context, namespace string, configMap *corev1.ConfigMap) (*corev1.ConfigMap, error) {
	args := c.Called(ctx, namespace, configMap)
	updated, _ := args.Get(0).(*corev1.ConfigMap)
	return updated, args.Error(1)
}

func (c *Client) DeleteConfigMap(ctx context.Context, namespace string, name string, deleteOptions metav1.DeleteOptions) error {
	args := c.Called(ctx, namespace, name, deleteOptions)
	return args.Error(0)
}

func (c *Client) ApplyConfigMap(ctx context.Context, namespace string, configMap *corev1.ConfigMap) (*corev1.ConfigMap, error) {
	args := c.Called(ctx, namespace, configMap)
	applied, _ := args.Get(0).(*corev1.ConfigMap)
	return applied, args.Error(1)
}

// --- Services ---

func (c *Client) GetService(ctx context.Context, namespace string, name string) (*corev1.Service, error) {
	args := c.Called(ctx, namespace, name)
	service, _ := args.Get(0).(*corev1.Service)
	return service, args.Error(1)
}

func (c *Client) ListServices(ctx context.Context, namespace string, options metav1.ListOptions) (*corev1.ServiceList, error) {
	args := c.Called(ctx, namespace, options)
	list, _ := args.Get(0).(*corev1.ServiceList)
	return list, args.Error(1)
}

func (c *Client) CreateService(ctx context.Context, namespace string, service *corev1.Service) (*corev1.Service, error) {
	args := c.Called(ctx, namespace, service)
	created, _ := args.Get(0).(*corev1.Service)
	return created, args.Error(1)
}

func (c *Client) UpdateService(ctx context.Context, namespace string, service *corev1.Service) (*corev1.Service, error) {
	args := c.Called(ctx, namespace, service)
	updated, _ := args.Get(0).(*corev1.Service)
	return updated, args.Error(1)
}

func (c *Client) DeleteService(ctx context.Context, namespace string, name string, deleteOptions metav1.DeleteOptions) error {
	args := c.Called(ctx, namespace, name, deleteOptions)
	return args.Error(0)
}

func (c *Client) PatchService(ctx context.Context, namespace string, name string, patchType types.PatchType, data []byte) (*corev1.Service, error) {
	args := c.Called(ctx, namespace, name, patchType, data)
	patched, _ := args.Get(0).(*corev1.Service)
	return patched, args.Error(1)
}

// --- Deployments ---

func (c *Client) ListDeployments(ctx context.Context, namespace string, listOptions metav1.ListOptions) (*appsv1.DeploymentList, error) {
	args := c.Called(ctx, namespace, listOptions)
	list, _ := args.Get(0).(*appsv1.DeploymentList)
	return list, args.Error(1)
}

func (c *Client) GetDeployment(ctx context.Context, namespace string, name string) (*appsv1.Deployment, error) {
	args := c.Called(ctx, namespace, name)
	deployment, _ := args.Get(0).(*appsv1.Deployment)
	return deployment, args.Error(1)
}

func (c *Client) CreateDeployment(ctx context.Context, namespace string, deployment *appsv1.Deployment) (*appsv1.Deployment, error) {
	args := c.Called(ctx, namespace, deployment)
	created, _ := args.Get(0).(*appsv1.Deployment)
	return created, args.Error(1)
}

func (c *Client) UpdateDeployment(ctx context.Context, namespace string, deployment *appsv1.Deployment) (*appsv1.Deployment, error) {
	args := c.Called(ctx, namespace, deployment)
	updated, _ := args.Get(0).(*appsv1.Deployment)
	return updated, args.Error(1)
}

func (c *Client) DeleteDeployment(ctx context.Context, namespace string, name string, deleteOptions metav1.DeleteOptions) error {
	args := c.Called(ctx, namespace, name, deleteOptions)
	return args.Error(0)
}

// --- ReplicaSets ---

func (c *Client) DeleteCollectionReplicaSets(ctx context.Context, namespace string, deleteOptions metav1.DeleteOptions, listOptions metav1.ListOptions) error {
	args := c.Called(ctx, namespace, deleteOptions, listOptions)
	return args.Error(0)
}

// --- Pods ---

func (c *Client) ListPods(ctx context.Context, namespace string, listOptions metav1.ListOptions) (*corev1.PodList, error) {
	args := c.Called(ctx, namespace, listOptions)
	list, _ := args.Get(0).(*corev1.PodList)
	return list, args.Error(1)
}

func (c *Client) GetPod(ctx context.Context, namespace string, name string) (*corev1.Pod, error) {
	args := c.Called(ctx, namespace, name)
	pod, _ := args.Get(0).(*corev1.Pod)
	return pod, args.Error(1)
}

func (c *Client) StreamPodLogs(ctx context.Context, namespace, podName string, options *corev1.PodLogOptions) (io.ReadCloser, error) {
	args := c.Called(ctx, namespace, podName, options)
	stream, _ := args.Get(0).(io.ReadCloser)
	return stream, args.Error(1)
}

func (c *Client) DeletePod(ctx context.Context, namespace string, name string, deleteOptions metav1.DeleteOptions) error {
	args := c.Called(ctx, namespace, name, deleteOptions)
	return args.Error(0)
}

func (c *Client) DeleteCollectionPods(ctx context.Context, namespace string, deleteOptions metav1.DeleteOptions, listOptions metav1.ListOptions) error {
	args := c.Called(ctx, namespace, deleteOptions, listOptions)
	return args.Error(0)
}

// --- Jobs ---

func (c *Client) GetJob(ctx context.Context, namespace string, name string) (*batchv1.Job, error) {
	args := c.Called(ctx, namespace, name)
	job, _ := args.Get(0).(*batchv1.Job)
	return job, args.Error(1)
}

func (c *Client) CreateJob(ctx context.Context, namespace string, job *batchv1.Job) (*batchv1.Job, error) {
	args := c.Called(ctx, namespace, job)
	created, _ := args.Get(0).(*batchv1.Job)
	return created, args.Error(1)
}

func (c *Client) DeleteJob(ctx context.Context, namespace string, name string, deleteOptions metav1.DeleteOptions) error {
	args := c.Called(ctx, namespace, name, deleteOptions)
	return args.Error(0)
}

func (c *Client) ListJobs(ctx context.Context, namespace string, listOptions metav1.ListOptions) (*batchv1.JobList, error) {
	args := c.Called(ctx, namespace, listOptions)
	list, _ := args.Get(0).(*batchv1.JobList)
	return list, args.Error(1)
}

// --- CronJobs ---

func (c *Client) ListCronJobs(ctx context.Context, namespace string, listOptions metav1.ListOptions) (*batchv1.CronJobList, error) {
	args := c.Called(ctx, namespace, listOptions)
	list, _ := args.Get(0).(*batchv1.CronJobList)
	return list, args.Error(1)
}

func (c *Client) CreateCronJob(ctx context.Context, namespace string, job *batchv1.CronJob) (*batchv1.CronJob, error) {
	args := c.Called(ctx, namespace, job)
	created, _ := args.Get(0).(*batchv1.CronJob)
	return created, args.Error(1)
}

func (c *Client) UpdateCronJob(ctx context.Context, namespace string, job *batchv1.CronJob) (*batchv1.CronJob, error) {
	args := c.Called(ctx, namespace, job)
	updated, _ := args.Get(0).(*batchv1.CronJob)
	return updated, args.Error(1)
}

func (c *Client) DeleteCronJob(ctx context.Context, namespace string, name string, deleteOptions metav1.DeleteOptions) error {
	args := c.Called(ctx, namespace, name, deleteOptions)
	return args.Error(0)
}

func (c *Client) DeleteCollectionCronJobs(ctx context.Context, namespace string, deleteOptions metav1.DeleteOptions, listOptions metav1.ListOptions) error {
	args := c.Called(ctx, namespace, deleteOptions, listOptions)
	return args.Error(0)
}

// --- Ingresses ---

func (c *Client) ListIngresses(ctx context.Context, namespace string, listOptions metav1.ListOptions) (*networkingv1.IngressList, error) {
	args := c.Called(ctx, namespace, listOptions)
	list, _ := args.Get(0).(*networkingv1.IngressList)
	return list, args.Error(1)
}

func (c *Client) GetIngress(ctx context.Context, namespace, name string) (*networkingv1.Ingress, error) {
	args := c.Called(ctx, namespace, name)
	ingress, _ := args.Get(0).(*networkingv1.Ingress)
	return ingress, args.Error(1)
}

func (c *Client) CreateIngress(ctx context.Context, namespace string, ingress *networkingv1.Ingress) (*networkingv1.Ingress, error) {
	args := c.Called(ctx, namespace, ingress)
	created, _ := args.Get(0).(*networkingv1.Ingress)
	return created, args.Error(1)
}

func (c *Client) UpdateIngress(ctx context.Context, namespace string, ingress *networkingv1.Ingress) (*networkingv1.Ingress, error) {
	args := c.Called(ctx, namespace, ingress)
	updated, _ := args.Get(0).(*networkingv1.Ingress)
	return updated, args.Error(1)
}

func (c *Client) DeleteIngress(ctx context.Context, namespace, name string, deleteOptions metav1.DeleteOptions) error {
	args := c.Called(ctx, namespace, name, deleteOptions)
	return args.Error(0)
}

// --- HorizontalPodAutoscalers (HPA) ---

func (c *Client) GetHorizontalPodAutoscaler(ctx context.Context, namespace string, name string) (*autosv2.HorizontalPodAutoscaler, error) {
	args := c.Called(ctx, namespace, name)
	hpa, _ := args.Get(0).(*autosv2.HorizontalPodAutoscaler)
	return hpa, args.Error(1)
}

func (c *Client) CreateHorizontalPodAutoscaler(ctx context.Context, namespace string, hpa *autosv2.HorizontalPodAutoscaler) (*autosv2.HorizontalPodAutoscaler, error) {
	args := c.Called(ctx, namespace, hpa)
	created, _ := args.Get(0).(*autosv2.HorizontalPodAutoscaler)
	return created, args.Error(1)
}

func (c *Client) UpdateHorizontalPodAutoscaler(ctx context.Context, namespace string, hpa *autosv2.HorizontalPodAutoscaler) (*autosv2.HorizontalPodAutoscaler, error) {
	args := c.Called(ctx, namespace, hpa)
	updated, _ := args.Get(0).(*autosv2.HorizontalPodAutoscaler)
	return updated, args.Error(1)
}

func (c *Client) DeleteHorizontalPodAutoscaler(ctx context.Context, namespace string, name string, deleteOptions metav1.DeleteOptions) error {
	args := c.Called(ctx, namespace, name, deleteOptions)
	return args.Error(0)
}

// --- Events ---

func (c *Client) ListEvents(ctx context.Context, namespace string, listOptions metav1.ListOptions) (*corev1.EventList, error) {
	args := c.Called(ctx, namespace, listOptions)
	list, _ := args.Get(0).(*corev1.EventList)
	return list, args.Error(1)
}

// --- Namespaces ---

func (c *Client) ListNamespaces(ctx context.Context, listOptions metav1.ListOptions) (*corev1.NamespaceList, error) {
	args := c.Called(ctx, listOptions)
	list, _ := args.Get(0).(*corev1.NamespaceList)
	return list, args.Error(1)
}
