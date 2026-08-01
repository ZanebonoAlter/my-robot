package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"syntopica-backend/internal/platform/aisettings"
	"syntopica-backend/internal/platform/logging"
	"syntopica-backend/internal/platform/ws"
	daily_report "syntopica-backend/internal/topicgraph"
)

// NextDailyReportTime computes the next wall-clock time for the daily report.
// It reads the configured HH:MM from AISettings (default "21:00") and returns
// today at that time if it hasn't passed yet, otherwise tomorrow at that time.
func NextDailyReportTime(now time.Time) time.Time {
	hhmm, err := aisettings.LoadDailyReportTimeConfig()
	if err != nil {
		logging.Warnf("daily_report: failed to load time config, using default: %v", err)
		hhmm = "21:00"
	}

	h, m := 21, 0
	_, scanErr := fmt.Sscanf(hhmm, "%d:%d", &h, &m)
	if scanErr != nil {
		logging.Warnf("daily_report: failed to parse time %q, using default 21:00: %v", hhmm, scanErr)
		h, m = 21, 0
	}

	today := time.Date(now.Year(), now.Month(), now.Day(), h, m, 0, 0, now.Location())
	if now.Before(today) {
		return today
	}
	return today.Add(24 * time.Hour)
}

// DailyReportJob generates daily reports for all active semantic boards.
// When no targetDate is provided (nil), it reports on the current local time.
func DailyReportJob(targetDate ...time.Time) JobFunc {
	return func(ctx context.Context) (*JobResult, error) {
		startTime := time.Now()

		date := time.Now().In(time.Local)
		if len(targetDate) > 0 {
			date = targetDate[0]
		}

		boardIDs, err := daily_report.CollectBoardIDsForDate(date)
		if err != nil {
			return nil, fmt.Errorf("failed to collect board IDs: %w", err)
		}

		ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
		defer cancel()

		reportCount := 0
		for _, boardID := range boardIDs {
			report, genErr := daily_report.GenerateAndSaveReport(ctx, boardID, date)
			if genErr != nil {
				logging.Warnf("daily-report: generate/save failed for board %d: %v", boardID, genErr)
				continue
			}
			if report == nil {
				continue
			}
			reportCount++
		}

		// Broadcast completion
		msg := map[string]interface{}{
			"type":         "daily_report_complete",
			"report_count": reportCount,
			"date":         date.Format("2006-01-02"),
			"timestamp":    time.Now().Format(time.RFC3339),
		}
		data, _ := json.Marshal(msg)
		ws.GetHub().BroadcastRaw(data)

		return &JobResult{
			Data: map[string]interface{}{
				"report_count":   reportCount,
				"trigger_source": "scheduled",
				"started_at":     startTime.Format(time.RFC3339),
				"finished_at":    time.Now().Format(time.RFC3339),
			},
			Summary: fmt.Sprintf("generated %d reports for %s", reportCount, date.Format("2006-01-02")),
		}, nil
	}
}

// DailyReportSchedulerWrapper wraps BaseScheduler to add TriggerNowWithDate.
type DailyReportSchedulerWrapper struct {
	*BaseScheduler
	targetDateFn func() time.Time // default target date (today), overridable
}

// NewDailyReportSchedulerWrapper creates a DailyReportSchedulerWrapper that
// embeds BaseScheduler and adds TriggerNowWithDate support.
func NewDailyReportSchedulerWrapper(bs *BaseScheduler) *DailyReportSchedulerWrapper {
	return &DailyReportSchedulerWrapper{
		BaseScheduler: bs,
		targetDateFn:  func() time.Time { return time.Now().In(time.Local) },
	}
}

// TriggerNowWithDate triggers daily report generation for a specific date.
// This is accessed by the handler via type assertion on the Scheduler interface.
func (d *DailyReportSchedulerWrapper) TriggerNowWithDate(dateStr string) map[string]interface{} {
	if !d.TrySetExecuting() {
		return map[string]interface{}{
			"accepted":    false,
			"started":     false,
			"reason":      "already_running",
			"message":     "日报生成正在执行中，请稍后再试。",
			"status_code": http.StatusConflict,
		}
	}

	targetDate := d.targetDateFn()
	if dateStr != "" {
		parsed, err := time.ParseInLocation("2006-01-02", dateStr, time.Local)
		if err != nil {
			d.ClearExecuting()
			return map[string]interface{}{
				"accepted":    false,
				"started":     false,
				"reason":      "invalid_date",
				"message":     "日期格式无效，请使用 YYYY-MM-DD。",
				"status_code": http.StatusBadRequest,
			}
		}
		targetDate = parsed
	}

	go func() {
		defer func() {
			d.ClearExecuting()
			if r := recover(); r != nil {
				logging.Errorf("PANIC in manual daily-report trigger: %v", r)
			}
		}()
		d.executeWithDate(targetDate)
	}()

	return map[string]interface{}{
		"accepted": true,
		"started":  true,
		"reason":   "manual_run_started",
		"message":  fmt.Sprintf("日报生成已经开始运行（目标日期: %s）。", targetDate.Format("2006-01-02")),
	}
}

func (d *DailyReportSchedulerWrapper) executeWithDate(targetDate time.Time) {
	job := DailyReportJob(targetDate)
	result, err := job(context.Background())
	if err != nil {
		logging.Errorf("Daily report job failed: %v", err)
	} else if result != nil {
		logging.Infof("Daily report: %s", result.Summary)
	}
}
