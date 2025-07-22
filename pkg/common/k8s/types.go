package k8s

import (
	"context"
	"github.com/nuclio/errors"
	"io"
	appsv1 "k8s.io/api/apps/v1"
	autosv2 "k8s.io/api/autoscaling/v2"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"time"
)

type ClientWithRetry interface {
	ListSecrets(ctx context.Context, namespace string, listOptions metav1.ListOptions) (*corev1.SecretList, error)
	ListDeployments(ctx context.Context, namespace string, listOptions metav1.ListOptions) (*appsv1.DeploymentList, error)
	ListIngresses(ctx context.Context, namespace string, listOptions metav1.ListOptions) (*networkingv1.IngressList, error)
	ListNamespaces(ctx context.Context, listOptions metav1.ListOptions) (*corev1.NamespaceList, error)
	ListPods(ctx context.Context, namespace string, listOptions metav1.ListOptions) (*corev1.PodList, error)
	ListEvents(ctx context.Context, namespace string, listOptions metav1.ListOptions) (*corev1.EventList, error)

	GetSecret(ctx context.Context, namespace, name string) (*corev1.Secret, error)
	GetPod(ctx context.Context, namespace string, name string) (*corev1.Pod, error)
	GetJob(ctx context.Context, namespace string, name string) (*batchv1.Job, error)
	GetDeployment(ctx context.Context, namespace string, name string) (*appsv1.Deployment, error)
	GetHorizontalPodAutoscaler(ctx context.Context, namespace string, name string) (*autosv2.HorizontalPodAutoscaler, error)
	GetIngress(ctx context.Context, namespace, name string) (*networkingv1.Ingress, error)

	CreateSecret(ctx context.Context, namespace string, secret *corev1.Secret) (*corev1.Secret, error)
	CreateJob(ctx context.Context, namespace string, job *batchv1.Job) (*batchv1.Job, error)
	CreateIngress(ctx context.Context, namespace string, ingress *networkingv1.Ingress) (*networkingv1.Ingress, error)

	UpdateSecret(ctx context.Context, namespace string, secret *corev1.Secret) (*corev1.Secret, error)
	UpdateIngress(ctx context.Context, namespace string, ingress *networkingv1.Ingress) (*networkingv1.Ingress, error)

	StreamPodLogs(ctx context.Context, namespace, podName string, options *corev1.PodLogOptions) (io.ReadCloser, error)

	DeleteSecret(ctx context.Context, namespace, name string) (err error)
	DeleteJob(ctx context.Context, namespace string, name string, options metav1.DeleteOptions) (err error)
	DeleteIngress(ctx context.Context, namespace, name string, options metav1.DeleteOptions) (err error)
	DeleteDeployment(ctx context.Context, namespace string, name string, deleteOptions metav1.DeleteOptions) (err error)
}

type clientWithRetry struct {
	kubernetes.Interface
	retries int
	delay   time.Duration
}

