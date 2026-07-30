package board

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"syntopica-backend/internal/models"
	"syntopica-backend/internal/platform/testutil"
	"syntopica-backend/internal/tagmanagement/repository"
	"syntopica-backend/internal/tagmanagement/service/core"
)

func setupSemanticBoardMatchingTestDB(t *testing.T) *gorm.DB {
	db := testutil.SetupTestDB(t)
	repository.InitRepository(db)
	InvalidateBoardCache() // 避免包级缓存跨测试残留（ResetTestData 清 db 但不清内存缓存）
	return db
}

func TestSemanticBoardMatchingDirectHit(t *testing.T) {
	db := setupSemanticBoardMatchingTestDB(t)
	tag := createMatchTag(t, db, "openai-gpt-5")
	auxiliary := createMatchLabel(t, db, "OpenAI", "openai", "auxiliary", "active", []float64{1, 0, 0})
	secondaryAuxiliary := createMatchLabel(t, db, "GPT", "gpt", "auxiliary", "active", []float64{0, 1, 0})
	board := createMatchLabel(t, db, "AI Board", "ai-board", "board", "active", nil)
	require.NoError(t, db.Create(&models.TopicTagSemanticLabel{TopicTagID: tag.ID, SemanticLabelID: auxiliary.ID}).Error)
	require.NoError(t, db.Create(&models.TopicTagSemanticLabel{TopicTagID: tag.ID, SemanticLabelID: secondaryAuxiliary.ID}).Error)
	require.NoError(t, db.Create(&models.BoardComposition{BoardID: board.ID, AuxiliaryLabelID: auxiliary.ID}).Error)
	require.NoError(t, db.Create(&models.BoardComposition{BoardID: board.ID, AuxiliaryLabelID: secondaryAuxiliary.ID}).Error)
	service := NewSemanticBoardMatchingService(db)

	results, err := service.MatchTopicTag(context.Background(), tag.ID)

	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, board.ID, results[0].SemanticBoardID)
	require.Equal(t, 1.0, results[0].Score)
	require.Equal(t, "direct_hit", results[0].MatchReason)

	var rows []models.TopicTagBoardLabel
	require.NoError(t, db.Find(&rows).Error)
	require.Len(t, rows, 1)
	require.Equal(t, tag.ID, rows[0].TopicTagID)
	require.Equal(t, board.ID, rows[0].SemanticBoardID)
	require.Equal(t, "direct_hit", rows[0].MatchReason)
}

func TestSemanticBoardMatchingThreeRules(t *testing.T) {
	db := setupSemanticBoardMatchingTestDB(t)
	upsertMatchSetting(t, db, "semantic_board_match_sim_threshold", "0.6")
	upsertMatchSetting(t, db, "semantic_board_match_direct_max_sim_min_hits", "1")
	upsertMatchSetting(t, db, "semantic_board_match_direct_max_sim_min_hit_rate", "0")
	upsertMatchSetting(t, db, "semantic_board_match_min_effective_sample", "1")
	upsertMatchSetting(t, db, "semantic_board_match_direct_hit_min_overlap", "1")
	tag := createMatchTag(t, db, "model-release")
	tagAuxA := createMatchLabel(t, db, "OpenAI", "openai", "auxiliary", "active", []float64{1, 0, 0})
	tagAuxB := createMatchLabel(t, db, "Release", "release", "auxiliary", "active", []float64{0, 1, 0})
	require.NoError(t, db.Create(&models.TopicTagSemanticLabel{TopicTagID: tag.ID, SemanticLabelID: tagAuxA.ID}).Error)
	require.NoError(t, db.Create(&models.TopicTagSemanticLabel{TopicTagID: tag.ID, SemanticLabelID: tagAuxB.ID}).Error)

	hitRateBoard := createMatchBoardWithAuxiliaries(t, db, "hit-rate", [][]float64{{0.7, 0.5, 0.509901951359279}, {0.5, 0.7, 0.509901951359279}})
	maxSimBoard := createMatchBoardWithAuxiliaries(t, db, "max-sim", [][]float64{{1, 0, 0}})
	weightedBoard := createMatchBoardWithAuxiliaries(t, db, "weighted", [][]float64{{0.7, 0.5, 0.509901951359279}})
	service := NewSemanticBoardMatchingService(db)

	results, err := service.MatchTopicTag(context.Background(), tag.ID)

	require.NoError(t, err)
	require.Len(t, results, 3)
	byBoard := map[uint]SemanticBoardMatchResult{}
	for _, result := range results {
		byBoard[result.SemanticBoardID] = result
	}
	require.Equal(t, "hit_rate", byBoard[hitRateBoard.ID].MatchReason)
	require.InDelta(t, 0.79, byBoard[hitRateBoard.ID].Score, 0.01)
	require.Equal(t, "max_sim", byBoard[maxSimBoard.ID].MatchReason)
	require.InDelta(t, 1.0, byBoard[maxSimBoard.ID].Score, 0.0001)
	require.Equal(t, "weighted", byBoard[weightedBoard.ID].MatchReason)
	require.InDelta(t, 0.62, byBoard[weightedBoard.ID].Score, 0.0001)
}

