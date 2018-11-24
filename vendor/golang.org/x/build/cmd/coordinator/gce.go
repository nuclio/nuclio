// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Code interacting with Google Compute Engine (GCE) and
// a GCE implementation of the BuildletPool interface.

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/compute/metadata"
	"cloud.google.com/go/datastore"
	"cloud.google.com/go/errorreporting"
	monapi "cloud.google.com/go/monitoring/apiv3"
	"cloud.google.com/go/storage"
	"golang.org/x/build/buildenv"
	"golang.org/x/build/buildlet"
	"golang.org/x/build/cmd/coordinator/spanlog"
	"golang.org/x/build/dashboard"
	"golang.org/x/build/gerrit"
	"golang.org/x/build/internal/buildstats"
	"golang.org/x/build/internal/lru"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	compute "google.golang.org/api/compute/v1"
	"google.golang.org/api/googleapi"
)

func init() {
	buildlet.GCEGate = gceAPIGate
}

// apiCallTicker ticks regularly, preventing us from accidentally making
// GCE API calls too quickly. Our quota is 20 QPS, but we temporarily
// limit ourselves to less than that.
var apiCallTicker = time.NewTicker(time.Second / 10)

func gceAPIGate() {
	<-apiCallTicker.C
}

// Initialized by initGCE:
var (
	buildEnv *buildenv.Environment

	dsClient       *datastore.Client
	computeService *compute.Service
	gcpCreds       *google.Credentials
	errTryDeps     error // non-nil if try bots are disabled
	gerritClient   *gerrit.Client
	storageClient  *storage.Client
	metricsClient  *monapi.MetricClient
	inStaging      bool                   // are we running in the staging project? (named -dev)
	errorsClient   *errorreporting.Client // Stackdriver errors client

	initGCECalled bool
)

// oAuthHTTPClient is the OAuth2 HTTP client used to make API calls to Google Cloud APIs.
// It is initialized by initGCE.
var oAuthHTTPClient *http.Client

func initGCE() error {
	initGCECalled = true
	var err error

	// If the coordinator is running on a GCE instance and a
	// buildEnv was not specified with the env flag, set the
	// buildEnvName to the project ID
	if *buildEnvName == "" {
		if *mode == "dev" {
			*buildEnvName = "dev"
		} else if metadata.OnGCE() {
			*buildEnvName, err = metadata.ProjectID()
			if err != nil {
				log.Fatalf("metadata.ProjectID: %v", err)
			}
		}
	}

	buildEnv = buildenv.ByProjectID(*buildEnvName)
	inStaging = (buildEnv == buildenv.Staging)

	// If running on GCE, override the zone and static IP, and check service account permissions.
	if metadata.OnGCE() {
		projectZone, err := metadata.Get("instance/zone")
		if err != nil || projectZone == "" {
			return fmt.Errorf("failed to get current GCE zone: %v", err)
		}
		// Convert the zone from "projects/1234/zones/us-central1-a" to "us-central1-a".
		projectZone = path.Base(projectZone)
		buildEnv.Zone = projectZone

		if buildEnv.StaticIP == "" {
			buildEnv.StaticIP, err = metadata.ExternalIP()
			if err != nil {
				return fmt.Errorf("ExternalIP: %v", err)
			}
		}

		if !hasComputeScope() {
			return errors.New("coordinator is not running with access to read and write Compute resources. VM support disabled")
		}

		if value, err := metadata.ProjectAttributeValue("farmer-run-bench"); err == nil {
			*shouldRunBench, _ = strconv.ParseBool(value)
		}
	}

	cfgDump, _ := json.MarshalIndent(buildEnv, "", "  ")
	log.Printf("Loaded configuration %q for project %q:\n%s", *buildEnvName, buildEnv.ProjectName, cfgDump)

	ctx := context.Background()
	if *mode != "dev" {
		storageClient, err = storage.NewClient(ctx)
		if err != nil {
			log.Fatalf("storage.NewClient: %v", err)
		}

		metricsClient, err = monapi.NewMetricClient(ctx)
		if err != nil {
			log.Fatalf("monapi.NewMetricClient: %v", err)
		}
	}

	dsClient, err = datastore.NewClient(ctx, buildEnv.ProjectName)
	if err != nil {
		if *mode == "dev" {
			log.Printf("Error creating datastore client: %v", err)
		} else {
			log.Fatalf("Error creating datastore client: %v", err)
		}
	}

	// don't send dev errors to Stackdriver.
	if *mode != "dev" {
		errorsClient, err = errorreporting.NewClient(ctx, buildEnv.ProjectName, errorreporting.Config{
			ServiceName: "coordinator",
		})
		if err != nil {
			// don't exit, we still want to run coordinator
			log.Printf("Error creating errors client: %v", err)
		}
	}

	gcpCreds, err = buildEnv.Credentials(ctx)
	if err != nil {
		if *mode == "dev" {
			// don't try to do anything else with GCE, as it will likely fail
			return nil
		}
		log.Fatalf("failed to get a token source: %v", err)
	}
	oAuthHTTPClient = oauth2.NewClient(ctx, gcpCreds.TokenSource)
	computeService, _ = compute.New(oAuthHTTPClient)
	errTryDeps = checkTryBuildDeps()
	if errTryDeps != nil {
		log.Printf("TryBot builders disabled due to error: %v", errTryDeps)
	} else {
		log.Printf("TryBot builders enabled.")
	}

	if *mode != "dev" {
		go syncBuildStatsLoop(buildEnv)
	}

	go gcePool.pollQuotaLoop()
	return nil
}

