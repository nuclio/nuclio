package controller

import (
	"fmt"
	"strings"
	"time"

	"github.com/nuclio/logger"
	"k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type cronJobStalePodsDeleter struct {
	logger           logger.Logger
	controller       *Controller
	interval *time.Duration
}

func NewCronJobStalePodsDeleter(parentLogger logger.Logger,
	controller *Controller,
	interval *time.Duration) (*cronJobStalePodsDeleter, error) {

	loggerInstance := parentLogger.GetChild("cron_job_stale_pods_deleter")

	newCronJobStalePodsDeleter := &cronJobStalePodsDeleter{
		logger:           loggerInstance,
		controller:       controller,
		interval: interval,
	}

	parentLogger.DebugWith("Successfuly created cron job stale pods deleter instance",
		"interval", interval)

	return newCronJobStalePodsDeleter, nil
}

// run cron job stale pods deletion loop (as k8s lacks this logic, CronJobs are never deleted)
func (cjpd *cronJobStalePodsDeleter) start() error {
	go cjpd.startStaleCronJobPodsDeletionLoop() // nolint: errcheck

	return nil
}

// delete all stale cron job pods - identify by status.phase of the cron job pods
func (cjpd *cronJobStalePodsDeleter) startStaleCronJobPodsDeletionLoop() error {
	var fieldSelectors []string

	// prepare field selectors - filter out non stale pods
	nonStalePodPhases := []v1.PodPhase{v1.PodPending, v1.PodRunning}
	for _, nonStalePodPhase := range nonStalePodPhases {
		selector := fmt.Sprintf("status.phase!=%s", string(nonStalePodPhase))
		fieldSelectors = append(fieldSelectors, selector)
	}

	cjpd.logger.InfoWith("Starting stale cron job pods deletion loop",
		"interval", cjpd.interval,
		"fieldSelectors", fieldSelectors)
	for {

		// sleep until next deletion time interval
		time.Sleep(*cjpd.interval)

		err := cjpd.controller.kubeClientSet.
			CoreV1().
			Pods(cjpd.controller.namespace).
			DeleteCollection(&meta_v1.DeleteOptions{},
				meta_v1.ListOptions{
					LabelSelector: "nuclio.io/function-cron-job-pod=true",
					FieldSelector: strings.Join(fieldSelectors, ","),
				})
		if err != nil {
			cjpd.logger.WarnWith("Failed to delete stale cron job pods", "err", err)
		}
	}
}
