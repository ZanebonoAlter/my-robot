package service

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"strings"
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

	got := threadBatches[0][0].FitDistance
	if got == nil {
		t.Fatalf("on-topic thread FitDistance should be set (non-nil), got nil")
	}
	if *got > 0.05 {
		t.Fatalf("on-topic thread FitDistance = %v, want <= 0.05", *got)
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

	got := threadBatches[0][0].FitDistance
	if got == nil {
		t.Fatalf("off-topic thread FitDistance should be set (non-nil), got nil")
	}
	if *got < 0.9 {
		t.Fatalf("off-topic thread FitDistance = %v, want >= 0.9 (orthogonal)", *got)
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
		// "No signal" is encoded as a nil pointer, not a zero value, so it can
		// be told apart from a perfect-fit 0.0 downstream.
		if th.FitDistance != nil {
			t.Fatalf("thread %d FitDistance should be nil (no signal) on embed error, got %v", i, *th.FitDistance)
		}
	}
}

func TestComputeThreadFitDistances_SectionWithoutEmbeddingSkipped(t *testing.T) {
	// Section with empty Embedding must be skipped entirely — its threads stay
	// zero-signal and embed is NOT called for them.
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
	if threadBatches[0][0].Embedding != "" || threadBatches[0][0].FitDistance != nil {
		t.Fatalf("thread under section without embedding should keep zero signal, got Embedding=%q FitDistance=%v",
			threadBatches[0][0].Embedding, threadBatches[0][0].FitDistance)
	}
}

func TestComputeThreadFitDistances_PerfectFitDistanceIsNotNil(t *testing.T) {
	// Regression for the FitDistance zero-value ambiguity (review Major M1).
	// Under the old `FitDistance float64` + `omitempty`, a perfect-fit thread
	// (cosine distance exactly 0.0) serialized WITHOUT a `fit_distance` field
	// — indistinguishable from "no signal" (embed failure / missing section
	// embedding), which also produced 0.0 and was also dropped. The frontend
	// could mistake the best possible fit for no signal.
	//
	// With `*float64`, perfect-fit keeps a non-nil pointer to 0.0 (serialized
	// as `"fit_distance":0`) while no-signal stays nil (omitted). This test
	// pins the nil-vs-0.0 distinction at both the Go-struct and JSON levels.
	secVec := []float64{1, 0, 0, 0}
	threadVec := []float64{1, 0, 0, 0} // identical → cosine distance == 0.0 exactly

	sections := []repository.DailyReportSection{
		{ID: 1, Embedding: repository.FloatsToPgVector(secVec)},
	}
	threadBatches := [][]repository.DailyReportThread{
		{{Title: "perfect-fit thread"}},
	}

	embed := mockEmbedFunc([][]float64{threadVec}, nil)
	computeThreadFitDistances(context.Background(), sections, threadBatches, 1, embed)

	got := threadBatches[0][0].FitDistance
	if got == nil {
		t.Fatalf("perfect-fit thread FitDistance must be non-nil; nil conflates 0.0 with no-signal")
	}
	if *got != 0.0 {
		t.Fatalf("perfect-fit thread FitDistance = %v, want exactly 0.0", *got)
	}

	// Decisive JSON proof: 0.0 must round-trip through serialization, not be
	// swallowed by omitempty (which is exactly what the old float64 did).
	out, err := json.Marshal(threadBatches[0][0])
	if err != nil {
		t.Fatalf("marshal thread: %v", err)
	}
	if !strings.Contains(string(out), `"fit_distance":0`) {
		t.Fatalf("perfect-fit thread JSON must contain fit_distance:0 (0.0 must survive omitempty), got %s", out)
	}

	// Contrast: a no-signal thread (nil pointer) must omit the field entirely.
	noSignal := repository.DailyReportThread{Title: "no-signal"}
	nsOut, _ := json.Marshal(noSignal)
	if strings.Contains(string(nsOut), "fit_distance") {
		t.Fatalf("no-signal thread JSON must omit fit_distance (nil), got %s", nsOut)
	}
}

