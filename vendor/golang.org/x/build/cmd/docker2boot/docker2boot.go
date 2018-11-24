// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The docker2boot command converts a Docker image into a bootable GCE
// VM image.
package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

var (
	numGB   = flag.Int("gb", 2, "size of raw disk, in gigabytes")
	rawFile = flag.String("disk", "disk.raw", "temporary raw disk file to create and delete")
	img     = flag.String("image", "", "Docker image to convert. Required.")
	outFile = flag.String("out", "image.tar.gz", "GCE output .tar.gz image file to create")

	justRaw = flag.Bool("justraw", false, "If true, stop after preparing the raw file, but before creating the tar.gz")
)

// This is a Linux kernel and initrd that boots on GCE. It's the
// standard one that comes with the GCE Debian image.
const (
	bootTarURL = "https://storage.googleapis.com/go-builder-data/boot-linux-3.16-0.bpo.3-amd64.tar.gz"

	// bootUUID is the filesystem UUID in the bootTarURL snapshot.
	// TODO(bradfitz): parse this out of boot/grub/grub.cfg
	// instead, or write that file completely, so this doesn't
	// need to exist and stay in sync with the kernel snapshot.
	bootUUID = "906181f7-4e10-4a4e-8fd8-43b20ec980ff"
)

func main() {
	flag.Parse()
	defer os.Exit(1) // otherwise we call os.Exit(0) at the bottom
	if runtime.GOOS != "linux" {
		failf("docker2boot only runs on Linux")
	}
	if *img == "" {
		failf("Missing required --image Docker image flag.")
	}
	if *outFile == "" {
		failf("Missing required --out flag")
	}
	if strings.Contains(slurpFile("/proc/mounts"), "nbd0p1") {
		failf("/proc/mounts shows nbd0p1 already mounted. Unmount that first.")
	}

	checkDeps()

	mntDir, err := ioutil.TempDir("", "docker2boot")
	if err != nil {
		failf("Failed to create mount temp dir: %v", err)
	}
	defer os.RemoveAll(mntDir)

	out, err := exec.Command("docker", "run", "-d", *img, "/bin/true").CombinedOutput()
	if err != nil {
		failf("Error creating container to snapshot: %v, %s", err, out)
	}
	container := strings.TrimSpace(string(out))

	if os.Getenv("USER") != "root" {
		failf("this tool requires root. Re-run with sudo.")
	}

	// Install the kernel's network block device driver, if it's not already.
	// The qemu-nbd command would probably do this too, but this is a good place
	// to fail early if it's not available.
	run("modprobe", "nbd")

	if strings.Contains(slurpFile("/proc/partitions"), "nbd0") {
		// TODO(bradfitz): make the nbd device configurable,
		// or auto-select a free one.  Hard-coding the first
		// one is lazy, but works. Who uses NBD anyway?
		failf("Looks like /dev/nbd0 is already in use. Maybe a previous run failed in the middle? Try sudo qemu-nbd -d /dev/nbd0")
	}
	if _, err := os.Stat(*rawFile); !os.IsNotExist(err) {
		failf("File %s already exists. Delete it and try again, or use a different --disk flag value.", *rawFile)
	}
	defer os.Remove(*rawFile)

	// Make a big empty file full of zeros. Using fallocate to make a sparse
	// file is much quicker (~immediate) than using dd to write from /dev/zero.
	// GCE requires disk images to be sized by the gigabyte.
	run("fallocate", "-l", strconv.Itoa(*numGB)+"G", *rawFile)

	// Start a NBD server so the kernel's /dev/nbd0 reads/writes
	// from our disk image, currently all zeros.
	run("qemu-nbd", "-c", "/dev/nbd0", "--format=raw", *rawFile)
	defer exec.Command("qemu-nbd", "-d", "/dev/nbd0").Run()

	// Put a MS-DOS partition table on it (GCE requirement), with
	// the first partition's initial sector far enough in to leave
	// room for the grub boot loader.
	fdisk := exec.Command("/sbin/fdisk", "/dev/nbd0")
	fdisk.Stdin = strings.NewReader("o\nn\np\n1\n2048\n\nw\n")
	out, err = fdisk.CombinedOutput()
	if err != nil {
		failf("fdisk: %v, %s", err, out)
	}

	// Wait for the kernel to notice the partition. fdisk does an ioctl
	// to make the kernel rescan for partitions.
	deadline := time.Now().Add(5 * time.Second)
	for !strings.Contains(slurpFile("/proc/partitions"), "nbd0p1") {
		if time.Now().After(deadline) {
			failf("timeout waiting for nbd0p1 to appear")
		}
		time.Sleep(50 * time.Millisecond)
	}

	// Now that the partition is available, make a filesystem on it.
	run("mkfs.ext4", "/dev/nbd0p1")
	run("mount", "/dev/nbd0p1", mntDir)
	defer exec.Command("umount", mntDir).Run()

	log.Printf("Populating /boot/ partition from %s", bootTarURL)
	pipeInto(httpGet(bootTarURL), "tar", "-zx", "-C", mntDir)

	log.Printf("Exporting Docker container %s into fs", container)
	exp := exec.Command("docker", "export", container)
	tarPipe, err := exp.StdoutPipe()
	if err != nil {
		failf("Pipe: %v", err)
	}
	if err := exp.Start(); err != nil {
		failf("docker export: %v", err)
	}
	pipeInto(tarPipe, "tar", "-x", "-C", mntDir)
	if err := exp.Wait(); err != nil {
		failf("docker export: %v", err)
	}

	// Docker normally provides these etc files, so they're not in
	// the export and we have to include them ourselves.
	writeFile(filepath.Join(mntDir, "etc", "hosts"), "127.0.0.1\tlocalhost\n")
	writeFile(filepath.Join(mntDir, "etc", "resolv.conf"), "nameserver 8.8.8.8\n")

	// Append the source image id & docker version to /etc/issue.
	issue, err := ioutil.ReadFile("/etc/issue")
	if err != nil && !os.IsNotExist(err) {
		failf("Failed to read /etc/issue: %v", err)
	}
	out, err = exec.Command("docker", "inspect", "-f", "{{.Id}}", *img).CombinedOutput()
	if err != nil {
		failf("Error getting image id: %v, %s", err, out)
	}
	id := strings.TrimSpace(string(out))
	out, err = exec.Command("docker", "-v").CombinedOutput()
	if err != nil {
		failf("Error getting docker version: %v, %s", err, out)
	}
	dockerVersion := strings.TrimSpace(string(out))
	d2bissue := fmt.Sprintf("%s\nPrepared by docker2boot\nSource Docker image: %s %s\n%s\n", issue, *img, id, dockerVersion)
	writeFile(filepath.Join(mntDir, "etc", "issue"), d2bissue)

	// Install grub. Adjust the grub.cfg to have the correct
	// filesystem UUID of the filesystem made above.
	fsUUID := filesystemUUID()
	grubCfgFile := filepath.Join(mntDir, "boot/grub/grub.cfg")
	writeFile(grubCfgFile, strings.Replace(slurpFile(grubCfgFile), bootUUID, fsUUID, -1))
	run("rm", filepath.Join(mntDir, "boot/grub/device.map"))
	run("grub-install", "--boot-directory="+filepath.Join(mntDir, "boot"), "/dev/nbd0")
	fstabFile := filepath.Join(mntDir, "etc/fstab")
	writeFile(fstabFile, fmt.Sprintf("UUID=%s / ext4 errors=remount-ro 0 1", fsUUID))

	// Set some password for testing.
	run("chroot", mntDir, "/bin/bash", "-c", "echo root:r | chpasswd")

	run("umount", mntDir)
	run("qemu-nbd", "-d", "/dev/nbd0")
	if *justRaw {
		log.Printf("Stopping, and leaving %s alone.\nRun with:\n\n$ qemu-system-x86_64 -machine accel=kvm -nographic -curses -nodefconfig -smp 2 -drive if=virtio,file=%s -net nic,model=virtio -net user -boot once=d\n\n", *rawFile, *rawFile)
		os.Exit(0)
	}

	// Write out a sparse tarball. GCE creates images from sparse
	// tarballs on Google Cloud Storage.
	run("tar", "-Szcf", *outFile, *rawFile)

	os.Remove(*rawFile)
	os.Exit(0)
}

