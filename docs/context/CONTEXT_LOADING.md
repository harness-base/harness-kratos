---
title: 按需加载档位
status: active
owner: harness
last_updated: 2026-06-26
source_files: []
related_docs:
  - ../README.md
---

# 按需加载档位（读多少）

默认少读。**按"用户要的产物 + 任务证据 + 目标文件"判断档位，不按关键词。** 拿不准就先按低档起步，遇到证据再升档。

| 档位 | 典型任务 | 至少读 |
|---|---|---|
| L0 | 查事实 / 纯问答 | `AGENTS.md` + 相关 1–2 个文件 |
| L1 | 轻量修补、文案 | + `CURRENT_STATUS.md` + 目标文件 |
| L2 | 小功能、明确改动 | + `docs/README.md` 路由 + 相关 `rules/` + 目标工程 `AGENTS.md` |
| L3 | 标准功能 | + 对应 `features/` 需求包 + `harness/VERIFICATION_ROUTING.md` |
| L4 | 跨模块 / 较大功能 | + `docs/architecture/` + 相关 `decisions/` |
| L5 | 架构级改动 | + 全量相关 ADR |
| L6 | 重大 / 全局 | 最全相关上下文 |

## 按目录加载（就近 AGENTS.md）

档位管"读多深"，目录管"读哪儿"——两者叠加：

- 在某目录下**读或改**代码前，从该位置**向上找最近的 `AGENTS.md`** 加载（本层没有就用上层）。只加载最近那一个 + 它显式指向的，不把一路祖先全吞进来。
- 例：动 `projects/backend-service/internal/data/` → 加载 `internal/data/AGENTS.md`（数据层规矩，如"用 ent、非必要不写 raw SQL"）；动别处时它根本不进上下文。
- 与档位叠加：L2 起读就近 `AGENTS.md`；L3+ 再按它的指针展开（工程级 rules / 相关文档）。
- 工程级规则就靠这个分层放：工程根 `AGENTS.md` 精简 + 指针，细则下沉到各层 `AGENTS.md`。
- **同时若该目录有 `README.md`，也先读一下**（不必通读，目的是了解这里有什么、该挑哪个）。`AGENTS.md` 走 `@import` 自动加载、内容是必须遵守的红线；README 不在加载机制内、装的是查阅型材料，靠根 `AGENTS.md` 启动顺序第 5 条触发。

## 配套门禁

- L2 以上任务、关键决策点：收尾前必须跑 `make eval`（见 `../eval/README.md`、rule-0005）。
- 升档要有理由（用户要求 / 任务证据 / 目标文件触发），写进 `tasks/todo.md` 的 Review。
