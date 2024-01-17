/*
Copyright 2023 The Nuclio Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package common

import (
	"context"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/nuclio/logger"
)

type Report interface {
	// SaveToFile saves a report to file on a given path
	SaveToFile(ctx context.Context, loggerInstance logger.Logger, path string)

	// SprintfError generates string with detailed error
	SprintfError() string

	// PrintAsTable adds rows to the provided table writer
	PrintAsTable(t table.Writer, onlyFailed bool)
}
