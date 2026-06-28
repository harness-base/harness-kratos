# 当前任务

> 只记手头这一件事；干完清空、旧的 roll 进 `archive/`。保持轻。
> 元：level: L4 ｜ task: prd-orchestration

## 当前：prd-elicitation 编排式重构（按 plan 执行）
设计稿 `docs/superpowers/specs/2026-06-29-prd-orchestration-design.md`、计划 `docs/superpowers/plans/2026-06-29-prd-orchestration.md`（均 approved）。产品总监(主 agent)调度 7 worker（6 建双栈 subagent + 外部调研复用 deep-research），必选/可选·权重 + 确认门 + 并行 + review loop（框并行、回原 worker、只重跑有问题的）。
- [ ] T1 ADR-0010 + 重写 SKILL 总谱
- [ ] T2 prd-reviewer 子 agent（双栈）
- [ ] T3 5 个产出 worker 子 agent（双栈）
- [ ] T4 Workflow 编排模板
- [ ] T5 doc-sync + verify
- [ ] T6 对抗挑刺(dogfood)+ 收尾 eval + 补 Review

## 已闭：dev-skill（L4，已提交 7b6576d，eval green）
写代码统一入口替代 feature-delivery/bugfix；两轮挑刺修 8 处。下次清理滚 archive。

## 当前：dev skill——写代码统一入口（按"深度级"自走，进行中）

**设计（用户逐步定）**：独立"写代码"skill，统管 功能/工程/重构/改 bug；主线 想清楚→plan→确认→写（不假设·决策点问·防债）→挑刺(对抗 review)→提醒测。
- **两级**：常规（小而明确：plan→写→1-2 个 code-reviewer 挑刺到无 bug→提醒测）/ 深度（动核心·接口·不可逆·要写 ADR：brainstorm→正式 plan→写(决策点必问)→多视角挑刺到零→收尾 eval→提醒测）。默认 agent 按升级信号自判、用户可指定。
- **挑刺派活**：Claude Code 用 workflow（agentType:'code-reviewer'）；Codex 用原生派同名；云端深审提醒用户跑 `/code-review ultra`。
- **共享 `code-reviewer` 子 agent 双栈**；**借用不重写** superpowers + 收尾 eval；需求包门禁(rule-0001)保留、dev 深度级接管。

**进度**：
- [x] `.agents/skills/dev/SKILL.md` + `.claude/agents/code-reviewer.md` + `.codex/agents/code-reviewer.toml` + ADR-0009
- [x] 删 `.agents/skills/feature-delivery/` + `.agents/skills/bugfix/`
- [x] 改 ~20 处活引用（feature-delivery/bugfix → dev）+ ADR-0003 related_docs 去悬空
- [x] regen skills-index + `.claude/agents` 索引；登记 ADR-0009；`make verify` 全绿（docs-audit 无悬空）
- [x] 两轮深度级挑刺（R2 用新建的 code-reviewer dogfood）→ 修 8 处（含真缺口 .codex 未注册 code-reviewer、AGENTS 子代理清单漂移）→ 全仓 grep 确认删改完整 → make verify + docs-audit 绿
- [x] 收尾 eval green（task=dev-skill，010/011/014 pass）
- [ ] 提交

## Review
- **任务**：新建统一"写代码" `dev` skill（L4），替代并删除旧 `feature-delivery` + `bugfix`；统管 功能/工程/重构/改 bug/迁移；两级（常规/深度，默认自判·可指定）；纪律常开（不假设/决策点问/防债/提醒测）；挑刺=对抗 review 派共享 `code-reviewer` 子 agent（双栈），借用 superpowers + 收尾 eval、不重写；需求包门禁保留 dev 接管。
- **产物**：`dev/SKILL.md`、`code-reviewer` 双栈（`.claude/.md` + `.codex/.toml` + config 注册）、ADR-0009；删 2 skill；改 ~20 处活引用 → dev；AGENTS 子代理行指针化（rule-0012）。
- **质量**：两轮对抗挑刺（R2 dogfood 新 code-reviewer）共修 8 真问题——含**真缺口**（`.codex/config.toml` 漏注册 code-reviewer = Codex 调不到）+ 一片"删旧建新连带漂移"（AGENTS 子代理清单 / self-evolution SKILL 仍当 bugfix 缺口 / docs.md 提已删 bugfix）；根因（枚举无 --check 必漂）指针化根治；全仓 grep 确认残留全为历史记录。lessons 记 2 条（删 skill 连带漂移复发 + 指代误读）。
- **验证**：`make verify` 绿（skills/.claude/agents 索引去旧增新无漂、rule-0012 不硬编码、shim 齐）、`make docs-audit` 28 篇绿、删除干净（ls 报不存在）。收尾 eval green（`docs/eval/task-reviews/20260628T145456Z-dev-skill/`）。
- **未决（用户定"逐步"）**：其余旧 skill 是否并入 dev 后续评估；loop-engineering 流程仍缺口。
