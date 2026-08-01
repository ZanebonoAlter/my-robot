package dataenrichment

import "syntopica-backend/internal/platform/airouter"

// CapabilityNews routes cycle A LLM calls (summarize_context — pure news aggregation).
// See design.md §11, decision ②.
const CapabilityNews airouter.Capability = "data_enrichment_news"

// CapabilityAnalysis routes cycle B LLM calls (interpret / tool_use / analyze / review_judge).
// See design.md §11, decision ②.
const CapabilityAnalysis airouter.Capability = "data_enrichment_analysis"
