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
	saveReportToFile(ctx, loggerInstance, pr.Reports, path)
}

func (pr *ProjectReports) SprintfError() string {
	report := ""
	for _, projectReport := range pr.Reports {
		report += projectReport.SprintfError()
	}
	return report
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

func (fr *FunctionReports) AddFailure(name string, err error, canBeAutoFixed bool) {
	fr.mutex.Lock()
	defer fr.mutex.Unlock()

	fr.Failed[name] = &FailReport{
		CanBeAutoFixed: canBeAutoFixed,
		FailReason:     errors.RootCause(err).Error(),
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
