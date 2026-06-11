package auxlabel

import (
	"gorm.io/gorm"

	"syntopica-backend/internal/tagmanagement/service/core"
)

func init() {
	core.AuxServiceFactory = func(db *gorm.DB, embedder interface{}) core.AuxService {
		var emb AuxiliaryLabelEmbedder
		if embedder != nil {
			emb = embedder.(AuxiliaryLabelEmbedder)
		}
		return NewAuxiliaryLabelService(db, emb)
	}
}
