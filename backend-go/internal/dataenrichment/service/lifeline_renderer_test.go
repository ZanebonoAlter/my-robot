package service_test

import (
	"strings"
	"testing"
	"time"

	"syntopica-backend/internal/dataenrichment/service"
)

// mockLifelineReader implements service.LifelineReader.
type mockLifelineReader struct {
	data service.SectionTimelineData
}

func (m *mockLifelineReader) GetTopicLifeline(topicID uint) (service.SectionTimelineData, error) {
	return m.data, nil
}

func TestRenderLifelineForAgent_ContainsTopicBody(t *testing.T) {
	renderer := service.NewLifelineRenderer()
	reader := &mockLifelineReader{
		data: service.SectionTimelineData{
			Topic: service.TopicBrief{
				ID:              1,
				Label:           "中东地缘紧张与能源连锁反应",
				Description:     "产油国设施遭袭 → 油价飙升 → 下游成本承压",
				Status:          "active",
				Source:          "auto",
				FirstSeenDate:   time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
				LastSeenDate:    time.Date(2026, 7, 4, 0, 0, 0, 0, time.UTC),
				HitCount:        4,
				ConsecutiveHits: 4,
			},
			Sections: []service.TimelineSectionNode{
				{
					SectionID: 101, PeriodDate: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
					ClusterLabel: "产油国石油设施遭袭", Status: "emerging",
					TopicMatchConfidence: "auto_new", ArticleCount: 8, ThreadCount: 2,
					ThreadTitles: []string{"布伦特原油涨幅超6%", "市场恐慌蔓延"},
				},
				{
					SectionID: 102, PeriodDate: time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC),
					ClusterLabel: "油价飙升传导", Status: "continuing",
					TopicMatchConfidence: "anchor_hit", ArticleCount: 12, ThreadCount: 3,
					ThreadTitles: []string{"油气开采股价走高", "化工行业承压"},
				},
			},
		},
	}

	output, err := renderer.RenderLifelineForAgent(reader, 1, 14)
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	// Topic body checks
	if !strings.Contains(output, "中东地缘紧张与能源连锁反应") {
		t.Fatal("output should contain topic label")
	}
	if !strings.Contains(output, "产油国设施遭袭") {
		t.Fatal("output should contain topic description")
	}
}

func TestRenderLifelineForAgent_ContainsDayByDaySections(t *testing.T) {
	renderer := service.NewLifelineRenderer()
	reader := &mockLifelineReader{
		data: service.SectionTimelineData{
			Topic: service.TopicBrief{
				ID: 1, Label: "test", Status: "active", Source: "auto",
				FirstSeenDate: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
				LastSeenDate:  time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC),
			},
			Sections: []service.TimelineSectionNode{
				{SectionID: 1, PeriodDate: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC), ClusterLabel: "day1", Status: "emerging", TopicMatchConfidence: "auto_new", ArticleCount: 3, ThreadCount: 1},
				{SectionID: 2, PeriodDate: time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC), ClusterLabel: "day2", Status: "continuing", TopicMatchConfidence: "anchor_hit", ArticleCount: 5, ThreadCount: 2, ThreadTitles: []string{"t1"}},
			},
		},
	}

	output, err := renderer.RenderLifelineForAgent(reader, 1, 14)
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	if !strings.Contains(output, "07-01") || !strings.Contains(output, "07-02") {
		t.Fatal("output should contain day-by-day dates")
	}
}

func TestRenderLifelineForAgent_StatusChineseMapping(t *testing.T) {
	renderer := service.NewLifelineRenderer()
	tests := []struct {
		status   string
		wantText string
	}{
		{"emerging", "涌现"},
		{"continuing", "延续"},
		{"split", "分叉"},
		{"merge", "合并"},
		{"ending", "结束"},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			reader := &mockLifelineReader{
				data: service.SectionTimelineData{
					Topic: service.TopicBrief{
						ID: 1, Label: "test", Status: "active", Source: "auto",
						FirstSeenDate: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
						LastSeenDate:  time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
					},
					Sections: []service.TimelineSectionNode{
						{SectionID: 1, PeriodDate: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC), ClusterLabel: "test", Status: tt.status, TopicMatchConfidence: "auto_new", ArticleCount: 1, ThreadCount: 1},
					},
				},
			}
			output, _ := renderer.RenderLifelineForAgent(reader, 1, 14)
			if !strings.Contains(output, tt.wantText) {
				t.Fatalf("output for status %q should contain %q, got:\n%s", tt.status, tt.wantText, output)
			}
		})
	}
}

func TestRenderLifelineForAgent_EmptySections(t *testing.T) {
	renderer := service.NewLifelineRenderer()
	reader := &mockLifelineReader{
		data: service.SectionTimelineData{
			Topic: service.TopicBrief{
				ID: 1, Label: "test", Status: "active", Source: "auto",
			},
			Sections: []service.TimelineSectionNode{},
		},
	}

	output, err := renderer.RenderLifelineForAgent(reader, 1, 14)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(output, "话题本体") {
		t.Fatal("even empty sections should render topic body")
	}
}
