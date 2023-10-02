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
	"net/http"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/common/headers"
	"github.com/nuclio/nuclio/pkg/errgroup"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	nuctlcommon "github.com/nuclio/nuclio/pkg/nuctl/command/common"

	"github.com/nuclio/errors"
	"github.com/nuclio/nuclio-sdk-go"
	"github.com/spf13/cobra"
)

type redeployCommandeer struct {
	cmd            *cobra.Command
	rootCommandeer *RootCommandeer

	betaCommandeer         *betaCommandeer
	outputManifest         *nuctlcommon.PatchManifest
	verifyExternalRegistry bool
	saveReport             bool
	fromReport             bool
	reportFilePath         string
	excludedProjects       []string
	excludedFunctions      []string
	excludeFunctionWithGPU bool
	importedOnly           bool
	waitForFunction        bool
	waitTimeout            time.Duration
	desiredState           string
}

func newRedeployCommandeer(ctx context.Context, rootCommandeer *RootCommandeer, betaCommandeer *betaCommandeer) *redeployCommandeer {
	commandeer := &redeployCommandeer{
		rootCommandeer: rootCommandeer,
		outputManifest: nuctlcommon.NewPatchManifest(),
		betaCommandeer: betaCommandeer,
	}

	cmd := &cobra.Command{
		Use:   "redeploy [<function>]",
		Short: "Redeploy one or more functions",
		Long: `
Redeploy one or more functions. If no function name is specified, 
all functions in the namespace will be redeployed.

Note: This command works on functions that were previously deployed, or imported functions.
	  To deploy a new function, use the 'deploy' command.

Arguments:
  <function> (string) The name of a function to redeploy. Can be specified multiple times.`,
		RunE: func(cmd *cobra.Command, args []string) error {

			// initialize root
			if err := rootCommandeer.initialize(); err != nil {
				return errors.Wrap(err, "Failed to initialize root")
			}
			if err := commandeer.betaCommandeer.initialize(); err != nil {
				return errors.Wrap(err, "Failed to initialize beta commandeer")
			}

			if err := commandeer.redeploy(ctx, args); err != nil {
				return errors.Wrap(err, "Failed to deploy function")
			}
			return nil
		},
	}

	addRedeployFlags(cmd, commandeer)

	commandeer.cmd = cmd

	return commandeer
}

func addRedeployFlags(cmd *cobra.Command,
	commandeer *redeployCommandeer) {
	cmd.Flags().BoolVar(&commandeer.verifyExternalRegistry, "verify-external-registry", false, "verify registry is external")
	cmd.Flags().BoolVar(&commandeer.saveReport, "save-report", false, "Save redeployment report to a file")
	cmd.Flags().BoolVar(&commandeer.fromReport, "from-report", false, "Redeploy failed and retryable functions from the given report file (if arguments are also given, they will be redeployed as well)")
	cmd.Flags().StringVar(&commandeer.reportFilePath, "report-file-path", "nuctl-redeployment-report.json", "Path to redeployment report")
	cmd.Flags().StringSliceVar(&commandeer.excludedProjects, "exclude-projects", []string{}, "Exclude projects to patch")
	cmd.Flags().StringSliceVar(&commandeer.excludedFunctions, "exclude-functions", []string{}, "Exclude functions to patch")
	cmd.Flags().BoolVar(&commandeer.excludeFunctionWithGPU, "exclude-functions-with-gpu", false, "Skip functions with GPU")
	cmd.Flags().BoolVar(&commandeer.importedOnly, "imported-only", false, "Deploy only imported functions")
	cmd.Flags().BoolVarP(&commandeer.waitForFunction, "wait", "w", false, "Wait for function deployment to complete")
	cmd.Flags().DurationVar(&commandeer.waitTimeout, "wait-timeout", 15*time.Minute, "Wait timeout duration for the function deployment, e.g 30s, 5m")
	cmd.Flags().StringVar(&commandeer.desiredState, "desired-state", "ready", "Desired function state")
}

