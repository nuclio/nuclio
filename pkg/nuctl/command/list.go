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

package command

import (
	"fmt"
	"sort"

	"github.com/nuclio/nuclio/pkg/processor/runtime"
	"github.com/nuclio/nuclio/pkg/processor/trigger"

	// This loads all triggers and runtimes
	_ "github.com/nuclio/nuclio/cmd/processor/app"

	"github.com/spf13/cobra"
)

type listCommandeer struct {
	cmd            *cobra.Command
	rootCommandeer *RootCommandeer
}

type WithKinds interface {
	GetKinds() []string
}

func printItems(name string, withKinds WithKinds) {
	fmt.Printf("%s:\n", name)
	kinds := withKinds.GetKinds()
	sort.Strings(kinds)
	for _, kind := range kinds {
		fmt.Printf("\t%s\n", kind)
	}
}

func newListCommandeer(rootCommandeer *RootCommandeer) *listCommandeer {
	commandeer := &listCommandeer{
		rootCommandeer: rootCommandeer,
	}

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "Display runtimes & triggers",
		RunE: func(cmd *cobra.Command, args []string) error {
			printItems("runtimes", &runtime.RegistrySingleton)
			printItems("triggers", &trigger.RegistrySingleton)
			return nil
		},
	}

	commandeer.cmd = cmd

	return commandeer
}
