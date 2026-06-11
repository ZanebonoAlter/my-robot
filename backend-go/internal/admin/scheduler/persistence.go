package scheduler

import (
	"encoding/json"
	"time"

	"syntopica-backend/internal/admin/repository"
	"syntopica-backend/internal/models"
)

// NewTaskPersistence creates a TaskPersistence with standard behavior:
// create/update the SchedulerTask row with execution stats.
func NewTaskPersistence(name, description string) *TaskPersistence {
	return &TaskPersistence{
		InitTask: func(n string, interval time.Duration) {
			initSchedulerTask(name, description, interval)
		},
		UpdateTask: func(n string, status string, startTime *time.Time, err error, result *JobResult) {
			updateSchedulerTask(name, description, n, status, startTime, err, result)
		},
		ResetTask: func(n string) error {
			return resetSchedulerTask(name, n)
		},
	}
}

func initSchedulerTask(name, description string, interval time.Duration) {
	var task models.SchedulerTask
	now := time.Now()
	nextRun := now.Add(interval)

	if err := repository.Repo.DB().Where("name = ?", name).First(&task).Error; err == nil {
		updates := map[string]interface{}{
			"description":         description,
			"check_interval":      int(interval.Seconds()),
			"next_execution_time": &nextRun,
		}
		if task.Status == "" || task.Status == "success" || task.Status == "failed" || task.Status == "running" {
			updates["status"] = "idle"
			if task.LastError != "" {
				updates["last_error"] = ""
			}
		}
		repository.Repo.DB().Model(&task).Updates(updates)
		return
	}

	task = models.SchedulerTask{
		Name:              name,
		Description:       description,
		CheckInterval:     int(interval.Seconds()),
		Status:            "idle",
		NextExecutionTime: &nextRun,
	}
	repository.Repo.DB().Create(&task)
}

func updateSchedulerTask(name, description string, _ string, status string, startTime *time.Time, err error, result *JobResult) {
	now := time.Now()

	// Calculate interval from the existing task or use default
	interval := 60 * time.Second
	var existing models.SchedulerTask
	if err2 := repository.Repo.DB().Where("name = ?", name).First(&existing).Error; err2 == nil {
		if existing.CheckInterval > 0 {
			interval = time.Duration(existing.CheckInterval) * time.Second
		}
	}
	nextExecution := now.Add(interval)

	resultJSON := ""
	if result != nil {
		summaryData := map[string]interface{}{
			"trigger_source": "scheduled",
		}
		if startTime != nil {
			summaryData["started_at"] = startTime.Format(time.RFC3339)
		}
		summaryData["finished_at"] = now.Format(time.RFC3339)
		if result.Data != nil {
			for k, v := range result.Data {
				summaryData[k] = v
			}
		}
		if result.Summary != "" {
			summaryData["reason"] = result.Summary
		}
		if encoded, marshalErr := json.Marshal(summaryData); marshalErr == nil {
			resultJSON = string(encoded)
		}
	} else if err != nil {
		summaryData := map[string]interface{}{
			"trigger_source": "scheduled",
			"finished_at":    now.Format(time.RFC3339),
			"error":          err.Error(),
		}
		if startTime != nil {
			summaryData["started_at"] = startTime.Format(time.RFC3339)
		}
		if encoded, marshalErr := json.Marshal(summaryData); marshalErr == nil {
			resultJSON = string(encoded)
		}
	}

	hasError := err != nil

	updates := map[string]interface{}{
		"status":              status,
		"next_execution_time": &nextExecution,
	}
	if hasError {
		updates["last_error"] = err.Error()
		errTime := now
		updates["last_error_time"] = &errTime
	} else {
		updates["last_error"] = ""
		updates["last_error_time"] = nil
	}

	if startTime != nil {
		duration := float64(time.Since(*startTime).Seconds())
		updates["last_execution_time"] = &now
		updates["last_execution_duration"] = &duration
		updates["last_execution_result"] = resultJSON
	}

	var task models.SchedulerTask
	if err2 := repository.Repo.DB().Where("name = ?", name).First(&task).Error; err2 == nil {
		if startTime != nil {
			updates["total_executions"] = task.TotalExecutions + 1
			if hasError {
				updates["failed_executions"] = task.FailedExecutions + 1
				updates["consecutive_failures"] = task.ConsecutiveFailures + 1
			} else {
				updates["successful_executions"] = task.SuccessfulExecutions + 1
				updates["consecutive_failures"] = 0
			}
		}
		repository.Repo.DB().Model(&task).Updates(updates)
		return
	}

	// Task doesn't exist yet - create it
	task = models.SchedulerTask{
		Name:              name,
		Description:       description,
		CheckInterval:     int(interval.Seconds()),
		Status:            status,
		LastError:         "",
		NextExecutionTime: &nextExecution,
	}
	if hasError {
		task.LastError = err.Error()
		errTime := now
		task.LastErrorTime = &errTime
	}
	if startTime != nil {
		duration := float64(time.Since(*startTime).Seconds())
		task.LastExecutionTime = &now
		task.LastExecutionDuration = &duration
		task.LastExecutionResult = resultJSON
		task.TotalExecutions = 1
		if hasError {
			task.FailedExecutions = 1
			task.ConsecutiveFailures = 1
		} else {
			task.SuccessfulExecutions = 1
		}
	}
	repository.Repo.DB().Create(&task)
}

func resetSchedulerTask(name, _ string) error {
	var task models.SchedulerTask
	if err := repository.Repo.DB().Where("name = ?", name).First(&task).Error; err != nil {
		return err
	}

	nextRun := time.Now().Add(time.Duration(task.CheckInterval) * time.Second)
	updates := map[string]interface{}{
		"status":                  "idle",
		"last_error":              "",
		"last_error_time":         nil,
		"total_executions":        0,
		"successful_executions":   0,
		"failed_executions":       0,
		"consecutive_failures":    0,
		"last_execution_time":     nil,
		"last_execution_duration": nil,
		"last_execution_result":   "",
		"next_execution_time":     &nextRun,
	}

	return repository.Repo.DB().Model(&task).Updates(updates).Error
}

// formatOptionalTime formats a *time.Time pointer to RFC3339 string.
func formatOptionalTime(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.Format(time.RFC3339)
}