func TestSemanticBoardMatchingMaxBoardsTruncation(t *testing.T) {
	db := setupSemanticBoardMatchingTestDB(t)
	upsertMatchSetting(t, db, "semantic_board_match_direct_hit_rate", "1")
	upsertMatchSetting(t, db, "semantic_board_match_max_boards", "2")
	upsertMatchSetting(t, db, "semantic_board_match_min_effective_sample", "1")
	upsertMatchSetting(t, db, "semantic_board_match_direct_hit_min_overlap", "1")
	tag := createMatchTag(t, db, "ranked-boards")
	tagAux := createMatchLabel(t, db, "GPU", "gpu", "auxiliary", "active", []float64{1, 0, 0})
	require.NoError(t, db.Create(&models.TopicTagSemanticLabel{TopicTagID: tag.ID, SemanticLabelID: tagAux.ID}).Error)
	top := createMatchBoardWithAuxiliaries(t, db, "top", [][]float64{{0.95, 0.31224989991992, 0}})
	second := createMatchBoardWithAuxiliaries(t, db, "second", [][]float64{{0.9, 0.435889894354067, 0}})
	third := createMatchBoardWithAuxiliaries(t, db, "third", [][]float64{{0.85, 0.526782687642637, 0}})
	service := NewSemanticBoardMatchingService(db)

	results, err := service.MatchTopicTag(context.Background(), tag.ID)

	require.NoError(t, err)
	require.Len(t, results, 2)
	require.Equal(t, top.ID, results[0].SemanticBoardID)
	require.Equal(t, second.ID, results[1].SemanticBoardID)
	for _, result := range results {
		require.NotEqual(t, third.ID, result.SemanticBoardID)
	}
	var rows []models.TopicTagBoardLabel
	require.NoError(t, db.Order("score desc").Find(&rows).Error)
	require.Len(t, rows, 2)
}

func TestSemanticBoardMatchingNoMatchReplacesExistingLabels(t *testing.T) {
	db := setupSemanticBoardMatchingTestDB(t)
	tag := createMatchTag(t, db, "no-match")
	otherTag := createMatchTag(t, db, "other")
	oldBoard := createMatchLabel(t, db, "Old Board", "old-board", "board", "active", nil)
	otherBoard := createMatchLabel(t, db, "Other Board", "other-board", "board", "active", nil)
	tagAux := createMatchLabel(t, db, "OpenAI", "openai", "auxiliary", "active", []float64{1, 0, 0})
	boardAux := createMatchLabel(t, db, "Hardware", "hardware", "auxiliary", "active", []float64{0, 1, 0})
	require.NoError(t, db.Create(&models.TopicTagSemanticLabel{TopicTagID: tag.ID, SemanticLabelID: tagAux.ID}).Error)
	require.NoError(t, db.Create(&models.BoardComposition{BoardID: oldBoard.ID, AuxiliaryLabelID: boardAux.ID}).Error)
	require.NoError(t, db.Create(&models.TopicTagBoardLabel{TopicTagID: tag.ID, SemanticBoardID: oldBoard.ID, Score: 0.8, MatchReason: "existing"}).Error)
	require.NoError(t, db.Create(&models.TopicTagBoardLabel{TopicTagID: otherTag.ID, SemanticBoardID: otherBoard.ID, Score: 0.9, MatchReason: "existing"}).Error)
	service := NewSemanticBoardMatchingService(db)

	results, err := service.MatchTopicTag(context.Background(), tag.ID)

	require.NoError(t, err)
	require.Empty(t, results)
	var tagRows int64
	require.NoError(t, db.Model(&models.TopicTagBoardLabel{}).Where("topic_tag_id = ?", tag.ID).Count(&tagRows).Error)
	require.Zero(t, tagRows)
	var otherRows int64
	require.NoError(t, db.Model(&models.TopicTagBoardLabel{}).Where("topic_tag_id = ?", otherTag.ID).Count(&otherRows).Error)
	require.Equal(t, int64(1), otherRows)
}

