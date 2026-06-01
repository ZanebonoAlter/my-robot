package daily_report

import (
	"fmt"

	"syntopica-backend/internal/domain/tagging"
	"syntopica-backend/internal/platform/database"
	"syntopica-backend/internal/platform/logging"
)

func init() {
	database.RegisterModels(
		&BoardDailyReport{},
		&DailyReportSection{},
		&DailyReportThread{},
	)
	tagging.RegisterVectorDimEnsurer(ensureSectionEmbeddingDimension)
}
