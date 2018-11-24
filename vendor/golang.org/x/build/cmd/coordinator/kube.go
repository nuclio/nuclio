// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/build/buildlet"
	"golang.org/x/build/dashboard"
	"golang.org/x/build/internal/sourcecache"
	"golang.org/x/build/kubernetes"
	"golang.org/x/build/kubernetes/api"
	"golang.org/x/build/kubernetes/gke"
	container "google.golang.org/api/container/v1"
)

/*
This file implements the Kubernetes-based buildlet pool.
*/

// Initialized by initKube:
var (
	buildletsKubeClient *kubernetes.Client // for "buildlets" cluster
	goKubeClient        *kubernetes.Client // for "go" cluster (misc jobs)
	kubeErr             error
	registryPrefix      = "gcr.io"
	kubeCluster         *container.Cluster
)

// initGCE must be called before initKube
func initKube() error {
	if buildEnv.KubeBuild.MaxNodes == 0 {
		return errors.New("Kubernetes builders disabled due to KubeBuild.MaxNodes == 0")
	}

	// projectID was set by initGCE
	registryPrefix += "/" + buildEnv.ProjectName
	if !hasCloudPlatformScope() {
		return errors.New("coordinator not running with access to the Cloud Platform scope.")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel() // ctx is only used for discovery and connect; not retained.
	var err error
	buildletsKubeClient, err = gke.NewClient(ctx,
		buildEnv.KubeBuild.Name,
		gke.OptZone(buildEnv.Zone),
		gke.OptProject(buildEnv.ProjectName),
		gke.OptTokenSource(gcpCreds.TokenSource))
	if err != nil {
		return err
	}

	goKubeClient, err = gke.NewClient(ctx,
		buildEnv.KubeTools.Name,
		gke.OptZone(buildEnv.Zone),
		gke.OptProject(buildEnv.ProjectName),
		gke.OptTokenSource(gcpCreds.TokenSource))
	if err != nil {
		return err
	}

	sourcecache.RegisterGitMirrorDial(func(ctx context.Context) (net.Conn, error) {
		return goKubeClient.DialServicePort(ctx, "gitmirror", "")
	})

	go kubePool.pollCapacityLoop()
	return nil
}

// kubeBuildletPool is the Kubernetes buildlet pool.
type kubeBuildletPool struct {
	mu sync.Mutex // guards all following

	pods             map[string]podHistory // pod instance name -> podHistory
	clusterResources *kubeResource         // cpu and memory resources of the Kubernetes cluster
	pendingResources *kubeResource         // cpu and memory resources waiting to be scheduled
	runningResources *kubeResource         // cpu and memory resources already running (periodically updated from API)
}

var kubePool = &kubeBuildletPool{
	clusterResources: &kubeResource{
		cpu:    api.NewQuantity(0, api.DecimalSI),
		memory: api.NewQuantity(0, api.BinarySI),
	},
	pendingResources: &kubeResource{
		cpu:    api.NewQuantity(0, api.DecimalSI),
		memory: api.NewQuantity(0, api.BinarySI),
	},
	runningResources: &kubeResource{
		cpu:    api.NewQuantity(0, api.DecimalSI),
		memory: api.NewQuantity(0, api.BinarySI),
	},
}

type kubeResource struct {
	cpu    *api.Quantity
	memory *api.Quantity
}

type podHistory struct {
	requestedAt time.Time
	readyAt     time.Time
	deletedAt   time.Time
}

func (p podHistory) String() string {
	return fmt.Sprintf("requested at %v, ready at %v, deleted at %v", p.requestedAt, p.readyAt, p.deletedAt)
}

func (p *kubeBuildletPool) pollCapacityLoop() {
	ctx := context.Background()
	for {
		p.pollCapacity(ctx)
		time.Sleep(15 * time.Second)
	}
}

func (p *kubeBuildletPool) pollCapacity(ctx context.Context) {
	nodes, err := buildletsKubeClient.GetNodes(ctx)
	if err != nil {
		log.Printf("failed to retrieve nodes to calculate cluster capacity for %s/%s: %v", buildEnv.ProjectName, buildEnv.Region(), err)
		return
	}
	pods, err := buildletsKubeClient.GetPods(ctx)
	if err != nil {
		log.Printf("failed to retrieve pods to calculate cluster capacity for %s/%s: %v", buildEnv.ProjectName, buildEnv.Region(), err)
		return
	}

	p.mu.Lock()
	// Calculate the total provisioned, pending, and running CPU and memory
	// in the cluster
	provisioned := &kubeResource{
		cpu:    api.NewQuantity(0, api.DecimalSI),
		memory: api.NewQuantity(0, api.BinarySI),
	}
	running := &kubeResource{
		cpu:    api.NewQuantity(0, api.DecimalSI),
		memory: api.NewQuantity(0, api.BinarySI),
	}
	pending := &kubeResource{
		cpu:    api.NewQuantity(0, api.DecimalSI),
		memory: api.NewQuantity(0, api.BinarySI),
	}

	// Resources used by running and pending pods
	var resourceCounter *kubeResource
	for _, pod := range pods {
		switch pod.Status.Phase {
		case api.PodPending:
			resourceCounter = pending
		case api.PodRunning:
			resourceCounter = running
		case api.PodSucceeded:
			// TODO(bradfitz,evanbrown): this was spamming
			// logs a lot. Don't count these resources, I
			// assume. We weren't before (when the
			// log.Printf below was firing) anyway.
			// TODO: clean these in cleanupOldPods once they're
			// over a certain age (few hours?). why aren't they already?
			continue
		case api.PodFailed:
			// These were also spamming logs.
			// TODO: clean these in cleanupOldPods once they're
			// over a certain age (few days?).
			continue
		default:
			log.Printf("Pod %s in unknown state (%q); ignoring", pod.ObjectMeta.Name, pod.Status.Phase)
			continue
		}
		for _, c := range pod.Spec.Containers {
			// The Kubernetes API rarely, but can, return a response
			// with an empty Requests map. Check to be sure...
			if _, ok := c.Resources.Requests[api.ResourceCPU]; ok {
				resourceCounter.cpu.Add(c.Resources.Requests[api.ResourceCPU])
			}
			if _, ok := c.Resources.Requests[api.ResourceMemory]; ok {
				resourceCounter.memory.Add(c.Resources.Requests[api.ResourceMemory])
			}
		}
	}
	p.runningResources = running
	p.pendingResources = pending

	// Resources provisioned to the cluster
	for _, n := range nodes {
		provisioned.cpu.Add(n.Status.Capacity[api.ResourceCPU])
		provisioned.memory.Add(n.Status.Capacity[api.ResourceMemory])
	}
	p.clusterResources = provisioned
	p.mu.Unlock()

}

func (p *kubeBuildletPool) HasCapacity(hostType string) bool {
	// TODO: implement. But for now we don't care because we only
	// use the kubePool for the cross-compiled builds and we have
	// very few hostTypes for those, and only one (ARM) that's
	// used day-to-day. So it's okay if we lie here and always try
	// to create buildlets. The scheduler will still give created
	// buildlets to the highest priority waiter.
	return true
}

func (p *kubeBuildletPool) GetBuildlet(ctx context.Context, hostType string, lg logger) (*buildlet.Client, error) {
	hconf, ok := dashboard.Hosts[hostType]
	if !ok || !hconf.IsContainer() {
		return nil, fmt.Errorf("kubepool: invalid host type %q", hostType)
	}
	if kubeErr != nil {
		return nil, kubeErr
	}
	if buildletsKubeClient == nil {
		panic("expect non-nil buildletsKubeClient")
	}

	deleteIn, ok := ctx.Value(buildletTimeoutOpt{}).(time.Duration)
	if !ok {
		deleteIn = podDeleteTimeout
	}

	podName := "buildlet-" + strings.TrimPrefix(hostType, "host-") + "-rn" + randHex(7)

	// Get an estimate for when the pod will be started/running and set
	// the context timeout based on that
	var needDelete bool

	lg.LogEventTime("creating_kube_pod", podName)
	log.Printf("Creating Kubernetes pod %q for %s", podName, hostType)

	bc, err := buildlet.StartPod(ctx, buildletsKubeClient, podName, hostType, buildlet.PodOpts{
		ProjectID:     buildEnv.ProjectName,
		ImageRegistry: registryPrefix,
		Description:   fmt.Sprintf("Go Builder for %s", hostType),
		DeleteIn:      deleteIn,
		OnPodCreating: func() {
			lg.LogEventTime("pod_creating")
			p.setPodUsed(podName, true)
			p.updatePodHistory(podName, podHistory{requestedAt: time.Now()})
			needDelete = true
		},
		OnPodCreated: func() {
			lg.LogEventTime("pod_created")
			p.updatePodHistory(podName, podHistory{readyAt: time.Now()})
		},
		OnGotPodInfo: func() {
			lg.LogEventTime("got_pod_info", "waiting_for_buildlet...")
		},
	})
	if err != nil {
		lg.LogEventTime("kube_buildlet_create_failure", fmt.Sprintf("%s: %v", podName, err))

		if needDelete {
			log.Printf("Deleting failed pod %q", podName)
			if err := buildletsKubeClient.DeletePod(context.Background(), podName); err != nil {
				log.Printf("Error deleting pod %q: %v", podName, err)
			}
			p.setPodUsed(podName, false)
		}
		return nil, err
	}

	bc.SetDescription("Kube Pod: " + podName)

	// The build's context will be canceled when the build completes (successfully
	// or not), or if the buildlet becomes unavailable. In any case, delete the pod
	// running the buildlet.
	go func() {
		<-ctx.Done()
		log.Printf("Deleting pod %q after build context completed", podName)
		// Giving DeletePod a new context here as the build ctx has been canceled
		buildletsKubeClient.DeletePod(context.Background(), podName)
		p.setPodUsed(podName, false)
	}()

	return bc, nil
}

func (p *kubeBuildletPool) WriteHTMLStatus(w io.Writer) {
	fmt.Fprintf(w, "<b>Kubernetes pool</b> capacity: %s", p.capacityString())
	const show = 6 // must be even
	active := p.podsActive()
	if len(active) > 0 {
		fmt.Fprintf(w, "<ul>")
		for i, pod := range active {
			if i < show/2 || i >= len(active)-(show/2) {
				fmt.Fprintf(w, "<li>%v, %v</li>\n", pod.name, time.Since(pod.creation))
			} else if i == show/2 {
				fmt.Fprintf(w, "<li>... %d of %d total omitted ...</li>\n", len(active)-show, len(active))
			}
		}
		fmt.Fprintf(w, "</ul>")
	}
}

func (p *kubeBuildletPool) capacityString() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return fmt.Sprintf("<ul><li>%v CPUs running, %v CPUs pending, %v total CPUs in cluster</li><li>%v memory running, %v memory pending, %v total memory in cluster</li></ul>",
		p.runningResources.cpu, p.pendingResources.cpu, p.clusterResources.cpu,
		p.runningResources.memory, p.pendingResources.memory, p.clusterResources.memory)
}

