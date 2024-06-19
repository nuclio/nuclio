/*
Copyright 2024 The Nuclio Authors.

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

package main

import (
	"fmt"
	"log"
	"os"

	"github.com/nuclio/nuclio/pkg/nuctl/command"

	"github.com/spf13/cobra/doc"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: generate_docs <output_path>")
		os.Exit(1)
	}

	outputPath := os.Args[1]
	cmd := command.NewRootCommandeer().GetCmd()
	cmd.DisableAutoGenTag = true

	if err := doc.GenMarkdownTree(cmd, outputPath); err != nil {
		log.Fatalf("Failed to generate Markdown documentation: %v", err)
	}

	fmt.Printf("Documentation generated at: %s\n", outputPath)
}
