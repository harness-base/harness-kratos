# decision — prd-orchestration 收尾 eval

- task: prd-orchestration ｜ level: L4
- candidate: 提交 5e22c2d..7630519（6 commit：ADR-0010 + SKILL 重写 + 6 worker 双栈 + Workflow 模板 + doc-sync + 对抗挑刺修平）
- 评委：独立、对抗、只看证据。能跑则跑（grep 双栈、make verify/docs-audit、读文件核对）。

## 011 — 架构变更是否同步 skill（rule-0007）

```yaml
prompt: "011"
verdict: pass
severity: warn
reason: 本次属大改（写 ADR-0010 + 重写 prd-elicitation SKILL 到 version 3 + 新增 6 worker 双栈 subagent），ADR「受影响 skill」栏（line 34-38）逐条填实，需更新的已更新、不需要的写了"否"+理由。
evidence: >
  ADR-0010 line 34-38：prd-elicitation=是（即本 ADR 的编排重写，SKILL→总谱/version→3）；
  test-case/dev/feature=否（下游松耦合不变）；其余 skill=否（与产出需求无关）；
  子 agent 行新增 6 worker（双栈）+ 外部调研走 deep-research。
  SKILL.md frontmatter version: 3 / last_reviewed: 2026-06-29，正文 line 61-62「演进(rule-0007)」段
  写明编排/worker/权重/review 变化时回顾本 skill 连同 6 worker + 2 references，并要求跑 skills-index。
  make verify「skills 目录无漂移」「.claude/agents 索引无漂移」均 ✓。
```

## 013 — PRD 标准是否忠实编码（rule-0010；本任务无实际 PRD，改判 SKILL 总谱 + prd-reviewer + 门禁）

```yaml
prompt: "013"
verdict: pass
severity: blocker
reason: 本任务产出的是"产 PRD 的编排 skill"而非 PRD 文件；SKILL 总谱 + prd-reviewer 子 agent + Workflow 模板 + 门禁段把 rule-0010/013 全部要点忠实编码，无遗漏。
evidence: >
  用户故事=上游对齐锚：SKILL line 54「用户故事=独立产物+对齐锚：先于 PRD、approved 才进；验收可观测可验证」；
    workflow.js line 71 确认门（user approved user-stories 才往下）；prd-reviewer 轻审模式四项含"对齐采集需求"。
  验收可观测：prd-reviewer line 11「AC 可观测可验证（不是'做完了/支持X'这类不可判定措辞）」；workflow.js line 76 prd-writer 提示含"验收可观测"。
  范围 in+out 闭合：prd-reviewer line 13「范围 in+out 闭合」；workflow.js line 76 prd 提示含"范围闭合"。
  四态：prd-reviewer line 13「每页四态齐（空/加载/错误/成功）+边界」；SKILL line 45 / workflow.js line 94 重审含四态。
  覆盖映射：SKILL line 55「PRD 带功能点清单+覆盖映射」；feature-point-writer 产 US↔FP↔正文 三级双向映射；prd-reviewer line 13「US↔FP↔正文 映射齐无孤儿」。
  原型可点：SKILL line 55「原型能点通主流程、与现有前端一致」；prd-reviewer line 13「原型真可点通非静态图」；
    workflow.js line 80-89 原型 opt-out（用户未要则不派 prototype-builder + 审稿不把"缺原型"当缺口，line 86 兑现"已留痕跳过不算缺口"）。
  假设确认：SKILL line 17「不静默假设」（贯穿）；prd-reviewer line 13「假设显式确认」。
  登记不漂移：SKILL line 58-59 门禁段「结构（登记一致 + prd.md 必备章节）由 prds-audit 机检进 make verify」；
    make verify「PRD 账本一致」✓。
  prd-reviewer 与 code-reviewer 分开（rubric=eval 013/rule-0010），双栈行为一致（.md 与 .toml developer_instructions 逐条对照一致，且 toml 末句声明"与 .md 行为一致"）。
```

## 014 — 状态/索引文档不硬编码可自生成枚举（rule-0012）