func checkDeps() {
	var missing []string
	for _, cmd := range []string{
		"docker",
		"dumpe2fs",
		"fallocate",
		"grub-install",
		"mkfs.ext4",
		"modprobe",
		"mount",
		"qemu-nbd",
		"rm",
		"tar",
		"umount",
	} {
		if _, err := exec.LookPath(cmd); err != nil {
			missing = append(missing, cmd)
		}
	}
	if len(missing) > 0 {
		failf("Missing dependency programs: %v", missing)
	}
}

func filesystemUUID() string {
	e2fs, err := exec.Command("dumpe2fs", "/dev/nbd0p1").Output()
	if err != nil {
		failf("dumpe2fs: %v", err)
	}
	m := regexp.MustCompile(`Filesystem UUID:\s+(\S+)`).FindStringSubmatch(string(e2fs))
	if m == nil || m[1] == "" {
		failf("failed to find filesystem UUID")
	}
	return m[1]
}

// failf is like log.Fatalf, but runs deferred functions.
func failf(msg string, args ...interface{}) {
	log.Printf(msg, args...)
	runtime.Goexit()
}

func httpGet(u string) io.Reader {
	res, err := http.Get(u)
	if err != nil {
		failf("Get %s: %v", u, err)
	}
	if res.StatusCode != 200 {
		failf("Get %s: %v", u, res.Status)
	}
	// Yeah, not closing it. This program is short-lived.
	return res.Body
}

func slurpFile(file string) string {
	v, err := ioutil.ReadFile(file)
	if err != nil {
		failf("Failed to read %s: %v", file, err)
	}
	return string(v)
}

func writeFile(file, contents string) {
	if err := ioutil.WriteFile(file, []byte(contents), 0644); err != nil {
		failf("writeFile %s: %v", file, err)
	}
}

func run(cmd string, args ...string) {
	log.Printf("Running %s %s", cmd, args)
	out, err := exec.Command(cmd, args...).CombinedOutput()
	if err != nil {
		failf("Error running %s %v: %v, %s", cmd, args, err, out)
	}
}

func pipeInto(stdin io.Reader, cmd string, args ...string) {
	log.Printf("Running %s %s", cmd, args)
	c := exec.Command(cmd, args...)
	c.Stdin = stdin
	out, err := c.CombinedOutput()
	if err != nil {
		failf("Error running %s %v: %v, %s", cmd, args, err, out)
	}
}
