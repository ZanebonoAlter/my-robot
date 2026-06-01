package tagging

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"syntopica-backend/internal/domain/models"
	"syntopica-backend/internal/platform/airouter"
	"syntopica-backend/internal/platform/database"
	"syntopica-backend/internal/platform/logging"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type AuxLabelGCMode string

const (
	AuxLabelGCModeDryRun      AuxLabelGCMode = "dry_run"
	AuxLabelGCModeDisable     AuxLabelGCMode = "disable"
	AuxLabelGCModeDelete      AuxLabelGCMode = "delete"
	AuxLabelGCModeRecalculate AuxLabelGCMode = "recalculate"
)

type AuxLabelGCRequest struct {
	Mode      AuxLabelGCMode `json:"mode"`
	GraceDays int            `json:"grace_days"`
}

type AuxLabelGCResult struct {
	EligibleCount  int                     `json:"eligible_count"`
	AffectedCount  int                     `json:"affected_count"`
	CorrectedCount int                     `json:"corrected_count,omitempty"`
	TotalCount     int                     `json:"total_count,omitempty"`
	Preview        []AuxLabelGCPreviewItem `json:"preview,omitempty"`
}

type AuxLabelGCPreviewItem struct {
	ID        uint   `json:"id"`
	Label     string `json:"label"`
	RefCount  int    `json:"ref_count"`
	CreatedAt string `json:"created_at"`
}

const auxiliaryLabelMergeThreshold = 0.95

var ensureVectorDimOnce sync.Once

type auxiliaryLabelEmbeddingMode string

const (
	auxiliaryLabelEmbeddingModeMerge   auxiliaryLabelEmbeddingMode = "merge"
	auxiliaryLabelEmbeddingModeStorage auxiliaryLabelEmbeddingMode = "storage"
)

type auxiliaryLabelEmbedder func(ctx context.Context, input string, mode auxiliaryLabelEmbeddingMode) (string, []float64, error)

type mergeMatcherFunc func(ctx context.Context, db *gorm.DB, labels []models.SemanticLabel, mergePgVector string, mergeVector []float64) (*models.SemanticLabel, error)

type AuxiliaryLabelService struct {
	db            *gorm.DB
	embedder      auxiliaryLabelEmbedder
	mergeMatcher  mergeMatcherFunc
}

func NewAuxiliaryLabelService(db *gorm.DB, embedder auxiliaryLabelEmbedder) *AuxiliaryLabelService {
	if db == nil {
		db = database.DB
	}
	if embedder == nil {
		embedder = defaultAuxiliaryLabelEmbedder
	}
	return &AuxiliaryLabelService{db: db, embedder: embedder, mergeMatcher: sqlMergeMatcher}
}

// vectorDimEnsurers holds callbacks registered by other packages to ensure their
// vector columns have the correct dimension. Called once at startup after the
// embedding dimension is determined.
var vectorDimEnsurers []func(dim int)

// RegisterVectorDimEnsurer registers a callback to be invoked at startup with the
// detected embedding dimension. Call from init() in domain packages that own vector columns.
func RegisterVectorDimEnsurer(fn func(dim int)) {
	vectorDimEnsurers = append(vectorDimEnsurers, fn)
}

// EnsureVectorDimensionOnce ensures all vector column dimensions match the embedder output.
// Called once at startup. Uses the global DB to avoid calling DDL inside a transaction.
func EnsureVectorDimensionOnce(ctx context.Context) {
	ensureVectorDimOnce.Do(func() {
		_, vector, err := defaultAuxiliaryLabelEmbedder(ctx, "dimension-check", auxiliaryLabelEmbeddingModeStorage)
		if err != nil {
			logging.Warnf("Failed to determine embedding dimension: %v", err)
			return
		}
		dim := len(vector)
		if err := EnsureSemanticLabelVectorDimension(dim); err != nil {
			logging.Warnf("Failed to ensure embedding vector dimension: %v", err)
		}
		if err := EnsureSemanticLabelMergeVectorDimension(dim); err != nil {
			logging.Warnf("Failed to ensure merge_embedding vector dimension: %v", err)
		}
		for _, fn := range vectorDimEnsurers {
			fn(dim)
		}
	})
}