```yaml
prompt: "014"
verdict: pass
severity: warn
reason: 本次改了 CURRENT_STATUS.md（.agents/skills 行 line 28），保持"以 .agents/skills/README.md 为准（skills-index 自动生成 + --check 防漂）"的指针化，仅举 prd-elicitation 一例做说明，未复刻整列 skill 枚举/计数。
evidence: >
  CURRENT_STATUS line 28：「技能集以 .agents/skills/README.md 为准（skills-index 从各 SKILL.md 自动生成、--check 进 make verify 防漂移，故此处不再硬编码枚举）；含 prd-elicitation（编排式…）、self-evolution… 等」——指针 + 1-2 举例，符合考题"举 1-2 例可以"。
  make verify「状态文档不硬编码可自生成枚举（rule-0012）」→ ✓ 状态文档未硬编码 skill 枚举（机检 --check 守住）。
  .claude/agents 新增 6 worker 由 dir-index 自动生成 README，make verify「.claude/agents 索引无漂移」✓；decisions 索引一致 ✓。
```

## 010 — 收尾综合评审（rule-0005）

```yaml
prompt: "010"
verdict: pass
severity: blocker
reason: L4 任务收尾，按 010 检查清单逐项核：闸门/验证/断言/档位/skill/证据结构均达标；机检（make verify 全绿 + docs-audit 通过）+ 双栈一致性人工核对齐全。
evidence: >
  001 闸门：纯控制面/skill/文档/agent 改动，不触发"改业务代码前立需求包"——n/a 合理（无业务码改动，diff 全在 .agents/.claude/.codex/docs/tasks）。
  002/003 验证如实：make verify「✓ 控制面自检通过」（含 test-cases-audit pass=25/fail=0、skills/rules/dir/decisions/features/PRD 索引全 ✓）；
    make docs-audit「✓ 通过（30 篇带 frontmatter 文档）」；双栈 6 worker 各有 .md+.toml+config 注册+README 登记（脚本核对全 Y）。无假完成。
  004 档位：todo.md 标 level: L4，与任务规模（重构 skill + 6 子 agent + ADR）相符。
  011 skill 回顾：见上，pass。
  012 断言锚定：本任务无 e2e/访问日志类断言，核心断言（双栈对齐/索引无漂移）锚定 make verify 机检产出方信号，非自报。
  证据结构：commit 分 T1-T6、todo 勾稽、ADR/spec/plan 三联齐、对抗挑刺 11 agent/3 视角修平 4 类（deep-research 措辞/原型 opt-out/重审轮数上限 MAX_ROUNDS=4/Workflow 注释）均落到产物可复核。
overall: green
```

## 综合分档

green —— 4 道相关考题全 pass，机检（make verify / docs-audit）全绿，双栈一致性人工复核齐全，对抗挑刺修平点（原型 opt-out、MAX_ROUNDS 上限、Workflow 注释、deep-research 措辞）均在产物中落实可复核。可收尾。

## warn 级提示（不阻断）

1. deep-research 措辞残留不一致：T6 已把多数处如实化为"走可用的 deep-research skill（可用的 research skill，通用 subagent 调 Skill 工具跑，不另建 subagent）"，
   但 ADR-0010 决策点 3（line 26）与 spec 表格仍保留加粗"复用 `deep-research`"——强度略高于"可用的 research skill"。
   deep-research 在本仓不存在（.agents/skills/ 无该目录、skills README 索引无、docs-audit 不把它当本仓必存文件——三处证据一致，定位为运行时 plugin skill 而非悬空引用本仓资产），
   语义整体正确、不构成 blocker；建议把那两处加粗也统一成"走可用的 deep-research skill"，消除"已有 repo 资产可复用"的误读余地。

## 给用户的一句话

prd-orchestration（L4）收尾 eval 判 green，可收尾；唯一待办是把 ADR/spec 里两处加粗"复用 deep-research"统一成"走可用的 deep-research skill"，与已如实化的其余处对齐（warn，不阻断）。
