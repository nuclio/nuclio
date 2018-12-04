package scaler

import (
	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	nuclioio_client "github.com/nuclio/nuclio/pkg/platform/kube/client/clientset/versioned"
	"github.com/nuclio/nuclio/pkg/version"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	metricsv1 "k8s.io/metrics/pkg/client/clientset_generated/clientset"
	custommetricsv1 "k8s.io/metrics/pkg/client/custom_metrics"
	"time"
)

type statKey struct {
	sourceType string
	namespace string
	functionName string
}

type entry struct {
	timestamp time.Time
	value     int64
	namespace string
	functionName string
	sourceType  string
}

type ZeroScaler struct {
	logger           logger.Logger
	namespace        string
	restConfig       *rest.Config
	kubeClientSet    kubernetes.Interface
	metricsOperator  *metricsOperator
	metricsClientset *metricsv1.Clientset
	customMetricsClientSet custommetricsv1.CustomMetricsClient
	nuclioClientSet        nuclioio_client.Interface
	autoscaler       *Autoscale
	scaleInterval    time.Duration
}

type Scaler interface {
	Scale(namespace string, functionName string, target int)
}

func NewScaler(parentLogger logger.Logger,
	namespace string,
	kubeClientSet kubernetes.Interface,
	metricsClientSet *metricsv1.Clientset,
	nuclioClientSet *nuclioio_client.Clientset,
	customMetricsClientSet custommetricsv1.CustomMetricsClient,
    scaleInterval time.Duration,
    metricsInterval time.Duration,
	resyncInterval time.Duration) (*ZeroScaler, error) {

	// replace "*" with "", which is actually "all" in kube-speak
	if namespace == "*" {
		namespace = ""
	}

	scaler := &ZeroScaler{
		logger:            parentLogger,
		namespace:         namespace,
		kubeClientSet:     kubeClientSet,
		metricsClientset:  metricsClientSet,
		customMetricsClientSet: customMetricsClientSet,
		nuclioClientSet:   nuclioClientSet,
		scaleInterval:     scaleInterval,
	}

	var err error

	// a shared statsChannel between autoscaler and sources
	statsChannel := make(chan entry)
	scaler.metricsOperator, err = NewMetricsOperator(scaler.logger, scaler, statsChannel, metricsInterval, namespace)
	if err != nil {
		return nil, err
	}
	scaler.autoscaler = NewAutoScaler(scaler.logger, namespace, statsChannel, scaler)

	// log version info
	version.Log(scaler.logger)

	return scaler, nil
}

func (c *ZeroScaler) Scale(namespace string, functionName string, target int) {
	c.logger.DebugWith("Scaling to zero", "functionName", functionName)
	function, err := c.nuclioClientSet.NuclioV1beta1().Functions(c.namespace).Get(functionName, metav1.GetOptions{})
	if err != nil {
		c.logger.Debug("error2", "err", err)
		return
	}

	function.Spec.MaxReplicas = 0
	function.Spec.MinReplicas = 0
	function.Spec.Replicas = 0
	_, err = c.nuclioClientSet.NuclioV1beta1().Functions(namespace).Update(function)
	if err != nil {
		c.logger.Debug("error", "err", err)
		return
	}
}

func (c *ZeroScaler) Start() error {
	c.logger.InfoWith("Starting", "namespace", c.namespace)

	err := c.metricsOperator.start()
	if err != nil {
		return err
	}

	c.autoscaler.start()

	ticker := time.NewTicker(c.scaleInterval)
	go func() {
		for range ticker.C {
			functions, err := c.nuclioClientSet.NuclioV1beta1().Functions(c.namespace).List(metav1.ListOptions{})
			if err != nil {
				return
			}

			functionMap := make(map[statKey]*functionconfig.Spec)

			for _, function := range functions.Items {
				for _, metricSource := range function.Spec.Metrics {
					if metricSource.SourceType == "" {
						continue
					}
					key := statKey{
						functionName: function.Name,
						namespace: function.Namespace,
						sourceType: metricSource.SourceType,
					}
					functionMap[key] = &function.Spec
				}
			}

			c.autoscaler.CheckToScale(time.Now(), functionMap)
		}
	}()
	return nil
}
