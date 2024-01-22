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
	"fmt"
	"sync"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
)

type ProjectReports struct {
	Reports map[string]*ProjectReport `json:"reports,omitempty"`
}

func NewProjectReports() *ProjectReports {
	return &ProjectReports{
		Reports: make(map[string]*ProjectReport),
	}
}

func (pr *ProjectReports) AddReport(report *ProjectReport) {
	pr.Reports[report.Name] = report
}

func (pr *ProjectReports) GetReport(projectName string) (report *ProjectReport, exists bool) {
	report, exists = pr.Reports[projectName]
	return
}

func (pr *ProjectReports) SaveToFile(ctx context.Context, loggerInstance logger.Logger, path string) {
	saveReportToFile(ctx, loggerInstance, pr, path)
}

func (pr *ProjectReports) SprintfError() string {
	report := ""
	for _, projectReport := range pr.Reports {
		report += projectReport.SprintfError()
	}
	return report
}

func (pr *ProjectReports) PrintAsTable(t table.Writer, onlyFailed bool) {
	t.AppendHeader(table.Row{"type", "name", "status", "fail description", "auto-fixable"})
	for _, report := range pr.Reports {
		report.PrintAsTable(t, onlyFailed)
	}
}

type ProjectReport struct {
	Name            string           `json:"name,omitempty"`
	Skipped         bool             `json:"skipped,omitempty"`
	Success         bool             `json:"success,omitempty"`
	Failed          *FailReport      `json:"failed,omitempty"`
	FunctionReports *FunctionReports `json:"functionReports,omitempty"`
}

func NewProjectReport(name string) *ProjectReport {
	return &ProjectReport{
		Name:            name,
		FunctionReports: NewFunctionReports(),
	}
}

// SetFailed sets the "Failed" status if it hasn't been set yet. In other words, it retains only the first error that occurred.
func (pr *ProjectReport) SetFailed(failReport *FailReport) {
	if pr.Failed == nil {
		pr.Failed = failReport
	}
}

func (pr *ProjectReport) SaveToFile(ctx context.Context, loggerInstance logger.Logger, path string) {
	saveReportToFile(ctx, loggerInstance, pr.FunctionReports, path)
}

func (pr *ProjectReport) SprintfError() string {
	if pr.Failed != nil {
		return fmt.Sprintf("Failed to import project `%s`. Reason: %s", pr.Name, pr.Failed.FailReason) + pr.FunctionReports.SprintfError()
	}
	return ""
}

func (pr *ProjectReport) PrintAsTable(t table.Writer, onlyFailed bool) {
	status := pr.getStatus()
	switch status {
	case "failed":
		t.AppendRow(table.Row{"project", pr.Name, pr.getStatus(), pr.Failed.FailReason, pr.Failed.CanBeAutoFixed})
	default:
		if !onlyFailed {
			t.AppendRow(table.Row{"project", pr.Name, pr.getStatus(), "", ""})
		}
	}
	pr.FunctionReports.PrintAsTable(t, onlyFailed)
	t.AppendSeparator()
}

func (pr *ProjectReport) getStatus() string {
	switch {
	case pr.Skipped:
		return "skipped"
	case pr.Failed != nil:
		return "failed"
	case pr.Success:
		return "success"
	default:
		return "unknown"
	}
}

type FunctionReports struct {
	Success []string               `json:"success,omitempty"`
	Failed  map[string]*FailReport `json:"failed,omitempty"`

	mutex sync.Mutex
}

func NewFunctionReports() *FunctionReports {
	return &FunctionReports{
		Failed: make(map[string]*FailReport),
		mutex:  sync.Mutex{},
	}
}

func (fr *FunctionReports) SaveToFile(ctx context.Context, loggerInstance logger.Logger, path string) {
	saveReportToFile(ctx, loggerInstance, fr, path)
}

func (fr *FunctionReports) SprintfError() string {
	report := ""
	for name, failReason := range fr.Failed {
		report += fmt.Sprintf("Failed to import function `%s`. Reason: %s.", name, failReason.FailReason)
	}

	return report
}

func (fr *FunctionReports) PrintAsTable(t table.Writer, onlyFailed bool) {
	t.ResetHeaders()
	t.AppendHeader(table.Row{"type", "name", "status", "fail description", "auto-fixable"})
	if !onlyFailed {
		for _, name := range fr.Success {
			t.AppendRow(table.Row{
				"function", name, "success", "", "",
			})
		}
	}
	for name, failReason := range fr.Failed {
		t.AppendRow(table.Row{
			"function", name, "failed", failReason.FailReason, failReason.CanBeAutoFixed,
		})
	}
}

func (fr *FunctionReports) AddFailure(name string, err error) {
	fr.mutex.Lock()
	defer fr.mutex.Unlock()

	fr.Failed[name] = &FailReport{
		FailReason: errors.RootCause(err).Error(),
	}
}

func (fr *FunctionReports) AddSuccess(name string) {
	fr.mutex.Lock()
	defer fr.mutex.Unlock()

	fr.Success = append(fr.Success, name)
}

type FailReport struct {
	FailReason     string `json:"failReason,omitempty"`
	CanBeAutoFixed bool   `json:"canBeAutoFixed,omitempty"`
}
