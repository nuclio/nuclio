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

	"github.com/nuclio/logger"
)

type ProjectReports struct {
	Report
	Reports map[string]*ProjectReport
}

func NewProjectReports() *ProjectReports {
	return &ProjectReports{
		Reports: make(map[string]*ProjectReport),
	}
}

func (psr *ProjectReports) AddReport(report *ProjectReport) {
	psr.Reports[report.Name] = report
}

func (psr *ProjectReports) GetReport(projectName string) (report *ProjectReport, exists bool) {
	report, exists = psr.Reports[projectName]
	return
}

func (psr *ProjectReports) SaveToFile(ctx context.Context, loggerInstance logger.Logger, path string) {
	saveReportToFile(ctx, loggerInstance, psr.Reports, path)
}

func (psr *ProjectReports) SprintfError() string {
	report := ""
	for _, projectReport := range psr.Reports {
		report += projectReport.SprintfError()
	}
	return report
}

type ProjectReport struct {
	Report
	Name            string
	Skipped         bool
	Success         bool
	Failed          *FailReport
	FunctionReports *FunctionReports
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

type FunctionReports struct {
	Report
	Success []string
	Failed  map[string]*FailReport

	mutex sync.Mutex
}

func NewFunctionReports() *FunctionReports {
	return &FunctionReports{Failed: make(map[string]*FailReport), mutex: sync.Mutex{}}
}

func (frs *FunctionReports) SaveToFile(ctx context.Context, loggerInstance logger.Logger, path string) {
	saveReportToFile(ctx, loggerInstance, frs, path)
}

func (frs *FunctionReports) SprintfError() string {
	report := ""
	for name, failReason := range frs.Failed {
		report += fmt.Sprintf("Failed to import function `%s`. Reason: %s.", name, failReason.FailReason)
	}

	return report
}

func (frs *FunctionReports) AddFailure(name string, err error) {
	frs.mutex.Lock()
	defer frs.mutex.Unlock()

	frs.Failed[name] = &FailReport{
		FailReason: err.Error(),
	}
}

func (frs *FunctionReports) AddSuccess(name string) {
	frs.mutex.Lock()
	defer frs.mutex.Unlock()

	frs.Success = append(frs.Success, name)
}

type FailReport struct {
	FailReason     string
	CanBeAutoFixed bool
}