func TestSemanticBoardMatchingColdStartNoBoard(t *testing.T) {
	db := setupSemanticBoardMatchingTestDB(t)
	tag := createMatchTag(t, db, "cold-start")
	oldBoard := createMatchLabel(t, db, "Old Board", "old-board", "board", "active", nil)
	auxiliary := createMatchLabel(t, db, "OpenAI", "openai", "auxiliary", "active", []float64{1, 0, 0})
	require.NoError(t, db.Create(&models.TopicTagSemanticLabel{TopicTagID: tag.ID, SemanticLabelID: auxiliary.ID}).Error)
	require.NoError(t, db.Create(&models.TopicTagBoardLabel{TopicTagID: tag.ID, SemanticBoardID: oldBoard.ID, Score: 0.8, MatchReason: "existing"}).Error)
	service := NewSemanticBoardMatchingService(db)

	results, err := service.MatchTopicTag(context.Background(), tag.ID)

	require.NoError(t, err)
	require.Empty(t, results)
	var rows int64
	require.NoError(t, db.Model(&models.TopicTagBoardLabel{}).Where("topic_tag_id = ?", tag.ID).Count(&rows).Error)
	require.Zero(t, rows)
}

func TestSemanticBoardMatchingIgnoresDisabledLabels(t *testing.T) {
	db := setupSemanticBoardMatchingTestDB(t)
	tag := createMatchTag(t, db, "disabled-labels")
	activeAux := createMatchLabel(t, db, "OpenAI", "openai", "auxiliary", "active", []float64{1, 0, 0})
	disabledAux := createMatchLabel(t, db, "Disabled", "disabled", "auxiliary", "disabled", []float64{0, 1, 0})
	activeBoard := createMatchLabel(t, db, "Active Board", "active-board", "board", "active", nil)
	disabledBoard := createMatchLabel(t, db, "Disabled Board", "disabled-board", "board", "disabled", nil)
	require.NoError(t, db.Create(&models.TopicTagSemanticLabel{TopicTagID: tag.ID, SemanticLabelID: activeAux.ID}).Error)
	require.NoError(t, db.Create(&models.TopicTagSemanticLabel{TopicTagID: tag.ID, SemanticLabelID: disabledAux.ID}).Error)
	require.NoError(t, db.Create(&models.BoardComposition{BoardID: activeBoard.ID, AuxiliaryLabelID: disabledAux.ID}).Error)
	require.NoError(t, db.Create(&models.BoardComposition{BoardID: disabledBoard.ID, AuxiliaryLabelID: activeAux.ID}).Error)
	service := NewSemanticBoardMatchingService(db)

	results, err := service.MatchTopicTag(context.Background(), tag.ID)

	require.NoError(t, err)
	require.Empty(t, results)
}

func createMatchTag(t *testing.T, db *gorm.DB, slug string) models.TopicTag {
	tag := models.TopicTag{Label: slug, Slug: slug, Category: "event", Status: "active"}
	require.NoError(t, db.Create(&tag).Error)
	return tag
}

func createMatchLabel(t *testing.T, db *gorm.DB, label string, slug string, labelType string, status string, vector []float64) models.SemanticLabel {
	t.Helper()
	semanticLabel := models.SemanticLabel{Label: label, Slug: slug, LabelType: labelType, Status: status}
	if vector != nil {
		pgVector := core.FloatsToPgVector(testutil.PadVector(vector, testutil.TestEmbeddingDim))
		semanticLabel.Embedding = &pgVector
	}
	require.NoError(t, db.Create(&semanticLabel).Error)
	return semanticLabel
}

func createMatchBoardWithAuxiliaries(t *testing.T, db *gorm.DB, slug string, vectors [][]float64) models.SemanticLabel {
	t.Helper()
	board := createMatchLabel(t, db, slug, slug, "board", "active", nil)
	for index, vector := range vectors {
		auxiliary := createMatchLabel(t, db, fmt.Sprintf("%s-aux-%d", slug, index), fmt.Sprintf("%s-aux-%d", slug, index), "auxiliary", "active", vector)
		require.NoError(t, db.Create(&models.BoardComposition{BoardID: board.ID, AuxiliaryLabelID: auxiliary.ID}).Error)
	}
	return board
}
