---
name: openspec-archive-change
description: Archive a completed change in the experimental workflow. Use when the user wants to finalize and archive a change after implementation is complete.
license: MIT
compatibility: Requires openspec CLI.
metadata:
  author: openspec
  version: "1.0"
  generatedBy: "1.3.1"
---

Archive a completed change in the experimental workflow.

**Input**: Optionally specify a change name. If omitted, check if it can be inferred from conversation context. If vague or ambiguous you MUST prompt for available changes.

**Steps**

1. **If no change name provided, prompt for selection**

   Run `openspec list --json` to get available changes. Use the **AskUserQuestion tool** to let the user select.

   Show only active changes (not already archived).
   Include the schema used for each change if available.

   **IMPORTANT**: Do NOT guess or auto-select a change. Always let the user choose.

2. **Check artifact completion status**

   Run `openspec status --change "<name>" --json` to check artifact completion.

   Parse the JSON to understand:
   - `schemaName`: The workflow being used
   - `artifacts`: List of artifacts with their status (`done` or other)

   **If any artifacts are not `done`:**
   - Display warning listing incomplete artifacts
   - Use **AskUserQuestion tool** to confirm user wants to proceed
   - Proceed if user confirms

3. **Check task completion status**

   Read the tasks file (typically `tasks.md`) to check for incomplete tasks.

   Count tasks marked with `- [ ]` (incomplete) vs `- [x]` (complete).

   **If incomplete tasks found:**
   - Display warning showing count of incomplete tasks
   - Use **AskUserQuestion tool** to confirm user wants to proceed
   - Proceed if user confirms

   **If no tasks file exists:** Proceed without task-related warning.

4. **Assess delta spec sync state**

   Check for delta specs at `openspec/changes/<name>/specs/`. If none exist, proceed without sync prompt.

   **If delta specs exist:**
   - Compare each delta spec with its corresponding main spec at `openspec/specs/<capability>/spec.md`
   - Determine what changes would be applied (adds, modifications, removals, renames)
   - Show a combined summary before prompting

   **Prompt options:**
   - If changes needed: "Sync now (recommended)", "Archive without syncing"
   - If already synced: "Archive now", "Sync anyway", "Cancel"

   If user chooses sync, use Task tool (subagent_type: "general-purpose", prompt: "Use Skill tool to invoke openspec-sync-specs for change '<name>'. Delta spec analysis: <include the analyzed delta spec summary>"). Proceed to archive regardless of choice.

5. **归档前强制门禁（不可跳过）**

   执行《开发执行规范》§11 归档门禁的两道校验。**任一 FAIL 必须阻断 archive**（不是警告确认，是硬拒绝）。

   **校验 1：openspec 制品 schema 一致性**
   ```bash
   openspec validate "<name>" --strict
   ```
   FAIL → 阻断，列出校验错误，要求修复后重试。

   **校验 2：代码↔文档一致性（项目专属 harness 校验）**
   ```bash
   bash scripts/check-standards.sh
   ```
   此脚本校验 A-D 段（代码规范结构、domain 白名单、双主题、防孤立引用）。E 段（flow 变更溯源）针对 archive 后校验，此处允许跳过。

   **FAIL 处理**：
   - 输出 `## Archive Blocked` + 失败项清单
   - 明确告知用户：归档被门禁阻断，必须先修复失败项再重试 archive
   - **不得**用 AskUserQuestion 让用户绕过（与"不完整 artifact/task"的软警告不同，这是硬门禁）
   - 退出，不执行步骤 6

   **PASS**：继续步骤 6。

6. **Perform the archive**

   Create the archive directory if it doesn't exist:
   ```bash
   mkdir -p openspec/changes/archive
   ```

   Generate target name using current date: `YYYY-MM-DD-<change-name>`

   **Check if target already exists:**
   - If yes: Fail with error, suggest renaming existing archive or using different date
   - If no: Move the change directory to archive

   ```bash
   mv openspec/changes/<name> openspec/changes/archive/YYYY-MM-DD-<name>
   ```

7. **Display summary**

   Show archive completion summary including:
   - Change name
   - Schema that was used
   - Archive location
   - Whether specs were synced (if applicable)
   - Note about any warnings (incomplete artifacts/tasks)

**Output On Success**

```
## Archive Complete

**Change:** <change-name>
**Schema:** <schema-name>
**Archived to:** openspec/changes/archive/YYYY-MM-DD-<name>/
**Specs:** ✓ Synced to main specs (or "No delta specs" or "Sync skipped")
**Gate:** ✓ check-standards.sh A-D 段通过 + openspec validate --strict 通过

All artifacts complete. All tasks complete.
```

**Output On Blocked (Gate Failed)**

```
## Archive Blocked

**Change:** <change-name>

归档门禁未通过，archive 被阻断。必须修复以下失败项后重试 `/archive`：

<check-standards.sh 或 openspec validate 的失败输出>

（这是硬门禁，不可跳过。与"不完整 artifact/task"的软警告不同。）
```

**Guardrails**
- Always prompt for change selection if not provided
- Use artifact graph (openspec status --json) for completion checking
- Don't block archive on **软警告**（不完整 artifact/task）- just inform and confirm；但步骤 5 的**门禁 FAIL 是硬阻断**，不得用 AskUserQuestion 绕过
- Preserve .openspec.yaml when moving to archive (it moves with the directory)
- Show clear summary of what happened
- If sync is requested, use openspec-sync-specs approach (agent-driven)
- If delta specs exist, always run the sync assessment and show the combined summary before prompting
- 步骤 5 两道校验（openspec validate --strict + check-standards.sh）是《开发执行规范》§11 归档门禁的执行点，FAIL 必须阻断
