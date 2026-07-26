package tracing

import (
	"context"
	"errors"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"gorm.io/gorm"
)

// GORMTracePlugin 是 GORM 插件，通过 callback 给每个数据库操作创建
// SpanKind=Client span（携带 db.system / db.operation / db.statement），
// 挂在当前 trace 父节点下。
//
// 自写而非使用 gorm.io/plugin/opentelemetry：后者最新版 v0.1.16 强制要求
// gorm v1.30，会把项目的 gorm 从 v1.25.12 大版本升级。本 change 范围是补
// trace 能力，不应顺带升级核心数据层依赖，故自写轻量插件（gorm 保持 v1.25）。
type GORMTracePlugin struct {
	tracer trace.Tracer
}

// NewGORMPlugin 返回挂载到 db.Use 的 GORM trace 插件。
func NewGORMPlugin() *GORMTracePlugin {
	return &GORMTracePlugin{tracer: otel.Tracer(ServiceName)}
}

// Name 实现 gorm.Plugin。
func (p *GORMTracePlugin) Name() string { return "otel:gorm-tracing" }

// Initialize 为 create/query/update/delete/row/raw 注册 before/after callback。
// GORM 的 Before/After 在各操作 processor（Create/Query/...）上，非 Callbacks 顶层。
func (p *GORMTracePlugin) Initialize(db *gorm.DB) error {
	cb := db.Callback()
	// create
	if err := cb.Create().Before("gorm:create").Register("otel:before:create", p.before("create")); err != nil {
		return err
	}
	if err := cb.Create().After("gorm:create").Register("otel:after:create", p.after("create")); err != nil {
		return err
	}
	// query
	if err := cb.Query().Before("gorm:query").Register("otel:before:query", p.before("query")); err != nil {
		return err
	}
	if err := cb.Query().After("gorm:query").Register("otel:after:query", p.after("query")); err != nil {
		return err
	}
	// update
	if err := cb.Update().Before("gorm:update").Register("otel:before:update", p.before("update")); err != nil {
		return err
	}
	if err := cb.Update().After("gorm:update").Register("otel:after:update", p.after("update")); err != nil {
		return err
	}
	// delete
	if err := cb.Delete().Before("gorm:delete").Register("otel:before:delete", p.before("delete")); err != nil {
		return err
	}
	if err := cb.Delete().After("gorm:delete").Register("otel:after:delete", p.after("delete")); err != nil {
		return err
	}
	// row
	if err := cb.Row().Before("gorm:row").Register("otel:before:row", p.before("row")); err != nil {
		return err
	}
	if err := cb.Row().After("gorm:row").Register("otel:after:row", p.after("row")); err != nil {
		return err
	}
	// raw
	if err := cb.Raw().Before("gorm:raw").Register("otel:before:raw", p.before("raw")); err != nil {
		return err
	}
	if err := cb.Raw().After("gorm:raw").Register("otel:after:raw", p.after("raw")); err != nil {
		return err
	}
	return nil
}

func (p *GORMTracePlugin) before(op string) func(*gorm.DB) {
	return func(db *gorm.DB) {
		// 跳过 otel_spans 表自身：DatabaseSpanExporter 的写入若被 trace，
		// 会触发 span → export → 写入 → span 自循环，导致表指数膨胀。
		if db.Statement != nil && db.Statement.Table == (OtelSpan{}).TableName() {
			return
		}
		ctx := db.Statement.Context
		if ctx == nil {
			ctx = context.Background()
		}
		// Start 创建的 span 经 newCtx 传播；after 用 SpanFromContext 取回结束。
		newCtx, _ := p.tracer.Start(ctx, "gorm."+op, trace.WithSpanKind(trace.SpanKindClient))
		db.Statement.Context = newCtx
	}
}

func (p *GORMTracePlugin) after(op string) func(*gorm.DB) {
	return func(db *gorm.DB) {
		if db.Statement != nil && db.Statement.Table == (OtelSpan{}).TableName() {
			return
		}
		span := trace.SpanFromContext(db.Statement.Context)
		if !span.IsRecording() {
			return
		}
		attrs := []attribute.KeyValue{
			attribute.String("db.system", "postgresql"),
			attribute.String("db.operation", op),
		}
		if db.Statement != nil {
			if db.Statement.Table != "" {
				attrs = append(attrs, attribute.String("db.sql.table", db.Statement.Table))
			}
			if db.Statement.SQL.String() != "" {
				attrs = append(attrs, attribute.String("db.statement", db.Statement.SQL.String()))
			}
		}
		span.SetAttributes(attrs...)
		if db.Error != nil && !errors.Is(db.Error, gorm.ErrRecordNotFound) {
			span.RecordError(db.Error)
			span.SetStatus(codes.Error, db.Error.Error())
		}
		span.End()
	}
}
