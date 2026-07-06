package dataenrichment

import "context"

// ActiveTopicLister returns the IDs of all active persistent topics.
// Production implementation queries board_persistent_topics WHERE status='active'.
type ActiveTopicLister interface {
	ListActiveTopicIDs(ctx context.Context) ([]uint, error)
}