func checkTryBuildDeps() error {
	if !hasStorageScope() {
		return errors.New("coordinator's GCE instance lacks the storage service scope")
	}
	if *mode == "dev" {
		return errors.New("running in dev mode")
	}
	wr := storageClient.Bucket(buildEnv.LogBucket).Object("hello.txt").NewWriter(context.Background())
	fmt.Fprintf(wr, "Hello, world! Coordinator start-up at %v", time.Now())
	if err := wr.Close(); err != nil {
		return fmt.Errorf("test write of a GCS object to bucket %q failed: %v", buildEnv.LogBucket, err)
	}
	if inStaging {
		// Don't expect to write to Gerrit in staging mode.
		gerritClient = gerrit.NewClient("https://go-review.googlesource.com", gerrit.NoAuth)
	} else {
		gobotPass, err := metadata.ProjectAttributeValue("gobot-password")
		if err != nil {
			return fmt.Errorf("failed to get project metadata 'gobot-password': %v", err)
		}
		gerritClient = gerrit.NewClient("https://go-review.googlesource.com",
			gerrit.BasicAuth("git-gobot.golang.org", strings.TrimSpace(string(gobotPass))))
	}

	return nil
}

var gcePool = &gceBuildletPool{}

var _ BuildletPool = (*gceBuildletPool)(nil)

// maxInstances is a temporary hack because we can't get buildlets to boot
// without IPs, and we only have 200 IP addresses.
// TODO(bradfitz): remove this once fixed.
const maxInstances = 190

type gceBuildletPool struct {
	mu sync.Mutex // guards all following

	disabled bool

	// CPU quota usage & limits.
	cpuLeft   int // dead-reckoning CPUs remain
	instLeft  int // dead-reckoning instances remain
	instUsage int
	cpuUsage  int
	addrUsage int
	inst      map[string]time.Time // GCE VM instance name -> creationTime
}

func (p *gceBuildletPool) pollQuotaLoop() {
	if computeService == nil {
		log.Printf("pollQuotaLoop: no GCE access; not checking quota.")
		return
	}
	if buildEnv.ProjectName == "" {
		log.Printf("pollQuotaLoop: no GCE project name configured; not checking quota.")
		return
	}
	for {
		p.pollQuota()
		time.Sleep(5 * time.Second)
	}
}

