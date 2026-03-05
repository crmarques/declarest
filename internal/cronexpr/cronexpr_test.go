package cronexpr

import (
	"testing"
	"time"
)

func TestParseRejectsInvalidFieldCount(t *testing.T) {
	t.Parallel()

	if _, err := Parse("*/5 * * *"); err == nil {
		t.Fatal("expected invalid field count error, got nil")
	}
}

func TestParseRejectsInvalidToken(t *testing.T) {
	t.Parallel()

	if _, err := Parse("*/5 * * * mon"); err == nil {
		t.Fatal("expected invalid token error, got nil")
	}
}

func TestScheduleNext(t *testing.T) {
	t.Parallel()

	schedule, err := Parse("*/15 * * * *")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	at := time.Date(2026, time.March, 4, 10, 7, 34, 0, time.UTC)
	next, ok := schedule.Next(at)
	if !ok {
		t.Fatal("expected next schedule, got none")
	}

	expected := time.Date(2026, time.March, 4, 10, 15, 0, 0, time.UTC)
	if !next.Equal(expected) {
		t.Fatalf("expected next %s, got %s", expected.Format(time.RFC3339), next.Format(time.RFC3339))
	}
}

func TestDayOfMonthAndDayOfWeekOrSemantics(t *testing.T) {
	t.Parallel()

	schedule, err := Parse("0 0 1 * 1")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	// Monday, but not the first of the month.
	if !schedule.Matches(time.Date(2026, time.March, 2, 0, 0, 0, 0, time.UTC)) {
		t.Fatal("expected monday to match due day-of-week")
	}

	// First day of month, but not Monday.
	if !schedule.Matches(time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatal("expected first day of month to match due day-of-month")
	}
}

func TestSundaySevenAlias(t *testing.T) {
	t.Parallel()

	schedule, err := Parse("0 0 * * 7")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	// Sunday.
	if !schedule.Matches(time.Date(2026, time.March, 8, 0, 0, 0, 0, time.UTC)) {
		t.Fatal("expected sunday match")
	}

	// Monday.
	if schedule.Matches(time.Date(2026, time.March, 9, 0, 0, 0, 0, time.UTC)) {
		t.Fatal("did not expect monday match")
	}
}