func (s *AuxiliaryLabelService) AttachAuxiliaryLabels(ctx context.Context, topicTagID uint, labels []AuxiliaryLabel) error {
	if topicTagID == 0 || len(labels) == 0 {
		return nil
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txService := NewAuxiliaryLabelService(tx, s.embedder)
		for _, item := range labels {
			label, err := txService.ResolveAuxiliaryLabel(ctx, item.Label, item.Description)
			if err != nil {
				return err
			}
			link := models.TopicTagSemanticLabel{TopicTagID: topicTagID, SemanticLabelID: label.ID}
			res := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&link)
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected > 0 {
				if err := tx.Model(&models.SemanticLabel{}).Where("id = ?", label.ID).UpdateColumn("ref_count", gorm.Expr("ref_count + 1")).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func (s *AuxiliaryLabelService) ResolveAuxiliaryLabel(ctx context.Context, rawLabel, description string) (*models.SemanticLabel, error) {
	label := strings.TrimSpace(rawLabel)
	if label == "" {
		return nil, fmt.Errorf("auxiliary label must not be empty")
	}
	if _, generic := genericAuxiliaryLabels[label]; generic {
		return nil, fmt.Errorf("auxiliary label %q is too generic", label)
	}
	slug := Slugify(label)
	if slug == "" {
		return nil, fmt.Errorf("auxiliary label slug is empty")
	}

	description = strings.TrimSpace(description)

	labels, err := s.loadActiveAuxiliaryLabels(ctx)
	if err != nil {
		return nil, err
	}

	// L1: exact match by slug or alias
	for _, existing := range labels {
		if existing.Slug == slug || semanticAliasesContain(existing.Aliases, label) {
			return &existing, nil
		}
	}

	// L2: merge embedding comparison (SQL for pgvector, Go fallback for SQLite tests)
	mergePgVector, mergeVector, err := s.embedder(ctx, label, auxiliaryLabelEmbeddingModeMerge)
	if err != nil {
		return nil, err
	}

	bestMatch, err := s.mergeMatcher(ctx, s.db, labels, mergePgVector, mergeVector)
	if err != nil {
		return nil, err
	}
	if bestMatch != nil {
		return s.addAlias(ctx, bestMatch, label)
	}

	// L3: create new — storage embedding from label+description, reuse L2 merge embedding
	storageInput := label
	if description != "" {
		storageInput = label + ": " + description
	}
	storagePgVector, _, err := s.embedder(ctx, storageInput, auxiliaryLabelEmbeddingModeStorage)
	if err != nil {
		return nil, err
	}

	created := models.SemanticLabel{
		Label:          label,
		Slug:           uniqueSemanticLabelSlug(s.db.WithContext(ctx), slug),
		LabelType:      "auxiliary",
		Source:         "llm_extract",
		Status:         "active",
		Embedding:      &storagePgVector,
		MergeEmbedding: &mergePgVector,
	}
	if description != "" {
		created.Description = description
	}
	if err := s.db.WithContext(ctx).Create(&created).Error; err != nil {
		return nil, err
	}
	return &created, nil
}

func (s *AuxiliaryLabelService) DisableAuxiliaryLabel(ctx context.Context, labelID uint) error {
	if labelID == 0 {
		return fmt.Errorf("auxiliary label id is required")
	}

	var label models.SemanticLabel
	if err := s.db.WithContext(ctx).Where("id = ? AND label_type = ?", labelID, "auxiliary").First(&label).Error; err != nil {
		return err
	}
	return s.db.WithContext(ctx).Model(&label).Update("status", "disabled").Error
}

func (s *AuxiliaryLabelService) MergeAuxiliaryLabelAlias(ctx context.Context, sourceID uint, targetID uint) error {
	if sourceID == 0 || targetID == 0 {
		return fmt.Errorf("source and target auxiliary label ids are required")
	}
	if sourceID == targetID {
		return fmt.Errorf("source and target auxiliary label ids must be different")
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var source models.SemanticLabel
		if err := tx.Where("id = ? AND label_type = ?", sourceID, "auxiliary").First(&source).Error; err != nil {
			return err
		}
		var target models.SemanticLabel
		if err := tx.Where("id = ? AND label_type = ?", targetID, "auxiliary").First(&target).Error; err != nil {
			return err
		}

		for _, alias := range append([]string{source.Label}, source.Aliases...) {
			if !strings.EqualFold(target.Label, alias) && !semanticAliasesContain(target.Aliases, alias) {
				target.Aliases = append(target.Aliases, alias)
			}
		}
		if err := tx.Save(&target).Error; err != nil {
			return err
		}

		var links []models.TopicTagSemanticLabel
		if err := tx.Where("semantic_label_id = ?", sourceID).Find(&links).Error; err != nil {
			return err
		}
		for _, link := range links {
			migrated := models.TopicTagSemanticLabel{TopicTagID: link.TopicTagID, SemanticLabelID: targetID}
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&migrated).Error; err != nil {
				return err
			}
		}
		if err := tx.Where("semantic_label_id = ?", sourceID).Delete(&models.TopicTagSemanticLabel{}).Error; err != nil {
			return err
		}

		var targetRefCount int64
		if err := tx.Model(&models.TopicTagSemanticLabel{}).Where("semantic_label_id = ?", targetID).Count(&targetRefCount).Error; err != nil {
			return err
		}
		var sourceRefCount int64
		if err := tx.Model(&models.TopicTagSemanticLabel{}).Where("semantic_label_id = ?", sourceID).Count(&sourceRefCount).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.SemanticLabel{}).Where("id = ?", targetID).Update("ref_count", int(targetRefCount)).Error; err != nil {
			return err
		}
		return tx.Model(&models.SemanticLabel{}).Where("id = ?", sourceID).Updates(map[string]any{"ref_count": int(sourceRefCount), "status": "disabled"}).Error
	})
}

