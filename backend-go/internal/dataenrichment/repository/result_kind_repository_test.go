package repository_test

import (
	"context"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"

	"syntopica-backend/internal/dataenrichment/repository"
	"syntopica-backend/internal/platform/database"
	"syntopica-backend/internal/platform/testutil"
)

func setupResultKindRepositoryDB(t *testing.T) *repository.Repository {
	t.Helper()
	if testing.Short() {
		t.Skip("integration test: requires testcontainer Postgres")
	}
	db := testutil.OpenTestDB(t)
	require.NoError(t, db.Exec("CREATE EXTENSION IF NOT EXISTS vector").Error)
	require.NoError(t, database.RunAutoMigrate(db))
	require.NoError(t, db.Exec(`DELETE FROM topic_enrichment_result WHERE session_id LIKE 'result-kind-repo-%'`).Error)
	t.Cleanup(func() {
		_ = db.Exec(`DELETE FROM topic_enrichment_result WHERE session_id LIKE 'result-kind-repo-%' AND parent_result_id IS NOT NULL`).Error
		_ = db.Exec(`DELETE FROM topic_enrichment_result WHERE session_id LIKE 'result-kind-repo-%'`).Error
	})
	return repository.NewRepository(db)
}

func stringPtr(value string) *string { return &value }

func TestQuestionKeyNormalizationHash(t *testing.T) {
	key := repository.ComputeQuestionKey("\t  美国\u3000  制造业\n回流  ")
	require.Equal(t, repository.ComputeQuestionKey("美国 制造业 回流"), key)
	require.Len(t, key, 64)
	_, err := hex.DecodeString(key)
	require.NoError(t, err)
	require.NotEqual(t, key, repository.ComputeQuestionKey("美国制造业回流"), "normalization must preserve token boundaries")
}

func TestBoardResultKindQueries(t *testing.T) {
	repo := setupResultKindRepositoryDB(t)
	ctx := context.Background()
	boardID := uint(98301)

	legacy := &repository.TopicEnrichmentResult{
		SemanticBoardID: repository.BoardIDPtr(boardID), AnalysisScope: "board",
		ResultKind: repository.ResultKindLegacyBoardAnalysis, SessionID: "result-kind-repo-legacy",
	}
	brief1 := &repository.TopicEnrichmentResult{
		SemanticBoardID: repository.BoardIDPtr(boardID), AnalysisScope: "board",
		ResultKind: repository.ResultKindBoardBrief, SessionID: "result-kind-repo-brief-1",
	}
	brief2 := &repository.TopicEnrichmentResult{
		SemanticBoardID: repository.BoardIDPtr(boardID), AnalysisScope: "board",
		ResultKind: repository.ResultKindBoardBrief, SessionID: "result-kind-repo-brief-2",
	}
	for _, result := range []*repository.TopicEnrichmentResult{legacy, brief1, brief2} {
		require.NoError(t, repo.CreateTopicEnrichmentResult(ctx, result))
	}

	briefs, err := repo.ListBoardEnrichmentResultsByKind(ctx, boardID, repository.ResultKindBoardBrief)
	require.NoError(t, err)
	require.Len(t, briefs, 2)
	require.Equal(t, brief2.ID, briefs[0].ID)

	latest, err := repo.GetLatestBoardEnrichmentResultByKind(ctx, boardID, repository.ResultKindBoardBrief)
	require.NoError(t, err)
	require.Equal(t, brief2.ID, latest.ID)
	prev, err := repo.GetPrevLatestBoardEnrichmentResultByKind(ctx, boardID, repository.ResultKindBoardBrief, brief2.ID)
	require.NoError(t, err)
	require.Equal(t, brief1.ID, prev.ID)

	all, err := repo.ListBoardEnrichmentResults(ctx, boardID)
	require.NoError(t, err)
	require.Len(t, all, 3, "legacy unfiltered API must remain compatible")
}

