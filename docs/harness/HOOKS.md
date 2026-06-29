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
  - ../../scripts/correction-nudge.sh
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

- 若 `tasks/todo.md` 声明 `level: L2`+ **且已补 `## Review` 段（= 在收尾，rule-0013）**，检查 `docs/eval/task-reviews/` 是否有本 task 的评审产出；没有 → 拦住收尾，提示先 `make eval`（rule-0005）。**只在收尾时拦、任务进行中（还没补 Review）不拦**——否则多轮 L2+ 任务每个 turn-end 都被误拦（lessons 2026-06-27）。由 `scripts/stop-check.test.sh` 自测。
- （原"踩坑记 lessons"的 exit-0 stderr 死提醒已删——它不注入上下文、没人看见；纠错提醒改由下方 UserPromptSubmit 钩子承担。）

**局限（诚实说）**：档位与 Review 都由 agent 在 `todo.md` 里自己声明，所以是"半强制"——声明了 L2+ 且补了 Review 却漏 eval 会被抓，但故意低报档位、或不补 `## Review` 段，仍能绕过。比纯靠自觉强很多，不是 100% 自动。

## 自进化兜底（Stop hook 第二段，ADR-0005）

`stop-check.sh` 在收尾闸门之后调 `scripts/turn-backstop.sh`，**机械触发**的廉价兜底（脚本管 WHEN、Haiku 管 WHAT）：

- **触发**（脚本顶部常量可调）：`K` 轮到点（默认 8）/ commit 边界（HEAD 变）/ 变更文件数**增量** ≥ 阈值（默认 10，是"涨多少"非绝对值）。状态存 `tasks/.turn-count`、`tasks/.last-backstop`（gitignore）。
- 触发后 **headless `claude -p --model haiku`** 复查最近 transcript，捞"做了决策 / 学了偏好 / 有知识却没写进文件"的遗漏；其中"改了文件没同步文档"一类，**对照 `.agents/skills/doc-sync/SKILL.md` 漂移对照表的 `🔴手` 行**判断（单一来源，不自抄子集；机器能兜的 `✅机检` 行交给 `make verify`），追加 `tasks/optimization-log.md` 并 stderr 提醒。
- **安全**：递归 guard（headless 调用带 `HARNESS_TRIAGE=1` + 从中性目录 `/tmp` 跑、不加载项目钩子）；本机无 `timeout`/`gtimeout`，用 perl `alarm` 包超时；budget 封顶；**best-effort——任何失败一律 exit 0，绝不阻断收尾**。安全性由 `scripts/turn-backstop.test.sh` 自测（不调 Haiku）。

为什么不"每轮跑 eval"：纯讨论轮空转、烧 token / 拖慢。为什么触发机械而非 agent 判断：漏记的 agent 同样会忘记触发兜底——故触发必须独立于 agent。重判断（多维优化）走 `.claude/agents/self-optimize.md` 子 agent，按需 spawn。

## 纠错提醒（UserPromptSubmit hook，rule-0011）

`.claude/settings.json` 的 `UserPromptSubmit` hook 每轮调 `scripts/correction-nudge.sh`，把「自检：用户上一句是否在纠正你？是则当轮记 `tasks/lessons.md` 三段式」**注入 agent 当轮上下文**（UserPromptSubmit 的 stdout 在 exit 0 时注入）。

- **判断交给 agent**：是不是纠正靠 agent 自己判（它上下文最全，比关键词 / 小模型都准），钩子只负责把提醒**可靠塞到眼前**——替掉原 `stop-check.sh` 里那行 exit-0 stderr 死提醒（不注入、没人看见）。
- **顺带整理提醒（step 4）**：同一钩子还跑 `scripts/lessons-promote-check.sh` 数 `tasks/lessons.md` 里"没 `<!-- opt: -->` 标记"的 lesson，超阈值（默认 10）就**多注入一句**，提示整理——走 `self-evolution` 挑、`add-rule` 升、不值得标 `skip`、提醒过未决定标 `seen`（标记约定见 `tasks/lessons.md` 头部）。计数由 `scripts/lessons-promote-check.test.sh` 自测。
- **best-effort**：消费 stdin、永远 exit 0，**绝不阻断 prompt**（不用 exit 2）。由 `scripts/correction-nudge.test.sh` 自测（进 `make verify`）。
- **局限（诚实说）**：每轮注入同一句，仍有"被 tune out"的 wallpaper 风险——比死提醒强（真注入 + 贴着用户消息新鲜出现），但仍是软提醒；若实测仍漏记，再加检测让它**只在疑似纠正时**响（soft→hard）。
- **codex 对等**：settings.json 钩子仅 Claude Code 吃；但规则本体 rule-0011 在根 `AGENTS.md`（Codex 原生读），软提醒 claude-only 可接受（gates-hooks.md：软 hook 只是早提醒、非对等机制）。
