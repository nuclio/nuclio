// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package gke contains code for interacting with Google Container Engine (GKE),
// the hosted version of Kubernetes on Google Cloud Platform.
//
// The API is not subject to the Go 1 compatibility promise and may change at
// any time. Users should vendor this package and deal with API changes.
package gke

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"net/http"

	"cloud.google.com/go/compute/metadata"

	"golang.org/x/build/kubernetes"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	compute "google.golang.org/api/compute/v1"
	"google.golang.org/api/container/v1"
)

// ClientOpt represents an option that can be passed to the Client function.
type ClientOpt interface {
	modify(*clientOpt)
}

type clientOpt struct {
	Project     string
	TokenSource oauth2.TokenSource
	Zone        string
}

type clientOptFunc func(*clientOpt)

func (f clientOptFunc) modify(o *clientOpt) { f(o) }

// OptProject returns an option setting the GCE Project ID to projectName.
// This is the named project ID, not the numeric ID.
// If unspecified, the current active project ID is used, if the program is running
// on a GCE intance.
func OptProject(projectName string) ClientOpt {
	return clientOptFunc(func(o *clientOpt) {
		o.Project = projectName
	})
}

// OptZone specifies the GCP zone the cluster is located in.
// This is necessary if and only if there are multiple GKE clusters with
// the same name in different zones.
func OptZone(zoneName string) ClientOpt {
	return clientOptFunc(func(o *clientOpt) {
		o.Zone = zoneName
	})
}

// OptTokenSource sets the oauth2 token source for making
// authenticated requests to the GKE API. If unset, the default token
// source is used (https://godoc.org/golang.org/x/oauth2/google#DefaultTokenSource).
func OptTokenSource(ts oauth2.TokenSource) ClientOpt {
	return clientOptFunc(func(o *clientOpt) {
		o.TokenSource = ts
	})
}

// NewClient returns an Kubernetes client to a GKE cluster.
func NewClient(ctx context.Context, clusterName string, opts ...ClientOpt) (*kubernetes.Client, error) {
	var opt clientOpt
	for _, o := range opts {
		o.modify(&opt)
	}
	if opt.TokenSource == nil {
		var err error
		opt.TokenSource, err = google.DefaultTokenSource(ctx, compute.CloudPlatformScope)
		if err != nil {
			return nil, fmt.Errorf("failed to get a token source: %v", err)
		}
	}
	if opt.Project == "" {
		proj, err := metadata.ProjectID()
		if err != nil {
			return nil, fmt.Errorf("metadata.ProjectID: %v", err)
		}
		opt.Project = proj
	}

	httpClient := oauth2.NewClient(ctx, opt.TokenSource)
	containerService, err := container.New(httpClient)
	if err != nil {
		return nil, fmt.Errorf("could not create client for Google Container Engine: %v", err)
	}

	var cluster *container.Cluster
	if opt.Zone == "" {
		clusters, err := containerService.Projects.Zones.Clusters.List(opt.Project, "-").Context(ctx).Do()
		if err != nil {
			return nil, err
		}
		if len(clusters.MissingZones) > 0 {
			return nil, fmt.Errorf("GKE cluster list response contains missing zones: %v", clusters.MissingZones)
		}
		matches := 0
		for _, cl := range clusters.Clusters {
			if cl.Name == clusterName {
				cluster = cl
				matches++
			}
		}
		if matches == 0 {
			return nil, fmt.Errorf("cluster %q not found in any zone", clusterName)
		}
		if matches > 1 {
			return nil, fmt.Errorf("cluster %q is ambiguous without using gke.OptZone to specify a zone", clusterName)
		}
	} else {
		cluster, err = containerService.Projects.Zones.Clusters.Get(opt.Project, opt.Zone, clusterName).Context(ctx).Do()
		if err != nil {
			return nil, fmt.Errorf("cluster %q could not be found in project %q, zone %q: %v", clusterName, opt.Project, opt.Zone, err)
		}
	}

	// Decode certs
	decode := func(which string, cert string) []byte {
		if err != nil {
			return nil
		}
		s, decErr := base64.StdEncoding.DecodeString(cert)
		if decErr != nil {
			err = fmt.Errorf("error decoding %s cert: %v", which, decErr)
		}
		return []byte(s)
	}
	clientCert := decode("client cert", cluster.MasterAuth.ClientCertificate)
	clientKey := decode("client key", cluster.MasterAuth.ClientKey)
	caCert := decode("cluster cert", cluster.MasterAuth.ClusterCaCertificate)
	if err != nil {
		return nil, err
	}

	// HTTPS client
	cert, err := tls.X509KeyPair(clientCert, clientKey)
	if err != nil {
		return nil, fmt.Errorf("x509 client key pair could not be generated: %v", err)
	}

	// CA Cert from kube master
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM([]byte(caCert))

	// Setup TLS config
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      caCertPool,
	}
	tlsConfig.BuildNameToCertificate()

	kubeHTTPClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
	}

	kubeClient, err := kubernetes.NewClient("https://"+cluster.Endpoint, kubeHTTPClient)
	if err != nil {
		return nil, fmt.Errorf("kubernetes HTTP client could not be created: %v", err)
	}
	return kubeClient, nil
}
