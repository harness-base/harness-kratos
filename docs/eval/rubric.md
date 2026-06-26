---
title: eval 评分标准
status: active
owner: harness
last_updated: 2026-05-29
source_files: []
related_docs:
  - README.md
  - evaluator.md
---

# eval 评分标准

每道考题输出一个 verdict + 理由 + 证据。

## verdict 口径

- **pass**：满足该考题全部通过标准，有证据。
- **fail**：违反规则或缺关键证据。
- **blocked**：因无关的环境 / 外部原因无法判定（写清阻塞点）。
- **skipped**：本任务不适用该考题（写清原因）。
- **n/a**：考题与本任务无关。

## 综合分档（010 收尾用）

- **green**：全部相关考题 pass。
- **yellow**：有 warn 级问题或 blocked，需说明，可有条件收尾。
- **red**：有 blocker 级 fail，MUST STOP，先修。

## 证据要求

每个 verdict 必须引具体证据（命令输出要点 / 文件路径 / case id），不接受"应该没问题"。