func (p *kubeBuildletPool) setPodUsed(podName string, used bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.pods == nil {
		p.pods = make(map[string]podHistory)
	}
	if used {
		p.pods[podName] = podHistory{requestedAt: time.Now()}

	} else {
		p.pods[podName] = podHistory{deletedAt: time.Now()}
		// TODO(evanbrown): log this podHistory data for analytics purposes before deleting
		delete(p.pods, podName)
	}
}

func (p *kubeBuildletPool) updatePodHistory(podName string, updatedHistory podHistory) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	ph, ok := p.pods[podName]
	if !ok {
		return fmt.Errorf("pod %q does not exist", podName)
	}

	if !updatedHistory.readyAt.IsZero() {
		ph.readyAt = updatedHistory.readyAt
	}
	if !updatedHistory.requestedAt.IsZero() {
		ph.requestedAt = updatedHistory.requestedAt
	}
	if !updatedHistory.deletedAt.IsZero() {
		ph.deletedAt = updatedHistory.deletedAt
	}
	p.pods[podName] = ph
	return nil
}

func (p *kubeBuildletPool) podUsed(podName string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	_, ok := p.pods[podName]
	return ok
}

func (p *kubeBuildletPool) podsActive() (ret []resourceTime) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for name, ph := range p.pods {
		ret = append(ret, resourceTime{
			name:     name,
			creation: ph.requestedAt,
		})
	}
	sort.Sort(byCreationTime(ret))
	return ret
}

