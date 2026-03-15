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

package metadata

import (
	"path"
	"strings"

	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/resource"
)

type PathDescriptor struct {
	Selector     string
	Segments     []string
	Collection   bool
	SelectorMode bool
}

func ParsePathDescriptor(logicalPath string) (PathDescriptor, error) {
	parsedPath, err := resource.ParseRawPathWithOptions(logicalPath, resource.RawPathParseOptions{})
	if err != nil {
		return PathDescriptor{}, err
	}

	segments := append([]string(nil), parsedPath.Segments...)
	for _, segment := range segments {
		if hasWildcardPattern(segment) {
			if _, err := path.Match(segment, "sample"); err != nil {
				return PathDescriptor{}, faults.NewTypedError(
					faults.ValidationError,
					"logical path contains invalid wildcard expression",
					err,
				)
			}
		}
	}

	collectionTarget := parsedPath.Normalized == "/" || parsedPath.ExplicitCollectionTarget
	selectorMode := collectionTarget
	if len(segments) > 0 && segments[len(segments)-1] == "_" {
		collectionTarget = true
		selectorMode = true
		segments = segments[:len(segments)-1]
	}
	for _, segment := range segments {
		if segment == "_" || hasWildcardPattern(segment) {
			collectionTarget = true
			selectorMode = true
		}
	}

	selector := parsedPath.Normalized
	if len(segments) == 0 {
		selector = "/"
	} else if len(segments) != len(parsedPath.Segments) || collectionTarget {
		selector = "/" + joinPathSegments(segments)
	}
	selector = path.Clean(selector)
	if !strings.HasPrefix(selector, "/") {
		return PathDescriptor{}, faults.NewTypedError(
			faults.ValidationError,
			"logical path must be absolute",
			nil,
		)
	}
	if selector != "/" {
		selector = strings.TrimSuffix(selector, "/")
	}

	return PathDescriptor{
		Selector:     selector,
		Segments:     splitPathSegments(selector),
		Collection:   collectionTarget,
		SelectorMode: selectorMode,
	}, nil
}

func joinPathSegments(segments []string) string {
	if len(segments) == 0 {
		return ""
	}
	return strings.Join(segments, "/")
}
