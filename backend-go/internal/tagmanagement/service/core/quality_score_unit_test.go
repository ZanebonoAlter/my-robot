package core

import (
	"testing"
)

func TestPercentileRankStableRange(t *testing.T) {
	values := map[uint]float64{1: 10, 2: 20, 3: 20, 4: 50}

	lowest := percentileRank(values, 1)
	middle := percentileRank(values, 2)
	highest := percentileRank(values, 4)

	if lowest < 0 || lowest > 1 {
		t.Fatalf("lowest percentile out of range: %f", lowest)
	}
	if middle < 0 || middle > 1 {
		t.Fatalf("middle percentile out of range: %f", middle)
	}
	if highest < 0 || highest > 1 {
		t.Fatalf("highest percentile out of range: %f", highest)
	}
	if !(lowest < middle && middle < highest) {
		t.Fatalf("expected ordered percentiles, got lowest=%f middle=%f highest=%f", lowest, middle, highest)
	}
}
