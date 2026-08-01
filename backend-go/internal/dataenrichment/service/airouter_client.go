package service

import (
	"context"

	"syntopica-backend/internal/platform/airouter"
)

// AirRouter abstracts airouter.Router.Chat for testability.
type AirRouter interface {
	Chat(ctx context.Context, req airouter.ChatRequest) (*airouter.ChatResult, error)
}