func NewClientWithRetry(config *rest.Config) (ClientWithRetry, error) {
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

func (r *clientWithRetry) ListDeployments(ctx context.Context, namespace string, listOptions metav1.ListOptions) (*appsv1.DeploymentList, error) {
	return requestWithRetry[*appsv1.DeploymentList](func() (*appsv1.DeploymentList, error) {
		return r.AppsV1().Deployments(namespace).List(ctx, listOptions)
	}, r.retries, r.delay)
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

func (r *clientWithRetry) DeleteSecret(ctx context.Context, namespace, name string) (err error) {
	_, err = requestWithRetry(func() (any, error) {
		return nil, r.CoreV1().Secrets(namespace).Delete(ctx,
			name,
			metav1.DeleteOptions{})
	}, r.retries, r.delay)
	return
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

func (r *clientWithRetry) GetIngress(ctx context.Context, namespace, name string) (*networkingv1.Ingress, error) {
	return requestWithRetry[*networkingv1.Ingress](func() (*networkingv1.Ingress, error) {
		return r.NetworkingV1().
			Ingresses(namespace).
			Get(ctx, name, metav1.GetOptions{})
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

func (r *clientWithRetry) ListIngresses(ctx context.Context, namespace string, listOptions metav1.ListOptions) (*networkingv1.IngressList, error) {
	return requestWithRetry[*networkingv1.IngressList](func() (*networkingv1.IngressList, error) {
		return r.NetworkingV1().
			Ingresses(namespace).
			List(ctx, listOptions)
	}, r.retries, r.delay)
}

func (r *clientWithRetry) ListNamespaces(ctx context.Context, listOptions metav1.ListOptions) (*corev1.NamespaceList, error) {
	return requestWithRetry[*corev1.NamespaceList](func() (*corev1.NamespaceList, error) {
		return r.CoreV1().Namespaces().List(ctx, listOptions)
	}, r.retries, r.delay)
}

func (r *clientWithRetry) ListPods(ctx context.Context, namespace string, listOptions metav1.ListOptions) (*corev1.PodList, error) {
	return requestWithRetry[*corev1.PodList](func() (*corev1.PodList, error) {
		return r.CoreV1().Pods(namespace).List(ctx, listOptions)
	}, r.retries, r.delay)
}

func (r *clientWithRetry) ListEvents(ctx context.Context, namespace string, listOptions metav1.ListOptions) (*corev1.EventList, error) {
	return requestWithRetry[*corev1.EventList](func() (*corev1.EventList, error) {
		return r.CoreV1().Events(namespace).List(ctx, listOptions)
	}, r.retries, r.delay)
}

func (r *clientWithRetry) GetPod(ctx context.Context, namespace string, name string) (*corev1.Pod, error) {
	return requestWithRetry[*corev1.Pod](func() (*corev1.Pod, error) {
		return r.CoreV1().
			Pods(namespace).
			Get(ctx, name, metav1.GetOptions{})
	}, r.retries, r.delay)
}

func (r *clientWithRetry) StreamPodLogs(ctx context.Context, namespace, podName string, options *corev1.PodLogOptions) (io.ReadCloser, error) {
	return requestWithRetry[io.ReadCloser](func() (io.ReadCloser, error) {
		return r.CoreV1().
			Pods(namespace).
			GetLogs(podName, options).
			Stream(ctx)
	}, r.retries, r.delay)
}

func (r *clientWithRetry) CreateJob(ctx context.Context, namespace string, job *batchv1.Job) (*batchv1.Job, error) {
	return requestWithRetry[*batchv1.Job](func() (*batchv1.Job, error) {
		return r.BatchV1().
			Jobs(namespace).
			Create(ctx, job, metav1.CreateOptions{})
	}, r.retries, r.delay)
}

func (r *clientWithRetry) GetJob(ctx context.Context, namespace string, name string) (*batchv1.Job, error) {
	return requestWithRetry[*batchv1.Job](func() (*batchv1.Job, error) {
		return r.BatchV1().
			Jobs(namespace).
			Get(ctx, name, metav1.GetOptions{})
	}, r.retries, r.delay)
}

func (r *clientWithRetry) GetDeployment(ctx context.Context, namespace string, name string) (*appsv1.Deployment, error) {
	return requestWithRetry[*appsv1.Deployment](func() (*appsv1.Deployment, error) {
		return r.AppsV1().
			Deployments(namespace).
			Get(ctx, name, metav1.GetOptions{})
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

func (r *clientWithRetry) GetHorizontalPodAutoscaler(ctx context.Context, namespace string, name string) (*autosv2.HorizontalPodAutoscaler, error) {
	return requestWithRetry[*autosv2.HorizontalPodAutoscaler](func() (*autosv2.HorizontalPodAutoscaler, error) {
		return r.AutoscalingV2().
			HorizontalPodAutoscalers(namespace).
			Get(ctx, name, metav1.GetOptions{})
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
