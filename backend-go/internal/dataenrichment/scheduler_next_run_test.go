package dataenrichment

import (
	"testing"
	"time"
)

func TestNextWeeklyLifelineTime_Midweek(t *testing.T) {
	// July 1, 2026 is Wednesday. Next Monday is July 6 at 03:00 CST.
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Skipf("cannot load Asia/Shanghai: %v", err)
	}
	now := time.Date(2026, 7, 1, 12, 0, 0, 0, loc)
	next := NextWeeklyLifelineTime(now)
	expected := time.Date(2026, 7, 6, 3, 0, 0, 0, loc)
	if !next.Equal(expected) {
		t.Fatalf("expected %s, got %s", expected, next)
	}
}

func TestNextWeeklyLifelineTime_MondayBefore3AM(t *testing.T) {
	// Monday 02:00 → next trigger at 03:00 same day.
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Skipf("cannot load Asia/Shanghai: %v", err)
	}
	now := time.Date(2026, 7, 6, 2, 0, 0, 0, loc)
	next := NextWeeklyLifelineTime(now)
	expected := time.Date(2026, 7, 6, 3, 0, 0, 0, loc)
	if !next.Equal(expected) {
		t.Fatalf("expected %s, got %s", expected, next)
	}
}

func TestNextWeeklyLifelineTime_MondayAfter3AM(t *testing.T) {
	// Monday 03:01 → next trigger at 03:00 next Monday.
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Skipf("cannot load Asia/Shanghai: %v", err)
	}
	now := time.Date(2026, 7, 6, 3, 1, 0, 0, loc)
	next := NextWeeklyLifelineTime(now)
	expected := time.Date(2026, 7, 13, 3, 0, 0, 0, loc)
	if !next.Equal(expected) {
		t.Fatalf("expected %s, got %s", expected, next)
	}
}

func TestNextMonthlyLifelineTime_MidMonth(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Skipf("cannot load Asia/Shanghai: %v", err)
	}
	// July 15 → next is Aug 1 at 03:30.
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, loc)
	next := NextMonthlyLifelineTime(now)
	expected := time.Date(2026, 8, 1, 3, 30, 0, 0, loc)
	if !next.Equal(expected) {
		t.Fatalf("expected %s, got %s", expected, next)
	}
}

func TestNextMonthlyLifelineTime_FirstBeforeTrigger(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Skipf("cannot load Asia/Shanghai: %v", err)
	}
	// Aug 1 at 02:00 → next is Aug 1 at 03:30.
	now := time.Date(2026, 8, 1, 2, 0, 0, 0, loc)
	next := NextMonthlyLifelineTime(now)
	expected := time.Date(2026, 8, 1, 3, 30, 0, 0, loc)
	if !next.Equal(expected) {
		t.Fatalf("expected %s, got %s", expected, next)
	}
}

func TestNextMonthlyLifelineTime_FirstAfterTrigger(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Skipf("cannot load Asia/Shanghai: %v", err)
	}
	// Aug 1 at 04:00 → next is Sep 1 at 03:30.
	now := time.Date(2026, 8, 1, 4, 0, 0, 0, loc)
	next := NextMonthlyLifelineTime(now)
	expected := time.Date(2026, 9, 1, 3, 30, 0, 0, loc)
	if !next.Equal(expected) {
		t.Fatalf("expected %s, got %s", expected, next)
	}
}

func TestNextMonthlyLifelineTime_YearCross(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Skipf("cannot load Asia/Shanghai: %v", err)
	}
	// Dec 25 → next is Jan 1 at 03:30.
	now := time.Date(2026, 12, 25, 12, 0, 0, 0, loc)
	next := NextMonthlyLifelineTime(now)
	expected := time.Date(2027, 1, 1, 3, 30, 0, 0, loc)
	if !next.Equal(expected) {
		t.Fatalf("expected %s, got %s", expected, next)
	}
}

func TestNextYearlyLifelineTime_MidYear(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Skipf("cannot load Asia/Shanghai: %v", err)
	}
	// Jul 5, 2026 → next is Jan 1, 2027 at 04:00.
	now := time.Date(2026, 7, 5, 12, 0, 0, 0, loc)
	next := NextYearlyLifelineTime(now)
	expected := time.Date(2027, 1, 1, 4, 0, 0, 0, loc)
	if !next.Equal(expected) {
		t.Fatalf("expected %s, got %s", expected, next)
	}
}

func TestNextYearlyLifelineTime_Jan1Before4AM(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Skipf("cannot load Asia/Shanghai: %v", err)
	}
	// Jan 1, 2027 at 03:00 → next is Jan 1, 2027 at 04:00.
	now := time.Date(2027, 1, 1, 3, 0, 0, 0, loc)
	next := NextYearlyLifelineTime(now)
	expected := time.Date(2027, 1, 1, 4, 0, 0, 0, loc)
	if !next.Equal(expected) {
		t.Fatalf("expected %s, got %s", expected, next)
	}
}
