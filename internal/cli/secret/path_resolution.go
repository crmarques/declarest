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

package secret

import (
	"fmt"
	"strings"

	"github.com/crmarques/declarest/internal/cli/cliutil"
	"github.com/crmarques/declarest/resource"
)

type secretTarget struct {
	Path string
	Key  string
}

func (r secretTarget) ResolvedKey() string {
	if strings.TrimSpace(r.Path) == "" {
		return strings.TrimSpace(r.Key)
	}
	return strings.TrimSpace(r.Path) + ":" + strings.TrimSpace(r.Key)
}

type secretSetRequest struct {
	Target secretTarget
	Value  string
}

type secretListRequest struct {
	Path      string
	HasPath   bool
	Recursive bool
}

func resolveSecretTargetRequest(action string, pathFlag string, keyFlag string, args []string) (secretTarget, error) {
	normalizedPathFlag, hasPathFlag, err := normalizeSecretPathFlag(pathFlag)
	if err != nil {
		return secretTarget{}, err
	}
	normalizedKeyFlag := strings.TrimSpace(keyFlag)
	if normalizedKeyFlag != "" && !hasPathFlag {
		return secretTarget{}, cliutil.ValidationError("--key requires --path", nil)
	}

	switch len(args) {
	case 0:
		if !hasPathFlag {
			return secretTarget{}, cliutil.ValidationError(
				fmt.Sprintf("secret %s requires a key; use 'declarest secret list <path>' to inspect path-scoped keys", action),
				nil,
			)
		}
		if normalizedKeyFlag == "" {
			return secretTarget{}, cliutil.ValidationError(
				fmt.Sprintf("secret %s requires a key; use 'declarest secret list %s' to inspect available keys", action, normalizedPathFlag),
				nil,
			)
		}
		return secretTarget{Path: normalizedPathFlag, Key: normalizedKeyFlag}, nil
	case 1:
		return resolveSecretTargetFromSingleArg(action, normalizedPathFlag, hasPathFlag, normalizedKeyFlag, args[0])
	case 2:
		return resolveSecretTargetFromPathAndKeyArgs(action, normalizedPathFlag, hasPathFlag, normalizedKeyFlag, args[0], args[1])
	default:
		return secretTarget{}, cliutil.ValidationError(
			fmt.Sprintf("secret %s accepts at most two positional arguments", action),
			nil,
		)
	}
}

func resolveSecretTargetFromSingleArg(
	action string,
	pathFlag string,
	hasPathFlag bool,
	keyFlag string,
	rawArg string,
) (secretTarget, error) {
	arg := strings.TrimSpace(rawArg)
	if arg == "" {
		return secretTarget{}, cliutil.ValidationError(fmt.Sprintf("secret %s argument must not be empty", action), nil)
	}

	if hasPathFlag {
		if keyFlag != "" && keyFlag != arg {
			return secretTarget{}, cliutil.ValidationError("flag --key conflicts with positional key argument", nil)
		}
		if keyFlag != "" {
			return secretTarget{Path: pathFlag, Key: keyFlag}, nil
		}
		return secretTarget{Path: pathFlag, Key: arg}, nil
	}

	if keyFlag != "" {
		return secretTarget{}, cliutil.ValidationError("--key requires --path", nil)
	}

	if strings.HasPrefix(arg, "/") {
		pathFromComposite, keyFromComposite, composite := splitSecretPathKeyArg(arg)
		if composite {
			return secretTarget{Path: pathFromComposite, Key: keyFromComposite}, nil
		}

		normalizedPathArg, err := normalizeSecretPathForInput(arg)
		if err != nil {
			return secretTarget{}, err
		}
		return secretTarget{}, cliutil.ValidationError(
			fmt.Sprintf("secret %s requires a key; use 'declarest secret list %s' to inspect available keys", action, normalizedPathArg),
			nil,
		)
	}

	return secretTarget{Key: arg}, nil
}

