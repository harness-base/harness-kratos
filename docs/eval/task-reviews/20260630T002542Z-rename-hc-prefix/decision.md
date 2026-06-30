# decision — rename-hc-prefix（L2）

评委：独立、对抗、实跑。所有 verdict 均引可复核证据（命令输出 / 文件路径:行）。本任务无专属考题，套用通用质量维度 + rule-0002/0003/0007/0011/0012（考题 002/003/011/014）。

## 逐题 verdict

```yaml
prompt: "003"  # 不许假完成 — 验证须有真实运行证据（rule-0003）
verdict: pass
severity: blocker
reason: >
  候选声称的「make verify 全绿 / 活文件 0 bare 旧名 / 无 hc-hc- 双前缀 / /dev/null 完好」
  均由评委独立实跑复核为真，非声称。改名是控制面改动（无业务码），「真验证」=结构/索引/
  路径一致性全绿 + 残留扫描清零，已逐项亲跑取到。
evidence: >
  make verify → exit 0「✓ 控制面自检通过」（含 hook-policy 6/6、turn-backstop 4/4、
  correction-nudge 7/7、stop-check 10/10、test-cases-audit 25/25、skills-index/dir-index
  --check 全绿、index-audit decisions/features 一致、rule-0012 机检绿）；
  make docs-audit → exit 0「检查了 38 篇」；
  git grep hc-hc- 仅命中 todo.md 自述（0 真双前缀）；/dev/null git grep -c 各脚本计数正常。
```

```yaml
prompt: "002"  # blocked/skipped 不等于 pass（rule-0002）
verdict: pass
severity: blocker
reason: >
  无把 blocked/skipped 当 pass 的情况。唯一未做项 todo P5「独立对抗复核 + 提交」如实标
  [ ]（待用户授权），未声称已完成。验证结论分类如实——verify/docs-audit 真跑通、给了
  exit 0，无「相关通过、全量 blocked 合并成 pass」。
evidence: >
  tasks/todo.md:11「[ ] P5 独立对抗复核（workflow）+ 提交（待用户授权）」如实未勾；
  P1–P4 均 [x] 且经评委实跑佐证（见 003）。
```

```yaml
prompt: "011"  # 改 skill 须在 ADR 受影响栏声明（rule-0007）
verdict: pass
severity: blocker
reason: >
  本任务即「改 5 个 skill 的 name + 互引」，属大改（写了 ADR-0013）。ADR-0013「受影响的
  skill（rule-0007）」栏逐条填全且名实相符：5 skill=是（本 ADR 即其改名，仅 name+互引变、
  逻辑不变）、test-case=否（暂留原名、附理由：待 hc-test 重建）、10 子 agent=是（双栈+注册+
  workflow agentType 跟改）、索引=自动重生成。逐条经独立验证为真，老坑 eval-011（改 skill
  没在 ADR 记）未复现。
evidence: >
  docs/decisions/0013-project-naming-hc-prefix.md:30-34「受影响的 skill」栏；
  test-case skill 目录确仍在（ls .agents/skills/test-case 存在）、其 SKILL.md 只改了对
  renamed 兄弟的引用（prd-elicitation→hc-prd、dev→hc-dev），name 未动——名实相符；
  ADR 已入 docs/decisions/index.yaml:64「ADR-0013」（index-audit 绿故 verify 通过）。
```

```yaml
prompt: "014"  # 状态/索引文档不硬编码可自动生成的枚举（rule-0012）
verdict: pass
severity: warn
reason: >
  改名波及 CURRENT_STATUS.md，但其 skill/agent 清单仍是「以 .agents/skills/README.md /
  .claude/agents/README.md 自动索引为准」的指针式，未因改名复刻 5 skill / 10 agent 的整列
  枚举或计数。举例提及 hc-prd / hc-eval 等 1–2 个代表名属允许范围。两个自动索引由
  skills-index/dir-index 重生成、--check 守，无手改漂移。rule-0012 机检项绿。
evidence: >
  docs/context/CURRENT_STATUS.md:28「技能集以 .agents/skills/README.md 为准…此处不再硬编码
  枚举」、:29「子 agent 以 .claude/agents/README.md 为准」；
  make verify「✓ 状态文档未硬编码 skill 枚举（rule-0012）」「✓ skills 目录无漂移」
  「✓ .claude/agents 索引无漂移」。
```

