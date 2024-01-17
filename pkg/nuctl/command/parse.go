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

package command

import (
	"context"
	"encoding/json"
	"os"

	"github.com/nuclio/nuclio/pkg/nuctl/command/common"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/spf13/cobra"
)

type parseCommandeer struct {
	cmd            *cobra.Command
	rootCommandeer *RootCommandeer
	reportFilePath string
	onlyFailed     bool
	output         string
}

func newParseCommandeer(ctx context.Context, rootCommandeer *RootCommandeer) *parseCommandeer {
	commandeer := &parseCommandeer{
		rootCommandeer: rootCommandeer,
	}

	cmd := &cobra.Command{
		Use:     "parse [options]",
		Aliases: []string{"bu"},
		Short:   "Parse report",
		RunE: func(cmd *cobra.Command, args []string) error {
			// initialize root
			if err := rootCommandeer.initialize(); err != nil {
				return errors.Wrap(err, "Failed to initialize root")
			}

			return ParseReport(ctx, rootCommandeer.loggerInstance, commandeer.reportFilePath, commandeer.onlyFailed, commandeer.output)
		},
	}

	cmd.Flags().StringVar(&commandeer.reportFilePath, "report-file-path", "nuctl-import-report.json", "Path to report")
	cmd.Flags().BoolVar(&commandeer.onlyFailed, "only-failed", false, "Show only failures")
	cmd.Flags().StringVarP(&commandeer.output, "output", "o", "", "Path to save output")

	commandeer.cmd = cmd

	return commandeer
}

func ParseReport(ctx context.Context, logger logger.Logger, reportPath string, onlyFailed bool, outputPath string) error {
	reportData, err := os.ReadFile(reportPath)
	if err != nil {
		return errors.Wrap(err, "Failed to read report file")
	}
	supportedReportTypes := []common.Report{
		&common.ProjectReports{},
		&common.FunctionReports{},
	}

	for _, possibleType := range supportedReportTypes {
		if err = json.Unmarshal(reportData, possibleType); err == nil {
			t := table.NewWriter()
			possibleType.PrintAsTable(t, onlyFailed)
			output := t.Render()
			if outputPath != "" {
				if err := os.WriteFile(outputPath, []byte(output), 0644); err != nil {
					logger.WarnWithCtx(ctx, "Failed to write output to file",
						"path", outputPath,
						"error", err.Error())
				}
			}
			logger.Info(t.Render())
			return nil
		}
	}

	return errors.New("Could not parse report to any of known types")
}