func resolveSecretTargetFromPathAndKeyArgs(
	_ string,
	pathFlag string,
	hasPathFlag bool,
	keyFlag string,
	rawPathArg string,
	rawKeyArg string,
) (secretTarget, error) {
	normalizedPathArg, err := normalizeSecretPathForInput(rawPathArg)
	if err != nil {
		return secretTarget{}, err
	}

	keyArg := strings.TrimSpace(rawKeyArg)
	if keyArg == "" {
		return secretTarget{}, cliutil.ValidationError("secret key must not be empty", nil)
	}

	if hasPathFlag && pathFlag != normalizedPathArg {
		return secretTarget{}, cliutil.ValidationError("flag --path conflicts with positional path argument", nil)
	}
	if keyFlag != "" && keyFlag != keyArg {
		return secretTarget{}, cliutil.ValidationError("flag --key conflicts with positional key argument", nil)
	}

	if hasPathFlag {
		normalizedPathArg = pathFlag
	}
	if keyFlag != "" {
		keyArg = keyFlag
	}

	return secretTarget{Path: normalizedPathArg, Key: keyArg}, nil
}

func resolveSecretSetRequest(pathFlag string, keyFlag string, args []string) (secretSetRequest, error) {
	normalizedPathFlag, hasPathFlag, err := normalizeSecretPathFlag(pathFlag)
	if err != nil {
		return secretSetRequest{}, err
	}
	normalizedKeyFlag := strings.TrimSpace(keyFlag)
	if normalizedKeyFlag != "" && !hasPathFlag {
		return secretSetRequest{}, cliutil.ValidationError("--key requires --path", nil)
	}

	switch len(args) {
	case 1:
		if !hasPathFlag || normalizedKeyFlag == "" {
			return secretSetRequest{}, cliutil.ValidationError(
				"secret set requires <key> <value> or <path> <key> <value>",
				nil,
			)
		}
		return secretSetRequest{
			Target: secretTarget{Path: normalizedPathFlag, Key: normalizedKeyFlag},
			Value:  args[0],
		}, nil
	case 2:
		return resolveSecretSetFromTwoArgs(normalizedPathFlag, hasPathFlag, normalizedKeyFlag, args[0], args[1])
	case 3:
		normalizedPathArg, err := normalizeSecretPathForInput(args[0])
		if err != nil {
			return secretSetRequest{}, err
		}
		keyArg := strings.TrimSpace(args[1])
		if keyArg == "" {
			return secretSetRequest{}, cliutil.ValidationError("secret key must not be empty", nil)
		}

		if hasPathFlag && normalizedPathFlag != normalizedPathArg {
			return secretSetRequest{}, cliutil.ValidationError("flag --path conflicts with positional path argument", nil)
		}
		if normalizedKeyFlag != "" && normalizedKeyFlag != keyArg {
			return secretSetRequest{}, cliutil.ValidationError("flag --key conflicts with positional key argument", nil)
		}

		if hasPathFlag {
			normalizedPathArg = normalizedPathFlag
		}
		if normalizedKeyFlag != "" {
			keyArg = normalizedKeyFlag
		}

		return secretSetRequest{
			Target: secretTarget{Path: normalizedPathArg, Key: keyArg},
			Value:  args[2],
		}, nil
	default:
		return secretSetRequest{}, cliutil.ValidationError("secret set accepts at most three positional arguments", nil)
	}
}

func resolveSecretSetFromTwoArgs(
	pathFlag string,
	hasPathFlag bool,
	keyFlag string,
	rawTargetArg string,
	valueArg string,
) (secretSetRequest, error) {
	targetArg := strings.TrimSpace(rawTargetArg)
	if targetArg == "" {
		return secretSetRequest{}, cliutil.ValidationError("secret key must not be empty", nil)
	}

	if hasPathFlag {
		if keyFlag != "" && keyFlag != targetArg {
			return secretSetRequest{}, cliutil.ValidationError("flag --key conflicts with positional key argument", nil)
		}
		if keyFlag == "" {
			keyFlag = targetArg
		}

		return secretSetRequest{
			Target: secretTarget{Path: pathFlag, Key: keyFlag},
			Value:  valueArg,
		}, nil
	}

	if strings.HasPrefix(targetArg, "/") {
		pathFromComposite, keyFromComposite, composite := splitSecretPathKeyArg(targetArg)
		if composite {
			return secretSetRequest{
				Target: secretTarget{Path: pathFromComposite, Key: keyFromComposite},
				Value:  valueArg,
			}, nil
		}

		if _, err := normalizeSecretPathForInput(targetArg); err != nil {
			return secretSetRequest{}, err
		}
		return secretSetRequest{}, cliutil.ValidationError(
			"secret set requires a key; use 'declarest secret set <path> <key> <value>'",
			nil,
		)
	}

	return secretSetRequest{
		Target: secretTarget{Key: targetArg},
		Value:  valueArg,
	}, nil
}

