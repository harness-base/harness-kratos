# templates 维度 审查手册

> 链路：**标准产出该有模板 → 模板字段反映当前规范 → 起草时真用模板（不手搓省栏）**。任一环断 = 强制栏漂移、字段过期。

## 规范（健康长什么样 / 不变量）

- 每类**反复出现的标准产出**都有模板，全在 `templates/`：`adr.md`、`feature-package.md`、`prd.md`、`plan.md`、`doc.md`、`eval-rubric.md`、`skill/SKILL.md`。
- 模板字段**反映当前规范**：规则一改（尤其新增强制披露栏），相关模板同步。例：`templates/adr.md` 必须有 `## 受影响的 skill（rule-0007）` 栏。
- 起草用模板、不手搓：ADR/feature/PRD/plan/skill 起草时从对应模板拷字段，省栏 = 漏强制项。
- 模板被操作 skill 显式引用（投送）：`hc-dev`（深度级）→ `templates/feature-package.md`；`hc-prd` → `templates/prd.md`；指针不悬空。
- `templates/README.md` 是**自动生成的索引**（`scripts/dir-index.sh templates`），禁手改、进 `make verify`。

## 怎么检索现状（命令可直接跑）

```bash
# 1. 模板清单 + 自动索引是否漂移（已进 make verify: verify-control-plane.sh:36）
ls templates/ templates/skill/
bash scripts/dir-index.sh templates --check

# 2. 谁引用模板（操作 skill 是否真把模板当起草源）
grep -rn "templates/" --include="*.md" .agents/skills docs

# 3. 强制栏漂移核心检查：哪些 ADR 缺了模板强制的"受影响的 skill"栏
grep -L "受影响的 skill" docs/decisions/000*.md   # 输出 = 缺栏的 ADR

# 4. feature 产出是否带模板强制字段
for f in docs/features/000*.md; do grep -c "delivery_status\|implementation_allowed" "$f"; done

# 5. 反查"有产出无模板"：列 docs/ 下反复出现的产出类型，比对 templates/
ls docs/eval/task-reviews/*/    # summary.md/decision.md/candidate.md —— 反复产出，但 templates/ 无对应模板
```

`templates/adr.md:16-17` 是受影响-skill 栏的源；`docs/decisions/0002`（line 109-112）的同名段是正确范例。

## 怎么判（逐条可判定）

- **缺模板**：某类产出在 `docs/` 反复出现（≥2 份、结构同形），`templates/` 却没有对应模板 → 缺口。当前已知：eval task-review 的 `summary.md`/`decision.md`/`candidate.md` 三联无模板。
- **字段脱规范**：模板里的强制栏与现行规则对不上——规则要求披露某项（如 rule-0007 受影响 skill），模板却没这一栏 → 漏洞（强制项无处可填，必被省）。
- **产出不依模板**：同类产出之间字段不齐——部分有强制栏、部分没（如 `grep -L` 抓到的缺栏 ADR）→ 手搓省栏的实锤。
- **索引漂移**：`dir-index.sh templates --check` 非零 → README 与目录不一致（手改了模板没重生成索引）。
- **指针悬空**：skill 里写的 `templates/xxx` 文件不存在 → 投送断。
- **注意**：`dir-index.sh --check` 只校验 README **是否漏列文件**，**不校验产出是否真符合模板**——字段脱规范、产出缺栏这两类**没有机器门，必须人工逐份判**（这是本维度最大盲区）。

## 常见漏洞模式（本仓真实案例）

- **手搓省了强制栏**（已发生，最典型）：`tasks/lessons.md` 2026-06-26 条 + eval 评审 `docs/eval/task-reviews/20260626T014408Z-harness-rules-distribution/`（decision.md / summary.md）——架构大改改了 `hc-add-rule` skill，但 ADR-0004 漏掉 `templates/adr.md` 强制的"受影响的 skill（rule-0007）"栏，`context-loading` 也没回顾声明 → eval-011 直接判 **blocker fail**（"做了没记 = 没履行"）。lesson 的 prevention 明写：**ADR 用 `templates/adr.md` 起草别手搓省栏**。
- **同类漏洞至今仍在仓里**：`grep -L "受影响的 skill" docs/decisions/000*.md` 现仍命中 **ADR-0001、ADR-0003**——它们缺该栏（旧 ADR 未回填）。即"修了一份、同模式残留多份"。
- **缺模板导致格式各凭手感**：eval task-review 三联（summary/decision/candidate）无模板，新评审只能照抄旧目录，字段易漂。
- **字段过期**（防范类）：规则演进后模板没跟。判据 = 拿 `docs/rules/index.yaml` 里每条 blocker 规则，反查它要求披露的项在对应模板里有没有栏位。

## 修复用哪个操作 skill / 脚本

- **改/加模板**：直接编辑 `templates/<x>.md`（加强制栏、对齐规则字段），然后 `bash scripts/dir-index.sh templates` 重生成索引。
- **规则牵动模板**：用 `hc-add-rule` skill 落规则时，同步把"该规则需披露的项"加进相关模板的栏位（rule ↔ 模板字段成对维护）。
- **回填已发生的缺栏漂移**：按 `templates/adr.md` 给缺栏的 ADR（如 0001/0003）补 `## 受影响的 skill（rule-0007）` 段，逐条写"已更新 / 无需更新+理由"。
- **补缺模板**：为反复产出（如 eval task-review 三联）新建 `templates/<x>.md`，纳入 `dir-index.sh` 索引，并在产出该物的 skill / 评委文档里引用它。
- **收口**：`make verify`（含 `templates --check`）必须绿。
