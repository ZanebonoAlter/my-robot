package repository

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"syntopica-backend/internal/models"
)

func TestNormalizeReportDateKeepsRequestedDate(t *testing.T) {
	requested, err := time.ParseInLocation("2006-01-02", "2026-05-26", models.ShanghaiTZ)
	require.NoError(t, err)

	got := NormalizeReportDate(requested)

	require.Equal(t, "2026-05-26", got.Format("2006-01-02"))
	require.Equal(t, time.UTC, got.Location())
	require.Equal(t, 12, got.Hour())
}
