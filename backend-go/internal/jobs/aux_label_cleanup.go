package jobs

import (
	"context"
	"fmt"
	"sync"
	"time"

	"syntopica-backend/internal/domain/tagging"
	"syntopica-backend/internal/platform/database"
	"syntopica-backend/internal/platform/logging"
	"syntopica-backend/internal/platform/tracing"
)

const auxLabelCleanupStartupDelay = 10 * time.Minute

type AuxLabelCleanupScheduler struct {
	checkInterval int
	stopChan      chan bool
	wg            sync.WaitGroup
	mu            sync.Mutex
	running       bool
	isExecuting   bool
	nextRun       *time.Time
	lastRun       *time.Time
	lastError     string
	totalRuns     int
	successRuns   int
	failedRuns    int

	lastDisabledCount int
}

func NewAuxLabelCleanupScheduler(intervalSeconds int) *AuxLabelCleanupScheduler {
	return &AuxLabelCleanupScheduler{
		checkInterval: intervalSeconds,
		stopChan:      make(chan bool),
		running:       false,
	}
}

func (s *AuxLabelCleanupScheduler) Start() error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return nil
	}
	s.running = true
	s.wg.Add(1)
	nextRun := time.Now().Add(auxLabelCleanupStartupDelay)
	s.nextRun = &nextRun
	s.mu.Unlock()

	go func() {
		defer s.wg.Done()

		timer := time.NewTimer(auxLabelCleanupStartupDelay)
		defer timer.Stop()

		select {
		case <-timer.C:
			s.runCleanupCycle()
		case <-s.stopChan:
			logging.Infof("Aux label cleanup scheduler stopped during startup delay")
			return
		}

		ticker := time.NewTicker(time.Duration(s.checkInterval) * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				s.runCleanupCycle()
				s.updateNextRun(time.Now().Add(time.Duration(s.checkInterval) * time.Second))
			case <-s.stopChan:
				logging.Infof("Aux label cleanup scheduler stopped")
				return
			}
		}
	}()

	logging.Infof("Aux label cleanup scheduler started (interval: %d seconds, first run in %v)", s.checkInterval, auxLabelCleanupStartupDelay)
	return nil
}

func (s *AuxLabelCleanupScheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.running {
		return
	}
	s.running = false
	close(s.stopChan)
	s.wg.Wait()
	s.stopChan = make(chan bool)
	s.nextRun = nil
}

func (s *AuxLabelCleanupScheduler) runCleanupCycle() {
	tracing.TraceSchedulerTick("aux_label_cleanup", "cron", func(ctx context.Context) {
		s.mu.Lock()
		if s.isExecuting {
			s.mu.Unlock()
			return
		}
		s.isExecuting = true
		now := time.Now()
		s.lastRun = &now
		s.lastError = ""
		s.mu.Unlock()
		defer func() {
			s.mu.Lock()
			s.isExecuting = false
			s.mu.Unlock()
		}()

		logging.Infof("Running aux label cleanup (mode=disable)...")

		service := tagging.NewAuxiliaryLabelService(database.DB, nil)
		result, err := service.GC(ctx, tagging.AuxLabelGCRequest{
			Mode:      tagging.AuxLabelGCModeDisable,
			GraceDays: 1,
		})
		if err != nil {
			s.mu.Lock()
			s.totalRuns++
			s.failedRuns++
			s.lastError = err.Error()
			s.mu.Unlock()
			logging.Errorf("AuxLabelCleanup: GC failed: %v", err)
			return
		}

		s.mu.Lock()
		s.totalRuns++
		s.successRuns++
		s.lastError = ""
		s.lastDisabledCount = result.AffectedCount
		s.mu.Unlock()

		if result.AffectedCount > 0 {
			logging.Infof("Aux label cleanup completed: disabled %d labels", result.AffectedCount)
		} else {
			logging.Infof("Aux label cleanup completed: no labels to clean")
		}
	})
}

