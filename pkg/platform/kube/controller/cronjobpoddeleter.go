package controller

import (
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"time"

	"github.com/nuclio/logger"

	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type cronJobStalePodsDeleter struct {
	logger           logger.Logger
	controller       *Controller
	deletionInterval *time.Duration
}

func newCronJobStalePodsDeleter(parentLogger logger.Logger,
	controller *Controller,
	deletionInterval *time.Duration) (*cronJobStalePodsDeleter, error) {

	loggerInstance := parentLogger.GetChild("cron_job_stale_pods_deleter")

	newCronJobStalePodsDeleter := &cronJobStalePodsDeleter{
		logger:           loggerInstance,
		controller:       controller,
		deletionInterval: deletionInterval,
	}

	parentLogger.DebugWith("Created cron job stale pods deleter", "deletionInterval", deletionInterval)

	return newCronJobStalePodsDeleter, nil
}

func (cjpd *cronJobStalePodsDeleter) start() error {
	go cjpd.startStaleCronJobPodsDeletionLoop() // nolint: errcheck

	return nil
}

// delete all stale cron job pods - identify by status.phase of the cron job pods
func (cjpd *cronJobStalePodsDeleter) startStaleCronJobPodsDeletionLoop() error {
	var fieldSelectors []fields.Selector

	// prepare field selectors
	stalePodPhases := []v1.PodPhase{v1.PodFailed, v1.PodSucceeded, v1.PodUnknown}
	for stalePodPhase := range stalePodPhases {
		selector := fields.OneTermEqualSelector("status.phase", string(stalePodPhase))
		fieldSelectors = append(fieldSelectors, selector)
	}

	cjpd.logger.InfoWith("Starting stale cron job pods deletion loop",
		"deletionInterval", cjpd.deletionInterval)
	for {

		// sleep until next deletion time interval
		time.Sleep(*cjpd.deletionInterval)

		err := cjpd.controller.kubeClientSet.CoreV1().Pods(cjpd.controller.namespace).DeleteCollection(&meta_v1.DeleteOptions{},
			meta_v1.ListOptions{
				LabelSelector: "nuclio.io/function-cron-job-pod=true",
				FieldSelector: fields.AndSelectors(fieldSelectors...).String(),
			})
		if err != nil {
			cjpd.logger.WarnWith("Failed to delete stale cron job pods", "err", err)
		}
	}
}
