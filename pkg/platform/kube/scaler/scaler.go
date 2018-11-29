package scaler

import (
	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio/pkg/version"
	metricsv1 "k8s.io/metrics/pkg/client/clientset_generated/clientset"
	custommetricsv1 "k8s.io/metrics/pkg/client/custom_metrics"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"time"
)

type Scaler struct {
	logger           logger.Logger
	namespace        string
	restConfig       *rest.Config
	kubeClientSet    kubernetes.Interface
	metricsOperator  *metricsOperator
	metricsClientset *metricsv1.Clientset
	customMetricsClientSet custommetricsv1.CustomMetricsClient
}

func NewScaler(parentLogger logger.Logger,
	namespace string,
	kubeClientSet kubernetes.Interface,
	metricsClientSet *metricsv1.Clientset,
	customMetricsClientSet custommetricsv1.CustomMetricsClient,
	resyncInterval time.Duration) (*Scaler, error) {

	// replace "*" with "", which is actually "all" in kube-speak
	if namespace == "*" {
		namespace = ""
	}

	newController := &Scaler{
		logger:            parentLogger,
		namespace:         namespace,
		kubeClientSet:     kubeClientSet,
		metricsClientset:  metricsClientSet,
	}

	var err error
	newController.metricsOperator, err = newMetricsOperator(newController.logger, newController)
	if err != nil {
		return nil, err
	}

	// log version info
	version.Log(newController.logger)

	return newController, nil
}

func (c *Scaler) ScaleToZero(namespace string, name string) {
	replicas := int32(0)
	err := c.kubeClientSet.AutoscalingV2beta1().HorizontalPodAutoscalers(namespace).Delete(name,
		&metav1.DeleteOptions{})
	if err != nil {
		c.logger.Debug("error2", "err", err)
	}

	d, err := c.kubeClientSet.AppsV1beta1().Deployments(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		c.logger.Debug("error", "err", err)
	}
	d.Spec.Replicas = &replicas

	_, err = c.kubeClientSet.AppsV1beta1().Deployments(namespace).Update(d)
	if err != nil {
		c.logger.Debug("error", "err", err)
	}
}

func (c *Scaler) Start() error {
	c.logger.InfoWith("Starting", "namespace", c.namespace)

	err := c.metricsOperator.start()
	if err != nil {
		return err
	}
	return nil
}
