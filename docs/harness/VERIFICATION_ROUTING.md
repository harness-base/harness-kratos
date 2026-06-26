---
title: 验证路由
status: active
owner: harness
last_updated: 2026-05-29
source_files:
  - ../../workspace/verification.yaml
related_docs:
  - CI.md
---

# 验证路由

控制面**不存放业务测试本体**，只回答"每个被管工程怎么验证"。事实源是 `workspace/verification.yaml`。

## 两层测试

| 层 | 放哪 | 谁跑 |
|---|---|---|
| 被管工程的 unit / API / E2E | `projects/<name>/` 里，与代码同处 | 该工程自己的命令（见路由表） |
| 控制面自检（结构 / 文档 / hook policy） | `scripts/`、`.githooks/` 旁 | `make verify` |

## 路由表（随被管工程填）

`workspace/verification.yaml` 为每个工程登记：

- `verify`：最小收口检查命令。
- `unit` / `api` / `e2e`：各层测试命令与工作目录。
- `sandbox`：E2E 需要的环境（接后端时补 `local-sandbox-docker/` + `SANDBOX_E2E_ENV.md`）。

## 证据

跑完记录命令 / 时间 / 环境 / 结果 / 分类（pass·fail·blocked·skipped，见 rule-0002）/ 对应 case id；测试"够不够好"由 `../eval/` 评委按 rubric 打分。
