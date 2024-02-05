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
	"fmt"
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
	outputPath     string
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
			if err := rootCommandeer.initialize(false); err != nil {
				return errors.Wrap(err, "Failed to initialize root")
			}

			return commandeer.ParseReport(ctx, rootCommandeer.loggerInstance)
		},
	}

	cmd.Flags().StringVar(&commandeer.reportFilePath, "report-file-path", "nuctl-import-report.json", "Path to import report")
	cmd.Flags().BoolVar(&commandeer.onlyFailed, "failed", false, "Show only failures")
	cmd.Flags().StringVarP(&commandeer.outputPath, "output-path", "", "", "File path to save the parsed report")

	commandeer.cmd = cmd

	return commandeer
}

func (pc *parseCommandeer) ParseReport(ctx context.Context, logger logger.Logger) error {
	reportData, err := os.ReadFile(pc.reportFilePath)
	if err != nil {
		return errors.Wrap(err, "Failed to read report file")
	}
	supportedReportKinds := []common.Report{
		&common.ProjectReports{},
		&common.FunctionReports{},
	}

	// best-effort trying to parse each report kind
	for _, kind := range supportedReportKinds {
		if err = json.Unmarshal(reportData, kind); err == nil {
			t := table.NewWriter()
			kind.PrintAsTable(t, pc.onlyFailed)

			// do not print a report if it is empty
			if t.Length() == 0 {
				return nil
			}
			output := t.Render()
			if pc.outputPath != "" {
				if err := os.WriteFile(pc.outputPath, []byte(output), 0644); err != nil {
					logger.WarnWithCtx(ctx, "Failed to write outputPath to file",
						"path", pc.outputPath,
						"error", err.Error())
				}
			}
			fmt.Println(output)
			return nil
		}
	}

	return errors.New("Could not parse report to any of known types")
}