func (s *AuxiliaryLabelService) RemoveBoardComposition(ctx context.Context, boardID uint, auxiliaryLabelID uint) error {
	if boardID == 0 || auxiliaryLabelID == 0 {
		return fmt.Errorf("board and auxiliary label ids are required")
	}

	var board models.SemanticLabel
	if err := s.db.WithContext(ctx).Where("id = ? AND label_type = ?", boardID, "board").First(&board).Error; err != nil {
		return err
	}
	var auxiliary models.SemanticLabel
	if err := s.db.WithContext(ctx).Where("id = ? AND label_type = ?", auxiliaryLabelID, "auxiliary").First(&auxiliary).Error; err != nil {
		return err
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("board_id = ? AND auxiliary_label_id = ?", boardID, auxiliaryLabelID).
			Delete(&models.BoardComposition{}).Error; err != nil {
			return err
		}
		if err := tx.Exec(
			`DELETE FROM topic_tag_board_labels
			 WHERE semantic_board_id = ?
			 AND EXISTS (
			   SELECT 1 FROM topic_tag_semantic_labels
			   WHERE topic_tag_semantic_labels.topic_tag_id = topic_tag_board_labels.topic_tag_id
			   AND topic_tag_semantic_labels.semantic_label_id = ?
			 )`, boardID, auxiliaryLabelID,
		).Error; err != nil {
			return err
		}
		return nil
	})
}

// RecountRefs recalculates ref_count for the given auxiliary label IDs by counting
// actual topic_tag_semantic_labels rows. This is self-healing: it corrects any
// accumulated drift in ref_count values.
func (s *AuxiliaryLabelService) RecountRefs(ctx context.Context, ids []uint) error {
	if len(ids) == 0 {
		return nil
	}
	for _, id := range ids {
		var count int64
		if err := s.db.WithContext(ctx).Model(&models.TopicTagSemanticLabel{}).
			Where("semantic_label_id = ?", id).Count(&count).Error; err != nil {
			return fmt.Errorf("recount refs for semantic_label %d: %w", id, err)
		}
		if err := s.db.WithContext(ctx).Model(&models.SemanticLabel{}).
			Where("id = ?", id).Update("ref_count", int(count)).Error; err != nil {
			return fmt.Errorf("update ref_count for semantic_label %d: %w", id, err)
		}
	}
	return nil
}

