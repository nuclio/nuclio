package controller

import (
	"fmt"
	"strings"
	"time"

	"github.com/nuclio/logger"
	"k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type cronJobMonitoring struct {
	logger                           logger.Logger
	controller                       *Controller
	staleCronJobPodsDeletionInterval *time.Duration
}

func NewCronJobMonitoring(parentLogger logger.Logger,
	controller *Controller,
	staleCronJobPodsDeletionInterval *time.Duration) (*cronJobMonitoring, error) {

	loggerInstance := parentLogger.GetChild("cron_job_monitoring")

	newCronJobMonitoring := &cronJobMonitoring{
		logger:                           loggerInstance,
		controller:                       controller,
		staleCronJobPodsDeletionInterval: staleCronJobPodsDeletionInterval,
	}

	parentLogger.DebugWith("Successfully created cron job monitoring instance",
		"staleCronJobPodsDeletionInterval", staleCronJobPodsDeletionInterval)

	return newCronJobMonitoring, nil
}

func (cjpd *cronJobMonitoring) start() error {

	go cjpd.startStaleCronJobPodsDeletionLoop() // nolint: errcheck

	return nil
}

// delete all stale cron job pods (as k8s lacks this logic, and CronJob pods are never deleted)
func (cjpd *cronJobMonitoring) startStaleCronJobPodsDeletionLoop() error {
	stalePodsFieldSelector := cjpd.compileStalePodsFieldSelector()

	cjpd.logger.InfoWith("Starting stale cron job pods deletion loop",
		"staleCronJobPodsDeletionInterval", cjpd.staleCronJobPodsDeletionInterval,
		"fieldSelectors", stalePodsFieldSelector)

	for {

		// sleep until next deletion time staleCronJobPodsDeletionInterval
		time.Sleep(*cjpd.staleCronJobPodsDeletionInterval)

		err := cjpd.controller.kubeClientSet.
			CoreV1().
			Pods(cjpd.controller.namespace).
			DeleteCollection(&meta_v1.DeleteOptions{},
				meta_v1.ListOptions{
					LabelSelector: "nuclio.io/function-cron-job-pod=true",
					FieldSelector: stalePodsFieldSelector,
				})
		if err != nil {
			cjpd.logger.WarnWith("Failed to delete stale cron job pods", "err", err)
		}
	}
}

// create a field selector(string) for stale pods
func (cjpd *cronJobMonitoring) compileStalePodsFieldSelector() string {
	var fieldSelectors []string

	// filter out non stale pods by their phase
	nonStalePodPhases := []v1.PodPhase{v1.PodPending, v1.PodRunning}
	for _, nonStalePodPhase := range nonStalePodPhases {
		selector := fmt.Sprintf("status.phase!=%s", string(nonStalePodPhase))
		fieldSelectors = append(fieldSelectors, selector)
	}

	return strings.Join(fieldSelectors, ",")
}
