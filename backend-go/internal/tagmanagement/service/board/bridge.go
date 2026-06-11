package board

import (
	"context"

	"syntopica-backend/internal/tagmanagement/service/core"
)

func init() {
	core.BoardMatcherFactory = func() core.BoardMatcher {
		s := getSemanticBoardMatchingService()
		if s == nil {
			return nil
		}
		return &boardMatcherAdapter{s: s}
	}
}

type boardMatcherAdapter struct {
	s *SemanticBoardMatchingService
}

func (a *boardMatcherAdapter) MatchTopicTag(ctx context.Context, topicTagID uint) (interface{}, error) {
	return a.s.MatchTopicTag(ctx, topicTagID)
}
