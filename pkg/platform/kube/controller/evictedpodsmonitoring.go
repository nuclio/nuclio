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
	"strings"
	"time"

	"github.com/nuclio/nuclio/pkg/common"

	"github.com/nuclio/logger"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type EvictedPodsMonitoring struct {
	logger                     logger.Logger
	controller                 *Controller
	evictedPodsCleanupInterval *time.Duration
	podNamesToDelete           []string
	stopChan                   chan struct{}
}

func NewEvictedPodsMonitoring(ctx context.Context,
	parentLogger logger.Logger,
	controller *Controller,
	evictedPodsCleanupInterval *time.Duration) *EvictedPodsMonitoring {

	loggerInstance := parentLogger.GetChild("cron_job_monitoring")

	newEvictedPodsMonitoring := &EvictedPodsMonitoring{
		logger:                     loggerInstance,
		controller:                 controller,
		evictedPodsCleanupInterval: evictedPodsCleanupInterval,
	}

	parentLogger.DebugWithCtx(ctx,
		"Successfully created evicted pods monitoring instance",
		"evictedPodsCleanupInterval", evictedPodsCleanupInterval)

	return newEvictedPodsMonitoring
}

func (epm *EvictedPodsMonitoring) start(ctx context.Context) {

	if epm.evictedPodsCleanupInterval == nil || *epm.evictedPodsCleanupInterval == 0*time.Second {
		epm.logger.DebugWithCtx(ctx, "Evicted pods cleanup is disabled")
		return
	}

	// create stop channel
	epm.stopChan = make(chan struct{}, 1)

	// start a go routine that will periodically clean up stale resources
	go epm.cleanupEvictedPods(ctx)
}

func (epm *EvictedPodsMonitoring) stop(ctx context.Context) {
	epm.logger.DebugWithCtx(ctx, "Stopping evicted pods monitoring")

	if epm.stopChan != nil {
		epm.stopChan <- struct{}{}
	}
}

func (epm *EvictedPodsMonitoring) cleanupEvictedPods(ctx context.Context) {
	epm.logger.DebugWithCtx(ctx, "Starting evicted pods cleanup")

	// run forever
	for {
		select {
		case <-epm.stopChan:
			epm.logger.DebugWithCtx(ctx, "Stopping evicted pods cleanup")

			// remove all pods that were marked for deletion
			epm.podNamesToDelete = nil
			return

		case <-time.After(*epm.evictedPodsCleanupInterval):

			// get all failed function pods
			stalePodsFieldSelector := common.CompileStalePodsFieldSelector()
			pods, err := epm.controller.kubeClientSet.CoreV1().Pods(epm.controller.namespace).List(ctx, metav1.ListOptions{
				LabelSelector: "nuclio.io/class=function",
				FieldSelector: stalePodsFieldSelector,
			})
			if err != nil {
				epm.logger.WarnWithCtx(ctx, "Failed to list pods", "err", err)
				continue
			}

			var deletedPodNames []string

			// iterate over pods
			for _, pod := range pods.Items {

				if strings.Contains(pod.Status.Reason, "Evicted") {

					// evicted pod was found in the last interval, delete it
					if common.StringSliceContainsString(epm.podNamesToDelete, pod.Name) {
						deletedPodNames = append(deletedPodNames, pod.Name)
						epm.logger.DebugWithCtx(ctx, "Deleting evicted pod", "podName", pod.Name)

						// delete pod
						err := epm.controller.kubeClientSet.CoreV1().Pods(pod.Namespace).Delete(ctx, pod.Name, metav1.DeleteOptions{})
						if err != nil && !apierrors.IsNotFound(err) {
							epm.logger.WarnWithCtx(ctx, "Failed to delete pod",
								"podName", pod.Name,
								"err", err)
						}
					} else {

						// a newly-evicted pod, add it to the list of pods to delete
						// this is a hack to make sure the pods are evicted for longer than the interval
						epm.podNamesToDelete = append(epm.podNamesToDelete, pod.Name)
					}
				}
			}

			// remove deleted pods from the list of pods to delete
			epm.podNamesToDelete = common.RemoveStringSliceItemsFromStringSlice(epm.podNamesToDelete, deletedPodNames)
		}
	}
}
