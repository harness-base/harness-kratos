---
title: eval 评分体系
status: active
owner: harness
last_updated: 2026-05-29
source_files:
  - ../../scripts/run-eval.sh
  - ../../.claude/agents/hc-eval.md
  - ../../.codex/agents/hc-eval.toml
related_docs:
  - rubric.md
  - evaluator.md
---

# eval 评分体系

**eval = 给 agent 干的活请独立"评委"按固定标准打分挑刺，不靠 agent 自评。**

## 何时触发（rule-0005）

- **L2 以上任务** + **关键决策点**（能不能开工 / 测试够不够 / 验证结论分类对不对）收尾前必须过。
- L0 / L1 轻量任务不触发。

## 怎么跑

**默认（免 key，推荐）**：让 **hc-eval 子 agent** 跑——它用当前会话的模型打分，无需任何 API key / curl。**两个运行时都配好了**：Claude Code 用 `.claude/agents/hc-eval.md`，Codex 用 `.codex/agents/hc-eval.toml`（在 `.codex/config.toml` 注册）。主 agent 通过子 agent 机制调它，传：任务档位、candidate、要套用的 prompts。

**可选（CI / headless，需 key）**：

```bash
make eval ARGS="--context-level L3 --candidate-file <产出文件> --prompts 010"
# 等价：bash scripts/run-eval.sh ...
```

`run-eval.sh` 用 bash + curl 调外部模型（`EVAL_API_BASE` / `EVAL_API_KEY` / `EVAL_MODEL`），适合无人值守自动跑分；不用 Python、不接知识库服务。

两条路写**同样的 `task-reviews/` 产出**，Stop hook 只认产出在不在——**没配 key 也不会卡住**（用子 agent 即可）。

## 组成

| 文件/目录 | 作用 |
|---|---|
| `index.yaml` | 考题登记表（每道盯一个翻车点，多数对应一条 rule） |
| `rubric.md` | 评分标准（pass/fail/blocked/skipped 口径 + 分档） |
| `evaluator.md` | 评委设定（身份、态度、输出格式） |
| `prompts/` | 一摞考题 |
| `task-reviews/` | 评审产出：`<时间戳-任务名>/` → `candidate.md` `decision.md` `summary.md` |
| `results/` | 批量跑分结果 |
| `fix-proposals/` | 问题 → 修复建议 |

`make verify-eval` 检查"该评的有没有评、产出结构对不对"。
