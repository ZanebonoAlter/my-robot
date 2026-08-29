package service_test

import (
	"context"
	"strings"
	"testing"

	"syntopica-backend/internal/dataenrichment/service"
)

// ── 版块形态 prompt 分支（tasks 3.3）────────────────────────────────────────

func newBoardAnalysisOrch(t *testing.T) (*service.OrchestratorService, *mockAirRouter) {
	t.Helper()
	router := newMockAirRouter()
	repo := setupOrchTestDB(t)
	orch := service.NewOrchestratorService(
		router, repo, &orchMockLifelineReader{}, service.NewLifelineRenderer(),
		service.NewRegistry(&nilFetcher{}), &mockBoardConfigReader{cfg: service.DefaultBoardConfig()}, testCap,
	)
	return orch, router
}

// 3.3-a 版块 analyze prompt 装配：含层级递进机制层骨架 + 跨泳道织入纪律 +
// lane 引用格式 + 证据多样性纪律（D10）+ 参考角色附录 + 态势卡与论据块。
func TestBoardAnalyzePrompt_Assembly(t *testing.T) {
	orch, _ := newBoardAnalysisOrch(t)
	cardsMD := "## 泳道态势卡\n- 泳道#9《测试》"
	prompt := orch.AssembleBoardAnalyzePromptForTest(context.Background(),
		"美债不是无风险资产，而是政治议价的抵押品", "概念重命名", cardsMD,
		[]map[string]any{{"topic": "历史机制", "data": "某机构 2024 报告摘录"}})

	for _, want := range []string{
		"层级递进机制层",                         // 论证骨架
		"并列罗列是失败",                         // 跨泳道织入纪律
		`source_type="lane", ref=lane_id`, // lane 引用格式
		"证据多样性纪律",                         // D10
		"≥3 类",                            // 多样性阈值
		"优先一手源",                           // 检索引导
		"板块泳道态势卡:",                        // 输入块
		"【历史机制】",                          // 论据块
		"某机构 2024 报告摘录",                   // 论据内容透传
		"美债不是无风险资产，而是政治议价的抵押品", // thesis 注入（多处）
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("board analyze prompt missing %q", want)
		}
	}
	if strings.Count(prompt, "美债不是无风险资产，而是政治议价的抵押品") < 2 {
		t.Fatalf("thesis should appear in both header and JSON contract")
	}
}

// 3.3-b 版块 agent loop system prompt：内部工具优先 + max_loops 防御复用。
func TestBoardAgentLoop_SystemPromptAndDefenses(t *testing.T) {
	orch, router := newBoardAnalysisOrch(t)
	cardsMD := "## 泳道态势卡\n- 泳道#9《测试》"

	// Exhaust the loop: never finish → maxLoops=6 must cap it.
	for i := 0; i < service.MaxAgentLoopsForTest(); i++ {
		router.addResponse(`{"action":"call_tool","thought":"t","tool":"web_search","args":{"query":"q` + string(rune('a'+i)) + `"}}`)
	}

	res, err := orch.RunBoardAgentLoopForTest(context.Background(), "board-sess", "历史机制", "测试命题", cardsMD, []string{"web_search"})
	if err != nil {
		t.Fatalf("board agent loop: %v", err)
	}
	if res.Loops != service.MaxAgentLoopsForTest() {
		t.Fatalf("max_loops defense must be reused: loops=%d want %d", res.Loops, service.MaxAgentLoopsForTest())
	}
	if res.Error == "" {
		t.Fatal("exhausted loop must carry an error marker")
	}
	// System prompt carries the board branch (internal tools first).
	sys := router.Calls[0].Messages[0].Content
	for _, want := range []string{"版块级深度分析", "优先用内部工具", "单条泳道下钻最多 2 次", "测试命题", "态势卡"} {
		if !strings.Contains(sys, want) {
			t.Fatalf("board system prompt missing %q", want)
		}
	}
}

// 3.3-c SessionID 形态：data_enrichment_board_{board_id}_{uuid8}。
func TestBoardSessionID_Format(t *testing.T) {
	id := service.GenerateBoardSessionIDForTest(77)
	if !strings.HasPrefix(id, "data_enrichment_board_77_") {
		t.Fatalf("session id format wrong: %s", id)
	}
	suffix := strings.TrimPrefix(id, "data_enrichment_board_77_")
	if len(suffix) != 8 {
		t.Fatalf("uuid8 suffix should be 8 hex chars, got %q", suffix)
	}
	// Uniqueness across calls.
	if service.GenerateBoardSessionIDForTest(77) == id {
		t.Fatal("session ids must be unique")
	}
}

// 3.7-b 单泳道 analyze prompt 同样携带证据多样性纪律（D10 双分支）。
func TestSingleLaneAnalyzePrompt_DiversityDiscipline(t *testing.T) {
	orch, _ := newBoardAnalysisOrch(t)
	prompt := orch.AssembleSingleLaneAnalyzePromptForTest(context.Background(), "structural", "测试视角", "脉络", "", nil)
	for _, want := range []string{"证据多样性纪律", "≥3 类", "优先一手源", "诚实标注"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("single-lane analyze prompt missing %q", want)
		}
	}
}
