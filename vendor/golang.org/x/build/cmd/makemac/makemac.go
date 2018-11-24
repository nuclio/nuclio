// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
The makemac command starts OS X VMs for the builders.
It is currently just a thin wrapper around govc.

See https://github.com/vmware/govmomi/tree/master/govc

Usage:

  $ makemac <osx_minor_version>  # e.g, 8, 9, 10, 11, 12

*/
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/build/types"
)

func usage() {
	fmt.Fprintf(os.Stderr, `Usage:
    makemac <osx_minor_version>
    makemac -status
    makemac -auto
`)
	os.Exit(1)
}

var (
	flagStatus = flag.Bool("status", false, "print status only")
	flagAuto   = flag.Bool("auto", false, "Automatically create & destroy as needed, reacting to https://farmer.golang.org/status/reverse.json status.")
	flagListen = flag.String("listen", ":8713", "HTTP status port; used by auto mode only")
	flagNuke   = flag.Bool("destroy-all", false, "immediately destroy all running Mac VMs")
)

func main() {
	flag.Parse()
	numArg := flag.NArg()
	if *flagStatus {
		numArg++
	}
	if *flagAuto {
		numArg++
	}
	if *flagNuke {
		numArg++
	}
	if numArg != 1 {
		usage()
	}
	if *flagAuto {
		autoLoop()
		return
	}
	ctx := context.Background()
	if *flagNuke {
		state, err := getState(ctx)
		if err != nil {
			log.Fatal(err)
		}
		if err := state.DestroyAllMacs(ctx); err != nil {
			log.Fatal(err)
		}
		return
	}
	minor, err := strconv.Atoi(flag.Arg(0))
	if err != nil && !*flagStatus {
		usage()
	}

	state, err := getState(ctx)
	if err != nil {
		log.Fatal(err)
	}

	if *flagStatus {
		stj, _ := json.MarshalIndent(state, "", "  ")
		fmt.Printf("%s\n", stj)
		return
	}

	_, err = state.CreateMac(ctx, minor)
	if err != nil {
		log.Fatal(err)
	}
}

// State is the state of the world.
type State struct {
	mu sync.Mutex

	Hosts  map[string]int    // IP address -> running Mac VM count (including 0)
	VMHost map[string]string // "mac_10_8_host02b" => "10.0.0.0"
	HostIP map[string]string // "host-5" -> "10.0.0.0"
	VMInfo map[string]VMInfo // "mac_10_8_host02b" => ...

	// VMOfSlot maps from a "slot name" to the VMWare VM name.
	//
	// A slot name is a tuple of (host number, "a"|"b"), where "a"
	// and "b" are the two possible guests that can run per host.
	// This slot name of the form "macstadium_host02b" is what's
	// reported as the host name to the coordinator.
	//
	// The map value is the VMWare vm name, such as "mac_10_8_host02b",
	// and is the map key of VMHost and VMInfo above.
	VMOfSlot map[string]string // "macstadium_host02b" => "mac_10_8_host02b"
}

type VMInfo struct {
	IP       string
	BootTime time.Time

	// SlotName is the name of a place where we can run a VM.
	// As of 2017-08-04 we have 20 slots total over 10 physical
	// machines. (Two VMs per physical Mac Mini running ESXi)
	// We use slot names of the form "macstadium_host02b"
	// with a %02d digit host number and suffix 'a' and 'b'
	// for which VM it is on that host.
	//
	// This slot name is also the name passed to the build
	// coordinator as the coordinator's "host name". (which exists
	// both for debugging, and for monitoring last-seen/uptime of
	// dedicated builders.)
	SlotName string
}

// NumCreatableVMs returns the number of VMs that can be created given
// the current capacity.
func (st *State) NumCreatableVMs() int {
	st.mu.Lock()
	defer st.mu.Unlock()
	n := 0
	for _, cur := range st.Hosts {
		if cur < 2 {
			n += 2 - cur
		}
	}
	return n
}