func (s *AuxLabelCleanupScheduler) TriggerNow() map[string]interface{} {
	s.mu.Lock()
	if s.isExecuting {
		s.mu.Unlock()
		return map[string]interface{}{
			"accepted":    false,
			"started":     false,
			"reason":      "already_running",
			"message":     "辅助标签清理正在执行中，稍后再试。",
			"status_code": 409,
		}
	}
	s.mu.Unlock()

	logging.Infof("Manual aux label cleanup triggered")
	s.runCleanupCycle()

	s.mu.Lock()
	defer s.mu.Unlock()

	return map[string]interface{}{
		"accepted":            true,
		"started":             true,
		"message":             "Aux label cleanup triggered",
		"last_disabled_count": s.lastDisabledCount,
	}
}

func (s *AuxLabelCleanupScheduler) GetStatus() map[string]interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()

	status := "stopped"
	if s.isExecuting {
		status = "running"
	} else if s.running {
		status = "idle"
	}

	return map[string]interface{}{
		"status":                status,
		"check_interval":        s.checkInterval,
		"is_executing":          s.isExecuting,
		"next_run":              formatOptionalTime(s.nextRun),
		"last_execution_time":   formatOptionalTime(s.lastRun),
		"last_error":            s.lastError,
		"total_executions":      s.totalRuns,
		"successful_executions": s.successRuns,
		"failed_executions":     s.failedRuns,
		"last_disabled_count":   s.lastDisabledCount,
	}
}

// GetTaskStatusDetails returns the status details used by enrichStatus to populate
// database_state and last_run_summary in the API response. Without this method,
// the scheduler panel in the frontend shows no execution results.
func (s *AuxLabelCleanupScheduler) GetTaskStatusDetails() map[string]interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()

	status := "stopped"
	if s.isExecuting {
		status = "running"
	} else if s.running {
		status = "idle"
	}

	successRate := 0.0
	if s.totalRuns > 0 {
		successRate = float64(s.successRuns) / float64(s.totalRuns) * 100
	}

	lastExecutionTime := ""
	if s.lastRun != nil {
		lastExecutionTime = s.lastRun.Format(time.RFC3339)
	}

	databaseState := map[string]interface{}{
		"name":                  "aux_label_cleanup",
		"status":                status,
		"total_executions":      s.totalRuns,
		"successful_executions": s.successRuns,
		"failed_executions":     s.failedRuns,
		"last_execution_time":   lastExecutionTime,
		"last_error":            s.lastError,
		"success_rate":          successRate,
	}

	lastRunSummary := map[string]interface{}{
		"disabled_count": s.lastDisabledCount,
	}
	if s.lastRun != nil {
		lastRunSummary["started_at"] = s.lastRun.Format(time.RFC3339)
	}

	return map[string]interface{}{
		"status":           status,
		"is_executing":     s.isExecuting,
		"check_interval":   s.checkInterval,
		"next_run":         formatOptionalTime(s.nextRun),
		"database_state":   databaseState,
		"last_run_summary": lastRunSummary,
	}
}

func (s *AuxLabelCleanupScheduler) ResetStats() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastRun = nil
	s.lastError = ""
	s.totalRuns = 0
	s.successRuns = 0
	s.failedRuns = 0
	s.lastDisabledCount = 0
	return nil
}

func (s *AuxLabelCleanupScheduler) UpdateInterval(interval int) error {
	if interval <= 0 {
		return fmt.Errorf("interval must be positive")
	}
	wasRunning := false
	s.mu.Lock()
	wasRunning = s.running
	s.mu.Unlock()

	if wasRunning {
		s.Stop()
	}

	s.mu.Lock()
	s.checkInterval = interval
	s.mu.Unlock()

	if wasRunning {
		return s.Start()
	}

	s.updateNextRun(time.Now().Add(time.Duration(interval) * time.Second))
	return nil
}

func (s *AuxLabelCleanupScheduler) updateNextRun(next time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextRun = &next
}
