package dataenrichment

import (
	"gorm.io/gorm"

	"syntopica-backend/internal/dataenrichment/handler"
	"syntopica-backend/internal/dataenrichment/repository"
	"syntopica-backend/internal/dataenrichment/service"
	"syntopica-backend/internal/platform/airouter"
)

// InitRepository initializes the dataenrichment repository singleton.
func InitRepository(db *gorm.DB) {
	repository.InitRepo(db)
}

// GetRepo returns the dataenrichment repository singleton.
func GetRepo() *repository.Repository {
	return repository.Repo
}

// Package-level service singletons built by Init. The runtime schedulers
// (registered in app/runtime.go) consume them via the getters below.
var (
	lifelineSvc *service.LifelineContextService
	topicLister ActiveTopicLister
)

// Init wires the full data-enrichment domain: repository singleton, cycle-A
// lifeline context service, cycle-B orchestrator, and the HTTP handler
// singleton.
//
// MUST be called before dataenrichment.RegisterRoutes, which means it runs in
// main.go BEFORE app.SetupRoutes — mirroring how the other domains call
// InitRepository in main.go prior to route registration. (app.StartRuntime only
// registers schedulers and runs AFTER SetupRoutes, so it is too late for the
// handler singleton that RegisterRoutes dereferences.)
func Init(db *gorm.DB) {
	InitRepository(db)
	repo := GetRepo()

	// Cycle A: lifeline context summary service (news-only, scheduled).
	lifelineSvc = service.NewLifelineContextService(
		airouter.NewRouter(),
		repo,
		NewTopicGraphSectionReader(db),
		CapabilityNews,
	)
	topicLister = NewDBTopicLister(db)

	// Cycle B: three-role orchestration (interpret → agent loop → analyze + review).
	boardConfigReader := NewDBBoardConfigReader(db)
	lifelineReader := NewDBLifelineReader(db)
	renderer := service.NewLifelineRenderer()
	toolRegistry := service.NewRegistry(service.NewDefaultHTTPFetcher())
	orchestrator := service.NewOrchestratorService(
		airouter.NewRouter(),
		repo,
		lifelineReader,
		renderer,
		toolRegistry,
		boardConfigReader,
	)

	// HTTP handler singleton consumed by handler.RegisterRoutes.
	handler.InitHandler(repo, lifelineSvc, orchestrator, boardConfigReader, db)
}

// GetLifelineService returns the cycle-A service built by Init, for scheduler
// registration in app/runtime.go.
func GetLifelineService() *service.LifelineContextService {
	return lifelineSvc
}

// GetTopicLister returns the active-topic lister built by Init, for scheduler
// registration in app/runtime.go.
func GetTopicLister() ActiveTopicLister {
	return topicLister
}
