---
title: 文档地图
status: active
owner: harness
last_updated: 2026-05-29
source_files: []
related_docs:
  - context/CURRENT_STATUS.md
  - context/CONTEXT_LOADING.md
---

# 文档地图

按需读，不全读。下面是推荐阅读顺序和各目录职责。

## 阅读顺序

1. `../AGENTS.md` —— 常驻规则源（启动顺序、硬规则红线、验证命令）。
2. `context/CURRENT_STATUS.md` —— 当前真实状态。
3. `context/CONTEXT_LOADING.md` —— 本次任务该读多少。
4. `rules/index.yaml` —— 带编号规则库（按需查某条）。
5. `harness/VERIFICATION_ROUTING.md` —— 怎么验证。
6. `eval/README.md` —— 评分体系怎么用、何时触发。
7. 接工程时：`harness/PROJECT_ONBOARDING.md` —— 把工程挂进来的步骤。

## 目录职责

| 目录 | 职责 |
|---|---|
| `context/` | 项目简报、真实状态、按需加载档位 |
| `rules/` | 带编号的规则库；可被 eval / ADR / feature 按号引用 |
| `decisions/` | ADR 架构决策记录 |
| `eval/` | 评分体系：考题、rubric、评委、评审产出 |
| `harness/` | 验证路由、CI、hooks 说明、工程接入指南 |
| `features/` | 需求 / 工作包（空账本，随被管工程填） |

`architecture/` 待接入 `projects/` 后再建；`drift/` 区已弃用、漂移处理并入自进化闭环（见 `decisions/0006-drop-drift-area.md`）。
