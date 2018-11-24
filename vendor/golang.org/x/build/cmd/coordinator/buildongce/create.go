// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main // import "golang.org/x/build/cmd/coordinator/buildongce"

import (
	"bytes"
	"context"
	"crypto/rand"
	"flag"
	"fmt"
	"log"
	"strings"
	"text/template"
	"time"

	monapi "cloud.google.com/go/monitoring/apiv3"
	"golang.org/x/build/buildenv"
	"golang.org/x/build/cmd/coordinator/metrics"
	"golang.org/x/build/internal/buildgo"
	dm "google.golang.org/api/deploymentmanager/v2"
	"google.golang.org/api/option"
	monpb "google.golang.org/genproto/googleapis/monitoring/v3"
)

var (
	makeClusters = flag.String("make-clusters", "go,buildlets", "comma-separated list of clusters to create. Empty means none.")
	makeDisks    = flag.Bool("make-basepin", false, "Create the basepin disk images for all builders, then stop. Does not create the VM.")
	makeMetrics  = flag.Bool("make-metrics", false, "Create the Stackdriver metrics for buildlet monitoring.")
)

// Deployment Manager V2 manifest for creating a Google Container Engine
// cluster to run buildlets, as well as an autoscaler attached to the
// cluster's instance group to add capacity based on CPU utilization
const kubeConfig = `
resources:
- name: "{{ .Kube.Name }}"
  type: container.v1.cluster
  properties:
    zone: "{{ .Env.Zone }}"
    cluster:
      initial_node_count: {{ .Kube.MinNodes }}
      network: "default"
      logging_service: "logging.googleapis.com"
      monitoring_service: "none"
      node_config:
        machine_type: "{{ .Kube.MachineType }}"
        oauth_scopes:
          - "https://www.googleapis.com/auth/cloud-platform"
          - "https://www.googleapis.com/auth/userinfo.email"
      master_auth:
        username: "admin"
        password: "{{ .Password }}"`

// Old autoscaler part:
/*
`
- name: autoscaler
  type: compute.v1.autoscaler
  properties:
    zone: "{{ .Zone }}"
    name: "{{ .KubeName }}"
    target: "$(ref.{{ .KubeName }}.instanceGroupUrls[0])"
    autoscalingPolicy:
      minNumReplicas: {{ .KubeMinNodes }}
      maxNumReplicas: {{ .KubeMaxNodes }}
      coolDownPeriodSec: 1200
      cpuUtilization:
        utilizationTarget: .6`
*/

func main() {
	buildenv.RegisterStagingFlag()
	flag.Parse()

	buildenv.CheckUserCredentials()
	buildEnv := buildenv.FromFlags()
	ctx := context.Background()

	bgc, err := buildgo.NewClient(ctx, buildEnv)
	if err != nil {
		log.Fatalf("could not create client: %v", err)
	}

	if *makeDisks {
		if err := bgc.MakeBasepinDisks(ctx); err != nil {
			log.Fatalf("could not create basepin disks: %v", err)
		}
		return
	}

	for _, c := range []*buildenv.KubeConfig{&buildEnv.KubeBuild, &buildEnv.KubeTools} {
		err := createCluster(bgc, c)
		if err != nil {
			log.Fatalf("Error creating Kubernetes cluster %q: %v", c.Name, err)
		}
	}

	if *makeMetrics {
		if err := createMetrics(bgc); err != nil {
			log.Fatalf("could not create metrics: %v", err)
		}
	}
}

type deploymentTemplateData struct {
	Env      *buildenv.Environment
	Kube     *buildenv.KubeConfig
	Password string
}

func wantClusterCreate(name string) bool {
	for _, want := range strings.Split(*makeClusters, ",") {
		if want == name {
			return true
		}
	}
	return false
}

func createCluster(bgc *buildgo.Client, kube *buildenv.KubeConfig) error {
	if !wantClusterCreate(kube.Name) {
		log.Printf("skipping kubernetes cluster %q per flag", kube.Name)
		return nil
	}
	log.Printf("Creating Kubernetes cluster: %v", kube.Name)
	deploySvc, _ := dm.New(bgc.Client)

	if kube.MaxNodes == 0 || kube.MinNodes == 0 {
		return fmt.Errorf("MaxNodes/MinNodes values cannot be 0")
	}

	tpl, err := template.New("kube").Parse(kubeConfig)
	if err != nil {
		return fmt.Errorf("could not parse Deployment Manager template: %v", err)
	}

	var result bytes.Buffer
	err = tpl.Execute(&result, deploymentTemplateData{
		Env:      bgc.Env,
		Kube:     kube,
		Password: randomPassword(),
	})
	if err != nil {
		return fmt.Errorf("could not execute Deployment Manager template: %v", err)
	}

	deployment := &dm.Deployment{
		Name: kube.Name,
		Target: &dm.TargetConfiguration{
			Config: &dm.ConfigFile{
				Content: result.String(),
			},
		},
	}
	op, err := deploySvc.Deployments.Insert(bgc.Env.ProjectName, deployment).Do()
	if err != nil {
		return fmt.Errorf("Failed to create cluster with Deployment Manager: %v", err)
	}
	opName := op.Name
	log.Printf("Created. Waiting on operation %v", opName)
OpLoop:
	for {
		time.Sleep(2 * time.Second)
		op, err := deploySvc.Operations.Get(bgc.Env.ProjectName, opName).Do()
		if err != nil {
			return fmt.Errorf("Failed to get op %s: %v", opName, err)
		}
		switch op.Status {
		case "PENDING", "RUNNING":
			log.Printf("Waiting on operation %v", opName)
			continue
		case "DONE":
			// If no errors occurred, op.StatusMessage is empty.
			if op.StatusMessage != "" {
				log.Printf("Error: %+v", op.StatusMessage)
				return fmt.Errorf("Failed to create.")
			}
			log.Printf("Success.")
			break OpLoop
		default:
			return fmt.Errorf("Unknown status %q: %+v", op.Status, op)
		}
	}
	return nil
}

func randomPassword() string {
	buf := make([]byte, 10)
	if _, err := rand.Read(buf); err != nil {
		log.Fatalf("randomPassword: %v", err)
	}
	return fmt.Sprintf("%x", buf)
}

// createMetrics creates the Stackdriver metric types required to monitor
// buildlets on Stackdriver.
func createMetrics(bgc *buildgo.Client) error {
	ctx := context.Background()
	c, err := monapi.NewMetricClient(ctx, option.WithCredentials(bgc.Creds))
	if err != nil {
		return err
	}

	for _, m := range metrics.Metrics {
		if _, err = c.CreateMetricDescriptor(ctx, &monpb.CreateMetricDescriptorRequest{
			Name:             m.DescriptorPath(bgc.Env.ProjectName),
			MetricDescriptor: m.Descriptor,
		}); err != nil {
			return err
		}
	}

	return nil
}
