// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gke_test

import (
	"context"
	"strings"
	"testing"

	"cloud.google.com/go/compute/metadata"
	"golang.org/x/build/kubernetes"
	"golang.org/x/build/kubernetes/gke"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	compute "google.golang.org/api/compute/v1"
	container "google.golang.org/api/container/v1"
)

// Tests NewClient and also Dialer.
func TestNewClient(t *testing.T) {
	ctx := context.Background()
	foreachCluster(t, func(cl *container.Cluster, kc *kubernetes.Client) {
		_, err := kc.GetPods(ctx)
		if err != nil {
			t.Fatal(err)
		}
	})
}

func TestDialPod(t *testing.T) {
	var passed bool
	var candidates int
	ctx := context.Background()
	foreachCluster(t, func(cl *container.Cluster, kc *kubernetes.Client) {
		if passed {
			return
		}
		pods, err := kc.GetPods(ctx)
		if err != nil {
			t.Fatal(err)
		}

		for _, pod := range pods {
			if pod.Status.Phase != "Running" {
				continue
			}
			for _, container := range pod.Spec.Containers {
				for _, port := range container.Ports {
					if strings.ToLower(string(port.Protocol)) == "udp" || port.ContainerPort == 0 {
						continue
					}
					candidates++
					c, err := kc.DialPod(ctx, pod.Name, port.ContainerPort)
					if err != nil {
						t.Logf("Dial %q/%q/%d: %v", cl.Name, pod.Name, port.ContainerPort, err)
						continue
					}
					c.Close()
					t.Logf("Dialed %q/%q/%d.", cl.Name, pod.Name, port.ContainerPort)
					passed = true
					return
				}
			}
		}
	})
	if candidates == 0 {
		t.Skip("no pods to dial")
	}
	if !passed {
		t.Errorf("dial failures")
	}
}

func TestDialService(t *testing.T) {
	var passed bool
	var candidates int
	ctx := context.Background()
	foreachCluster(t, func(cl *container.Cluster, kc *kubernetes.Client) {
		if passed {
			return
		}
		svcs, err := kc.GetServices(ctx)
		if err != nil {
			t.Fatal(err)
		}
		for _, svc := range svcs {
			eps, err := kc.GetServiceEndpoints(ctx, svc.Name, "")
			if err != nil {
				t.Fatal(err)
			}
			if len(eps) != 1 {
				continue
			}
			candidates++
			conn, err := kc.DialServicePort(ctx, svc.Name, "")
			if err != nil {
				t.Logf("%s: DialServicePort(%q) error: %v", cl.Name, svc.Name, err)
				continue
			}
			conn.Close()
			passed = true
			t.Logf("Dialed cluster %q service %q.", cl.Name, svc.Name)
			return
		}

	})
	if candidates == 0 {
		t.Skip("no services to dial")
	}
	if !passed {
		t.Errorf("dial failures")
	}
}

func foreachCluster(t *testing.T, fn func(*container.Cluster, *kubernetes.Client)) {
	if !metadata.OnGCE() {
		t.Skip("not on GCE; skipping")
	}
	ctx := context.Background()
	ts, err := google.DefaultTokenSource(ctx, compute.CloudPlatformScope)
	if err != nil {
		t.Fatal(err)
	}
	httpClient := oauth2.NewClient(ctx, ts)
	containerService, err := container.New(httpClient)
	if err != nil {
		t.Fatal(err)
	}
	proj, err := metadata.ProjectID()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ts.Token(); err != nil {
		val, err := metadata.InstanceAttributeValue("service-accounts/default/token")
		if val == "" {
			t.Skip("skipping on GCE instance without a service account")
		}
		t.Skipf("default token source doesn't work; skipping test: %v", err)
	}

	clusters, err := containerService.Projects.Zones.Clusters.List(proj, "-").Context(ctx).Do()
	if err != nil {
		t.Fatal(err)
	}

	if len(clusters.Clusters) == 0 {
		t.Skip("no GKE clusters")
	}
	for _, cl := range clusters.Clusters {
		kc, err := gke.NewClient(ctx, cl.Name, gke.OptZone(cl.Zone))
		if err != nil {
			t.Fatal(err)
		}
		fn(cl, kc)
		kc.Close()
	}
}

func TestGetNodes(t *testing.T) {
	var passed bool
	ctx := context.Background()
	foreachCluster(t, func(cl *container.Cluster, kc *kubernetes.Client) {
		if passed {
			return
		}
		nodes, err := kc.GetNodes(ctx)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("%d nodes in cluster %s", len(nodes), cl.Name)
		passed = true
	})
}