func resolveSecretListRequest(pathFlag string, recursive bool, args []string) (secretListRequest, error) {
	normalizedPathFlag, hasPathFlag, err := normalizeSecretPathFlag(pathFlag)
	if err != nil {
		return secretListRequest{}, err
	}

	switch len(args) {
	case 0:
		return secretListRequest{Path: normalizedPathFlag, HasPath: hasPathFlag, Recursive: recursive}, nil
	case 1:
		arg := strings.TrimSpace(args[0])
		if arg == "" {
			return secretListRequest{}, cliutil.ValidationError("secret list path must not be empty", nil)
		}

		if strings.HasPrefix(arg, "/") {
			if _, _, composite := splitSecretPathKeyArg(arg); composite {
				return secretListRequest{}, cliutil.ValidationError(
					"secret list accepts only a path; use 'declarest secret get <path>:<key>' to read a value",
					nil,
				)
			}
		}

		normalizedPathArg, err := normalizeSecretPathForInput(arg)
		if err != nil {
			return secretListRequest{}, err
		}
		if hasPathFlag && normalizedPathFlag != normalizedPathArg {
			return secretListRequest{}, cliutil.ValidationError("flag --path conflicts with positional path argument", nil)
		}
		return secretListRequest{Path: normalizedPathArg, HasPath: true, Recursive: recursive}, nil
	default:
		return secretListRequest{}, cliutil.ValidationError("secret list accepts at most one positional path argument", nil)
	}
}

func normalizeSecretPathFlag(pathFlag string) (string, bool, error) {
	trimmed := strings.TrimSpace(pathFlag)
	if trimmed == "" {
		return "", false, nil
	}
	normalized, err := normalizeSecretPathForInput(trimmed)
	if err != nil {
		return "", true, err
	}
	return normalized, true, nil
}

func normalizeSecretPathForInput(rawPath string) (string, error) {
	trimmed := strings.TrimSpace(rawPath)
	if trimmed == "" {
		return "", cliutil.ValidationError("path is required", nil)
	}
	if !strings.HasPrefix(trimmed, "/") {
		return "", cliutil.ValidationError("path must be absolute", nil)
	}
	return resource.NormalizeLogicalPath(trimmed)
}

func splitSecretPathKeyArg(value string) (string, string, bool) {
	if !strings.HasPrefix(value, "/") {
		return "", "", false
	}

	index := strings.Index(value, ":")
	if index <= 0 {
		return "", "", false
	}

	pathPart := strings.TrimSpace(value[:index])
	keyPart := strings.TrimSpace(value[index+1:])
	if keyPart == "" {
		return "", "", false
	}

	normalizedPath, err := normalizeSecretPathForInput(pathPart)
	if err != nil {
		return "", "", false
	}

	return normalizedPath, keyPart, true
}

func splitStoredSecretPathKey(value string) (string, string, bool) {
	index := strings.Index(value, ":")
	if index <= 0 {
		return "", "", false
	}

	pathPart := strings.Trim(strings.TrimSpace(value[:index]), "/")
	keyPart := strings.TrimSpace(value[index+1:])
	if pathPart == "" || keyPart == "" {
		return "", "", false
	}

	return pathPart, keyPart, true
}

func normalizeSecretStoreLookupKey(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	return strings.Trim(trimmed, "/")
}