func (p *gceBuildletPool) pollQuota() {
	gceAPIGate()
	reg, err := computeService.Regions.Get(buildEnv.ProjectName, buildEnv.Region()).Do()
	if err != nil {
		log.Printf("Failed to get quota for %s/%s: %v", buildEnv.ProjectName, buildEnv.Region(), err)
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, quota := range reg.Quotas {
		switch quota.Metric {
		case "CPUS":
			p.cpuLeft = int(quota.Limit) - int(quota.Usage)
			p.cpuUsage = int(quota.Usage)
		case "INSTANCES":
			p.instLeft = int(quota.Limit) - int(quota.Usage)
			p.instUsage = int(quota.Usage)
		case "IN_USE_ADDRESSES":
			p.addrUsage = int(quota.Usage)
		}
	}
}

func (p *gceBuildletPool) SetEnabled(enabled bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.disabled = !enabled
}

func (p *gceBuildletPool) GetBuildlet(ctx context.Context, hostType string, lg logger) (bc *buildlet.Client, err error) {
	hconf, ok := dashboard.Hosts[hostType]
	if !ok {
		return nil, fmt.Errorf("gcepool: unknown host type %q", hostType)
	}
	qsp := lg.CreateSpan("awaiting_gce_quota")
	err = p.awaitVMCountQuota(ctx, hconf.GCENumCPU())
	qsp.Done(err)
	if err != nil {
		return nil, err
	}

	deleteIn, ok := ctx.Value(buildletTimeoutOpt{}).(time.Duration)
	if !ok {
		deleteIn = vmDeleteTimeout
	}

	instName := "buildlet-" + strings.TrimPrefix(hostType, "host-") + "-rn" + randHex(7)
	instName = strings.Replace(instName, "_", "-", -1) // Issue 22905; can't use underscores in GCE VMs
	p.setInstanceUsed(instName, true)

	gceBuildletSpan := lg.CreateSpan("create_gce_buildlet", instName)
	defer func() { gceBuildletSpan.Done(err) }()

	var (
		needDelete   bool
		createSpan   = lg.CreateSpan("create_gce_instance", instName)
		waitBuildlet spanlog.Span // made after create is done
		curSpan      = createSpan // either instSpan or waitBuildlet
	)

	log.Printf("Creating GCE VM %q for %s", instName, hostType)
	bc, err = buildlet.StartNewVM(gcpCreds, buildEnv, instName, hostType, buildlet.VMOpts{
		DeleteIn: deleteIn,
		OnInstanceRequested: func() {
			log.Printf("GCE VM %q now booting", instName)
		},
		OnInstanceCreated: func() {
			needDelete = true

			createSpan.Done(nil)
			waitBuildlet = lg.CreateSpan("wait_buildlet_start", instName)
			curSpan = waitBuildlet
		},
		OnGotInstanceInfo: func() {
			lg.LogEventTime("got_instance_info", "waiting_for_buildlet...")
		},
	})
	if err != nil {
		curSpan.Done(err)
		log.Printf("Failed to create VM for %s: %v", hostType, err)
		if needDelete {
			deleteVM(buildEnv.Zone, instName)
			p.putVMCountQuota(hconf.GCENumCPU())
		}
		p.setInstanceUsed(instName, false)
		return nil, err
	}
	waitBuildlet.Done(nil)
	bc.SetDescription("GCE VM: " + instName)
	bc.SetOnHeartbeatFailure(func() {
		p.putBuildlet(bc, hostType, instName)
	})
	return bc, nil
}

func (p *gceBuildletPool) putBuildlet(bc *buildlet.Client, hostType, instName string) error {
	// TODO(bradfitz): add the buildlet to a freelist (of max N
	// items) for up to 10 minutes since when it got started if
	// it's never seen a command execution failure, and we can
	// wipe all its disk content? (perhaps wipe its disk content
	// when it's retrieved, like the reverse buildlet pool) But
	// this will require re-introducing a distinction in the
	// buildlet client library between Close, Destroy/Halt, and
	// tracking execution errors.  That was all half-baked before
	// and thus removed. Now Close always destroys everything.
	deleteVM(buildEnv.Zone, instName)
	p.setInstanceUsed(instName, false)

	hconf, ok := dashboard.Hosts[hostType]
	if !ok {
		panic("failed to lookup conf") // should've worked if we did it before
	}
	p.putVMCountQuota(hconf.GCENumCPU())
	return nil
}

func (p *gceBuildletPool) WriteHTMLStatus(w io.Writer) {
	fmt.Fprintf(w, "<b>GCE pool</b> capacity: %s", p.capacityString())
	const show = 6 // must be even
	active := p.instancesActive()
	if len(active) > 0 {
		fmt.Fprintf(w, "<ul>")
		for i, inst := range active {
			if i < show/2 || i >= len(active)-(show/2) {
				fmt.Fprintf(w, "<li>%v, %s</li>\n", inst.name, friendlyDuration(time.Since(inst.creation)))
			} else if i == show/2 {
				fmt.Fprintf(w, "<li>... %d of %d total omitted ...</li>\n", len(active)-show, len(active))
			}
		}
		fmt.Fprintf(w, "</ul>")
	}
}

func (p *gceBuildletPool) String() string {
	return fmt.Sprintf("GCE pool capacity: %s", p.capacityString())
}

func (p *gceBuildletPool) capacityString() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return fmt.Sprintf("%d/%d instances; %d/%d CPUs",
		len(p.inst), p.instUsage+p.instLeft,
		p.cpuUsage, p.cpuUsage+p.cpuLeft)
}

