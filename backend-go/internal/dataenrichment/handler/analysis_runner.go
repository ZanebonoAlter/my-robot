package handler

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"
)

// ── 分析异步执行器（fix-board-analysis-material 8.x）────────────────────────
//
// 背景：循环 B 分析（含补全门备料）是 10 分钟级长任务。同步 HTTP 下客户端
// 断连（离开页面/关标签页/网络抖动）会把 request-context 的 cancel 传播进
// 整条分析链——2026-08-27 实锤：跑满 10 分钟的 analyze 环节 "context canceled"，
// 全部 LLM 调用作废、无报告落库。
//
// 模型：trigger 立即返回 202，分析在独立 context 的 goroutine 里跑完落库；
// 前端轮询 status 接口拿 running/finished/error/result_id。单实例单用户产品，
// 内存态足够（进程重启 = 分析本就死了，无状态可恢复）。

const (
	// analysisJobTimeout 是单次分析（含补全门 40 次 LLM 上限 + agent loop）的
	// 宽松上限；到点强杀防止 goroutine 泄漏。
	analysisJobTimeout = 30 * time.Minute

	AnalysisScopeBoard = "board"
	AnalysisScopeTopic = "topic"
)

// ErrAlreadyRunning marks a rejected duplicate trigger.
var ErrAlreadyRunning = errors.New("analysis already running")

// AnalysisStatus is the poller-facing snapshot of one analysis job.
type AnalysisStatus struct {
	Scope     string    `json:"scope"`
	TargetID  uint      `json:"target_id"`
	Running   bool      `json:"running"`
	StartedAt time.Time `json:"started_at"`
	Finished  bool      `json:"finished"`
	Error     string    `json:"error,omitempty"`
	ResultID  uint      `json:"result_id,omitempty"`
}

type analysisJob struct {
	scope     string
	targetID  uint
	startedAt time.Time
	done      bool
	err       string
	resultID  uint
}

func jobKey(scope string, id uint) string {
	return scope + ":" + strconv.FormatUint(uint64(id), 10)
}

// analysisRunner serializes analysis jobs per (scope, id): a target already
// running rejects re-trigger; the last finished job is kept for status polls.
type analysisRunner struct {
	mu   sync.Mutex
	jobs map[string]*analysisJob
}

func newAnalysisRunner() *analysisRunner {
	return &analysisRunner{jobs: map[string]*analysisJob{}}
}

// Start launches fn in a detached goroutine. Returns ErrAlreadyRunning if the
// same (scope, id) job is still running. fn receives a context that survives
// the triggering HTTP request (client disconnects no longer kill the run) and
// returns the persisted result id for pollers.
func (r *analysisRunner) Start(scope string, id uint, timeout time.Duration, fn func(ctx context.Context) (uint, error)) error {
	k := jobKey(scope, id)
	r.mu.Lock()
	if job, exists := r.jobs[k]; exists && !job.done {
		r.mu.Unlock()
		return ErrAlreadyRunning
	}
	job := &analysisJob{scope: scope, targetID: id, startedAt: time.Now()}
	r.jobs[k] = job
	r.mu.Unlock()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		resultID, err := safeRun(ctx, fn)
		r.mu.Lock()
		defer r.mu.Unlock()
		if cur := r.jobs[k]; cur == job && !cur.done {
			cur.done = true
			cur.err = errString(err)
			cur.resultID = resultID
		}
	}()
	return nil
}

// Status returns the current (or last finished) job snapshot for a target.
func (r *analysisRunner) Status(scope string, id uint) (AnalysisStatus, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	j, ok := r.jobs[jobKey(scope, id)]
	if !ok {
		return AnalysisStatus{}, false
	}
	return AnalysisStatus{
		Scope:     j.scope,
		TargetID:  j.targetID,
		Running:   !j.done,
		StartedAt: j.startedAt,
		Finished:  j.done,
		Error:     j.err,
		ResultID:  j.resultID,
	}, true
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// safeRun guards fn against panics so a crashing analysis surfaces as an error
// status instead of taking down the process.
func safeRun(ctx context.Context, fn func(ctx context.Context) (uint, error)) (resultID uint, err error) {
	defer func() {
		if rec := recover(); rec != nil {
			resultID, err = 0, fmt.Errorf("analysis panic: %v", rec)
		}
	}()
	return fn(ctx)
}