func (d *redeployCommandeer) redeploy(ctx context.Context, args []string) error {

	if d.fromReport {
		manifest, err := nuctlcommon.NewPatchManifestFromFile(d.reportFilePath)
		if err != nil {
			return errors.Wrap(err, "Problem with reading report file")
		}
		retryableFunctions := manifest.GetRetryableFunctionNames()
		if len(retryableFunctions) != 0 {
			d.rootCommandeer.loggerInstance.InfoWith("Found retryable functions in report file",
				"reportFile", d.reportFilePath,
				"functions", retryableFunctions)
			args = common.RemoveDuplicatesFromSliceString(append(args, retryableFunctions...))
		} else {
			d.rootCommandeer.loggerInstance.InfoWith("No retryable functions found in report file",
				"reportFile", d.reportFilePath)
		}
	}

	if len(args) == 0 {

		// redeploy all functions in the namespace
		if err := d.redeployAllFunctions(ctx); err != nil {
			return errors.Wrap(err, "Failed to redeploy all functions")
		}
	} else {

		// redeploy the given functions
		if err := d.redeployFunctions(ctx, args); err != nil {
			return errors.Wrap(err, "Failed to redeploy functions")
		}
	}

	return nil
}

func (d *redeployCommandeer) redeployAllFunctions(ctx context.Context) error {
	d.rootCommandeer.loggerInstance.DebugWithCtx(ctx, "Redeploying all functions")

	// get function names to redeploy
	functionNames, err := d.getFunctionNames(ctx)
	if err != nil {
		return errors.Wrap(err, "Failed to get functions")
	}

	return d.redeployFunctions(ctx, functionNames)
}

func (d *redeployCommandeer) getFunctionNames(ctx context.Context) ([]string, error) {
	d.rootCommandeer.loggerInstance.DebugWithCtx(ctx, "Getting function names")

	functionConfigs, err := d.betaCommandeer.apiClient.GetFunctions(ctx, d.rootCommandeer.namespace)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get functions")
	}

	functionNames := make([]string, 0)
	for functionName, functionConfigWithStatus := range functionConfigs {

		// filter excluded functions
		if d.shouldSkipFunction(functionConfigWithStatus.Config) {
			d.rootCommandeer.loggerInstance.DebugWithCtx(ctx, "Excluding function", "function", functionName)
			d.outputManifest.AddSkipped(functionName)
			continue
		}
		functionNames = append(functionNames, functionName)
	}

	return functionNames, nil
}

func (d *redeployCommandeer) redeployFunctions(ctx context.Context, functionNames []string) error {
	d.rootCommandeer.loggerInstance.InfoWithCtx(ctx, "Redeploying functions", "functionNames", functionNames)

	if !functionconfig.FunctionStateInSlice(functionconfig.FunctionState(d.desiredState),
		[]functionconfig.FunctionState{
			functionconfig.FunctionStateReady,
			functionconfig.FunctionStateScaledToZero,
		}) {
		return errors.New("Desired status is not allowed to be set")
	}

	patchErrGroup, _ := errgroup.WithContextSemaphore(ctx, d.rootCommandeer.loggerInstance, uint(d.betaCommandeer.concurrency))
	for _, function := range functionNames {
		function := function
		patchErrGroup.Go("patch function", func() error {
			if err := d.patchFunction(ctx, function, d.desiredState); err != nil {
				d.outputManifest.AddFailure(function, err, d.isRedeploymentRetryable(err))
				return errors.Wrap(err, "Failed to patch function")
			}
			d.outputManifest.AddSuccess(function)
			return nil
		})
	}

	if err := patchErrGroup.Wait(); err != nil {

		// Functions that failed to patch are included in the output manifest,
		// so we don't need to fail the entire operation here
		d.rootCommandeer.loggerInstance.WarnWithCtx(ctx, "Failed to patch functions", "err", err)
	}

	d.outputManifest.LogOutput(ctx, d.rootCommandeer.loggerInstance)
	if d.saveReport {
		d.outputManifest.SaveToFile(ctx, d.rootCommandeer.loggerInstance, d.reportFilePath)
	}

	return nil
}