// NumMacVMsOfVersion reports how many VMs are running Mac OS X 10.<ver>.
func (st *State) NumMacVMsOfVersion(ver int) int {
	st.mu.Lock()
	defer st.mu.Unlock()
	prefix := fmt.Sprintf("mac_10_%v_", ver)
	n := 0
	for name := range st.VMInfo {
		if strings.HasPrefix(name, prefix) {
			n++
		}
	}
	return n
}

// DestroyAllMacs runs "govc vm.destroy" on each running Mac VM.
func (st *State) DestroyAllMacs(ctx context.Context) error {
	st.mu.Lock()
	defer st.mu.Unlock()
	var ret error
	for name := range st.VMInfo {
		log.Printf("destroying %s ...", name)
		err := govc(ctx, "vm.destroy", name)
		log.Printf("vm.destroy(%q) = %v", name, err)
		if err != nil && ret == nil {
			ret = err
		}
	}
	return ret
}

// CreateMac creates an Mac VM running OS X 10.<minor>.
func (st *State) CreateMac(ctx context.Context, minor int) (slotName string, err error) {
	// TODO(bradfitz): return VM name, update state, etc.

	st.mu.Lock()
	defer st.mu.Unlock()

	var guestType string
	switch minor {
	case 8:
		guestType = "darwin12_64Guest"
	case 9:
		guestType = "darwin13_64Guest"
	case 10, 11, 12:
		guestType = "darwin14_64Guest"
	default:
		return "", fmt.Errorf("unsupported makemac minor OS X version %d", minor)
	}

	builderType := fmt.Sprintf("darwin-amd64-10_%d", minor)
	key, err := ioutil.ReadFile(filepath.Join(os.Getenv("HOME"), "keys", builderType))
	if err != nil {
		return "", err
	}

	// Find the top-level datastore directory hosting the vmdk COW disk for
	// the linked clone. This is usually named "osx_9_frozen", but may be named
	// with a "_1", "_2", etc suffix. Search for it.
	netAppDir, err := findFrozenDir(ctx, minor)
	if err != nil {
		return "", fmt.Errorf("failed to find osx_%d_frozen base directory: %v", minor, err)
	}

	hostNum, hostWhich, err := st.pickHost()
	if err != nil {
		return "", err
	}
	name := fmt.Sprintf("mac_10_%v_host%02d%s", minor, hostNum, hostWhich)
	slotName = fmt.Sprintf("macstadium_host%02d%s", hostNum, hostWhich)

	if err := govc(ctx, "vm.create",
		"-m", "4096",
		"-c", "6",
		"-on=false",
		"-net", "dvPortGroup-Private", // 10.50.0.0/16
		"-g", guestType,
		// Put the config on the host's datastore, which
		// forces the VM to run on that host:
		"-ds", fmt.Sprintf("BOOT_%d", hostNum),
		name,
	); err != nil {
		return "", err
	}
	defer func() {
		if err != nil {
			err := govc(ctx, "vm.destroy", name)
			if err != nil {
				log.Printf("failed to destroy %v: %v", name, err)
			}
		}
	}()

	if err := govc(ctx, "vm.change",
		"-e", "smc.present=TRUE",
		"-e", "ich7m.present=TRUE",
		"-e", "firmware=efi",
		"-e", fmt.Sprintf("guestinfo.key-%s=%s", builderType, strings.TrimSpace(string(key))),
		"-e", "guestinfo.name="+name,
		"-vm", name,
	); err != nil {
		return "", err
	}

	if err := govc(ctx, "device.usb.add", "-vm", name); err != nil {
		return "", err
	}

	if err := govc(ctx, "vm.disk.attach",
		"-vm", name,
		"-link=true",
		"-persist=false",
		"-ds=Pure1-1",
		"-disk", fmt.Sprintf("%s/osx_%d_frozen.vmdk", netAppDir, minor),
	); err != nil {
		return "", err
	}

	if err := govc(ctx, "vm.power", "-on", name); err != nil {
		return "", err
	}
	log.Printf("Success.")
	return slotName, nil
}