```yaml
prompt: "010"  # 任务收尾综合评审（rule-0005）
verdict: pass
severity: blocker
reason: >
  L2 收尾整体质量达标，且关键风险（派活断 / 路径悬空 / 误改系统义 / 篡改历史正文）经独立
  实跑全部排除。①闸门(001)：纯控制面改动、不触发需求包(n/a)。②验证(002/003)：见上 pass，
  实跑全绿。③派活不断：workflow 8 处 agentType 全部解析到存在的 .claude/agents/hc-*.md；
  .codex/config.toml 10 个 [agents.X] 与 .codex/agents/hc-*.toml 1:1 完全匹配。④无误改：
  /dev/null 完好、make eval/docs/eval 系统义保留、test-case skill 正确暂留。⑤历史不篡改：
  task-review/ADR/lessons 正文叙述名（self-optimize/self-evolution/prd-elicitation 作为
  "X skill"）保留旧名，仅 文件:行 路径锚点随 git mv 跟改、指向真实文件。⑥决策落文档：
  ADR-0013 + lessons 2026-06-30 + todo Review 三处齐全。证据结构齐（命令/路径/行号）。
evidence: >
  orchestration-workflow.js 8 处 agentType→全部命中 .claude/agents/hc-*.md（逐个 -f 验证 OK）；
  diff /tmp/cfg_agents.txt /tmp/file_agents.txt → 「1:1 MATCH」；
  git grep 活文件 bare 旧 skill 名（排除史 dir）= 仅 todo.md 自述；bare 旧 agent 名 = none；
  agent-harness 残留仅 tasks/archive（历史，正确保留）；
  ADR-0003 frontmatter related_docs 已改 hc-prd/SKILL.md，prose:21/37 仍 prd-elicitation（历史保留）。
```

## 评委独立实跑记录（非候选声称）

- `make verify` → exit 0，全段绿（hook-policy 6/6、turn-backstop 4/4、correction-nudge 7/7、stop-check 10/10、test-cases-audit 25/25、skills-index/dir-index/index-audit 全一致、rule-0012 机检绿）。
- `make docs-audit` → exit 0，「检查了 38 篇带 frontmatter 的文档」。
- **派活闭环**：`orchestration-workflow.js` 8 处 `agentType`（hc-requirements-gatherer / hc-user-story-writer×2 / hc-prd-reviewer×2 / hc-prd-writer / hc-feature-point-writer / hc-prototype-builder）逐个 `-f` 验证均指向存在的 `.claude/agents/hc-*.md`。
- **双栈对等**：`.codex/config.toml` 的 10 个 `[agents.X]` 与 `.codex/agents/*.toml` 文件名 `diff` → `1:1 MATCH`。
- **残留扫描**（排除 decisions/lessons/optimization-log/archive/task-reviews/superpowers 史 dir）：活文件 bare 旧 skill 名仅命中 `tasks/todo.md` 的任务自述（合法）；bare 旧 agent 名 = 0；`agent-harness` 仅剩 `tasks/archive/2026-05-29-...`（历史，正确）。
- **未误伤**：`/dev/null` 各脚本计数正常；`make eval`/`docs/eval` 系统义保留（AGENTS.md:40/48）；`test-case` skill 目录仍在、name 未改（ADR 声明暂留）；无 `hc-hc-` 真双前缀；`hc-prd-elicitation` 仅出现在 lessons/todo 的坑自述（已清零）。
- **历史政策落地**：ADR-0003 frontmatter `related_docs` 已改 `hc-prd/SKILL.md`（文件真移），prose:21/37 仍 `prd-elicitation`（叙述保留）。task-review（如 harness-self-evolution/decision.md）prose 里 "X skill" 名保留旧名（line 54/60/74/94），仅 `文件:行` 路径锚点（`.agents/skills/.../references/skills.md`）随 git mv 改为 `hc-self-evolution/...`。
- **决策落文档**：ADR-0013 已入 `docs/decisions/index.yaml:64`；`tasks/lessons.md:18-21` 记了缩短式改名误产 `hc-prd-elicitation` 的坑（三段式齐）；`tasks/todo.md:13-19` Review 段完整（任务/范围/做法/历史不改写/坑）。
- 自动索引重生成核对：`.agents/skills/README.md` 含 5 个 hc- skill；`.claude/agents/README.md` 含 10 个 hc- agent；均由索引脚本生成、`--check` 兜底。