// awaitVMCountQuota waits for numCPU CPUs of quota to become available,
// or returns ctx.Err.
func (p *gceBuildletPool) awaitVMCountQuota(ctx context.Context, numCPU int) error {
	// Poll every 2 seconds, which could be better, but works and
	// is simple.
	for {
		if p.tryAllocateQuota(numCPU) {
			return nil
		}
		select {
		case <-time.After(2 * time.Second):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (p *gceBuildletPool) HasCapacity(hostType string) bool {
	hconf, ok := dashboard.Hosts[hostType]
	if !ok {
		return false
	}
	numCPU := hconf.GCENumCPU()
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.haveQuotaLocked(numCPU)
}

// haveQuotaLocked reports whether the current GCE quota permits
// starting numCPU more CPUs.
//
// precondition: p.mu must be held.
func (p *gceBuildletPool) haveQuotaLocked(numCPU int) bool {
	return p.cpuLeft >= numCPU && p.instLeft >= 1 && len(p.inst) < maxInstances && p.addrUsage < maxInstances
}

func (p *gceBuildletPool) tryAllocateQuota(numCPU int) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.disabled {
		return false
	}
	if p.haveQuotaLocked(numCPU) {
		p.cpuUsage += numCPU
		p.cpuLeft -= numCPU
		p.instLeft--
		p.addrUsage++
		return true
	}
	return false
}

// putVMCountQuota adjusts the dead-reckoning of our quota usage by
// one instance and cpu CPUs.
func (p *gceBuildletPool) putVMCountQuota(cpu int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cpuUsage -= cpu
	p.cpuLeft += cpu
	p.instLeft++
}

func (p *gceBuildletPool) setInstanceUsed(instName string, used bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.inst == nil {
		p.inst = make(map[string]time.Time)
	}
	if used {
		p.inst[instName] = time.Now()
	} else {
		delete(p.inst, instName)
	}
}

func (p *gceBuildletPool) instanceUsed(instName string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	_, ok := p.inst[instName]
	return ok
}

func (p *gceBuildletPool) instancesActive() (ret []resourceTime) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for name, create := range p.inst {
		ret = append(ret, resourceTime{
			name:     name,
			creation: create,
		})
	}
	sort.Sort(byCreationTime(ret))
	return ret
}

// resourceTime is a GCE instance or Kube pod name and its creation time.
type resourceTime struct {
	name     string
	creation time.Time
}

type byCreationTime []resourceTime

func (s byCreationTime) Len() int           { return len(s) }
func (s byCreationTime) Less(i, j int) bool { return s[i].creation.Before(s[j].creation) }
func (s byCreationTime) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

// cleanUpOldVMs loops forever and periodically enumerates virtual
// machines and deletes those which have expired.
//
// A VM is considered expired if it has a "delete-at" metadata
// attribute having a unix timestamp before the current time.
//
// This is the safety mechanism to delete VMs which stray from the
// normal deleting process. VMs are created to run a single build and
// should be shut down by a controlling process. Due to various types
// of failures, they might get stranded. To prevent them from getting
// stranded and wasting resources forever, we instead set the
// "delete-at" metadata attribute on them when created to some time
// that's well beyond their expected lifetime.
func (p *gceBuildletPool) cleanUpOldVMs() {
	if *mode == "dev" {
		return
	}
	if computeService == nil {
		return
	}

	// TODO(bradfitz): remove this list and just query it from the compute API?
	// http://godoc.org/google.golang.org/api/compute/v1#RegionsService.Get
	// and Region.Zones: http://godoc.org/google.golang.org/api/compute/v1#Region

	for {
		for _, zone := range buildEnv.ZonesToClean {
			if err := p.cleanZoneVMs(zone); err != nil {
				log.Printf("Error cleaning VMs in zone %q: %v", zone, err)
			}
		}
		time.Sleep(time.Minute)
	}
}

// cleanZoneVMs is part of cleanUpOldVMs, operating on a single zone.
func (p *gceBuildletPool) cleanZoneVMs(zone string) error {
	// Fetch the first 500 (default) running instances and clean
	// thoes. We expect that we'll be running many fewer than
	// that. Even if we have more, eventually the first 500 will
	// either end or be cleaned, and then the next call will get a
	// partially-different 500.
	// TODO(bradfitz): revist this code if we ever start running
	// thousands of VMs.
	gceAPIGate()
	list, err := computeService.Instances.List(buildEnv.ProjectName, zone).Do()
	if err != nil {
		return fmt.Errorf("listing instances: %v", err)
	}
	for _, inst := range list.Items {
		if inst.Metadata == nil {
			// Defensive. Not seen in practice.
			continue
		}
		var sawDeleteAt bool
		var deleteReason string
		for _, it := range inst.Metadata.Items {
			if it.Key == "delete-at" {
				if it.Value == nil {
					log.Printf("missing delete-at value; ignoring")
					continue
				}
				unixDeadline, err := strconv.ParseInt(*it.Value, 10, 64)
				if err != nil {
					log.Printf("invalid delete-at value %q seen; ignoring", *it.Value)
					continue
				}
				sawDeleteAt = true
				if time.Now().Unix() > unixDeadline {
					deleteReason = "delete-at expiration"
				}
			}
		}
		isBuildlet := strings.HasPrefix(inst.Name, "buildlet-")

		if isBuildlet && !sawDeleteAt && !p.instanceUsed(inst.Name) {
			createdAt, _ := time.Parse(time.RFC3339Nano, inst.CreationTimestamp)
			if createdAt.Before(time.Now().Add(-3 * time.Hour)) {
				deleteReason = fmt.Sprintf("no delete-at, created at %s", inst.CreationTimestamp)
			}
		}

		// Delete buildlets (things we made) from previous
		// generations. Only deleting things starting with "buildlet-"
		// is a historical restriction, but still fine for paranoia.
		if deleteReason == "" && sawDeleteAt && isBuildlet && !p.instanceUsed(inst.Name) {
			if _, ok := deletedVMCache.Get(inst.Name); !ok {
				deleteReason = "from earlier coordinator generation"
			}
		}

		if deleteReason != "" {
			log.Printf("deleting VM %q in zone %q; %s ...", inst.Name, zone, deleteReason)
			deleteVM(zone, inst.Name)
		}

	}
	return nil
}

var deletedVMCache = lru.New(100) // keyed by instName

// deleteVM starts a delete of an instance in a given zone.
//
// It either returns an operation name (if delete is pending) or the
// empty string if the instance didn't exist.
func deleteVM(zone, instName string) (operation string, err error) {
	deletedVMCache.Add(instName, token{})
	gceAPIGate()
	op, err := computeService.Instances.Delete(buildEnv.ProjectName, zone, instName).Do()
	apiErr, ok := err.(*googleapi.Error)
	if ok {
		if apiErr.Code == 404 {
			return "", nil
		}
	}
	if err != nil {
		log.Printf("Failed to delete instance %q in zone %q: %v", instName, zone, err)
		return "", err
	}
	log.Printf("Sent request to delete instance %q in zone %q. Operation ID, Name: %v, %v", instName, zone, op.Id, op.Name)
	return op.Name, nil
}

func hasScope(want string) bool {
	// If not on GCE, assume full access
	if !metadata.OnGCE() {
		return true
	}
	scopes, err := metadata.Scopes("default")
	if err != nil {
		log.Printf("failed to query metadata default scopes: %v", err)
		return false
	}
	for _, v := range scopes {
		if v == want {
			return true
		}
	}
	return false
}

func hasComputeScope() bool {
	return hasScope(compute.ComputeScope) || hasScope(compute.CloudPlatformScope)
}

func hasStorageScope() bool {
	return hasScope(storage.ScopeReadWrite) || hasScope(storage.ScopeFullControl) || hasScope(compute.CloudPlatformScope)
}

func readGCSFile(name string) ([]byte, error) {
	if *mode == "dev" {
		b, ok := testFiles[name]
		if !ok {
			return nil, &os.PathError{
				Op:   "open",
				Path: name,
				Err:  os.ErrNotExist,
			}
		}
		return []byte(b), nil
	}

	r, err := storageClient.Bucket(buildEnv.BuildletBucket).Object(name).NewReader(context.Background())
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return ioutil.ReadAll(r)
}

// syncBuildStatsLoop runs forever in its own goroutine and syncs the
// coordinator's datastore Build & Span entities to BigQuery
// periodically.
func syncBuildStatsLoop(env *buildenv.Environment) {
	ticker := time.NewTicker(5 * time.Minute)
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		if err := buildstats.SyncBuilds(ctx, env); err != nil {
			log.Printf("buildstats: SyncBuilds: %v", err)
		}
		if err := buildstats.SyncSpans(ctx, env); err != nil {
			log.Printf("buildstats: SyncSpans: %v", err)
		}
		cancel()
		<-ticker.C
	}
}
