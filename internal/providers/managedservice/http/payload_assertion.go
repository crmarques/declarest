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

package http

import (
	"context"
	"fmt"
	"strings"

	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/resource"
)

func (g *Client) validateOperationAssertions(
	ctx context.Context,
	payload resource.Value,
	assertions []metadata.ValidationAssertion,
) error {
	if len(assertions) == 0 {
		return nil
	}

	for idx, assertion := range assertions {
		expression := strings.TrimSpace(assertion.JQ)
		if expression == "" {
			continue
		}

		code, err := g.compileListJQCode(ctx, expression)
		if err != nil {
			return faults.Invalid(
				fmt.Sprintf("invalid payload validation assertion[%d] jq expression", idx),
				err,
			)
		}

		runCtx := ctx
		if runCtx == nil {
			runCtx = context.Background()
		}

		iterator := code.RunWithContext(runCtx, payload)
		satisfied, evalErr := evaluateAssertionResults(iterator)
		if evalErr != nil {
			return faults.Invalid(
				fmt.Sprintf("failed to evaluate payload validation assertion[%d]", idx),
				evalErr,
			)
		}
		if satisfied {
			continue
		}

		message := strings.TrimSpace(assertion.Message)
		if message == "" {
			message = fmt.Sprintf("payload validation assertion[%d] failed", idx)
		}
		return faults.Invalid(message, nil)
	}

	return nil
}

func evaluateAssertionResults(iterator jqResultIterator) (bool, error) {
	hasResult := false
	satisfied := false

	for {
		value, ok := iterator.Next()
		if !ok {
			break
		}
		if valueErr, isErr := value.(error); isErr {
			return false, valueErr
		}
		hasResult = true
		if jqValueTruthy(value) {
			satisfied = true
		}
	}

	if !hasResult {
		return false, nil
	}
	return satisfied, nil
}

type jqResultIterator interface {
	Next() (any, bool)
}

func jqValueTruthy(value any) bool {
	if value == nil {
		return false
	}
	if typed, ok := value.(bool); ok {
		return typed
	}
	return true
}
