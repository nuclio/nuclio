package controller

import (
	"fmt"
	"runtime/debug"
	"strings"
	"time"

	"github.com/nuclio/logger"
	"k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type CronJobMonitoring struct {
	logger                               logger.Logger
	controller                           *Controller
	cronJobStaleResourcesCleanupInterval *time.Duration
	stopChan                             chan struct{}
}

func NewCronJobMonitoring(parentLogger logger.Logger,
	controller *Controller,
	cronJobStaleResourcesCleanupInterval *time.Duration) *CronJobMonitoring {

	loggerInstance := parentLogger.GetChild("cron_job_monitoring")

	newCronJobMonitoring := &CronJobMonitoring{
		logger:                               loggerInstance,
		controller:                           controller,
		cronJobStaleResourcesCleanupInterval: cronJobStaleResourcesCleanupInterval,
	}

	parentLogger.DebugWith("Successfully created cron job monitoring instance",
		"cronJobStaleResourcesCleanupInterval", cronJobStaleResourcesCleanupInterval)

	return newCronJobMonitoring
}

func (cjm *CronJobMonitoring) start() {

	// create stop channel
	cjm.stopChan = make(chan struct{}, 1)

	// spawn a goroutine for cronjob monitoring
	go func() {
		defer func() {
			if err := recover(); err != nil {
				callStack := debug.Stack()
				cjm.logger.ErrorWith("Panic caught while monitoring cronjobs",
					"err", err,
					"stack", string(callStack))
			}
		}()
		stalePodsFieldSelector := cjm.compileStalePodsFieldSelector()
		cjm.logger.InfoWith("Starting cron job stale resources cleanup loop",
			"cronJobStaleResourcesCleanupInterval", cjm.cronJobStaleResourcesCleanupInterval,
			"fieldSelectors", stalePodsFieldSelector)
		for {
			select {
			case <-time.After(*cjm.cronJobStaleResourcesCleanupInterval):

				// cleanup all cron job related stale resources (as k8s lacks this logic)
				cjm.deleteStaleJobs()
				cjm.deleteStalePods(stalePodsFieldSelector)

			case <-cjm.stopChan:
				cjm.logger.Debug("Stopped cronjob monitoring")
				return
			}
		}
	}()
}

func (cjm *CronJobMonitoring) stop() {
	cjm.logger.Info("Stopping cron job monitoring")

	// post to channel
	if cjm.stopChan != nil {
		cjm.stopChan <- struct{}{}
	}
}

func (cjm *CronJobMonitoring) deleteStalePods(stalePodsFieldSelector string) {
	err := cjm.controller.kubeClientSet.
		CoreV1().
		Pods(cjm.controller.namespace).
		DeleteCollection(&metav1.DeleteOptions{},
			metav1.ListOptions{
				LabelSelector: "nuclio.io/function-cron-job-pod=true",
				FieldSelector: stalePodsFieldSelector,
			})
	if err != nil {
		cjm.logger.WarnWith("Failed to delete stale cron-job pods",
			"namespace", cjm.controller.namespace,
			"err", err)
	}
}

func (cjm *CronJobMonitoring) deleteStaleJobs() {
	jobs, err := cjm.controller.kubeClientSet.
		BatchV1().
		Jobs(cjm.controller.namespace).
		List(metav1.ListOptions{
			LabelSelector: "nuclio.io/function-cron-job-pod=true",
		})
	if err != nil {
		cjm.logger.WarnWith("Failed to list cron-job jobs",
			"namespace", cjm.controller.namespace,
			"err", err)
	}

	for _, job := range jobs.Items {

		// check if the job is stale - a k8s bug that happens when a job fails more times than its backoff limit
		// whenever this happens, the job will not be automatically deleted
		isJobBackOffLimitExceeded := job.Spec.BackoffLimit != nil &&
			*job.Spec.BackoffLimit <= job.Status.Failed

		isJobCompleted := job.Status.Succeeded > 0

		if isJobBackOffLimitExceeded || isJobCompleted {
			err := cjm.controller.kubeClientSet.
				BatchV1().
				Jobs(cjm.controller.namespace).
				Delete(job.Name, &metav1.DeleteOptions{})
			if err != nil && !apierrors.IsNotFound(err) {
				cjm.logger.WarnWith("Failed to delete cron-job job",
					"name", job.Name,
					"namespace", job.Namespace,
					"err", err)
			}
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
