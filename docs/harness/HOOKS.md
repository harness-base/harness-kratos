---
title: Git Hooks
status: active
owner: harness
last_updated: 2026-06-26
source_files:
  - ../../scripts/hook-policy.sh
  - ../../scripts/install-hooks.sh
  - ../../scripts/stop-check.sh
  - ../../scripts/turn-backstop.sh
related_docs:
  - CI.md
  - ../decisions/0005-self-evolution-loop.md
---

# Git Hooks

把"不许干的事"做成**带测试的检查**，提交/推送时自动拦，不靠自觉（rule-0006）。

## 安装

```bash
make hooks   # git config core.hooksPath .githooks
```

## 拦什么

`.githooks/pre-commit` 调 `scripts/hook-policy.sh` 检查暂存内容：

- 疑似密钥 / token / Authorization header。
- 高危命令痕迹（`git reset --hard`、`rm -rf /` 等）。

`.githooks/pre-push` 跑 `make verify`。

## policy 必须可测

`scripts/hook-policy.sh` 的每条规则在 `scripts/hook-policy.test.sh` 里有对应用例；`make verify` 会跑这些测试。改 policy 必须同步改测试。

## 收尾闸门（Stop hook）

`.claude/settings.json` 的 Stop hook 在 agent 收尾时调 `scripts/stop-check.sh`：

- 若 `tasks/todo.md` 声明 `level: L2`+，检查 `docs/eval/task-reviews/` 是否有本 task 的评审产出；没有 → 拦住收尾，提示先 `make eval`（rule-0005）。
- 总是提醒：踩坑记 `tasks/lessons.md`。

**局限（诚实说）**：档位由 agent 在 `todo.md` 里自己声明，所以是"半强制"——声明了 L2+ 漏 eval 会被抓，但故意低报档位仍能绕过。比纯靠自觉强很多，不是 100% 自动。

## 自进化兜底（Stop hook 第二段，ADR-0005）

`stop-check.sh` 在收尾闸门之后调 `scripts/turn-backstop.sh`，**机械触发**的廉价兜底（脚本管 WHEN、Haiku 管 WHAT）：

- **触发**（脚本顶部常量可调）：`K` 轮到点（默认 8）/ commit 边界（HEAD 变）/ 变更文件数**增量** ≥ 阈值（默认 10，是"涨多少"非绝对值）。状态存 `tasks/.turn-count`、`tasks/.last-backstop`（gitignore）。
- 触发后 **headless `claude -p --model haiku`** 复查最近 transcript，捞"做了决策 / 学了偏好 / 有知识却没写进文件"的遗漏，追加 `tasks/optimization-log.md` 并 stderr 提醒。
- **安全**：递归 guard（headless 调用带 `HARNESS_TRIAGE=1` + 从中性目录 `/tmp` 跑、不加载项目钩子）；本机无 `timeout`/`gtimeout`，用 perl `alarm` 包超时；budget 封顶；**best-effort——任何失败一律 exit 0，绝不阻断收尾**。安全性由 `scripts/turn-backstop.test.sh` 自测（不调 Haiku）。

为什么不"每轮跑 eval"：纯讨论轮空转、烧 token / 拖慢。为什么触发机械而非 agent 判断：漏记的 agent 同样会忘记触发兜底——故触发必须独立于 agent。重判断（多维优化）走 `.claude/agents/self-optimize.md` 子 agent，按需 spawn。