func TestComputeThreadFitDistances_ThreadSectionPairingByIndex(t *testing.T) {
	// 2 sections x 2 threads. Uses FOUR mutually-distinct thread vectors (the
	// unit basis vectors) so that any intra-batch or cross-batch index mix-up
	// produces a detectably wrong distance. The previous version reused only
	// two vectors in a criss-cross (threadA1==threadB0, threadA0==threadB1),
	// which masked "threads swapping places within a batch".
	//
	// Sections are also basis vectors offset so each batch's two threads land
	// at different distances (parallel ~0 vs orthogonal ~1), making an
	// intra-batch swap fail the assertion.
	secA := []float64{1, 0, 0, 0}
	secB := []float64{0, 0, 1, 0}

	threadA0 := []float64{1, 0, 0, 0} // basis 0 — parallel to secA
	threadA1 := []float64{0, 1, 0, 0} // basis 1 — orthogonal to secA
	threadB0 := []float64{0, 0, 1, 0} // basis 2 — parallel to secB
	threadB1 := []float64{0, 0, 0, 1} // basis 3 — orthogonal to secB

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
		{0, 0, 0, 0.05},    // A0 parallel to secA → ~0
		{0, 1, 0.95, 1.05}, // A1 orthogonal to secA → ~1
		{1, 0, 0, 0.05},    // B0 parallel to secB → ~0
		{1, 1, 0.95, 1.05}, // B1 orthogonal to secB → ~1
	}
	for _, e := range expect {
		ptr := threadBatches[e.sec][e.th].FitDistance
		if ptr == nil {
			t.Fatalf("threadBatches[%d][%d] FitDistance should be set (non-nil), got nil", e.sec, e.th)
		}
		got := *ptr
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

func TestComputeThreadFitDistances_ShortEmbeddingResultNoPanic(t *testing.T) {
	// Provider returns fewer embeddings than titles requested → function must
	// early-return without panic and leave every thread at zero signal.
	// Covers the `len(result.Embeddings) < len(texts)` branch.
	sections := []repository.DailyReportSection{
		{ID: 1, Embedding: repository.FloatsToPgVector([]float64{1, 0, 0, 0})},
	}
	threadBatches := [][]repository.DailyReportThread{
		{{Title: "a"}, {Title: "b"}, {Title: "c"}}, // 3 titles requested
	}

	embed := func(_ context.Context, _ airouter.EmbeddingRequest, _ airouter.Capability) (*airouter.EmbeddingResult, error) {
		return &airouter.EmbeddingResult{Embeddings: [][]float64{{1, 0, 0, 0}}}, nil // only 1 returned
	}

	computeThreadFitDistances(context.Background(), sections, threadBatches, 1, embed)

	for i, th := range threadBatches[0] {
		if th.FitDistance != nil {
			t.Fatalf("thread %d FitDistance should be nil (no signal) on short result, got %v", i, *th.FitDistance)
		}
		if th.Embedding != "" {
			t.Fatalf("thread %d Embedding should be empty on short result, got %q", i, th.Embedding)
		}
	}
}

func TestComputeThreadFitDistances_EmptyOrWhitespaceTitleSkipped(t *testing.T) {
	// Threads whose title is empty or pure whitespace must be skipped: not
	// collected, not sent to embed (no wasted embed call). Covers the
	// strings.TrimSpace skip branch.
	var captured []string
	embed := func(_ context.Context, req airouter.EmbeddingRequest, _ airouter.Capability) (*airouter.EmbeddingResult, error) {
		captured = append(captured, req.Input...)
		return &airouter.EmbeddingResult{Embeddings: [][]float64{{1, 0, 0, 0}}}, nil
	}

	sections := []repository.DailyReportSection{
		{ID: 1, Embedding: repository.FloatsToPgVector([]float64{1, 0, 0, 0})},
	}
	threadBatches := [][]repository.DailyReportThread{
		{
			{Title: "real thread"},
			{Title: "   "}, // pure whitespace — must be skipped
			{Title: ""},    // empty — must be skipped
		},
	}

	computeThreadFitDistances(context.Background(), sections, threadBatches, 1, embed)

	if len(captured) != 1 {
		t.Fatalf("embed should receive exactly 1 title (only the real one), got %d: %v", len(captured), captured)
	}
	if captured[0] != "real thread" {
		t.Fatalf("embed input = %q, want %q", captured[0], "real thread")
	}
	// Skipped threads keep zero signal.
	for _, idx := range []int{1, 2} {
		if threadBatches[0][idx].FitDistance != nil || threadBatches[0][idx].Embedding != "" {
			t.Fatalf("thread[%d] (empty/whitespace title) should be skipped, got FitDistance=%v Embedding=%q",
				idx, threadBatches[0][idx].FitDistance, threadBatches[0][idx].Embedding)
		}
	}
	// The real thread is populated.
	if threadBatches[0][0].FitDistance == nil {
		t.Fatalf("real thread FitDistance should be set (non-nil), got nil")
	}
	if threadBatches[0][0].Embedding == "" {
		t.Fatalf("real thread Embedding should be populated, got empty")
	}
}