// govc runs "govc <args...>" and ignores its output, unless there's an error.
func govc(ctx context.Context, args ...string) error {
	fmt.Fprintf(os.Stderr, "$ govc %v\n", strings.Join(args, " "))
	out, err := exec.CommandContext(ctx, "govc", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("govc %s ...: %v, %s", args[0], err, out)
	}
	return nil
}

const hostIPPrefix = "10.88.203." // with fourth octet starting at 10

var errNoHost = errors.New("no usable host found")

// st.mu must be held.
func (st *State) pickHost() (hostNum int, hostWhich string, err error) {
	for ip, inUse := range st.Hosts {
		if !strings.HasPrefix(ip, hostIPPrefix) {
			continue
		}
		if inUse >= 2 {
			// Apple policy.
			continue
		}
		hostNum, err = strconv.Atoi(strings.TrimPrefix(ip, hostIPPrefix))
		if err != nil {
			return 0, "", err
		}
		hostNum -= 10   // 10.88.203.11 is "BOOT_1" datastore.
		hostWhich = "a" // unless in use
		if st.whichAInUse(hostNum) {
			hostWhich = "b"
		}
		return
	}
	return 0, "", errNoHost
}

// whichAInUse reports whether a VM is running on the provided hostNum named
// with suffix "_host<%02d>a", hostnum.
//
// st.mu must be held
func (st *State) whichAInUse(hostNum int) bool {
	suffix := fmt.Sprintf("_host%02da", hostNum)
	for name := range st.VMHost {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	return false
}

// getStat queries govc to find the current state of the hosts and VMs.
func getState(ctx context.Context) (*State, error) {
	st := &State{
		VMHost:   make(map[string]string),
		Hosts:    make(map[string]int),
		HostIP:   make(map[string]string),
		VMInfo:   make(map[string]VMInfo),
		VMOfSlot: make(map[string]string),
	}

	var hosts elementList
	if err := govcJSONDecode(ctx, &hosts, "ls", "-json", "/MacStadium-ATL/host/MacMini_Cluster"); err != nil {
		return nil, fmt.Errorf("Reading /MacStadium-ATL/host/MacMini_Cluster: %v", err)
	}
	for _, h := range hosts.Elements {
		if h.Object.Self.Type == "HostSystem" {
			ip := path.Base(h.Path)
			st.Hosts[ip] = 0
			st.HostIP[h.Object.Self.Value] = ip
		}
	}

	var vms elementList
	if err := govcJSONDecode(ctx, &vms, "ls", "-json", "/MacStadium-ATL/vm"); err != nil {
		return nil, fmt.Errorf("Reading /MacStadium-ATL/vm: %v", err)
	}
	for _, h := range vms.Elements {
		if h.Object.Self.Type != "VirtualMachine" {
			continue
		}
		name := path.Base(h.Path)
		hostID := h.Object.Runtime.Host.Value
		hostIP := st.HostIP[hostID]
		st.VMHost[name] = hostIP
		if hostIP != "" && strings.HasPrefix(name, "mac_10_") {
			st.Hosts[hostIP]++
			var bootTime time.Time
			if bt := h.Object.Summary.Runtime.BootTime; bt != "" {
				bootTime, _ = time.Parse(time.RFC3339, bt)
			}

			var slotName string
			if p := strings.Index(name, "_host"); p != -1 {
				slotName = "macstadium" + name[p:] // macstadium_host02a

				if exist := st.VMOfSlot[slotName]; exist != "" {
					// Should never happen, but just in case.
					log.Printf("ERROR: existing VM %q found in slot %q; destroying later VM %q", exist, slotName, name)
					err := govc(ctx, "vm.destroy", name)
					log.Printf("vm.destroy(%q) = %v", name, err)
				} else {
					st.VMOfSlot[slotName] = name // macstadium_host02a => mac_10_8_host02a
				}
			}

			vi := VMInfo{
				IP:       hostIP,
				BootTime: bootTime,
				SlotName: slotName,
			}
			st.VMInfo[name] = vi
		}
	}

	return st, nil
}

// objRef is a VMWare "Managed Object Reference".
type objRef struct {
	Type  string // e.g. "VirtualMachine"
	Value string // e.g. "host-12"
}

type elementList struct {
	Elements []*elementJSON `json:"elements"`
}

type elementJSON struct {
	Path   string
	Object struct {
		Self    objRef
		Runtime struct {
			Host objRef // for VMs; not present otherwise
		}
		Summary struct {
			Runtime struct {
				BootTime string // time.RFC3339 format, or empty if not running
			}
		}
	}
}

// govcJSONDecode runs "govc <args...>" and decodes its JSON output into dst.
func govcJSONDecode(ctx context.Context, dst interface{}, args ...string) error {
	cmd := exec.CommandContext(ctx, "govc", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	err = json.NewDecoder(stdout).Decode(dst)
	cmd.Process.Kill() // usually unnecessary
	if werr := cmd.Wait(); werr != nil && err == nil {
		err = werr
	}
	return err
}

// findFrozenDir returns the name of the top-level directory on the
// Pure1-1 shared datastore containing a directory starting with
// "osx_<minor>_frozen". It might be that just that, or have a suffix
// like "_1" or "_2".
func findFrozenDir(ctx context.Context, minor int) (string, error) {
	out, err := exec.CommandContext(ctx, "govc", "datastore.ls", "-ds=Pure1-1").Output()
	if err != nil {
		return "", err
	}
	prefix := fmt.Sprintf("osx_%d_frozen", minor)
	for _, dir := range strings.Fields(string(out)) {
		if strings.HasPrefix(dir, prefix) {
			return dir, nil
		}
	}
	return "", os.ErrNotExist
}

const autoAdjustTimeout = 5 * time.Minute

var status struct {
	sync.Mutex
	lastCheck time.Time
	lastLog   string
	lastState *State
}

func init() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		status.Lock()
		defer status.Unlock()
		w.Header().Set("Content-Type", "application/json")

		// Locking the lastState shouldn't matter since we
		// currently only set status.lastState once the
		// *Status is no longer in use, but lock it anyway, in
		// case usage changes in the future.
		if st := status.lastState; st != nil {
			st.mu.Lock()
			defer st.mu.Unlock()
		}

		// TODO: probably more status, as needed.
		res := &struct {
			LastCheck string
			LastLog   string
			LastState *State
		}{
			LastCheck: status.lastCheck.UTC().Format(time.RFC3339),
			LastLog:   status.lastLog,
			LastState: status.lastState,
		}
		j, _ := json.MarshalIndent(res, "", "\t")
		w.Write(j)
	})
}

