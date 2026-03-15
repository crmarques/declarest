// Copyright 2026 Carlos Marques
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package testkit

import (
	"bytes"
	"strings"
	"sync"

	"github.com/spf13/cobra"
)

var executeCommandForTestMu sync.Mutex

func ExecuteCommandForTest(command *cobra.Command, stdin string, args ...string) (string, error) {
	output, _, err := ExecuteCommandForTestWithStreams(command, stdin, args...)
	return output, err
}

func ExecuteCommandForTestWithStreams(command *cobra.Command, stdin string, args ...string) (string, string, error) {
	// Cobra mutates command/flag annotation maps while serving completion and help output.
	// Many CLI tests run in parallel, so serialize execution to avoid test-only races.
	executeCommandForTestMu.Lock()
	defer executeCommandForTestMu.Unlock()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	command.SetOut(stdout)
	command.SetErr(stderr)
	command.SetIn(strings.NewReader(stdin))
	command.SetArgs(args)

	err := command.Execute()
	return stdout.String(), stderr.String(), err
}

func RegisteredPaths(command *cobra.Command, prefix []string) [][]string {
	paths := make([][]string, 0)
	for _, child := range command.Commands() {
		name := child.Name()
		if name == "help" || len(name) > 1 && name[:2] == "__" {
			continue
		}
		current := append(append([]string{}, prefix...), name)
		paths = append(paths, current)
		paths = append(paths, RegisteredPaths(child, current)...)
	}
	return paths
}

func JoinPath(path []string) string {
	if len(path) == 0 {
		return "root"
	}
	joined := path[0]
	for i := 1; i < len(path); i++ {
		joined += " " + path[i]
	}
	return joined
}
