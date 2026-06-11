package repository

import (
	"syntopica-backend/internal/platform/database"
	tagging "syntopica-backend/internal/tagmanagement"
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
