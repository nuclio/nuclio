package scaler

import (
	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"time"
)

type metricsOperator struct {
	logger       logger.Logger
	scaler       *ZeroScaler
	statsChannel chan entry
	ticker       *time.Ticker
}

func NewMetricsOperator(parentLogger logger.Logger,
	scaler *ZeroScaler,
	statsChannel chan entry,
	interval time.Duration) (*metricsOperator, error) {
	var err error

	loggerInstance := parentLogger.GetChild("metrics")

	ticker := time.NewTicker(interval)

	newMetricsOperator := &metricsOperator{
		logger:       loggerInstance,
		scaler:       scaler,
		statsChannel: statsChannel,
		ticker:       ticker,
	}
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create project operator")
	}

	return newMetricsOperator, nil
}

func (po *metricsOperator) getCPUStats() {
	podMetrices, err := po.scaler.metricsClientset.MetricsV1beta1().PodMetricses("default").List(metav1.ListOptions{})
	pods, err := po.scaler.kubeClientSet.CoreV1().Pods("default").List(metav1.ListOptions{
		LabelSelector: "nuclio.io/class=function",
	})

	if err != nil {
		po.logger.DebugWith("Error", "err", err)
		return
	}

	for _, podMetric := range podMetrices.Items {
		functionName, err := po.getFunctionNameByPodName(pods, podMetric.Name)
		if err != nil {
			po.logger.ErrorWith("Error", "err", err)
		}
		for _, container := range podMetric.Containers {
			int64Val := container.Usage.Cpu().MilliValue()

			po.logger.DebugWith("Container status", "cpu", container.Usage.Cpu())
			newEntry := entry{
				timestamp:    time.Now(),
				value:        int64Val,
				namespace:    podMetric.Namespace,
				functionName: functionName,
				sourceType:   "cpu",
			}
			po.statsChannel <- newEntry
		}
	}
}

func (po *metricsOperator) getFunctionNameByPodName(podList *corev1.PodList, name string) (string, error) {
	pod, err := po.getPodByName(podList, name)
	if err != nil {
		return "", err
	}
	functionName := pod.Labels["nuclio.io/function-name"]
	if functionName == "" {
		return "", errors.New("Failed to get function name")
	}
	return functionName, nil
}

func (po *metricsOperator) getPodByName(podList *corev1.PodList, name string) (*corev1.Pod, error) {
	for _, pod := range podList.Items {
		if pod.Name == name {
			return &pod, nil
		}
	}
	return nil, errors.New("Failed to locate pod in list")
}

func (po *metricsOperator) start() error {
	go func() {
		for range po.ticker.C {
			po.getCPUStats()
		}
	}()

	return nil
}
