package scaler

import (
	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/platform/kube/operator"
	"golang.org/x/net/context"
	autoscalingv2 "k8s.io/api/autoscaling/v2beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	"time"
)

type hpaOperator struct {
	logger   logger.Logger
	scaler   *Scaler
	operator operator.Operator
	stats    map[string][]entry
	ticker   *time.Ticker
}

func newHPAOperator(parentLogger logger.Logger,
	scaler *Scaler,
	resyncInterval *time.Duration) (*hpaOperator, error) {
	var err error

	loggerInstance := parentLogger.GetChild("project")

	ticker := time.NewTicker(1 * time.Second)

	newMetricsOperator := &hpaOperator{
		logger: loggerInstance,
		scaler: scaler,
		stats:  make(map[string][]entry),
		ticker: ticker,
	}

	// create a hpa operator
	newMetricsOperator.operator, err = operator.NewMultiWorker(loggerInstance,
		2,
		newMetricsOperator.getListWatcher(scaler.namespace),
		&autoscalingv2.HorizontalPodAutoscaler{},
		resyncInterval,
		newMetricsOperator)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create project operator")
	}

	go func() {
		for range ticker.C {
			newMetricsOperator.onTick()
		}
	}()

	return newMetricsOperator, nil
}

// CreateOrUpdate handles creation/update of an object
func (po *hpaOperator) CreateOrUpdate(ctx context.Context, object runtime.Object) error {
	//po.logger.DebugWith("Created/updated", "object", object)
	hpaObject := object.(*autoscalingv2.HorizontalPodAutoscaler)

	_, found := po.stats[hpaObject.Name]
	if !found {
		po.stats[hpaObject.Name] = []entry{
			{
				time.Now(),
				1,
				hpaObject.ObjectMeta.Namespace,
			},
		}
	}

	return nil
}

// Delete handles delete of an object
func (po *hpaOperator) Delete(ctx context.Context, namespace string, name string) error {
	po.logger.DebugWith("Deleted", "namespace", namespace, "name", name)

	return nil
}

func (po *hpaOperator) getListWatcher(namespace string) cache.ListerWatcher {
	return &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			return po.scaler.kubeClientSet.AutoscalingV2beta1().HorizontalPodAutoscalers(namespace).List(options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			return po.scaler.kubeClientSet.AutoscalingV2beta1().HorizontalPodAutoscalers(namespace).Watch(options)
		},
	}
}

func (po *hpaOperator) onTick() {
	po.listAllNuclioPods()
	for _, values := range po.stats {
		if len(values) < 1 {
			return
		}
		if values[0].value == 0 {

		}
	}
}

func (po *hpaOperator) listAllNuclioPods() {
	pods, err := po.scaler.metricsClientset.MetricsV1beta1().PodMetricses("default").List(metav1.ListOptions{
		LabelSelector: "nuclio.io/class=function",
	})

	if err != nil {
		po.logger.DebugWith("ERror", "err", err)
		return
	}

	for _, pod := range pods.Items {
		for _, container := range pod.Containers {
			po.logger.DebugWith("Container status", "cpu", container.Usage.Cpu())
		}
	}
}

func (po *hpaOperator) start() error {
	go po.operator.Start() // nolint: errcheck

	return nil
}