// patchFunction patches a single function
func (d *redeployCommandeer) patchFunction(ctx context.Context, functionName string, desiredState string) error {

	d.rootCommandeer.loggerInstance.InfoWithCtx(ctx, "Redeploying function", "function", functionName)

	// patch function
	patchOptions := map[string]string{
		"desiredState": desiredState,
	}

	payload, err := json.Marshal(patchOptions)
	if err != nil {
		return errors.Wrap(err, "Failed to marshal payload")
	}

	requestHeaders := d.resolveRequestHeaders()

	if err := d.betaCommandeer.apiClient.PatchFunction(ctx,
		functionName,
		d.rootCommandeer.namespace,
		payload,
		requestHeaders); err != nil {
		switch typedError := err.(type) {
		case *nuclio.ErrorWithStatusCode:
			return nuclio.GetWrapByStatusCode(typedError.StatusCode())(errors.Wrap(err, "Failed to patch function"))
		default:
			return errors.Wrap(typedError, "Failed to patch function")
		}
	}

	d.rootCommandeer.loggerInstance.DebugWithCtx(ctx,
		"Function redeploy request sent successfully",
		"function", functionName)

	if d.waitForFunction {
		return d.waitForFunctionDeployment(ctx, functionName)
	}

	return nil
}

func (d *redeployCommandeer) waitForFunctionDeployment(ctx context.Context, functionName string) error {
	var waitError error
	err := common.RetryUntilSuccessful(d.waitTimeout, 5*time.Second, func() bool {
		isTerminal, err := d.isFunctionInTerminalState(ctx, functionName)
		if !isTerminal {
			// function isn't in terminal state yet, retry
			return false
		}
		if err != nil {
			// function is terminal, but not ready, stop and return error
			waitError = err
			return true
		}

		// function is ready
		return true
	})
	if waitError != nil {
		return waitError
	}
	if err != nil {
		return errors.New(fmt.Sprintf("Timed out waiting for function '%s' to be ready", functionName))
	}
	return nil
}

// isFunctionInTerminalState checks if the function is in terminal state
// if the function is ready, it returns true and no error
// if the function is in another terminal state, it returns true and an error
// else it returns false
func (d *redeployCommandeer) isFunctionInTerminalState(ctx context.Context, functionName string) (bool, error) {

	// get function and poll its status
	function, err := d.betaCommandeer.apiClient.GetFunction(ctx, functionName, d.rootCommandeer.namespace)
	if err != nil {
		d.rootCommandeer.loggerInstance.WarnWithCtx(ctx, "Failed to get function", "functionName", functionName)
		return false, err
	}
	if function.Status.State == functionconfig.FunctionStateReady {
		d.rootCommandeer.loggerInstance.InfoWithCtx(ctx,
			"Function redeployed successfully",
			"functionName", functionName)
		return true, nil
	}

	// we use this function to check if the function is in terminal state, as we already checked if it's ready
	if functionconfig.FunctionStateInSlice(function.Status.State,
		[]functionconfig.FunctionState{
			functionconfig.FunctionStateError,
			functionconfig.FunctionStateUnhealthy,
			functionconfig.FunctionStateScaledToZero,
		}) {
		return true, errors.New(fmt.Sprintf("Function '%s' is in terminal state '%s' but not ready",
			functionName, function.Status.State))
	}

	d.rootCommandeer.loggerInstance.DebugWithCtx(ctx,
		"Function not ready yet",
		"functionName", functionName,
		"functionState", function.Status.State)

	return false, nil
}

// shouldSkipFunction returns true if the function patch should be skipped
func (d *redeployCommandeer) shouldSkipFunction(functionConfig functionconfig.Config) bool {
	functionName := functionConfig.Meta.Name
	projectName := functionConfig.Meta.Labels[common.NuclioResourceLabelKeyProjectName]

	return common.StringSliceContainsString(d.excludedFunctions, functionName) ||
		common.StringSliceContainsString(d.excludedProjects, projectName) ||
		(d.excludeFunctionWithGPU && functionConfig.Spec.PositiveGPUResourceLimit())
}

func (d *redeployCommandeer) resolveRequestHeaders() map[string]string {
	requestHeaders := map[string]string{}
	if d.importedOnly {

		// add a header that will tell the API to only deploy imported functions
		requestHeaders[headers.ImportedFunctionOnly] = "true"
	}
	if d.verifyExternalRegistry {
		requestHeaders[headers.VerifyExternalRegistry] = "true"
	}
	return requestHeaders
}

func (d *redeployCommandeer) isRedeploymentRetryable(err error) bool {
	switch typedError := err.(type) {
	case *nuclio.ErrorWithStatusCode:
		// if the status code is 412, then another redeployment will not help because there is something wrong with the configuration
		return typedError.StatusCode() != http.StatusPreconditionFailed
	case error:
		return true
	}
	return true
}