// GC performs garbage collection on auxiliary labels based on the requested mode.
func (s *AuxiliaryLabelService) GC(ctx context.Context, req AuxLabelGCRequest) (*AuxLabelGCResult, error) {
	switch req.Mode {
	case AuxLabelGCModeRecalculate:
		return s.gcRecalculate(ctx)
	case AuxLabelGCModeDryRun, AuxLabelGCModeDisable, AuxLabelGCModeDelete:
		return s.gcCleanup(ctx, req)
	default:
		return nil, fmt.Errorf("invalid mode %q, must be one of: dry_run, disable, delete, recalculate", req.Mode)
	}
}

func (s *AuxiliaryLabelService) gcRecalculate(ctx context.Context) (*AuxLabelGCResult, error) {
	var ids []uint
	if err := s.db.WithContext(ctx).Model(&models.SemanticLabel{}).
		Where("label_type = ?", "auxiliary").
		Pluck("id", &ids).Error; err != nil {
		return nil, fmt.Errorf("query auxiliary labels: %w", err)
	}

	if len(ids) == 0 {
		return &AuxLabelGCResult{TotalCount: 0, CorrectedCount: 0}, nil
	}

	// Collect before-state for diff
	type refRow struct {
		ID       uint
		RefCount int
	}
	var before []refRow
	s.db.WithContext(ctx).Model(&models.SemanticLabel{}).
		Select("id, ref_count").
		Where("id IN ?", ids).
		Find(&before)
	beforeCounts := make(map[uint]int, len(before))
	for _, r := range before {
		beforeCounts[r.ID] = r.RefCount
	}

	if err := s.RecountRefs(ctx, ids); err != nil {
		return nil, fmt.Errorf("recount refs: %w", err)
	}

	// Count how many actually changed
	var after []refRow
	s.db.WithContext(ctx).Model(&models.SemanticLabel{}).
		Select("id, ref_count").
		Where("id IN ?", ids).
		Find(&after)
	corrected := 0
	for _, r := range after {
		if beforeCounts[r.ID] != r.RefCount {
			corrected++
		}
	}

	return &AuxLabelGCResult{TotalCount: len(ids), CorrectedCount: corrected}, nil
}

func (s *AuxiliaryLabelService) gcCleanup(ctx context.Context, req AuxLabelGCRequest) (*AuxLabelGCResult, error) {
	if req.GraceDays <= 0 {
		req.GraceDays = 1
	}

	cutoff := time.Now().AddDate(0, 0, -req.GraceDays)

	// Find eligible labels: active, unprotected, past grace period, no topic_tag_semantic_labels
	var eligible []models.SemanticLabel
	err := s.db.WithContext(ctx).
		Where("label_type = ? AND status = ? AND protected = false", "auxiliary", "active").
		Where("created_at < ?", cutoff).
		Where("id NOT IN (SELECT DISTINCT semantic_label_id FROM topic_tag_semantic_labels)").
		Find(&eligible).Error
	if err != nil {
		return nil, fmt.Errorf("query eligible labels: %w", err)
	}

	result := &AuxLabelGCResult{EligibleCount: len(eligible)}

	// Preview (first 20)
	previewLimit := 20
	if len(eligible) < previewLimit {
		previewLimit = len(eligible)
	}
	result.Preview = make([]AuxLabelGCPreviewItem, previewLimit)
	for i := 0; i < previewLimit; i++ {
		result.Preview[i] = AuxLabelGCPreviewItem{
			ID:        eligible[i].ID,
			Label:     eligible[i].Label,
			RefCount:  eligible[i].RefCount,
			CreatedAt: eligible[i].CreatedAt.Format(time.RFC3339),
		}
	}

	if req.Mode == AuxLabelGCModeDryRun || len(eligible) == 0 {
		return result, nil
	}

	ids := make([]uint, len(eligible))
	for i, l := range eligible {
		ids[i] = l.ID
	}

	if req.Mode == AuxLabelGCModeDisable {
		// Delete board_composition rows referencing these labels
		if err := s.db.WithContext(ctx).Where("auxiliary_label_id IN ?", ids).
			Delete(&models.BoardComposition{}).Error; err != nil {
			return nil, fmt.Errorf("delete board_composition: %w", err)
		}
		// Soft-delete: update status to disabled
		if err := s.db.WithContext(ctx).Model(&models.SemanticLabel{}).
			Where("id IN ?", ids).
			Update("status", "disabled").Error; err != nil {
			return nil, fmt.Errorf("disable labels: %w", err)
		}
		result.AffectedCount = len(ids)
	} else { // delete
		if err := s.db.WithContext(ctx).Where("id IN ?", ids).
			Delete(&models.SemanticLabel{}).Error; err != nil {
			return nil, fmt.Errorf("delete labels: %w", err)
		}
		result.AffectedCount = len(ids)
	}

	return result, nil
}

