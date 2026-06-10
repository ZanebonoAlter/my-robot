package repository

import (
	tagging "syntopica-backend/internal/tagmanagement"
	"syntopica-backend/internal/platform/database"
)

func init() {
	database.RegisterModels(
		&BoardDailyReport{},
		&DailyReportSection{},
		&DailyReportThread{},
		&SectionRelation{},
	)
	tagging.RegisterVectorDimEnsurer(ensureSectionEmbeddingDimension)
}
