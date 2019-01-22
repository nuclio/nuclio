package scaler

import (
	"time"

	"k8s.io/client-go/kubernetes"
	custommetricsv1 "k8s.io/metrics/pkg/client/custom_metrics"
)

type AutoScalerOptions struct {
	// not needed to be provided by ResourceScalerConfig
	KubeClientSet  kubernetes.Interface
	ResourceScaler ResourceScaler

	Namespace     string
	ScaleInterval time.Duration
	ScaleWindow   time.Duration
	MetricName    string
	Threshold     int64
}

type PollerOptions struct {
	// not needed to be provided by ResourceScalerConfig
	CustomMetricsClientSet custommetricsv1.CustomMetricsClient
	ResourceScaler         ResourceScaler

	MetricInterval time.Duration
	MetricName     string
	Namespace      string
}

type ResourceScalerConfig struct {
	KubeconfigPath    string
	AutoScalerOptions AutoScalerOptions
	PollerOptions     PollerOptions
	DLXOptions        DLXOptions
}

type DLXOptions struct {
	// not needed to be provided by ResourceScalerConfig
	ResourceScaler ResourceScaler

	Namespace        string
	TargetNameHeader string
	TargetPathHeader string
	TargetPort       int
	ListenAddress    string
}

type ResourceScaler interface {
	SetScale(string, Resource, int) error
	GetResources() ([]Resource, error)
	GetConfig() (*ResourceScalerConfig, error)
}

type Resource string
