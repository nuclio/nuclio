package logProxier

import (
	"context"
	"io"

	"github.com/nuclio/nuclio/pkg/platform"
)

type LogProxier interface {
	ProxyFunctionLogs(ctx context.Context, options *platform.ProxyFunctionLogsOptions) (io.ReadCloser, error)

	GetFunctionReplicas(ctx context.Context, options *GetFunctionReplicaOptions) ([]string, error)
}

type GetFunctionReplicaOptions struct {
	TimeFilter   *platform.TimeFilter
	FunctionName string
}
