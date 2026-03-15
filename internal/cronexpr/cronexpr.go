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

package cronexpr

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

type field struct {
	min      int
	max      int
	allowed  []bool
	wildcard bool
}

func (f field) has(value int) bool {
	if value < f.min || value > f.max {
		return false
	}
	return f.allowed[value]
}

// Schedule represents a standard 5-field cron schedule:
// minute hour day-of-month month day-of-week.
type Schedule struct {
	minute     field
	hour       field
	dayOfMonth field
	month      field
	dayOfWeek  field
}

// Parse parses a standard 5-field cron expression.
func Parse(raw string) (Schedule, error) {
	parts := strings.Fields(strings.TrimSpace(raw))
	if len(parts) != 5 {
		return Schedule{}, fmt.Errorf("expected 5 cron fields, got %d", len(parts))
	}

	minute, err := parseField(parts[0], 0, 59, false)
	if err != nil {
		return Schedule{}, fmt.Errorf("minute: %w", err)
	}
	hour, err := parseField(parts[1], 0, 23, false)
	if err != nil {
		return Schedule{}, fmt.Errorf("hour: %w", err)
	}
	dayOfMonth, err := parseField(parts[2], 1, 31, false)
	if err != nil {
		return Schedule{}, fmt.Errorf("day-of-month: %w", err)
	}
	month, err := parseField(parts[3], 1, 12, false)
	if err != nil {
		return Schedule{}, fmt.Errorf("month: %w", err)
	}
	dayOfWeek, err := parseField(parts[4], 0, 6, true)
	if err != nil {
		return Schedule{}, fmt.Errorf("day-of-week: %w", err)
	}

	return Schedule{
		minute:     minute,
		hour:       hour,
		dayOfMonth: dayOfMonth,
		month:      month,
		dayOfWeek:  dayOfWeek,
	}, nil
}

// Matches reports whether the schedule matches the provided time.
func (s Schedule) Matches(at time.Time) bool {
	value := at.UTC()
	if !s.minute.has(value.Minute()) {
		return false
	}
	if !s.hour.has(value.Hour()) {
		return false
	}
	if !s.month.has(int(value.Month())) {
		return false
	}

	dayOfMonthMatch := s.dayOfMonth.has(value.Day())
	dayOfWeekMatch := s.dayOfWeek.has(int(value.Weekday()))

	switch {
	case s.dayOfMonth.wildcard && s.dayOfWeek.wildcard:
		return true
	case s.dayOfMonth.wildcard:
		return dayOfWeekMatch
	case s.dayOfWeek.wildcard:
		return dayOfMonthMatch
	default:
		return dayOfMonthMatch || dayOfWeekMatch
	}
}

// Next returns the first schedule time strictly after the provided time.
func (s Schedule) Next(after time.Time) (time.Time, bool) {
	candidate := after.UTC().Truncate(time.Minute).Add(time.Minute)
	limit := candidate.AddDate(5, 0, 0)
	for !candidate.After(limit) {
		if s.Matches(candidate) {
			return candidate, true
		}
		candidate = candidate.Add(time.Minute)
	}
	return time.Time{}, false
}

func parseField(raw string, min int, max int, allowSundaySeven bool) (field, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return field{}, fmt.Errorf("field is empty")
	}

	out := field{
		min:      min,
		max:      max,
		allowed:  make([]bool, max+1),
		wildcard: value == "*",
	}

	parts := strings.Split(value, ",")
	for _, part := range parts {
		if err := addFieldPart(strings.TrimSpace(part), min, max, allowSundaySeven, out.allowed); err != nil {
			return field{}, err
		}
	}

	hasValue := false
	for idx := min; idx <= max; idx++ {
		if out.allowed[idx] {
			hasValue = true
			break
		}
	}
	if !hasValue {
		return field{}, fmt.Errorf("field has no valid values")
	}

	return out, nil
}

func addFieldPart(raw string, min int, max int, allowSundaySeven bool, allowed []bool) error {
	if raw == "" {
		return fmt.Errorf("invalid empty list item")
	}

	base := raw
	step := 1
	if strings.Contains(raw, "/") {
		pieces := strings.Split(raw, "/")
		if len(pieces) != 2 {
			return fmt.Errorf("invalid step syntax %q", raw)
		}
		base = strings.TrimSpace(pieces[0])
		stepRaw := strings.TrimSpace(pieces[1])
		parsedStep, err := strconv.Atoi(stepRaw)
		if err != nil || parsedStep <= 0 {
			return fmt.Errorf("invalid step value %q", stepRaw)
		}
		step = parsedStep
	}

	rangeStart := 0
	rangeEnd := 0
	switch {
	case base == "*":
		rangeStart = min
		rangeEnd = max
	case strings.Contains(base, "-"):
		rangePieces := strings.Split(base, "-")
		if len(rangePieces) != 2 {
			return fmt.Errorf("invalid range syntax %q", base)
		}
		left := strings.TrimSpace(rangePieces[0])
		right := strings.TrimSpace(rangePieces[1])
		if allowSundaySeven && (left == "7" || right == "7") {
			return fmt.Errorf("use 0 for sunday in ranges")
		}
		start, err := parseFieldValue(left, min, max, allowSundaySeven)
		if err != nil {
			return err
		}
		end, err := parseFieldValue(right, min, max, allowSundaySeven)
		if err != nil {
			return err
		}
		if start > end {
			return fmt.Errorf("range start must be <= end")
		}
		rangeStart = start
		rangeEnd = end
	default:
		single, err := parseFieldValue(base, min, max, allowSundaySeven)
		if err != nil {
			return err
		}
		rangeStart = single
		rangeEnd = single
	}

	for value := rangeStart; value <= rangeEnd; value += step {
		allowed[value] = true
	}
	return nil
}

func parseFieldValue(raw string, min int, max int, allowSundaySeven bool) (int, error) {
	parsed, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0, fmt.Errorf("invalid number %q", raw)
	}
	if allowSundaySeven && parsed == 7 {
		parsed = 0
	}
	if parsed < min || parsed > max {
		return 0, fmt.Errorf("value %d out of range [%d,%d]", parsed, min, max)
	}
	return parsed, nil
}
