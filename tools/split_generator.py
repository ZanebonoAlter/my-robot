#!/usr/bin/env python3
"""Split daily_report_generator.go into three files."""
import re

SRC = "/mnt/d/project/Syntopica/backend-go/internal/topicgraph/service/daily_report_generator.go"
DST_DIR = "/mnt/d/project/Syntopica/backend-go/internal/topicgraph/service"

with open(SRC, "r", encoding="utf-8") as f:
    lines = f.readlines()

# Line ranges (0-indexed) for each group:
# LLM: promptVersion(20), highlightsSystemPrompt(26), GenerateHighlights(39), 
#      buildHighlightsPrompt(89), parseHighlightsResponse(105), threadsSystemPrompt(141),
#      GenerateClusterThreads(156), buildThreadsPrompt(208), parseThreadsResponse(222),
#      llmMergePair type(556), llmArbitrateMerges(562)
# Merge: populateThreadArticles(488), parsePgVector(509), cosineDistance(529),
#        MergeSimilarSections(646)
# Orchestrator: GenerateDailyReport(259), collectBoardTags(843), 
#               findPreviousReportBrief(973), filterTagsByQuality(984), filterTagsByIDs(1018)

# Better approach: use function signature matching with brace counting
def find_func_end(lines, start_idx):
    """Find the end of a function body starting at start_idx, using brace counting."""
    brace_count = 0
    started = False
    for i in range(start_idx, len(lines)):
        line = lines[i]
        if not started:
            if '{' in line:
                started = True
                for ch in line:
                    if ch == '{': brace_count += 1
                    elif ch == '}': brace_count -= 1
        else:
            for ch in line:
                if ch == '{': brace_count += 1
                elif ch == '}': brace_count -= 1
        if started and brace_count == 0:
            return i + 1
    return len(lines)

# Match signatures to detect function starts
LLM_SIGS = [
    "func GenerateHighlights(",
    "func buildHighlightsPrompt(",
    "func parseHighlightsResponse(",
    "func GenerateClusterThreads(",
    "func buildThreadsPrompt(",
    "func parseThreadsResponse(",
    "func llmArbitrateMerges(",
]

MERGE_SIGS = [
    "func MergeSimilarSections(",
    "func populateThreadArticles(",
    "func parsePgVector(",
    "func cosineDistance(",
]

ORCH_SIGS = [
    "func GenerateDailyReport(",
    "func collectBoardTags(",
    "func findPreviousReportBrief(",
    "func filterTagsByQuality(",
    "func filterTagsByIDs(",
]

# Also need constants and types that appear standalone
LLM_CONST_AND_TYPES = {
    "promptVersion": None,  # will find by searching
    "highlightsSystemPrompt": None,
    "threadsSystemPrompt": None,
    "llmMergePair": None,
}

def find_consts_and_types(lines, names):
    """Find line ranges for constants and types."""
    results = []
    for name in names:
        for i, line in enumerate(lines):
            if (line.startswith(f"const {name}") or 
                line.startswith(f"type {name}") or
                line.startswith(f"const {name} ")):
                end = find_func_end(lines, i)
                results.append((i, end))
                print(f"  Found const/type {name}: lines {i+1}-{end}")
                break
    return results

# Find all function ranges
def find_ranges(lines, sigs):
    ranges = []
    for sig in sigs:
        for i, line in enumerate(lines):
            if line.startswith(sig):
                end = find_func_end(lines, i)
                ranges.append((i, end))
                print(f"  Found {sig.strip()}: lines {i+1}-{end}")
                break
    return ranges

print("LLM functions:")
llm_ranges = find_ranges(lines, LLM_SIGS)
# Add constants
llm_ranges.append((20, 26))  # promptVersion (single line)
llm_ranges.append((26, 39))  # highlightsSystemPrompt (multiline const)
llm_ranges.append((141, 156)) # threadsSystemPrompt (multiline const)
llm_ranges.append((556, 562)) # llmMergePair type

print(f"\nMerge functions:")  
merge_ranges = find_ranges(lines, MERGE_SIGS)

print(f"\nOrchestrator functions:")
orch_ranges = find_ranges(lines, ORCH_SIGS)

# Remove overlaps - handle nested ranges (const inside func range etc.)
def merge_overlapping(ranges):
    """Merge overlapping ranges."""
    if not ranges:
        return []
    sorted_r = sorted(ranges, key=lambda x: x[0])
    merged = [sorted_r[0]]
    for r in sorted_r[1:]:
        if r[0] <= merged[-1][1]:
            merged[-1] = (merged[-1][0], max(merged[-1][1], r[1]))
        else:
            merged.append(r)
    return merged

merged_ranges_llm = merge_overlapping(llm_ranges)
merged_ranges_merge = merge_overlapping(merge_ranges)
merged_ranges_orch = merge_overlapping(orch_ranges)

print(f"\nMerged ranges:")
print(f"  LLM: {merged_ranges_llm}")
print(f"  Merge: {merged_ranges_merge}")
print(f"  Orchestrator: {merged_ranges_orch}")

# Build set of removed lines for checking overlaps
all_removed = set()
for r in llm_ranges + merge_ranges + orch_ranges:
    for i in range(r[0], r[1]):
        all_removed.add(i)

# Check for gaps - lines that don't belong to any group
gaps = []
last_end = 0
for r in sorted(list(all_removed)):
    # This is messy. Let me just check the import block
    pass

import_block = lines[:20]  # package + imports

def write_file(filename, ranges, include_imports=True):
    """Write a file with lines from the given ranges."""
    sorted_r = sorted(ranges, key=lambda x: x[0])
    content = list(import_block) if include_imports else []
    for r in sorted_r:
        for i in range(r[0], r[1]):
            content.append(lines[i])
    # Add trailing newline
    if content and not content[-1].endswith('\n'):
        content[-1] += '\n'
    
    path = f"{DST_DIR}/{filename}"
    with open(path, "w", encoding="utf-8", newline="\n") as f:
        f.writelines(content)
    print(f"Written {len(content)} lines to {path}")

write_file("daily_report_llm.go", merged_ranges_llm)
write_file("daily_report_merge.go", merged_ranges_merge)
write_file("daily_report_orchestrator.go", merged_ranges_orch)

# Remove the original
import os
os.remove(SRC)
print(f"Deleted {SRC}")

print(f"\nSummary:")
print(f"  daily_report_llm.go:          {sum(r[1]-r[0] for r in merged_ranges_llm)} lines (+ imports)")
print(f"  daily_report_merge.go:        {sum(r[1]-r[0] for r in merged_ranges_merge)} lines (+ imports)")
print(f"  daily_report_orchestrator.go: {sum(r[1]-r[0] for r in merged_ranges_orch)} lines (+ imports)")