func TestBoardInvestigationParentValidationAndMultipleChildren(t *testing.T) {
	repo := setupResultKindRepositoryDB(t)
	ctx := context.Background()
	boardID := uint(98311)
	otherBoardID := uint(98312)

	brief := &repository.TopicEnrichmentResult{
		SemanticBoardID: repository.BoardIDPtr(boardID), AnalysisScope: "board",
		ResultKind: repository.ResultKindBoardBrief, SessionID: "result-kind-repo-parent",
	}
	otherBrief := &repository.TopicEnrichmentResult{
		SemanticBoardID: repository.BoardIDPtr(otherBoardID), AnalysisScope: "board",
		ResultKind: repository.ResultKindBoardBrief, SessionID: "result-kind-repo-other-parent",
	}
	legacy := &repository.TopicEnrichmentResult{
		SemanticBoardID: repository.BoardIDPtr(boardID), AnalysisScope: "board",
		ResultKind: repository.ResultKindLegacyBoardAnalysis, SessionID: "result-kind-repo-non-brief",
	}
	for _, result := range []*repository.TopicEnrichmentResult{brief, otherBrief, legacy} {
		require.NoError(t, repo.CreateTopicEnrichmentResult(ctx, result))
	}

	questionKey1 := repository.ComputeQuestionKey("为什么制造业投资增加？")
	questionKey2 := repository.ComputeQuestionKey("就业是否同步改善？")
	child1 := &repository.TopicEnrichmentResult{
		SemanticBoardID: repository.BoardIDPtr(boardID), AnalysisScope: "board",
		ParentResultID: repository.TopicIDPtr(brief.ID), QuestionKey: stringPtr(questionKey1),
		SessionID: "result-kind-repo-child-1",
	}
	child2 := &repository.TopicEnrichmentResult{
		SemanticBoardID: repository.BoardIDPtr(boardID), AnalysisScope: "board",
		ParentResultID: repository.TopicIDPtr(brief.ID), QuestionKey: stringPtr(questionKey2),
		SessionID: "result-kind-repo-child-2",
	}
	for _, child := range []*repository.TopicEnrichmentResult{child1, child2} {
		require.NoError(t, repo.CreateBoardInvestigationResult(ctx, child))
		require.Equal(t, repository.ResultKindBoardInvestigation, child.ResultKind)
	}

	children, err := repo.ListBoardEnrichmentResultsByParent(ctx, brief.ID)
	require.NoError(t, err)
	require.Len(t, children, 2)
	require.Equal(t, child2.ID, children[0].ID)

	for name, parentID := range map[string]uint{
		"cross-board": otherBrief.ID,
		"non-brief":   legacy.ID,
	} {
		t.Run(name, func(t *testing.T) {
			invalid := &repository.TopicEnrichmentResult{
				SemanticBoardID: repository.BoardIDPtr(boardID), AnalysisScope: "board",
				ParentResultID: repository.TopicIDPtr(parentID), QuestionKey: stringPtr(questionKey1),
				SessionID: "result-kind-repo-invalid-" + name,
			}
			require.Error(t, repo.CreateBoardInvestigationResult(ctx, invalid))
			require.Zero(t, invalid.ID)
		})
	}

	missingKey := &repository.TopicEnrichmentResult{
		SemanticBoardID: repository.BoardIDPtr(boardID), AnalysisScope: "board",
		ParentResultID: repository.TopicIDPtr(brief.ID), SessionID: "result-kind-repo-missing-key",
	}
	require.Error(t, repo.CreateBoardInvestigationResult(ctx, missingKey))

	illegalParent := &repository.TopicEnrichmentResult{
		SemanticBoardID: repository.BoardIDPtr(boardID), AnalysisScope: "board",
		ResultKind: repository.ResultKindBoardBrief, ParentResultID: repository.TopicIDPtr(brief.ID),
		SessionID: "result-kind-repo-illegal-parent",
	}
	require.Error(t, repo.CreateTopicEnrichmentResult(ctx, illegalParent))
}