func (s *AuxiliaryLabelService) loadActiveAuxiliaryLabels(ctx context.Context) ([]models.SemanticLabel, error) {
	var labels []models.SemanticLabel
	err := s.db.WithContext(ctx).
		Select("id, label, slug, label_type, aliases, ref_count, description, status, protected, source, display_order, created_at, updated_at").
		Where("label_type = ? AND status = ?", "auxiliary", "active").
		Find(&labels).Error
	return labels, err
}

// sqlMergeMatcher loads only id + merge_embedding columns and computes cosine
// similarity in Go. pgvector HNSW cannot index vector(2560) (>2000 dim limit),
// and halfvec expression indexes are not recognized by the query planner, so
// SQL-side ORDER BY <=> is equally slow (~3-5s full scan). Go-side computation
// on the slim result set avoids the 345 MB payload of SELECT *.
func sqlMergeMatcher(ctx context.Context, db *gorm.DB, labels []models.SemanticLabel, _ string, mergeVector []float64) (*models.SemanticLabel, error) {
	// Collect IDs of active auxiliary labels
	ids := make([]uint, 0, len(labels))
	for _, l := range labels {
		ids = append(ids, l.ID)
	}

	// Load only id + merge_embedding for candidates
	type row struct {
		ID             uint
		MergeEmbedding *string
	}
	var rows []row
	if err := db.WithContext(ctx).Model(&models.SemanticLabel{}).
		Select("id, merge_embedding").
		Where("id IN ?", ids).
		Find(&rows).Error; err != nil {
		return nil, err
	}

	// Compute cosine similarity and find best match
	var best *models.SemanticLabel
	idSimMap := make(map[uint]float64, len(rows))
	for _, r := range rows {
		if r.MergeEmbedding == nil || *r.MergeEmbedding == "" {
			continue
		}
		existingVec, err := parsePgVector(*r.MergeEmbedding)
		if err != nil {
			continue
		}
		sim, err := airouter.CosineSimilarity(mergeVector, existingVec)
		if err != nil || sim < auxiliaryLabelMergeThreshold {
			continue
		}
		idSimMap[r.ID] = sim
	}

	// Pick best by RefCount DESC, ID ASC among threshold-passing matches
	for i := range labels {
		_, ok := idSimMap[labels[i].ID]
		if !ok {
			continue
		}
		candidate := labels[i]
		if best == nil || candidate.RefCount > best.RefCount || (candidate.RefCount == best.RefCount && candidate.ID < best.ID) {
			best = &candidate
		}
	}

	return best, nil
}

func (s *AuxiliaryLabelService) addAlias(ctx context.Context, label *models.SemanticLabel, alias string) (*models.SemanticLabel, error) {
	if !semanticAliasesContain(label.Aliases, alias) && !strings.EqualFold(label.Label, alias) {
		label.Aliases = append(label.Aliases, alias)
		if err := s.db.WithContext(ctx).Save(label).Error; err != nil {
			return nil, err
		}
	}
	return label, nil
}

func semanticAliasesContain(aliases []string, label string) bool {
	for _, alias := range aliases {
		if strings.EqualFold(strings.TrimSpace(alias), strings.TrimSpace(label)) || Slugify(alias) == Slugify(label) {
			return true
		}
	}
	return false
}

func defaultAuxiliaryLabelEmbedder(ctx context.Context, input string, mode auxiliaryLabelEmbeddingMode) (string, []float64, error) {
	opName := "auxiliary_label_storage_embedding"
	if mode == auxiliaryLabelEmbeddingModeMerge {
		opName = "auxiliary_label_merge_embedding"
	}
	router := airouter.NewRouter()
	result, err := router.Embed(ctx, airouter.EmbeddingRequest{
		Input: []string{input},
		Metadata: map[string]any{
			"operation": opName,
			"label":     input,
		},
	}, airouter.CapabilityEmbedding)
	if err != nil {
		return "", nil, err
	}
	if result == nil || len(result.Embeddings) == 0 {
		return "", nil, fmt.Errorf("empty embedding result")
	}
	vector := result.Embeddings[0]
	return floatsToPgVector(vector), vector, nil
}