func dedupLogf(format string, args ...interface{}) {
	s := fmt.Sprintf(format, args...)
	status.Lock()
	defer status.Unlock()
	if s == status.lastLog {
		return
	}
	status.lastLog = s
	log.Print(s)
}

func autoLoop() {
	if addr := *flagListen; addr != "" {
		go func() {
			if err := http.ListenAndServe(*flagListen, nil); err != nil {
				log.Fatalf("ListenAndServe: %v", err)
			}
		}()
	}
	for {
		timer := time.AfterFunc(autoAdjustTimeout, watchdogFail)
		autoAdjust()
		timer.Stop()
		time.Sleep(2 * time.Second)
	}
}

func watchdogFail() {
	stacks := make([]byte, 1<<20)
	stacks = stacks[:runtime.Stack(stacks, true)]
	log.Fatalf("timeout after %v waiting for autoAdjust(). stacks:\n%s",
		autoAdjustTimeout, stacks)
}

func autoAdjust() {
	status.Lock()
	status.lastCheck = time.Now()
	status.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), autoAdjustTimeout)
	defer cancel()

	st, err := getState(ctx)
	if err != nil {
		log.Printf("getting VMWare state: %v", err)
		return
	}
	defer func() {
		// Set status.lastState once we're now longer using it.
		if st != nil {
			status.Lock()
			status.lastState = st
			status.Unlock()
		}
	}()

	req, _ := http.NewRequest("GET", "https://farmer.golang.org/status/reverse.json", nil)
	req = req.WithContext(ctx)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("getting reverse status: %v", err)
		return
	}
	defer res.Body.Close()
	var rstat types.ReverseBuilderStatus
	if err := json.NewDecoder(res.Body).Decode(&rstat); err != nil {
		log.Printf("decoding reverse.json: %v", err)
		return
	}

	revHost := make(map[string]*types.ReverseBuilder)
	for hostType, hostStatus := range rstat.HostTypes {
		if !strings.HasPrefix(hostType, "host-darwin-10") {
			continue
		}
		for name, revBuild := range hostStatus.Machines {
			revHost[name] = revBuild
		}
	}

	// Destroy running VMs that appear to be dead and not connected to the coordinator.
	dirty := false
	for name, vi := range st.VMInfo {
		if vi.BootTime.After(time.Now().Add(-3 * time.Minute)) {
			// Recently created. It takes about a minute
			// to boot and connect to the coordinator, so
			// give it 3 minutes of grace before killing
			// it.
			continue
		}
		rh := revHost[name]
		if rh == nil {
			// Look it up by its slot name instead.
			rh = revHost[vi.SlotName]
		}
		if rh == nil { //  || (!rh.Busy && rh.ConnectedSec > 50 && rh.HostType == "host-darwin-10_12") {
			log.Printf("Destroying VM %q unknown to coordinator...", name)
			err := govc(ctx, "vm.destroy", name)
			log.Printf("vm.destroy(%q) = %v", name, err)
			dirty = true
		}
	}
	for {
		if dirty {
			st, err = getState(ctx)
			if err != nil {
				log.Printf("getState: %v", err)
				return
			}
		}
		canCreate := st.NumCreatableVMs()
		if canCreate <= 0 {
			dedupLogf("All Mac VMs running.")
			return
		}
		ver := wantedMacVersionNext(st, &rstat)

		if ver == 0 {
			dedupLogf("Have capacity for %d more Mac VMs, but none requested by coordinator.", canCreate)
			return
		}
		dedupLogf("Have capacity for %d more Mac VMs; creating requested 10.%d ...", canCreate, ver)
		slotName, err := st.CreateMac(ctx, ver)
		if err != nil {
			log.Printf("Error creating 10.%d: %v", ver, err)
			return
		}
		log.Printf("Created 10.%d VM on %q", ver, slotName)
		dirty = true
	}
}

// wantedMacVersionNext returns the macOS 10.x version to create next,
// or 0 to not make anything. It gets the latest reverse buildlet
// status from the coordinator.
func wantedMacVersionNext(st *State, rstat *types.ReverseBuilderStatus) int {
	// TODO: improve this logic at some point, probably when the
	// coordinator has a proper scheduler (Issue 19178) and when
	// the coordinator keeps 1 of each builder type ready to go.
	// For now just use the static configuration in
	// dashboard/builders.go of how many are expected, which ends
	// up in ReverseBuilderStatus.
	for hostType, hostStatus := range rstat.HostTypes {
		if !strings.HasPrefix(hostType, "host-darwin-10_") {
			continue
		}
		ver, err := strconv.Atoi(strings.TrimPrefix(hostType, "host-darwin-10_"))
		if err != nil {
			log.Printf("ERROR: unexpected host type %q", hostType)
			continue
		}
		want := hostStatus.Expect - st.NumMacVMsOfVersion(ver)
		if want > 0 {
			return ver
		}
	}
	return 0
}
