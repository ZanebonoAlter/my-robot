
## doc-impact verify 对 gitignored 文档的盲区

doc-impact.sh verify 的 checkbox 文件对账对 gitignored 文档存在双重兜底盲区：规则 2 兜底逻辑为「文件不在 changed 集合时，查 git log 曾提交即算已更新」（scripts/doc-impact.sh:219-226），但 gitignored 文件（如 docs/research/ 整目录，.gitignore:51）既不出现在 git status（changed 集合 miss）也从未进 git 历史（git log miss）→ checkbox 里写了这类文件路径就永远报「声明了未更新」FAIL。本 change（tool-output-spill）tasks.md 5.1 因此误报，解法是 checkbox 不写裸 docs/ 路径（任务照样完成，只绕开提取正则 grep -oE 'docs/[^ "\`)]+\.md'）。根本修法（属 doc-impact-gate spec 变更，需开 change）：规则 2 兜底处加 `git check-ignore "$f"` 判断——ignored 文件只要存在于工作树即视为已更新。同类注意：scenario-trace.sh 的映射表单元格只认纯路径（空白/逗号分隔）或「人工：」前缀，说明文字（括号、加号续写）会被拆成不存在的路径而 FAIL。

<!-- pinned 2026-08-23T16:33:46Z -->
