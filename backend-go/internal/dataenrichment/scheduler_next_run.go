package dataenrichment

import "time"

// NextWeeklyLifelineTime returns the next Monday 03:00 Asia/Shanghai.
// If today is Monday before 03:00, returns today at 03:00.
// See design.md §11 decision ③.
func NextWeeklyLifelineTime(now time.Time) time.Time {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		loc = time.FixedZone("CST", 8*3600)
	}
	t := now.In(loc)

	// If today is Monday before 03:00, trigger today.
	if t.Weekday() == time.Monday {
		today := time.Date(t.Year(), t.Month(), t.Day(), 3, 0, 0, 0, loc)
		if t.Before(today) {
			return today
		}
	}

	// Otherwise, next Monday at 03:00.
	daysUntilMonday := (8 - int(t.Weekday())) % 7
	if daysUntilMonday == 0 {
		daysUntilMonday = 7
	}
	next := time.Date(t.Year(), t.Month(), t.Day(), 3, 0, 0, 0, loc).AddDate(0, 0, daysUntilMonday)
	return next
}

// NextMonthlyLifelineTime returns the next 1st of month 03:30 Asia/Shanghai.
// If today is the 1st before 03:30, returns today at 03:30.
func NextMonthlyLifelineTime(now time.Time) time.Time {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		loc = time.FixedZone("CST", 8*3600)
	}
	t := now.In(loc)

	// If today is the 1st before 03:30, trigger today.
	if t.Day() == 1 {
		today := time.Date(t.Year(), t.Month(), 1, 3, 30, 0, 0, loc)
		if t.Before(today) {
			return today
		}
	}

	// 1st of next month at 03:30.
	firstOfNext := time.Date(t.Year(), t.Month(), 1, 3, 30, 0, 0, loc).AddDate(0, 1, 0)
	return firstOfNext
}

// NextYearlyLifelineTime returns the next Jan 1 04:00 Asia/Shanghai.
// If today is Jan 1 before 04:00, returns today at 04:00.
func NextYearlyLifelineTime(now time.Time) time.Time {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		loc = time.FixedZone("CST", 8*3600)
	}
	t := now.In(loc)

	// If today is Jan 1 before 04:00, trigger today.
	if t.Month() == time.January && t.Day() == 1 {
		today := time.Date(t.Year(), 1, 1, 4, 0, 0, 0, loc)
		if t.Before(today) {
			return today
		}
	}

	// Jan 1 of next year at 04:00.
	next := time.Date(t.Year()+1, 1, 1, 4, 0, 0, 0, loc)
	return next
}
