/*
Copyright 2017 The Nuclio Authors.

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

package nucliobuild

import (
	"fmt"
	"path/filepath"

	"github.com/nuclio/nuclio/pkg/nuclio-build/build"
	"github.com/nuclio/nuclio/pkg/zap"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

func NewNuclioBuildCommand() *cobra.Command {
	var options build.Options
	var loggerLevel nucliozap.Level

	cmd := &cobra.Command{
		Use:   "nuclio-build PATH",
		Short: "Build a nuclio function",
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error

			switch len(args) {
			case 0:
				return fmt.Errorf("Missing function path")
			case 1: /* noop */
			default:
				return fmt.Errorf("Too many arguments")
			}

			options.FunctionPath, err = filepath.Abs(filepath.Clean(args[0]))
			if err != nil {
				return err
			}

			if options.OutputType != "docker" && options.OutputType != "binary" {
				return fmt.Errorf("output can only be 'docker' or 'binary' (provided: %s)", options.OutputType)
			}

			if options.Verbose {
				loggerLevel = nucliozap.DebugLevel
			} else {
				loggerLevel = nucliozap.InfoLevel
			}

			zap, err := nucliozap.NewNuclioZap("cmd", loggerLevel)
			if err != nil {
				return errors.Wrap(err, "Failed to create logger")
			}

			return build.NewBuilder(zap, &options).Build()
		},
	}

	cmd.PersistentFlags().BoolVarP(&options.Verbose, "verbose", "", false, "verbose output")
	cmd.Flags().StringVarP(&options.OutputType, "output", "o", "docker", "Build output type - docker|binary")
	cmd.Flags().StringVarP(&options.OutputName, "name", "n", "", "Generated output name (depending on type)")
	cmd.Flags().StringVarP(&options.Version, "version", "v", "latest", "Tag the output with version")
	cmd.Flags().StringVarP(&options.NuclioSourceDir, "nuclio-src-dir", "", "", "Rather than cloning nuclio, use source at a local directory")
	cmd.Flags().StringVarP(&options.NuclioSourceURL, "nuclio-src-url", "", "git@github.com:nuclio/nuclio.git", "Clone nuclio from the provided url")
	cmd.Flags().StringVarP(&options.PushRegistry, "push", "p", "", "URL of registry to push to")

	return cmd
}
