package handler

import (
	"context"
	"errors"
	"testing"
	"time"
)

// M8.1 分析 ctx 脱离触发请求：父 ctx 取消后后台分析照常完成。
// （2026-08-27 生产实锤：同步 HTTP 下客户端离开页面 → "context canceled"。）
func TestAnalysisRunner_SurvivesParentCancel(t *testing.T) {
	r := newAnalysisRunner()
	parent, cancelParent := context.WithCancel(context.Background())
	_ = parent // the runner deliberately ignores any parent — only its cancel matters

	err := r.Start(AnalysisScopeBoard, 1, time.Minute, func(ctx context.Context) (uint, error) {
		// Simulate the client leaving: the triggering request's ctx dies
		// mid-analysis. The runner-supplied ctx must stay alive.
		cancelParent()
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-time.After(50 * time.Millisecond):
			return 42, nil // survived — parent cancel did not propagate
		}
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for {
		st, ok := r.Status(AnalysisScopeBoard, 1)
		if ok && st.Finished {
			if st.Error != "" || st.ResultID != 42 {
				t.Fatalf("job result: %+v", st)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("job did not finish")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// M8.2 防重入：同目标在跑 → ErrAlreadyRunning；完成后可再次触发。
func TestAnalysisRunner_RejectsDuplicateWhileRunning(t *testing.T) {
	r := newAnalysisRunner()
	block := make(chan struct{})
	done := make(chan struct{})

	if err := r.Start(AnalysisScopeTopic, 7, time.Minute, func(ctx context.Context) (uint, error) {
		<-block
		close(done)
		return 9, nil
	}); err != nil {
		t.Fatalf("first start: %v", err)
	}
	if err := r.Start(AnalysisScopeTopic, 7, time.Minute, func(ctx context.Context) (uint, error) {
		return 0, errors.New("must not run")
	}); !errors.Is(err, ErrAlreadyRunning) {
		t.Fatalf("duplicate start: want ErrAlreadyRunning, got %v", err)
	}
	// Different target is unaffected.
	if err := r.Start(AnalysisScopeTopic, 8, time.Minute, func(ctx context.Context) (uint, error) {
		return 1, nil
	}); err != nil {
		t.Fatalf("other target start: %v", err)
	}

	close(block)
	<-done
	// After finish, re-trigger is allowed.
	if err := r.Start(AnalysisScopeTopic, 7, time.Minute, func(ctx context.Context) (uint, error) {
		return 10, nil
	}); err != nil {
		t.Fatalf("re-start after finish: %v", err)
	}
}

// M8.3 panic 恢复：崩溃的分析变成 error 状态，不拖垮进程。
func TestAnalysisRunner_PanicBecomesError(t *testing.T) {
	r := newAnalysisRunner()
	if err := r.Start(AnalysisScopeBoard, 3, time.Minute, func(ctx context.Context) (uint, error) {
		panic("boom")
	}); err != nil {
		t.Fatalf("start: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for {
		st, ok := r.Status(AnalysisScopeBoard, 3)
		if ok && st.Finished {
			if st.Error == "" || !contains(st.Error, "boom") {
				t.Fatalf("panic must surface as error, got %+v", st)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("job did not finish")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// M8.4 超时：fn 卡死时 ctx 到期，分析以 context deadline 错误收场。
func TestAnalysisRunner_TimeoutKillsRun(t *testing.T) {
	r := newAnalysisRunner()
	if err := r.Start(AnalysisScopeBoard, 4, 30*time.Millisecond, func(ctx context.Context) (uint, error) {
		<-ctx.Done()
		return 0, ctx.Err()
	}); err != nil {
		t.Fatalf("start: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for {
		st, ok := r.Status(AnalysisScopeBoard, 4)
		if ok && st.Finished {
			if st.Error == "" || st.ResultID != 0 {
				t.Fatalf("timed-out job: %+v", st)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("job did not finish")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
