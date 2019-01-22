package scaler

import (
	"time"

	"github.com/nuclio/logger"
)

type AutoScalerOptions struct {
	Namespace     string
	ScaleInterval time.Duration
	ScaleWindow   time.Duration
	MetricName    string
	Threshold     int64
}

type PollerOptions struct {
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
	Namespace        string
	TargetNameHeader string
	TargetPathHeader string
	TargetPort       int
	ListenAddress    string
}

type ResourceScaler interface {
	SetScale(logger.Logger, string, Resource, int) error
	GetResources(string) ([]Resource, error)
	GetConfig() (*ResourceScalerConfig, error)
}

type Resource string
