/*
Copyright 2023 The Nuclio Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"
	"runtime/debug"
	"time"

	"github.com/nuclio/nuclio/pkg/common"

	"github.com/nuclio/logger"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type CronJobMonitoring struct {
	logger                               logger.Logger
	controller                           *Controller
	cronJobStaleResourcesCleanupInterval *time.Duration
	stopChan                             chan struct{}
}

func NewCronJobMonitoring(ctx context.Context,
	parentLogger logger.Logger,
	controller *Controller,
	cronJobStaleResourcesCleanupInterval *time.Duration) *CronJobMonitoring {

	loggerInstance := parentLogger.GetChild("cron_job_monitoring")

	newCronJobMonitoring := &CronJobMonitoring{
		logger:                               loggerInstance,
		controller:                           controller,
		cronJobStaleResourcesCleanupInterval: cronJobStaleResourcesCleanupInterval,
	}

	parentLogger.DebugWithCtx(ctx, "Successfully created cron job monitoring instance",
		"cronJobStaleResourcesCleanupInterval", cronJobStaleResourcesCleanupInterval)

	return newCronJobMonitoring
}

func (cjm *CronJobMonitoring) start(ctx context.Context) {

	// create stop channel
	cjm.stopChan = make(chan struct{}, 1)

	// spawn a goroutine for cronjob monitoring
	go func() {
		defer func() {
			if err := recover(); err != nil {
				callStack := debug.Stack()
				cjm.logger.ErrorWithCtx(ctx, "Panic caught while monitoring cronjobs",
					"err", err,
					"stack", string(callStack))
			}
		}()
		stalePodsFieldSelector := common.CompileStalePodsFieldSelector()
		cjm.logger.InfoWithCtx(ctx, "Starting cron job stale resources cleanup loop",
			"cronJobStaleResourcesCleanupInterval", cjm.cronJobStaleResourcesCleanupInterval,
			"fieldSelectors", stalePodsFieldSelector)
		for {
			select {
			case <-time.After(*cjm.cronJobStaleResourcesCleanupInterval):

				// cleanup all cron job related stale resources (as k8s lacks this logic)
				cjm.deleteStaleJobs(ctx)
				cjm.deleteStalePods(ctx, stalePodsFieldSelector)

			case <-cjm.stopChan:
				cjm.logger.DebugCtx(ctx, "Stopped cronjob monitoring")
				return
			}
		}
	}()
}

func (cjm *CronJobMonitoring) stop(ctx context.Context) {
	cjm.logger.InfoCtx(ctx, "Stopping cron job monitoring")

	// post to channel
	if cjm.stopChan != nil {
		cjm.stopChan <- struct{}{}
	}
}

func (cjm *CronJobMonitoring) deleteStalePods(ctx context.Context, stalePodsFieldSelector string) {
	err := cjm.controller.kubeClientSet.
		CoreV1().
		Pods(cjm.controller.namespace).
		DeleteCollection(ctx, metav1.DeleteOptions{},
			metav1.ListOptions{
				LabelSelector: fmt.Sprintf("%s=true", common.NuclioLabelKeyFunctionCronJobPod),
				FieldSelector: stalePodsFieldSelector,
			})
	if err != nil {
		cjm.logger.WarnWithCtx(ctx, "Failed to delete stale cron-job pods",
			"namespace", cjm.controller.namespace,
			"err", err)
	}
}

func (cjm *CronJobMonitoring) deleteStaleJobs(ctx context.Context) {
	jobs, err := cjm.controller.kubeClientSet.
		BatchV1().
		Jobs(cjm.controller.namespace).
		List(ctx, metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=true", common.NuclioLabelKeyFunctionCronJobPod),
		})
	if err != nil {
		cjm.logger.WarnWithCtx(ctx, "Failed to list cron-job jobs",
			"namespace", cjm.controller.namespace,
			"err", err)
	}

	for _, job := range jobs.Items {

		// check if the job is stale - a k8s bug that happens when a job fails more times than its backoff limit
		// whenever this happens, the job will not be automatically deleted
		isJobBackOffLimitExceeded := job.Spec.BackoffLimit != nil &&
			*job.Spec.BackoffLimit <= job.Status.Failed

		isJobCompleted := job.Status.Succeeded > 0

		deleteForegroundPolicy := metav1.DeletePropagationBackground
		if isJobBackOffLimitExceeded || isJobCompleted {
			err := cjm.controller.kubeClientSet.
				BatchV1().
				Jobs(cjm.controller.namespace).
				Delete(ctx, job.Name, metav1.DeleteOptions{
					PropagationPolicy: &deleteForegroundPolicy,
				})
			if err != nil && !apierrors.IsNotFound(err) {
				cjm.logger.WarnWithCtx(ctx, "Failed to delete cron-job job",
					"name", job.Name,
					"namespace", job.Namespace,
					"err", err)
			}
		}
	}
}
