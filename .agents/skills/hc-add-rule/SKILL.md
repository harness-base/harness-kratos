---
name: hc-add-rule
description: 把一条规则真正落地（团队规范、踩坑约束、编码红线）。用户说"以后都要/不许/必须…"、或你想固化一条规范时用本 skill，走"定范围→写下来+登记→挂执行"三步，确保规则会被加载、违反会被发现，而不是写完没人理。
version: 2
last_reviewed: 2026-06-26
---

# 加一条规则（hc-add-rule）

规则写完没人读、没人拦 = 白写。本 skill 三步保证它放对地方、会被加载、会被执行。

## 何时用 / 何时不用
- 用：用户说"以后都要 / 不许 / 必须…"，或你发现一条该固化的规范。
- 不用：一次性临时提醒（那只记 `tasks/lessons.md`）。

## 三步（缺一步，规则就等于白加）

### 第 1 步：定范围 —— 这规则管谁？
规则一律**入驻就近的 `AGENTS.md`**（ADR-0004），放到"能覆盖其所有目标的最浅 `AGENTS.md`"：
- harness 全局治理（如"改业务码前先立项"）→ 根 `AGENTS.md`
- 某工程通用（如"这后端时间一律 UTC"）→ 该工程根 `projects/<x>/AGENTS.md`
- 只管某层（如"数据层用 ent、非必要不 raw SQL"）→ 离那层最近的 `<dir>/AGENTS.md`

### 第 2 步：写下来 + 登记 —— 让它会被读到
- 在选定的 `AGENTS.md` 加一条 bullet：一句"必须 / 禁止"（必要时带一行为什么 / 怎么做）。
- **带隐形标记**供索引扫描：`<!-- rule: rule-00NN | sev: blocker|warn | eval: <考题号，可空> -->`。编号取现有最大 +1（全仓唯一、稳定引用键，被 eval/ADR/feature 按号引用）。
- **重生成 catalog**：`bash scripts/rules-index.sh`（生成 `docs/rules/index.yaml`，**禁手改**）。
- 自检：`make verify` 绿（rules 索引无漂移 + 该 `AGENTS.md` 有 `CLAUDE.md` shim）= 已收录、就近可加载。

### 第 3 步：挂执行 —— 让违反会被发现
按"能不能机器判"分两路：
- **能机器判**（某命令 / 字符串、改 A 必须改 B、某路径模式）→ `scripts/hook-policy.sh` 加一条匹配 + `scripts/hook-policy.test.sh` 加正反用例 → 提交 / CI 自动拦。
- **要人判**（设计是否合理 / 过度）→ `docs/eval/prompts/` 加一道考题引用规则编号 + 登记 `docs/eval/index.yaml` → 收尾 eval 打分。
- 两者都不便 → 至少完成第 2 步（会被加载），软约束。

## 收尾自检
- [ ] 范围对（放在最近该管的地方）
- [ ] 登记了（`index.yaml` / 最近 `AGENTS.md` 指针）
- [ ] 挂了执行（hook 或 eval）或明确标"软约束"
- [ ] `make verify` 还绿

## 例子：加"数据层用 ent、非必要不 raw SQL"
1. 范围：后端数据层 → `projects/backend-service/internal/data/AGENTS.md`
2. 写 + 标记：在该 `AGENTS.md` 加一条 bullet + `<!-- rule: rule-00NN | sev: warn -->`，跑 `scripts/rules-index.sh` 重生成 catalog
3. 执行：`hook-policy.sh` 加"数据层外出现 raw SQL（`database/sql`、手拼 `.Query` / `.Exec`）→ 提醒" + 用例
4. `make verify` 绿 → 落地完成

## 演进（rule-0007）
随规则体系 / 工程结构演进时回顾本 skill；步骤变了更新 `version` / `last_reviewed`。
