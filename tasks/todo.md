# 当前任务

> 只记手头这一件事；干完清空、旧的 roll 进 `archive/`。保持轻。
> 元：level: L4 ｜ task: prd-orchestration

## 当前：prd-elicitation 编排式重构（按 plan 执行；收尾）
设计稿/计划 `docs/superpowers/{specs,plans}/2026-06-29-prd-orchestration*`（approved）。产品总监(主 agent)调度 7 worker（6 双栈 subagent + 外部调研走 deep-research skill），必选/可选·权重 + 确认门 + 并行 + review loop（框并行、回原 worker、只重跑有问题的）。
- [x] T1 ADR-0010 + 重写 SKILL 总谱（5e22c2d）
- [x] T2 prd-reviewer 子 agent 双栈（a33c349）
- [x] T3 5 个产出 worker 子 agent 双栈（fe442c7）
- [x] T4 Workflow 编排模板（7564fa7）
- [x] T5 doc-sync + verify（7bbef49）
- [x] T6 对抗挑刺(dogfood 11 agent) 修平 4 类（7630519）→ 收尾 eval green → 修 1 warn → 补 Review

## Review
- **任务**：把 `prd-elicitation` 从线性交互 skill 重构成**编排式**（L4，ADR-0010）：产品总监（编排逻辑·主 agent 当）调度 7 worker 角色，三层优先级（用户指令>必选>可选·权重）+ 确认门 + 并行产出 + 两段 review loop（轻审地基/重审下游、框并行、回原 worker、只重跑有问题的）。
- **产物**：ADR-0010 + 登记；SKILL 重写成编排总谱（version 3）；6 worker 子 agent 双栈（prd-reviewer + 5 产出员，各 `.claude/.md`+`.codex/.toml`+config 注册）；外部调研走可用的 `deep-research` skill（不另建 subagent）；`references/orchestration-workflow.js` 编排模板；doc-sync（CURRENT_STATUS 指针化 + docs/README）。6 个 commit `5e22c2d..`。
- **质量**：一轮对抗挑刺 dogfood code-reviewer（11 agent / 3 视角 / 每条独立证伪），确认 4 类真问题并修平——① deep-research「复用」措辞如实化（可用 plugin skill、非 repo 资产/非第7 subagent，走 Skill 工具调；六处对齐）② workflow 重审在用户跳过原型时不再误重跑 prototype-builder（`ran` 集合过滤 + 条件审稿提示）③ 重审 loop 加 `MAX_ROUNDS=4` 防不收敛 ④ 加注澄清 export+顶层 return 是 Workflow 约定（node 裸模块校验报错=误报）。1 条被驳。
- **验证**：`make verify` 全绿（索引无漂、rule-0012 不硬编码、双栈对齐）、`make docs-audit` 30 篇绿、workflow 模板包 async 函数后 `node --check` 过。**收尾 eval green**（独立评委：考题 011/013/014 全 pass、010 综合 green）：`docs/eval/task-reviews/20260628T175152Z-prd-orchestration/`；eval 提的 1 warn（ADR/spec 加粗措辞）已修。
- **未决（押后）**：常驻自主"产品总监 agent"、通用 loop-engineering 引擎、harness 自身 observability、外部 MCP（飞书/figma）。下一步：finishing-a-development-branch（是否推 + 后续）。

## 已闭（已提交，下次清理滚 archive）
- dev-skill（L4，7b6576d，eval green）：写代码统一入口替代 feature-delivery/bugfix；两轮挑刺修 8 处。
- test-case-skill（L3，c0c94f6，eval green）：产用例 + AC/FP 覆盖硬闸；4 轮挑刺修 25 处。
- prd-workflow-redesign（L3，cbfbc7b）：产出需求流程重做（ADR-0007）。
