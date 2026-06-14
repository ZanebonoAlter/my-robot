package board

import (
	"testing"

	"syntopica-backend/internal/tagmanagement/service/core"
)

func TestLoadClusterConfig_Defaults(t *testing.T) {
	cfg := core.DefaultClusterConfig

	if cfg.KwMinOverlap != 2 {
		t.Fatalf("default KwMinOverlap = %d, want 2", cfg.KwMinOverlap)
	}
	if cfg.SemThreshold != 0.80 {
		t.Fatalf("default SemThreshold = %.2f, want 0.80", cfg.SemThreshold)
	}
	if cfg.MaxTags != 500 {
		t.Fatalf("default MaxTags = %d, want 500", cfg.MaxTags)
	}
	if cfg.SimilarityThreshold != 0.85 {
		t.Fatalf("default SimilarityThreshold = %.2f, want 0.85", cfg.SimilarityThreshold)
	}
	if cfg.MaxClusterSize != 8 {
		t.Fatalf("default MaxClusterSize = %d, want 8", cfg.MaxClusterSize)
	}
}

func TestFindConnectedComponents_SinglePair(t *testing.T) {
	edges := []core.SimilarityEdge{
		{TagAID: 1, TagBID: 2, Similarity: 0.85},
	}
	comp := findConnectedComponents([]uint{1, 2}, edges)
	if len(comp) != 1 {
		t.Fatalf("expected 1 component, got %d", len(comp))
	}
	if len(comp[0]) != 2 {
		t.Fatalf("expected component size 2, got %d", len(comp[0]))
	}
}

func TestFindConnectedComponents_Chain(t *testing.T) {
	edges := []core.SimilarityEdge{
		{TagAID: 1, TagBID: 2, Similarity: 0.85},
		{TagAID: 2, TagBID: 3, Similarity: 0.82},
	}
	comp := findConnectedComponents([]uint{1, 2, 3}, edges)
	if len(comp) != 1 {
		t.Fatalf("expected 1 component, got %d", len(comp))
	}
	if len(comp[0]) != 3 {
		t.Fatalf("expected component size 3, got %d", len(comp[0]))
	}
}

func TestFindConnectedComponents_Disconnected(t *testing.T) {
	edges := []core.SimilarityEdge{
		{TagAID: 1, TagBID: 2, Similarity: 0.85},
		{TagAID: 3, TagBID: 4, Similarity: 0.90},
	}
	comp := findConnectedComponents([]uint{1, 2, 3, 4}, edges)
	if len(comp) != 2 {
		t.Fatalf("expected 2 components, got %d", len(comp))
	}
}

func TestFindConnectedComponents_SingleNode(t *testing.T) {
	edges := []core.SimilarityEdge{
		{TagAID: 1, TagBID: 2, Similarity: 0.85},
	}
	comp := findConnectedComponents([]uint{1, 2, 3}, edges)
	if len(comp) != 1 {
		t.Fatalf("expected 1 component (single node excluded), got %d", len(comp))
	}
}

func TestFindConnectedComponents_Empty(t *testing.T) {
	comp := findConnectedComponents(nil, nil)
	if len(comp) != 0 {
		t.Fatalf("expected 0 components, got %d", len(comp))
	}
}
