package controller

import (
	"fmt"
	"strings"
	"time"

	"github.com/nuclio/logger"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type CronJobMonitoring struct {
	logger                           logger.Logger
	controller                       *Controller
	staleCronJobPodsDeletionInterval *time.Duration
}

func NewCronJobMonitoring(parentLogger logger.Logger,
	controller *Controller,
	staleCronJobPodsDeletionInterval *time.Duration) *CronJobMonitoring {

	loggerInstance := parentLogger.GetChild("cron_job_monitoring")

	newCronJobMonitoring := &CronJobMonitoring{
		logger:                           loggerInstance,
		controller:                       controller,
		staleCronJobPodsDeletionInterval: staleCronJobPodsDeletionInterval,
	}

	parentLogger.DebugWith("Successfully created cron job monitoring instance",
		"staleCronJobPodsDeletionInterval", staleCronJobPodsDeletionInterval)

	return newCronJobMonitoring
}

func (cjm *CronJobMonitoring) start() {

	go cjm.startStaleCronJobPodsDeletionLoop() // nolint: errcheck

}

// delete all stale cron job pods (as k8s lacks this logic, and CronJob pods are never deleted)
func (cjm *CronJobMonitoring) startStaleCronJobPodsDeletionLoop() error {
	stalePodsFieldSelector := cjm.compileStalePodsFieldSelector()

	cjm.logger.InfoWith("Starting stale cron job pods deletion loop",
		"staleCronJobPodsDeletionInterval", cjm.staleCronJobPodsDeletionInterval,
		"fieldSelectors", stalePodsFieldSelector)

	for {

		// sleep until next deletion time staleCronJobPodsDeletionInterval
		time.Sleep(*cjm.staleCronJobPodsDeletionInterval)

		err := cjm.controller.kubeClientSet.
			CoreV1().
			Pods(cjm.controller.podNamespace).
			DeleteCollection(&metav1.DeleteOptions{},
				metav1.ListOptions{
					LabelSelector: "nuclio.io/function-cron-job-pod=true",
					FieldSelector: stalePodsFieldSelector,
				})
		if err != nil {
			cjm.logger.WarnWith("Failed to delete stale cron job pods", "err", err)
		}
	}
}

// create a field selector(string) for stale pods
func (cjm *CronJobMonitoring) compileStalePodsFieldSelector() string {
	var fieldSelectors []string

	// filter out non stale pods by their phase
	nonStalePodPhases := []v1.PodPhase{v1.PodPending, v1.PodRunning}
	for _, nonStalePodPhase := range nonStalePodPhases {
		selector := fmt.Sprintf("status.phase!=%s", string(nonStalePodPhase))
		fieldSelectors = append(fieldSelectors, selector)
	}

	return strings.Join(fieldSelectors, ",")
}
