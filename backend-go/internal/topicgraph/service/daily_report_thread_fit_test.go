package service

import (
	"context"
	"errors"
	"math"
	"testing"

	"syntopica-backend/internal/platform/airouter"
	"syntopica-backend/internal/topicgraph/repository"
)

// mockEmbedFunc builds an embedFunc that returns the given embeddings in order,
// or the given error (in which case embeddings are ignored).
func mockEmbedFunc(embeddings [][]float64, err error) embedFunc {
	var idx int
	return func(_ context.Context, _ airouter.EmbeddingRequest, _ airouter.Capability) (*airouter.EmbeddingResult, error) {
		if err != nil {
			return nil, err
		}
		if idx >= len(embeddings) {
			// Should not happen in well-formed tests; return a zero vector of
			// the same dimension as the last provided one to avoid panics.
			return &airouter.EmbeddingResult{Embeddings: embeddings}, nil
		}
		// Return ALL embeddings on the first call so batch consumers see them
		// in order. Subsequent calls (not expected) reuse the slice.
		out := &airouter.EmbeddingResult{Embeddings: embeddings[idx:]}
		idx = len(embeddings)
		return out, nil
	}
}

func TestComputeThreadFitDistances_OnTopicThreadSmallDistance(t *testing.T) {
	// Section embedding and mock thread embedding point the same direction
	// → cosine distance close to 0.
	secVec := []float64{1, 0, 0, 0}
	threadVec := []float64{0.999, 0.04, 0, 0} // ~6deg off, cosine dist ~0.002

	sections := []repository.DailyReportSection{
		{ID: 1, Embedding: repository.FloatsToPgVector(secVec)},
	}
	threadBatches := [][]repository.DailyReportThread{
		{{Title: "on-topic thread"}},
	}

	embed := mockEmbedFunc([][]float64{threadVec}, nil)
	computeThreadFitDistances(context.Background(), sections, threadBatches, 1, embed)

	if got := threadBatches[0][0].FitDistance; got > 0.05 {
		t.Fatalf("on-topic thread FitDistance = %v, want <= 0.05", got)
	}
	if threadBatches[0][0].Embedding == "" {
		t.Fatalf("on-topic thread Embedding should be populated, got empty")
	}
}

func TestComputeThreadFitDistances_OffTopicThreadLargeDistance(t *testing.T) {
	// Section vector vs orthogonal thread vector → cosine distance ~1.0.
	secVec := []float64{1, 0, 0, 0}
	threadVec := []float64{0, 1, 0, 0}

	sections := []repository.DailyReportSection{
		{ID: 1, Embedding: repository.FloatsToPgVector(secVec)},
	}
	threadBatches := [][]repository.DailyReportThread{
		{{Title: "off-topic thread"}},
	}

	embed := mockEmbedFunc([][]float64{threadVec}, nil)
	computeThreadFitDistances(context.Background(), sections, threadBatches, 1, embed)

	if got := threadBatches[0][0].FitDistance; got < 0.9 {
		t.Fatalf("off-topic thread FitDistance = %v, want >= 0.9 (orthogonal)", got)
	}
}

func TestComputeThreadFitDistances_EmbedErrorLeavesZeros(t *testing.T) {
	sections := []repository.DailyReportSection{
		{ID: 1, Embedding: repository.FloatsToPgVector([]float64{1, 0})},
	}
	threadBatches := [][]repository.DailyReportThread{
		{{Title: "thread a"}, {Title: "thread b"}},
	}

	embed := mockEmbedFunc(nil, errors.New("provider down"))
	computeThreadFitDistances(context.Background(), sections, threadBatches, 1, embed)

	for i, th := range threadBatches[0] {
		if th.Embedding != "" {
			t.Fatalf("thread %d Embedding should be empty on embed error, got %q", i, th.Embedding)
		}
		if th.FitDistance != 0 {
			t.Fatalf("thread %d FitDistance should be zero on embed error, got %v", i, th.FitDistance)
		}
	}
}

