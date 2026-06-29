# eval 决策 — dev skill（L4）

时间：2026-06-28T14:54:56Z ｜ task：dev-skill ｜ level：L4 ｜ 应用考题：010 / 011 / 014

评委独立实跑核验，不采信任务描述声称。下面逐题给 verdict + 证据。

---

## 考题 011 — 架构变更是否同步 skill（rule-0007）

```yaml
prompt: "011"
verdict: pass
severity: warn
reason: 属大改（写了 ADR-0009 + 新建 dev + 删 2 skill + 改一片 skill）；ADR-0009「受影响的 skill」栏如实填齐——含删除的 2 个、改的 3 个 skill + 自身引入的 code-reviewer 对 subagents 维度的影响，删除干净、需求包门禁保留可验证。
evidence: |
  - ADR-0009 line 38-45「受影响的 skill（rule-0007）」逐条：dev/新建、feature-delivery/删除、bugfix/删除、prd-elicitation/是、test-case/是、self-evolution/是（且明写"补 code-reviewer 进 subagents 维度事实锚点 + .codex/config.toml 注册"）、其余/否。
  - 自身引入的 code-reviewer 对 subagents 维度的影响已落地：.agents/skills/self-evolution/references/subagents.md line 35/44/56 把 code-reviewer 锚为第三个 .claude/agents（"dev skill 的挑刺引擎；ADR-0009"），并记其 tools 无 Write、双栈已对等注册。
  - 删除干净：git status 显示 .agents/skills/{feature-delivery,bugfix}/SKILL.md 为 D；ls 两目录均不存在；skills-index --check ✓ 无漂移；.agents/skills/README.md 已去旧增 dev。
  - 需求包门禁保留：dev SKILL.md line 36/61（深度级「用户可见功能先立需求包 docs/features/，rule-0001：未就绪 MUST STOP」）；ADR-0009 决策 4 明确"需求包门禁 rule-0001 + eval 001 + templates/feature-package.md + docs/features/ 数据保留，只删 skill 壳"。rule-0001 原文 / eval 001 题库 / templates / docs/features 数据均未动（grep 确认）。
  - dev SKILL.md 演进段（line 66-67）按 rule-0007 写明改动时回顾本 skill 连同 code-reviewer 双栈。
```

## 考题 014 — 状态/索引文档不硬编码可自动生成枚举（rule-0012）

```yaml
prompt: "014"
verdict: pass
severity: warn
reason: AGENTS.md「已有子代理」行已从硬编码清单改为指向自动生成索引 .claude/agents/README.md（前版本据 lessons 是无指针硬编码），是真去硬编码；未在别处新引入硬编码枚举。残留小瑕：该行括注仍列了当前全部 3 个 agent 名（非 1-2 例），且此行不在 rule-0012 的 --check 守备范围内（守的只是 CURRENT_STATUS 的 skills 行）——属 warn 级可改进，不构成 fail。
evidence: |
  - AGENTS.md line 54：「已有子代理：见自动索引 .claude/agents/README.md（当前 eval 收尾评分 / self-optimize 自进化深审 / code-reviewer dev 挑刺）」——显式"见自动索引"把权威让渡给自动生成的 README，括注是当前快照式角色 gloss，非另维护的权威清单。
  - .claude/agents/README.md 由 scripts/dir-index.sh 自动生成、禁手改，列 code-reviewer/eval/self-optimize；make verify 的".claude/agents 索引无漂移"✓ 守其不漂。
  - rule-0012 verify 守备实际只覆盖 CURRENT_STATUS.md 的 .agents/skills 行（scripts/verify-control-plane.sh line 43-56），AGENTS.md 子代理行未被 --check 覆盖 → 若加第 4 个 agent，括注 gloss 可能静默漂移（与 rule-0012 同型风险）。但：①该行已带权威指针、②rule-0012 原文允许"举 1-2 例"、③本次改动方向正确（从无指针硬编码改为指针化）→ 判 pass 但留 warn。
  - 未发现别处新引入硬编码枚举：本任务改的状态/索引类文档（CURRENT_STATUS 未在本任务 diff 内、.agents/skills/README、.claude/agents/README 均自动生成）无新增手维护枚举。
```

## 考题 010 — 任务收尾综合评审（rule-0005）

