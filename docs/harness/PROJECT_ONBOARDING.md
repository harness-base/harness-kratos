---
title: 工程接入指南
status: active
owner: harness
last_updated: 2026-05-29
source_files:
  - ../../workspace/verification.yaml
related_docs:
  - VERIFICATION_ROUTING.md
  - ../context/CONTEXT_LOADING.md
  - ../features/README.md
---

# 工程接入指南（把一个工程挂进 harness）

控制面先行、工程后挂。把第一个（或下一个）被管工程接进来，照下面走——**大多是往留好的口子里"填空"，基本不动控制面本身**。

## 0. 前提
- 控制面 `make verify` 是绿的。
- 工程代码准备好放进 `projects/<name>/`（`<name>` 用 kebab-case，如 `backend-service`）。

## 1. 放代码
把工程放进 `projects/<name>/`。控制面只"管"它、不持有它的业务测试本体（测试跟代码在一起）。

## 2. 给工程写入口 `AGENTS.md`
- `projects/<name>/AGENTS.md`：**精简**——工程红线 + 指针（如"数据层规范见 `internal/data`"）。
- 规则多就**下沉到各层**：`projects/<name>/<dir>/AGENTS.md`（就近优先；干哪层活只加载哪层，见 `../context/CONTEXT_LOADING.md`）。
- 别把所有规则堆进工程根 `AGENTS.md`。

## 3. 填验证路由
编辑 `workspace/verification.yaml`，给这个工程登记 `verify`（最小收口检查）/ `unit` / `api` / `e2e` / `sandbox` 的命令与工作目录。这样 `make verify` 和 CI 才知道怎么测它（详见 `VERIFICATION_ROUTING.md`）。

## 4. 立第一个需求包
用户可见的需求 / 行为 / 验收变化，动业务代码前先立项（rule-0001）：
- `templates/feature-package.md` 建包 → 登记 `docs/features/index.yaml`
- 补验收目标 + 测试设计，推进到 `tests_ready` 再写业务代码。
（走 `hc-dev` skill 深度级——含需求包门禁。）

## 5. 加工程级规则（用 `hc-add-rule` skill）
日常踩的坑、定的规范，按 `hc-add-rule` 三步落地：定范围 → 写下来 + 登记 → 挂执行。
- 跨工程 → `docs/rules/`；工程通用 → 工程根 `AGENTS.md`；只管某层 → 就近 `AGENTS.md`。
- 能机器判 → `scripts/hook-policy.sh`；要人判 → `docs/eval/prompts/` 加考题。
- 例："数据层用 ent、非必要不 raw SQL" → 放 `internal/data/AGENTS.md` + hook 拦 raw SQL。

## 6. 测试放哪
- 工程的 unit / API / E2E **在工程里**、跟代码同处。
- 控制面只做：路由（第 3 步）、执行（skill / CI）、评分（eval）、收证据；**不放测试本体**。

## 7. 接 sandbox / E2E（要跑 E2E 时）
建 `workspace/local-sandbox-docker/`（具名 sandbox + 状态文件）+ `docs/harness/SANDBOX_E2E_ENV.md`。E2E 用具名环境，不靠端口 / 域名猜。

## 8. 接文档维护（可选）
把 `docs-maintainer` skill 改造接进 `.agents/skills/`（按工程结构填它的"变更 → 文档"映射表），让文档随代码同步。

## 9. CI 扩展
`.github/workflows/verify.yml` 现在只跑控制面自检；接工程后加 affected verify（按 `workspace/verification.yaml` 路由，只跑跟改动相关的工程测试）。

## 10. 收尾
L2+ 任务收尾跑 eval（hc-eval 子 agent，免 key）；Stop hook 兜底检查。

## 接入校验清单
- [ ] 代码在 `projects/<name>/`
- [ ] 工程 `AGENTS.md`（精简 + 就近下沉）
- [ ] `workspace/verification.yaml` 填了路由
- [ ] 第一个 feature 包就绪
- [ ] 工程级规则按 `hc-add-rule` 落地（放对 + 登记 + 挂执行）
- [ ] `make verify` 绿
