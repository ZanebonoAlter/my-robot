package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"go.opentelemetry.io/otel"
	"syntopica-backend/internal/dataenrichment/repository"
	"syntopica-backend/internal/platform/tracing"
)

// DebateService orchestrates the FinGenius stock debate flow:
// submit → concurrent poll → distill → persist.
type DebateService struct {
	client    FinGeniusClient
	distiller *DebateDistiller
	repo      *repository.Repository
}

// NewDebateService creates a new DebateService.
func NewDebateService(client FinGeniusClient, distiller *DebateDistiller, repo *repository.Repository) *DebateService {
	return &DebateService{
		client:    client,
		distiller: distiller,
		repo:      repo,
	}
}

// RunDebate submits symbols for debate, polls concurrently, distills results,
// and persists to stock_debate_result. Each symbol's failure is non-fatal —
// failed symbols are saved with distill_status=failed.
//
// Returns the persisted results (one per symbol). Returns error only if the
// initial Submit itself fails (service unavailable).
func (s *DebateService) RunDebate(ctx context.Context, resultID, topicID uint, sessionID string, symbols []DebateSymbol) ([]*repository.StockDebateResult, error) {
	ctx, span := otel.Tracer(tracing.ServiceName).Start(ctx, "DebateService.RunDebate")
	defer span.End()
	if len(symbols) == 0 {
		return nil, fmt.Errorf("run debate: no symbols provided")
	}

	// 1. Submit all symbols to FinGenius.
	tasks, err := s.client.Submit(ctx, symbols)
	if err != nil {
		return nil, fmt.Errorf("run debate submit: %w", err)
	}

	// 2. Concurrently poll each task + distill + persist.
	results := make([]*repository.StockDebateResult, len(tasks))
	var wg sync.WaitGroup

	for i, task := range tasks {
		i, task := i, task // capture for goroutine
		wg.Add(1)
		go func() {
			defer wg.Done()
			dr := s.processOneTask(ctx, resultID, topicID, sessionID, task)
			results[i] = dr
			// Non-fatal: individual failures are recorded in the DB row, not returned as error.
		}()
	}

	wg.Wait()

	// Filter out nil entries (shouldn't happen but be safe).
	var out []*repository.StockDebateResult
	for _, r := range results {
		if r != nil {
			out = append(out, r)
		}
	}

	return out, nil
}

// processOneTask polls, distills, and persists a single debate task.
// Always returns a StockDebateResult — distill_status indicates success/failure.
func (s *DebateService) processOneTask(ctx context.Context, resultID, topicID uint, sessionID string, task DebateTask) *repository.StockDebateResult {
	// Poll until done or timeout.
	taskResult, pollErr := s.client.PollTask(ctx, task.TaskID)

	// Build base DB row with what we know from the task.
	dr := &repository.StockDebateResult{
		TopicEnrichmentResultID: resultID,
		PersistentTopicID:       topicID,
		Sector:                  task.Sector,
		Code:                    task.StockCode,
		Name:                    task.Name,
		FingeniusTaskID:         task.TaskID,
		DistillStatus:           "failed",
		Verdict:                 "flat",
	}

	// Save raw FinGenius data if available.
	if taskResult != nil && taskResult.Result != nil {
		dr.HTMLContent = taskResult.Result.HTMLContent
		if researchJSON, err := json.Marshal(taskResult.Result.Research); err == nil {
			dr.FingeniusResearch = researchJSON
		}
		if battleJSON, err := json.Marshal(taskResult.Result.Battle); err == nil {
			dr.FingeniusBattle = battleJSON
		}
	}

	switch {
	case pollErr != nil:
		// Poll failed or timed out — already set to failed/flat.
	case taskResult != nil && taskResult.Status == "done" && taskResult.Result != nil:
		// Distill via LLM.
		distilled, distillErr := s.distiller.Distill(ctx, sessionID, taskResult.Result.Research, taskResult.Result.Battle)
		if distillErr == nil && distilled != nil {
			dr.DistillStatus = "done"
			dr.Verdict = distilled.Verdict
			dr.Consensus = distilled.Consensus
			if agentsJSON, err := json.Marshal(distilled.Agents); err == nil {
				dr.Agents = agentsJSON
			}
			if votesJSON, err := json.Marshal(distilled.Votes); err == nil {
				dr.Votes = votesJSON
			}
		}
	default:
		// Task failed or no result — already set to failed/flat.
	}

	// Persist. Best-effort: log failure but don't propagate.
	_ = s.repo.CreateStockDebateResult(ctx, dr)

	return dr
}
