// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package buildlet

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/build/buildenv"
	"golang.org/x/build/dashboard"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/compute/v1"
)

// GCEGate optionally specifies a function to run before any GCE API call.
// It's intended to be used to bound QPS rate to GCE.
var GCEGate func()

func apiGate() {
	if GCEGate != nil {
		GCEGate()
	}
}

// VMOpts control how new VMs are started.
type VMOpts struct {
	// Zone is the GCE zone to create the VM in.
	// Optional; defaults to provided build environment's zone.
	Zone string

	// ProjectID is the GCE project ID (e.g. "foo-bar-123", not
	// the numeric ID).
	// Optional; defaults to provided build environment's project ID ("name").
	ProjectID string

	// TLS optionally specifies the TLS keypair to use.
	// If zero, http without auth is used.
	TLS KeyPair

	// Optional description of the VM.
	Description string

	// Optional metadata to put on the instance.
	Meta map[string]string

	// DeleteIn optionally specifies a duration at which
	// to delete the VM.
	// If zero, a reasonable default is used.
	// Negative means no deletion timeout.
	DeleteIn time.Duration

	// OnInstanceRequested optionally specifies a hook to run synchronously
	// after the computeService.Instances.Insert call, but before
	// waiting for its operation to proceed.
	OnInstanceRequested func()

	// OnInstanceCreated optionally specifies a hook to run synchronously
	// after the instance operation succeeds.
	OnInstanceCreated func()

	// OnInstanceCreated optionally specifies a hook to run synchronously
	// after the computeService.Instances.Get call.
	OnGotInstanceInfo func()

	// OnBeginBuildletProbe optionally specifies a hook to run synchronously
	// before StartNewVM tries to hit buildletURL to see if it's up yet.
	OnBeginBuildletProbe func(buildletURL string)

	// OnEndBuildletProbe optionally specifies a hook to run synchronously
	// after StartNewVM tries to hit the buildlet's URL to see if it's up.
	// The hook parameters are the return values from http.Get.
	OnEndBuildletProbe func(*http.Response, error)
}

