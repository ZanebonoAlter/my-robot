# backend-go/Makefile
#
# 方法级 OTel span 自动织入（AST 织入器）。
# 详见 docs/reference/architecture/method-instrumenter.md

.PHONY: instrument instrument-check

## instrument: 用 AST 织入器给 service 层方法注入 OTel span（幂等，可重复跑）
instrument:
	go run ./cmd/instrumenter

## instrument-check: CI 用——织入后再跑必须无 diff（幂等性校验，有未织入代码则失败）
instrument-check:
	@go run ./cmd/instrumenter >/dev/null
	@git diff --quiet -- internal/ || { \
		echo "FAIL: 织入器产生了未提交的 diff，请本地跑 'make instrument' 后提交"; \
		git diff --stat -- internal/; exit 1; \
	}
	@echo "OK: service 层 span 织入已是最新（幂等）"