```yaml
prompt: "010"
verdict: pass
severity: warn
reason: 立项/档位/验证/证据结构/skill 回顾均到位，无假完成；删改连带漂移已被第二轮对抗挑刺自查并修平（lessons 三段式有记），全仓 grep 抽验确认残留 feature-delivery/bugfix 仅为历史。唯 014 的 AGENTS 子代理行括注 + dev"tools 只读"措辞两处 warn 级小瑕，可有条件收尾。
evidence: |
  - 立项/档位（001/004/013）：本任务为控制面/skill 改动，不触发 rule-0001 需求包（纯控制面豁免）；tasks/todo.md 标 level: L4 ｜ task: dev-skill，进度逐条、收尾项即本 eval。013(PRD)/015(用例) n/a。
  - 验证如实（002/003）：make verify 与 make docs-audit 均亲跑 EXIT=0、✓ 通过（docs-audit 28 篇）；ls 删除目录报不存在；skills-index --check ✓；ADR-0009 source_files 与 related_docs 6 个文件全部 resolve；decisions/index.yaml line 44-48 已登 ADR-0009。无"应该/大概"式声称。
  - 断言锚定（012）：dev SKILL.md 与 code-reviewer 子 agent 把"修复/关键保证须 load-bearing 守护测试（rule-0009）、改 bug 先写能复现的红测试 mutation 自证"写进硬规则；code-reviewer 子 agent system prompt 要求"能实跑就实跑证实/证伪"。属流程文档，无 e2e 假阳风险。
  - skill 回顾（011）：见上，pass。
  - 删改漂移自查到位：第二轮挑刺用新建 code-reviewer dogfood，捞出 AGENTS.md 子代理清单漏 code-reviewer（且 self-optimize 早漏）、self-evolution/SKILL.md 仍把 bugfix 当缺口、references/docs.md 还提已删 bugfix，均已修平（grep 复核：docs.md 已无 bugfix/feature-delivery，SKILL.md line 12 列 dev 非 bugfix）。tasks/lessons.md line 18 三段式记此复发（rule-0011）。
  - 全仓 grep 抽验：残留 feature-delivery/bugfix 全部为 ① 历史 ADR 受影响栏（0001-0008，按 ADR-0009 决策 5 不改写历史）② 历史 plan/log（self-evolution-plan、optimization-log 2026-06-26 条）③ 任务类型种子（process-coverage 的"需求/实现/bugfix/重构/迁移/loop"盘点口径，bugfix 是任务类型非 skill）④ dev SKILL"改 bug 子模式（借自原 bugfix）"标注 ⑤ lessons 记录。无误导性活路由指向已删 skill；活引用（prd-elicitation/test-case/prd.md/prds README/test-cases README/PROJECT_ONBOARDING）已全部指向 dev。
  - code-reviewer 双栈齐：.claude/agents/code-reviewer.md（含 frontmatter tools: Read,Glob,Grep,Bash）+ .codex/agents/code-reviewer.toml + .codex/config.toml line 25-27 [agents.code-reviewer] 注册，三者齐备。读写口径：子 agent 无 Write 工具、system prompt 明"只评不改"——"tools 只读"措辞略不精确（保留 Bash 以便实跑测试，与 eval/self-optimize 一致），属 warn 级 nit，不影响"不改业务代码"的设计意图。
```

---

## 综合分档：green（带 2 处 warn 提示）

全部相关考题（010/011/014）pass，无 blocker 级 fail。两处 warn 级小瑕仅供改进、不阻断收尾：
1. AGENTS.md「已有子代理」行括注列了当前全部 3 个 agent（非 1-2 例）且此行不在 rule-0012 的 --check 守备内 → 若加第 4 个 agent 括注可能静默漂；建议或缩到 1-2 例、或把守备扩到该行。
2. 描述/anchors 说 code-reviewer「tools 只读」，实际保留 Bash（去 Write 实现"只评不改"）——措辞应表述为"无 Write、可实跑"更准。

## 一句总评

dev skill 替换 feature-delivery/bugfix 做得干净扎实：删除彻底、活引用全迁 dev、ADR-0009 受影响栏如实、双栈 code-reviewer 齐备、需求包门禁保留、make verify/docs-audit 全绿，且第二轮对抗挑刺真把删旧建新的连带漂移自查修平并记了 lesson——可收尾。