// StartNewVM boots a new VM on GCE and returns a buildlet client
// configured to speak to it.
func StartNewVM(creds *google.Credentials, buildEnv *buildenv.Environment, instName, hostType string, opts VMOpts) (*Client, error) {
	ctx := context.TODO()
	computeService, _ := compute.New(oauth2.NewClient(ctx, creds.TokenSource))

	if opts.Description == "" {
		opts.Description = fmt.Sprintf("Go Builder for %s", hostType)
	}
	if opts.ProjectID == "" {
		opts.ProjectID = buildEnv.ProjectName
	}
	if opts.Zone == "" {
		opts.Zone = buildEnv.Zone
	}
	if opts.DeleteIn == 0 {
		opts.DeleteIn = 30 * time.Minute
	}

	hconf, ok := dashboard.Hosts[hostType]
	if !ok {
		return nil, fmt.Errorf("invalid host type %q", hostType)
	}
	if !hconf.IsVM() && !hconf.IsContainer() {
		return nil, fmt.Errorf("host %q is type %q; want either a VM or container type", hostType, hconf.PoolName())
	}

	zone := opts.Zone
	if zone == "" {
		// TODO: automatic? maybe that's not useful.
		// For now just return an error.
		return nil, errors.New("buildlet: missing required Zone option")
	}
	projectID := opts.ProjectID
	if projectID == "" {
		return nil, errors.New("buildlet: missing required ProjectID option")
	}

	prefix := "https://www.googleapis.com/compute/v1/projects/" + projectID
	machType := prefix + "/zones/" + zone + "/machineTypes/" + hconf.MachineType()
	diskType := "https://www.googleapis.com/compute/v1/projects/" + projectID + "/zones/" + zone + "/diskTypes/pd-ssd"
	if hconf.RegularDisk {
		diskType = "" // a spinning disk
	}

	// Request an IP address if this is a world-facing buildlet.
	var accessConfigs []*compute.AccessConfig
	// TODO(bradfitz): remove the "true ||" part once we figure out why the buildlet
	// never boots without an IP address. Userspace seems to hang before we get to the buildlet?
	if true || !opts.TLS.IsZero() {
		accessConfigs = []*compute.AccessConfig{
			&compute.AccessConfig{
				Type: "ONE_TO_ONE_NAT",
				Name: "External NAT",
			},
		}
	}

	srcImage := "https://www.googleapis.com/compute/v1/projects/" + projectID + "/global/images/" + hconf.VMImage
	if hconf.IsContainer() {
		var err error
		srcImage, err = cosImage(ctx, computeService)
		if err != nil {
			return nil, fmt.Errorf("error find Container-Optimized OS image: %v", err)
		}
	}

	instance := &compute.Instance{
		Name:        instName,
		Description: opts.Description,
		MachineType: machType,
		Disks: []*compute.AttachedDisk{
			{
				AutoDelete: true,
				Boot:       true,
				Type:       "PERSISTENT",
				InitializeParams: &compute.AttachedDiskInitializeParams{
					DiskName:    instName,
					SourceImage: srcImage,
					DiskType:    diskType,
				},
			},
		},
		Tags: &compute.Tags{
			// Warning: do NOT list "http-server" or "allow-ssh" (our
			// project's custom tag to allow ssh access) here; the
			// buildlet provides full remote code execution.
			// The https-server is authenticated, though.
			Items: []string{"https-server"},
		},
		Metadata: &compute.Metadata{},
		NetworkInterfaces: []*compute.NetworkInterface{
			&compute.NetworkInterface{
				AccessConfigs: accessConfigs,
				Network:       prefix + "/global/networks/default",
			},
		},

		// Prior to git rev 1b1e086fd, we used preemptible
		// instances, as we were helping test the feature. It was
		// removed after git rev a23395d because we hadn't been
		// using it for some time. Our VMs are so short-lived that
		// the feature doesn't really help anyway. But if we ever
		// find we want it again, this comment is here to point to
		// code that might be useful to partially resurrect.
		Scheduling: &compute.Scheduling{Preemptible: false},
	}

	// Container builders use the COS image, which defaults to
	// logging to Cloud Logging, which requires the default
	// service account. So enable it when needed.
	// TODO: reduce this scope in the future, when we go wild with IAM.
	if hconf.IsContainer() {
		instance.ServiceAccounts = []*compute.ServiceAccount{
			{
				// This funky email address is the
				// "default service account" for GCE VMs:
				Email:  fmt.Sprintf("%v-compute@developer.gserviceaccount.com", buildEnv.ProjectNumber),
				Scopes: []string{compute.CloudPlatformScope},
			},
		}
	}

	addMeta := func(key, value string) {
		instance.Metadata.Items = append(instance.Metadata.Items, &compute.MetadataItems{
			Key:   key,
			Value: &value,
		})
	}
	// The buildlet-binary-url is the URL of the buildlet binary
	// which the VMs are configured to download at boot and run.
	// This lets us/ update the buildlet more easily than
	// rebuilding the whole VM image.
	addMeta("buildlet-binary-url", hconf.BuildletBinaryURL(buildenv.ByProjectID(opts.ProjectID)))
	addMeta("buildlet-host-type", hostType)
	if !opts.TLS.IsZero() {
		addMeta("tls-cert", opts.TLS.CertPEM)
		addMeta("tls-key", opts.TLS.KeyPEM)
		addMeta("password", opts.TLS.Password())
	}
	if hconf.IsContainer() {
		addMeta("gce-container-declaration", fmt.Sprintf(`spec:
  containers:
    - name: buildlet
      image: 'gcr.io/%s/%s'
      volumeMounts:
        - name: tmpfs-0
          mountPath: /workdir
      securityContext:
        privileged: true
      stdin: false
      tty: false
  restartPolicy: Always
  volumes:
    - name: tmpfs-0
      emptyDir:
        medium: Memory
`, opts.ProjectID, hconf.ContainerImage))
	}

	if opts.DeleteIn > 0 {
		// In case the VM gets away from us (generally: if the
		// coordinator dies while a build is running), then we
		// set this attribute of when it should be killed so
		// we can kill it later when the coordinator is
		// restarted. The cleanUpOldVMs goroutine loop handles
		// that killing.
		addMeta("delete-at", fmt.Sprint(time.Now().Add(opts.DeleteIn).Unix()))
	}

	for k, v := range opts.Meta {
		addMeta(k, v)
	}

	apiGate()
	op, err := computeService.Instances.Insert(projectID, zone, instance).Do()
	if err != nil {
		return nil, fmt.Errorf("Failed to create instance: %v", err)
	}
	condRun(opts.OnInstanceRequested)
	createOp := op.Name

	// Wait for instance create operation to succeed.
OpLoop:
	for {
		time.Sleep(2 * time.Second)
		apiGate()
		op, err := computeService.ZoneOperations.Get(projectID, zone, createOp).Do()
		if err != nil {
			return nil, fmt.Errorf("Failed to get op %s: %v", createOp, err)
		}
		switch op.Status {
		case "PENDING", "RUNNING":
			continue
		case "DONE":
			if op.Error != nil {
				for _, operr := range op.Error.Errors {
					log.Printf("failed to create instance %s in zone %s: %v", instName, zone, operr.Code)
					// TODO: catch Code=="QUOTA_EXCEEDED" and "Message" and return
					// a known error value/type.
					return nil, fmt.Errorf("Error creating instance: %+v", operr)
				}
				return nil, errors.New("Failed to start.")
			}
			break OpLoop
		default:
			return nil, fmt.Errorf("Unknown create status %q: %+v", op.Status, op)
		}
	}
	condRun(opts.OnInstanceCreated)

	apiGate()
	inst, err := computeService.Instances.Get(projectID, zone, instName).Do()
	if err != nil {
		return nil, fmt.Errorf("Error getting instance %s details after creation: %v", instName, err)
	}

	// Finds its internal and/or external IP addresses.
	intIP, extIP := instanceIPs(inst)

	// Wait for it to boot and its buildlet to come up.
	var buildletURL string
	var ipPort string
	if !opts.TLS.IsZero() {
		if extIP == "" {
			return nil, errors.New("didn't find its external IP address")
		}
		buildletURL = "https://" + extIP
		ipPort = extIP + ":443"
	} else {
		if intIP == "" {
			return nil, errors.New("didn't find its internal IP address")
		}
		buildletURL = "http://" + intIP
		ipPort = intIP + ":80"
	}
	condRun(opts.OnGotInstanceInfo)

	const timeout = 5 * time.Minute
	var alive bool
	impatientClient := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			Dial:              defaultDialer(),
			DisableKeepAlives: true,
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}
	deadline := time.Now().Add(timeout)
	try := 0
	for time.Now().Before(deadline) {
		try++
		if fn := opts.OnBeginBuildletProbe; fn != nil {
			fn(buildletURL)
		}
		res, err := impatientClient.Get(buildletURL)
		if fn := opts.OnEndBuildletProbe; fn != nil {
			fn(res, err)
		}
		if err != nil {
			time.Sleep(1 * time.Second)
			continue
		}
		res.Body.Close()
		if res.StatusCode != 200 {
			return nil, fmt.Errorf("buildlet returned HTTP status code %d on try number %d", res.StatusCode, try)
		}
		alive = true
		break
	}
	if !alive {
		return nil, fmt.Errorf("buildlet didn't come up at %s in %v", buildletURL, timeout)
	}

	return NewClient(ipPort, opts.TLS), nil
}