func TestComputeThreadFitDistances_SectionWithoutEmbeddingSkipped(t *testing.T) {
	// Section with empty Embedding must be skipped entirely — its threads stay
	// zero and embed is NOT called for them.
	called := false
	embed := func(_ context.Context, _ airouter.EmbeddingRequest, _ airouter.Capability) (*airouter.EmbeddingResult, error) {
		called = true
		return &airouter.EmbeddingResult{Embeddings: [][]float64{{1, 0}}}, nil
	}

	sections := []repository.DailyReportSection{
		{ID: 1, Embedding: ""}, // no embedding
	}
	threadBatches := [][]repository.DailyReportThread{
		{{Title: "thread x"}},
	}

	computeThreadFitDistances(context.Background(), sections, threadBatches, 1, embed)

	if called {
		t.Fatalf("embed must not be called when all owning sections lack embeddings")
	}
	if threadBatches[0][0].Embedding != "" || threadBatches[0][0].FitDistance != 0 {
		t.Fatalf("thread under section without embedding should keep zeros, got Embedding=%q FitDistance=%v",
			threadBatches[0][0].Embedding, threadBatches[0][0].FitDistance)
	}
}

func TestComputeThreadFitDistances_ThreadSectionPairingByIndex(t *testing.T) {
	// 2 sections x 2 threads. Each section has a distinct embedding; mock
	// returns a distinct thread vector per thread. We verify each thread's
	// FitDistance is computed against ITS OWN section vector (no cross-talk),
	// and the correct thread is written back.
	secA := []float64{1, 0, 0, 0}
	secB := []float64{0, 1, 0, 0}

	// threadA0 parallel to secA → dist ~0; threadA1 orthogonal to secA → dist ~1
	// threadB0 parallel to secB → dist ~0; threadB1 orthogonal to secB → dist ~1
	threadA0 := []float64{1, 0, 0, 0}
	threadA1 := []float64{0, 1, 0, 0}
	threadB0 := []float64{0, 1, 0, 0}
	threadB1 := []float64{1, 0, 0, 0}

	// Collection order: section 0 first (threads A0, A1), then section 1 (B0, B1).
	mockVecs := [][]float64{threadA0, threadA1, threadB0, threadB1}

	sections := []repository.DailyReportSection{
		{ID: 10, Embedding: repository.FloatsToPgVector(secA)},
		{ID: 20, Embedding: repository.FloatsToPgVector(secB)},
	}
	threadBatches := [][]repository.DailyReportThread{
		{{Title: "A0"}, {Title: "A1"}},
		{{Title: "B0"}, {Title: "B1"}},
	}

	embed := mockEmbedFunc(mockVecs, nil)
	computeThreadFitDistances(context.Background(), sections, threadBatches, 1, embed)

	expect := []struct {
		sec, th  int
		min, max float64
	}{
		{0, 0, 0, 0.05},    // A0 ~ parallel to secA
		{0, 1, 0.95, 1.05}, // A1 orthogonal to secA
		{1, 0, 0, 0.05},    // B0 ~ parallel to secB
		{1, 1, 0.95, 1.05}, // B1 orthogonal to secB
	}
	for _, e := range expect {
		got := threadBatches[e.sec][e.th].FitDistance
		if got < e.min || got > e.max {
			t.Fatalf("threadBatches[%d][%d] FitDistance = %v, want in [%v, %v]",
				e.sec, e.th, got, e.min, e.max)
		}
		if math.IsNaN(got) || math.IsInf(got, 0) {
			t.Fatalf("threadBatches[%d][%d] FitDistance is NaN/Inf", e.sec, e.th)
		}
		if threadBatches[e.sec][e.th].Embedding == "" {
			t.Fatalf("threadBatches[%d][%d] Embedding should be populated", e.sec, e.th)
		}
	}
}
