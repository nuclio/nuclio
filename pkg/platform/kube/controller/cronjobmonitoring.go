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

func (cjpd *CronJobMonitoring) start() {

	go cjpd.startStaleCronJobPodsDeletionLoop() // nolint: errcheck

}

// delete all stale cron job pods (as k8s lacks this logic, and CronJob pods are never deleted)
func (cjpd *CronJobMonitoring) startStaleCronJobPodsDeletionLoop() error {
	stalePodsFieldSelector := cjpd.compileStalePodsFieldSelector()

	cjpd.logger.InfoWith("Starting stale cron job pods deletion loop",
		"staleCronJobPodsDeletionInterval", cjpd.staleCronJobPodsDeletionInterval,
		"fieldSelectors", stalePodsFieldSelector)

	for {

		// sleep until next deletion time staleCronJobPodsDeletionInterval
		time.Sleep(*cjpd.staleCronJobPodsDeletionInterval)

		cjpd.logger.Debug("Deleting stale cron job pods")

		err := cjpd.controller.kubeClientSet.
			CoreV1().
			Pods(cjpd.controller.namespace).
			DeleteCollection(&metav1.DeleteOptions{},
				metav1.ListOptions{
					LabelSelector: "nuclio.io/function-cron-job-pod=true",
					FieldSelector: stalePodsFieldSelector,
				})
		if err != nil {
			cjpd.logger.WarnWith("Failed to delete stale cron job pods", "err", err)
		}
	}
}

// create a field selector(string) for stale pods
func (cjpd *CronJobMonitoring) compileStalePodsFieldSelector() string {
	var fieldSelectors []string

	// filter out non stale pods by their phase
	nonStalePodPhases := []v1.PodPhase{v1.PodPending, v1.PodRunning}
	for _, nonStalePodPhase := range nonStalePodPhases {
		selector := fmt.Sprintf("status.phase!=%s", string(nonStalePodPhase))
		fieldSelectors = append(fieldSelectors, selector)
	}

	return strings.Join(fieldSelectors, ",")
}