// DestroyVM sends a request to delete a VM. Actual VM description is
// currently (2015-01-19) very slow for no good reason. This function
// returns once it's been requested, not when it's done.
func DestroyVM(ts oauth2.TokenSource, proj, zone, instance string) error {
	computeService, _ := compute.New(oauth2.NewClient(context.TODO(), ts))
	apiGate()
	_, err := computeService.Instances.Delete(proj, zone, instance).Do()
	return err
}

type VM struct {
	// Name is the name of the GCE VM instance.
	// For example, it's of the form "mote-bradfitz-plan9-386-foo",
	// and not "plan9-386-foo".
	Name   string
	IPPort string
	TLS    KeyPair
	Type   string // buildlet type
}

// ListVMs lists all VMs.
func ListVMs(ts oauth2.TokenSource, proj, zone string) ([]VM, error) {
	var vms []VM
	computeService, _ := compute.New(oauth2.NewClient(context.TODO(), ts))

	// TODO(bradfitz): paging over results if more than 500
	apiGate()
	list, err := computeService.Instances.List(proj, zone).Do()
	if err != nil {
		return nil, err
	}
	for _, inst := range list.Items {
		if inst.Metadata == nil {
			// Defensive. Not seen in practice.
			continue
		}
		meta := map[string]string{}
		for _, it := range inst.Metadata.Items {
			if it.Value != nil {
				meta[it.Key] = *it.Value
			}
		}
		hostType := meta["buildlet-host-type"]
		if hostType == "" {
			continue
		}
		vm := VM{
			Name: inst.Name,
			Type: hostType,
			TLS: KeyPair{
				CertPEM: meta["tls-cert"],
				KeyPEM:  meta["tls-key"],
			},
		}
		_, extIP := instanceIPs(inst)
		if extIP == "" || vm.TLS.IsZero() {
			continue
		}
		vm.IPPort = extIP + ":443"
		vms = append(vms, vm)
	}
	return vms, nil
}

func instanceIPs(inst *compute.Instance) (intIP, extIP string) {
	for _, iface := range inst.NetworkInterfaces {
		if strings.HasPrefix(iface.NetworkIP, "10.") {
			intIP = iface.NetworkIP
		}
		for _, accessConfig := range iface.AccessConfigs {
			if accessConfig.Type == "ONE_TO_ONE_NAT" {
				extIP = accessConfig.NatIP
			}
		}
	}
	return
}

var (
	cosListMu      sync.Mutex
	cosCachedTime  time.Time
	cosCachedImage string
)

// cosImage returns the GCP VM image name of the latest stable
// Container-Optimized OS image. It caches results for 15 minutes.
func cosImage(ctx context.Context, svc *compute.Service) (string, error) {
	const cacheDuration = 15 * time.Minute
	cosListMu.Lock()
	defer cosListMu.Unlock()
	if cosCachedImage != "" && cosCachedTime.After(time.Now().Add(-cacheDuration)) {
		return cosCachedImage, nil
	}

	imList, err := svc.Images.List("cos-cloud").Filter(`(family eq "cos-stable")`).Context(ctx).Do()
	if err != nil {
		return "", err
	}
	if imList.NextPageToken != "" {
		return "", fmt.Errorf("too many images; pagination not supported")
	}
	ims := imList.Items
	if len(ims) == 0 {
		return "", errors.New("no image found")
	}
	sort.Slice(ims, func(i, j int) bool {
		if ims[i].Deprecated == nil && ims[j].Deprecated != nil {
			return true
		}
		return ims[i].CreationTimestamp > ims[j].CreationTimestamp
	})

	im := ims[0].SelfLink
	cosCachedImage = im
	cosCachedTime = time.Now()
	return im, nil
}
