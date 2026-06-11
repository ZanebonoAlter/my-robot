#!/usr/bin/env python3
"""Split semantic_board_handler.go into three files by content-based matching."""

SRC = "/mnt/d/project/Syntopica/backend-go/internal/tagmanagement/handler/semantic_board_handler.go"
DST_DIR = "/mnt/d/project/Syntopica/backend-go/internal/tagmanagement/handler"

with open(SRC, "r", encoding="utf-8") as f:
    lines = f.readlines()

# Function signatures that go to each file
MATCH_SIGS = [
    "type boardArticleTagDTO struct",
    "type matchDetailConfigDTO struct",
    "type DirectHitAuxiliaryDTO struct",
    "type MatchDetailPairDTO struct",
    "type matchDetailResponse struct",
    "func (h *semanticBoardHandler) getBoardArticles",
    "func (h *semanticBoardHandler) getTagMatchDetail",
    "func BuildDirectHitAuxiliaryDTOs",
    "func MatchTier",
    "func MatchDetailPairsToDTOs",
    "func matchDetailConfigToDTO",
    "func (h *semanticBoardHandler) rematchAll",
    "func (h *semanticBoardHandler) getMatchingConfig",
    "func (h *semanticBoardHandler) updateMatchingConfig",
    "func semanticBoardMatchConfigToMap",
]

UPGRADE_SIGS = [
    "type confirmSemanticBoardUpgradeHTTPRequest struct",
    "type semanticBoardUpgradeSuggestionDTO struct",
    "type semanticBoardUpgradeCandidateDTO struct",
    "type boardAffinityDTO struct",
    "type semanticBoardUpgradeClusterDTO struct",
    "type airouterSemanticBoardUpgradeLLM struct",
    "func (h *semanticBoardHandler) getUpgradeCandidates",
    "func (h *semanticBoardHandler) suggestUpgrades",
    "func (h *semanticBoardHandler) executeUpgrade",
    "func (h *semanticBoardHandler) enqueueBackfill",
    "func (h *semanticBoardHandler) getBackfillJob",
    "func (h *semanticBoardHandler) backfillBoardEmbeddings",
    "func (airouterSemanticBoardUpgradeLLM) SuggestSemanticBoardUpgrades",
    "func newSemanticBoardUpgradeLLM",
    "func upgradeCandidatesToDTO",
    "func upgradeClustersToDTO",
    "func (h *semanticBoardHandler) suggestionsToDTO",
    "func semanticBoardUpgradeConfigToMap",
]

# First pass: identify line ranges for each group
def find_func_ranges(lines, sigs_to_match):
    """Find all line ranges (start, end) for functions/types matching the given signatures."""
    ranges = []
    in_func = False
    brace_count = 0
    start_line = None
    
    for i, line in enumerate(lines):
        stripped = line.rstrip()
        
        # Check if this line starts a function/type/var that we care about
        for sig in sigs_to_match:
            if sig in stripped:
                in_func = True
                brace_count = 0
                start_line = i
                # Find first brace
                break
        
        if in_func:
            # Count braces to find the end
            for ch in line:
                if ch == '{':
                    brace_count += 1
                elif ch == '}':
                    brace_count -= 1
            
            if brace_count == 0 and start_line is not None and start_line != i:
                # Function ended, check if this is reasonable (not empty func)
                ranges.append((start_line, i + 1))
                in_func = False
                start_line = None
    
    # Handle the last function if still open (shouldn't happen with valid Go)
    if in_func and start_line is not None:
        ranges.append((start_line, len(lines)))
    
    return ranges

# Find ranges for each group
match_ranges = find_func_ranges(lines, MATCH_SIGS)
upgrade_ranges = find_func_ranges(lines, UPGRADE_SIGS)

print("Match ranges:", match_ranges)
print("Upgrade ranges:", upgrade_ranges)

# Also need to include the upgrade types that precede RegisterSemanticBoardRoutes
# These are at the beginning of the file but BEFORE the function-based detection
# Types like confirmSemanticBoardUpgradeHTTPRequest are at lines 96-140 area
# Our range finder should have caught those

# Check coverage
all_func_lines = set()
for r in match_ranges:
    for i in range(r[0], r[1]):
        all_func_lines.add(i)
for r in upgrade_ranges:
    for i in range(r[0], r[1]):
        all_func_lines.add(i)

def write_file(filename, content_lines):
    path = f"{DST_DIR}/{filename}"
    with open(path, "w", encoding="utf-8", newline="\n") as f:
        f.writelines(content_lines)
    print(f"Written {len(content_lines)} lines to {path}")

# Get package + imports (lines 0-23)
import_block = lines[:23]

def extract_funcs(ranges):
    """Extract lines for given ranges, sorted by line number."""
    result = list(import_block)
    for r in sorted(ranges, key=lambda x: x[0]):
        for i in range(r[0], r[1]):
            result.append(lines[i])
    # Add trailing newline
    if result and not result[-1].endswith('\n'):
        result[-1] += '\n'
    return result

# Write match file
match_content = extract_funcs(match_ranges)
write_file("board_match_handler.go", match_content)

# Write upgrade file
upgrade_content = extract_funcs(upgrade_ranges)
write_file("board_upgrade_handler.go", upgrade_content)

# Write CRUD file (everything except match and upgrade)
crud_lines = [import_block[0]]  # package line only, imports will be re-included
# Actually let's just iterate and exclude
crud_content = list(import_block)
removed = set()
for r in match_ranges + upgrade_ranges:
    for i in range(r[0], r[1]):
        removed.add(i)

# Also add any empty lines between functions for readability
for i in range(len(import_block), len(lines)):
    if i not in removed:
        crud_content.append(lines[i])

write_file("board_crud_handler.go", crud_content)

# Rename original
import os
os.rename(SRC, SRC.replace("semantic_board_handler.go", "semantic_board_handler.go.bak"))
print(f"Original renamed to .bak")

print(f"\nSummary:")
print(f"  board_crud_handler.go:   {len(crud_content)} lines (+ imports)")
print(f"  board_match_handler.go:  {len(match_content)} lines")
print(f"  board_upgrade_handler.go: {len(upgrade_content)} lines")
print(f"  Total match ranges: {len(match_ranges)}")
print(f"  Total upgrade ranges: {len(upgrade_ranges)}")
