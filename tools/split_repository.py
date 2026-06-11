#!/usr/bin/env python3
"""Split repository.go: move daily report methods into daily_report_repository.go."""

import os

SRC = "/mnt/d/project/Syntopica/backend-go/internal/topicgraph/repository/repository.go"
DRR = "/mnt/d/project/Syntopica/backend-go/internal/topicgraph/repository/daily_report_repository.go"

with open(SRC, "r", encoding="utf-8") as f:
    repo_lines = f.readlines()

with open(DRR, "r", encoding="utf-8") as f:
    drr_lines = f.readlines()

# In repository.go, find the daily report section boundaries
# It starts with the comment header and ends before the Topic Graph Service section
dr_start = None
dr_end = None

for i, line in enumerate(repo_lines):
    if "Daily Report Repository" in line:
        dr_start = i
    if dr_start is not None and "Topic Graph" in line:
        dr_end = i
        break

if dr_start is None or dr_end is None:
    print("ERROR: Could not find daily report section boundaries")
    print(f"dr_start={dr_start}, dr_end={dr_end}")
    exit(1)

print(f"Daily report section: lines {dr_start+1} to {dr_end+1}")
print(f"Total: {dr_end - dr_start} lines")

# Extract the daily report method lines (including the section header)
dr_methods = repo_lines[dr_start:dr_end]

# Now modify daily_report_repository.go:
# 1. Keep the package declaration and import block
# 2. Keep all standalone types and helper functions (NormalizeReportDate, DeriveSectionStatuses, etc.)
# 3. Remove the standalone function implementations that duplicate TopicGraphRepository methods
#    These are: SaveReport, GetReportByID, ListReports, ListReportsForAllBoards, 
#               CollectBoardIDsForDate, SaveThreads, GetBoardSectionTimeline, GetSectionLifecycle,
#               BackfillSectionEmbeddings, BackfillRelations, BackfillAllRelations
# 4. Add the TopicGraphRepository methods instead

# First, let me find what to remove from daily_report_repository.go
# The standalone funcs start with `func ` not `func (r *TopicGraphRepository) ` 
# And they duplicate the TopicGraphRepository methods

standalone_sigs = [
    "func SaveReport",
    "func GetReportByID",
    "func ListReports(",
    "func ListReportsForAllBoards",
    "func CollectBoardIDsForDate",
    "func SaveThreads",
    "func GetBoardSectionTimeline",
    "func GetSectionLifecycle",
    "func BackfillSectionEmbeddings",
    "func BackfillRelations",
    "func BackfillAllRelations",
]

# Find line ranges for standalone functions in daily_report_repository.go
def find_func_end(lines, start_idx):
    """Find the end of a function body starting at start_idx, using brace counting."""
    brace_count = 0
    started = False
    for i in range(start_idx, len(lines)):
        line = lines[i]
        # Check if this line starts a function
        if not started:
            # Wait until we hit the opening brace
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

remove_ranges = []
for sig in standalone_sigs:
    for i, line in enumerate(drr_lines):
        if line.startswith(sig):
            end = find_func_end(drr_lines, i)
            remove_ranges.append((i, end))
            print(f"Removing standalone {sig.strip()}: lines {i+1}-{end}")
            break

# Also check for ReportListItem type at the right position - it should stay
# Types that are not duplicated:
keep_lines = []
removed_idxs = set()
for r in remove_ranges:
    for i in range(r[0], r[1]):
        removed_idxs.add(i)

# Build new daily_report_repository.go
new_drr = []
for i, line in enumerate(drr_lines):
    if i == 0:
        new_drr.append(line)  # package
        continue
    if i not in removed_idxs:
        new_drr.append(line)

# Add the TopicGraphRepository methods from repository.go
new_drr.extend(dr_methods)

# Write updated daily_report_repository.go
with open(DRR, "w", encoding="utf-8", newline="\n") as f:
    f.writelines(new_drr)
print(f"\nWritten {len(new_drr)} lines to daily_report_repository.go")

# Now remove the daily report section from repository.go
new_repo = []
for i, line in enumerate(repo_lines):
    if dr_start <= i < dr_end:
        continue
    new_repo.append(line)

# Write updated repository.go
with open(SRC, "w", encoding="utf-8", newline="\n") as f:
    f.writelines(new_repo)
print(f"Written {len(new_repo)} lines to repository.go (was {len(repo_lines)})")

print("\nDone!")