func parsePgVector(value string) ([]float64, error) {
	value = strings.TrimSpace(strings.Trim(value, "[]"))
	if value == "" {
		return nil, fmt.Errorf("empty vector")
	}
	parts := strings.Split(value, ",")
	result := make([]float64, 0, len(parts))
	for _, part := range parts {
		f, err := strconv.ParseFloat(strings.TrimSpace(part), 64)
		if err != nil {
			return nil, err
		}
		result = append(result, f)
	}
	return result, nil
}

func uniqueSemanticLabelSlug(db *gorm.DB, base string) string {
	slug := base
	for i := 2; ; i++ {
		var count int64
		db.Model(&models.SemanticLabel{}).Where("slug = ?", slug).Count(&count)
		if count == 0 {
			return slug
		}
		slug = fmt.Sprintf("%s-%d", base, i)
	}
}

// EnsureSemanticLabelVectorDimension checks if the semantic_labels.embedding column
// matches the required dimension and alters it if not.
// Should only be called at startup; DDL operations use a 5s lock timeout to avoid
// blocking if other connections hold table locks.
func EnsureSemanticLabelVectorDimension(dim int) error {
	if err := database.DB.Exec("SET LOCAL lock_timeout = '5s'").Error; err != nil {
		logging.Warnf("Failed to set lock_timeout: %v", err)
	}

	var typeStr string
	if err := database.DB.Raw(`
		SELECT format_type(a.atttypid, a.atttypmod)
		FROM pg_attribute a
		JOIN pg_class c ON c.oid = a.attrelid
		WHERE c.relname = 'semantic_labels' AND a.attname = 'embedding'
	`).Row().Scan(&typeStr); err != nil {
		return nil // column may not exist yet, let migration handle it
	}

	expected := fmt.Sprintf("vector(%d)", dim)
	if typeStr == expected {
		return nil
	}

	logging.Infof("Altering semantic_labels.embedding column from %s to %s", typeStr, expected)

	_ = database.DB.Exec("DROP INDEX IF EXISTS idx_semantic_labels_embedding").Error

	if err := database.DB.Exec(fmt.Sprintf(
		"ALTER TABLE semantic_labels ALTER COLUMN embedding TYPE %s", expected,
	)).Error; err != nil {
		return fmt.Errorf("alter semantic_labels.embedding column to %s: %w", expected, err)
	}

	return nil
}

// EnsureSemanticLabelMergeVectorDimension checks if the semantic_labels.merge_embedding
// column matches the required dimension and alters it if not.
func EnsureSemanticLabelMergeVectorDimension(dim int) error {
	if err := database.DB.Exec("SET LOCAL lock_timeout = '5s'").Error; err != nil {
		logging.Warnf("Failed to set lock_timeout: %v", err)
	}

	var typeStr string
	if err := database.DB.Raw(`
		SELECT format_type(a.atttypid, a.atttypmod)
		FROM pg_attribute a
		JOIN pg_class c ON c.oid = a.attrelid
		WHERE c.relname = 'semantic_labels' AND a.attname = 'merge_embedding'
	`).Row().Scan(&typeStr); err != nil {
		return nil // column may not exist yet, let migration handle it
	}

	expected := fmt.Sprintf("vector(%d)", dim)
	if typeStr == expected {
		return nil
	}

	logging.Infof("Altering semantic_labels.merge_embedding column from %s to %s", typeStr, expected)

	_ = database.DB.Exec("DROP INDEX IF EXISTS idx_semantic_labels_merge_embedding").Error

	if err := database.DB.Exec(fmt.Sprintf(
		"ALTER TABLE semantic_labels ALTER COLUMN merge_embedding TYPE %s", expected,
	)).Error; err != nil {
		return fmt.Errorf("alter semantic_labels.merge_embedding column to %s: %w", expected, err)
	}

	return nil
}
