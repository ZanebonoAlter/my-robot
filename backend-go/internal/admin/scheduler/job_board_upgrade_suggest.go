package scheduler

import (
	"context"
	"fmt"
	"time"

	"syntopica-backend/internal/admin/repository"
	"syntopica-backend/internal/models"
	"syntopica-backend/internal/platform/logging"
	tagrepo "syntopica-backend/internal/tagmanagement/repository"
	boardsvc "syntopica-backend/internal/tagmanagement/service/board"
)

const defaultBoardUpgradeSuggestTime = "06:30"

// NextBoardUpgradeSuggestTime computes the next wall-clock trigger for the
// board-upgrade-suggest job (default 06:30 local). It reads HH:MM from
// ai_settings key semantic_board_upgrade_suggest_time (design D4: fixed time,
// loosely coupled — not guaranteed to follow the daily report) and falls back to
// 06:30 on any read/parse error. Mirrors NextDailyReportTime's shape.
func NextBoardUpgradeSuggestTime(now time.Time) time.Time {
	h, m := 6, 30
	var setting models.AISettings
	if err := repository.Repo.DB().Where("key = ?", "semantic_board_upgrade_suggest_time").First(&setting).Error; err == nil {
		if ph, pm, parseErr := parseBoardUpgradeHHMM(setting.Value); parseErr == nil {
			h, m = ph, pm
		} else {
			logging.Warnf("board_upgrade_suggest: invalid time %q, using default %s", setting.Value, defaultBoardUpgradeSuggestTime)
		}
	}
	today := time.Date(now.Year(), now.Month(), now.Day(), h, m, 0, 0, now.Location())
	if now.Before(today) {
		return today
	}
	return today.Add(24 * time.Hour)
}

func parseBoardUpgradeHHMM(s string) (int, int, error) {
	var h, m int
	if _, err := fmt.Sscanf(s, "%d:%d", &h, &m); err != nil {
		return 0, 0, err
	}
	if h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, 0, fmt.Errorf("out of range")
	}
	return h, m, nil
}

// BoardUpgradeSuggestJob runs one discover_new generation pass and then GCs
// stale watch suggestions (spec: scheduler 定期生成建议 + 观察池建议自动回收).
// Only discover_new is run (design D4). A generation failure is logged only and
// surfaces as a non-nil JobResult carrying the error string but a nil error —
// the scheduler loop stays healthy and sibling jobs keep running. Returns
// inserted/skipped/cooldown/watch_gc counts for status display.
func BoardUpgradeSuggestJob() JobFunc {
	return func(ctx context.Context) (*JobResult, error) {
		startTime := time.Now()
		db := repository.Repo.DB()
		svc := boardsvc.NewSemanticBoardUpgradeService(db, boardsvc.NewDefaultSemanticBoardUpgradeLLM(), nil)

		inserted, skipped, cooldownBlocked, err := svc.GenerateAndPersist(ctx, "discover_new")
		if err != nil {
			// Failure is logged only (design D4); return nil error so the scheduler
			// does not mark the task failed and sibling jobs are unaffected.
			logging.Errorf("board_upgrade_suggest: generation failed: %v", err)
			return &JobResult{
				Data: map[string]interface{}{
					"error":       err.Error(),
					"started_at":  startTime.Format(time.RFC3339),
					"finished_at": time.Now().Format(time.RFC3339),
				},
				Summary: fmt.Sprintf("board upgrade generation failed: %v", err),
			}, nil
		}

		// GC stale watch suggestions (spec: 观察池建议自动回收). Failure is logged
		// only; it does not negate the generation that already succeeded.
		gcDays := svc.LoadWatchGCDays(ctx)
		repo := tagrepo.NewBoardUpgradeSuggestionRepository(db)
		gcCount, gcErr := repo.GCOldWatch(ctx, gcDays)
		if gcErr != nil {
			logging.Errorf("board_upgrade_suggest: watch GC failed: %v", gcErr)
			gcCount = 0
		}

		return &JobResult{
			Data: map[string]interface{}{
				"inserted":         inserted,
				"skipped":          skipped,
				"cooldown_blocked": cooldownBlocked,
				"watch_gc":         gcCount,
				"started_at":       startTime.Format(time.RFC3339),
				"finished_at":      time.Now().Format(time.RFC3339),
			},
			Summary: fmt.Sprintf("board upgrade: inserted=%d skipped=%d cooldown=%d watch_gc=%d", inserted, skipped, cooldownBlocked, gcCount),
		}, nil
	}
}