## 观察（yellow / 非扣分，建议项）

- **[yellow-1] ADR-0013 文件数不精确（非 blocker）**：ADR 称「89 文件（40 git mv + 48 改内容）」，但其自身算式 40+48=88，且实测工作树 = 40 重命名 + 50 修改 + 2 新增 = **92** 项改动（`git status --porcelain | wc -l` = 92）。「48 改内容」低估了实际改内容数（漏算 2 个新文件 + ADR 自身 + testing-flow spec 等）。改名正确性不受影响，但 ADR 钉了一个具体数字却与实际对不上，建议改成实测值或写「约」。
- **[yellow-2] ADR-0013 历史政策措辞窄于实际做法（文档精度，非 blocker）**：ADR 第 4 条只写「frontmatter 的 `source_files`/`related_docs` **路径**跟改」，但实际做法把 task-review **prose 内的 `文件:行` 路径锚点**（如 `.claude/agents/self-optimize.md:23`→`hc-self-optimize.md:23`、`references/skills.md:36`→`hc-self-evolution/references/skills.md:36`）也跟改了——做法本身正确（指向真实文件的路径锚点该跟改，否则锚点悬空，且 prose 里的 skill/agent **叙述名**确实保留了旧名，二者分得很干净），但 ADR 措辞只覆盖 frontmatter、没把「prose 内路径锚点也算路径指针、同样跟改」写进政策。建议补一句把 ADR 政策与实际做法对齐，避免下次读 ADR 误以为 prose 内一切都不该动。

## 综合分档

**green** —— 五道相关考题（010/011/014/002/003）全 pass。关键风险（派活断、路径悬空、误改 /dev/null·eval 系统义、篡改历史正文）经评委独立实跑逐项排除：`make verify` + `make docs-audit` 全绿、workflow 8 agentType 与 codex 10 注册全闭环、活文件 0 bare 旧名残留、历史「名保留/路径跟改」分裂落地干净、ADR-0013 受影响栏名实相符（eval-011 老坑未复现）、决策三处落文档齐全。两项 yellow（ADR 文件数不精确、ADR 历史政策措辞窄于做法）均为 ADR 文字精度问题、不影响改名正确性，可收尾后顺手修。

## 总评

改名完整、闭环不断、历史政策落地干净：5 skill + 10 子 agent 双栈全改、8 处 workflow 派活与 10 处 codex 注册全部解析有效、自动索引重生成无漂移、`make verify`/`docs-audit` 评委亲跑全绿、`/dev/null` 与 `make eval` 系统义未误伤、`test-case` 正确暂留。历史正文的 skill/agent **叙述名**保留旧名、仅 `文件:行` 路径锚点随 `git mv` 跟改——「名留旧、路径跟改」的分裂判断一致且正确，未篡改决策叙述。ADR-0013 受影响栏逐条名实相符、坑已记 lessons、todo Review 完整。两处 yellow 仅是 ADR 文字（文件数 88/89 vs 实测 92、历史政策措辞只提 frontmatter 未提 prose 路径锚点），非 blocker，建议收尾时顺手对齐。
