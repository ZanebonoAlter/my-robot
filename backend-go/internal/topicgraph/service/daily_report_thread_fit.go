package service

import (
	"context"
	"strings"

	"syntopica-backend/internal/platform/airouter"
	"syntopica-backend/internal/platform/logging"
	"syntopica-backend/internal/platform/tracing"
	"syntopica-backend/internal/topicgraph/repository"
)

// embedFunc mirrors airouter.Router.Embed so tests can inject a mock instead
// of hitting the real provider pipeline.
type embedFunc func(ctx context.Context, req airouter.EmbeddingRequest, cap airouter.Capability) (*airouter.EmbeddingResult, error)

// computeThreadFitDistances batch-embeds thread titles and fills each thread's
// Embedding + FitDistance (cosine distance vs its owning section's title
// embedding). Non-fatal: on embed failure or missing section embedding, the
// affected threads keep zero values and generation continues.
//
// Thread↔section pairing is fixed by the shared index i: threadBatches[i]
// belongs to sections[i]. This MUST run before MergeSimilarSections, which is
// the step that may collapse/rewire section↔thread batches.
func computeThreadFitDistances(
	ctx context.Context,
	sections []repository.DailyReportSection,
	threadBatches [][]repository.DailyReportThread,
	boardID uint,
	embed embedFunc,
) {
	ctx, span := tracing.Tracer(tracing.ServiceName).Start(ctx, "workflow.daily_report.thread_fit")
	defer span.End()

	// 1. Collect titles for threads whose owning section HAS an embedding.
	//    Skip sections with empty Embedding and empty/whitespace titles.
	var texts []string
	type loc struct{ sec, th int }
	var locs []loc
	for i := range sections {
		if sections[i].Embedding == "" {
			continue
		}
		if i >= len(threadBatches) {
			continue
		}
		for k := range threadBatches[i] {
			title := threadBatches[i][k].Title
			if strings.TrimSpace(title) == "" {
				continue
			}
			texts = append(texts, title)
			locs = append(locs, loc{i, k})
		}
	}
	if len(texts) == 0 {
		return
	}

	// 2. Batch embed thread titles.
	result, err := embed(ctx, airouter.EmbeddingRequest{
		Input:     texts,
		Operation: "section.embedding",
		SessionID: SessionIDFromContext(ctx),
		Metadata: map[string]any{
			"operation": "daily_report_thread_embedding",
			"board_id":  boardID,
		},
	}, airouter.CapabilityEmbedding)
	if err != nil {
		logging.Warnf("daily-report: thread embedding failed for board %d: %v", boardID, err)
		return
	}
	if result == nil || len(result.Embeddings) < len(texts) {
		logging.Warnf("daily-report: thread embedding short result for board %d: got %d want %d",
			boardID, len(result.Embeddings), len(texts))
		return
	}

	// 3. Compute fit distance per thread vs its owning section's embedding.
	for j, l := range locs {
		threadEmb := repository.FloatsToPgVector(result.Embeddings[j])
		dist, derr := cosineDistance(threadEmb, sections[l.sec].Embedding)
		if derr != nil {
			logging.Warnf("daily-report: thread fit distance failed for board %d section %d thread %d: %v",
				boardID, sections[l.sec].ID, threadBatches[l.sec][l.th].ID, derr)
			continue
		}
		threadBatches[l.sec][l.th].Embedding = threadEmb
		d := dist
		threadBatches[l.sec][l.th].FitDistance = &d
	}
}