func (p *kubeBuildletPool) String() string {
	p.mu.Lock()
	inUse := 0
	total := 0
	// ...
	p.mu.Unlock()
	return fmt.Sprintf("Kubernetes pool capacity: %d/%d", inUse, total)
}

// cleanUpOldPods loops forever and periodically enumerates pods
// and deletes those which have expired.
//
// A Pod is considered expired if it has a "delete-at" metadata
// attribute having a unix timestamp before the current time.
//
// This is the safety mechanism to delete pods which stray from the
// normal deleting process. Pods are created to run a single build and
// should be shut down by a controlling process. Due to various types
// of failures, they might get stranded. To prevent them from getting
// stranded and wasting resources forever, we instead set the
// "delete-at" metadata attribute on them when created to some time
// that's well beyond their expected lifetime.
func (p *kubeBuildletPool) cleanUpOldPodsLoop(ctx context.Context) {
	if buildletsKubeClient == nil {
		log.Printf("cleanUpOldPods: no buildletsKubeClient configured; aborting.")
		return
	}
	for {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		p.cleanUpOldPods(ctx)
		cancel()
		time.Sleep(time.Minute)
	}
}

func (p *kubeBuildletPool) cleanUpOldPods(ctx context.Context) {
	pods, err := buildletsKubeClient.GetPods(ctx)
	if err != nil {
		log.Printf("cleanUpOldPods: error getting pods: %v", err)
		return
	}
	var stats struct {
		Pods          int
		WithAttr      int
		WithDelete    int
		DeletedOld    int // even if failed to delete
		StillUsed     int
		DeletedOldGen int // even if failed to delete
	}
	for _, pod := range pods {
		if pod.ObjectMeta.Annotations == nil {
			// Defensive. Not seen in practice.
			continue
		}
		stats.Pods++
		sawDeleteAt := false
		stats.WithAttr++
		for k, v := range pod.ObjectMeta.Annotations {
			if k == "delete-at" {
				stats.WithDelete++
				sawDeleteAt = true
				if v == "" {
					log.Printf("cleanUpOldPods: missing delete-at value; ignoring")
					continue
				}
				unixDeadline, err := strconv.ParseInt(v, 10, 64)
				if err != nil {
					log.Printf("cleanUpOldPods: invalid delete-at value %q seen; ignoring", v)
				}
				if err == nil && time.Now().Unix() > unixDeadline {
					stats.DeletedOld++
					log.Printf("cleanUpOldPods: Deleting expired pod %q in zone %q ...", pod.Name, buildEnv.Zone)
					err = buildletsKubeClient.DeletePod(ctx, pod.Name)
					if err != nil {
						log.Printf("cleanUpOldPods: problem deleting old pod %q: %v", pod.Name, err)
					}
				}
			}
		}
		// Delete buildlets (things we made) from previous
		// generations. Only deleting things starting with "buildlet-"
		// is a historical restriction, but still fine for paranoia.
		if sawDeleteAt && strings.HasPrefix(pod.Name, "buildlet-") {
			if p.podUsed(pod.Name) {
				stats.StillUsed++
			} else {
				stats.DeletedOldGen++
				log.Printf("cleanUpOldPods: deleting pod %q from an earlier coordinator generation ...", pod.Name)
				err = buildletsKubeClient.DeletePod(ctx, pod.Name)
				if err != nil {
					log.Printf("cleanUpOldPods: problem deleting pod: %v", err)
				}
			}
		}
	}
	if stats.Pods > 0 {
		log.Printf("cleanUpOldPods: loop stats: %+v", stats)
	}
}

func hasCloudPlatformScope() bool {
	return hasScope(container.CloudPlatformScope)
}
