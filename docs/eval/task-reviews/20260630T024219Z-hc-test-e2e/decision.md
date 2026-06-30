# 评审结论 — hc-test 测试编排 · e2e build（ADR-0014）

评委：hc-eval 子 agent（`.claude/agents/hc-eval.md` 路径，会话模型，免 key）
时间：2026-06-30T02:42:19Z ｜ 档位：L3+（ADR + skill + 双栈子 agent + 机检扩展 + 删并）
判据：考题 015 / 011 / 010（+ 002 / 003 / 009 / 012 综合套用）

所有 verdict 基于评委亲跑取证，未采信候选声称。

---

## 逐题 verdict

```yaml
prompt: "015"   # 测试用例产出标准——本批建的是 ENFORCE 该标准的 skill + 机检 + reviewer
verdict: pass
severity: blocker
reason: >
  双层防线名实相符、互不重叠且真生效。机检（结构层）矩阵硬闸亲测四象限全对、有守护测试 + 变异自证；
  reviewer（判断层）4 块约束（主观覆盖率/引用源符合理解偏差/用例质量/结构层不重复）逐条写进 agent 正文，
  覆盖 015 的 AC+FP 双轴/语义真覆盖/正常边界异常齐/covers 唯一真相源/不碰执行结果全部要点。
evidence: |
  ① 矩阵硬闸独立实测（TC_DIR= 隔离真账本，伪账本造于 scratchpad）：
     - 失败格空着 → EXIT=1 红（报「失败格空着/占位、又没写无·理由」）
     - 三格全填(TC/无·理由) → EXIT=0 绿
     - 格留占位 <TC-NN 或 无·理由:…> → EXIT=1 红（占位当未填，fail-closed）
     - 无矩阵段(api/旧风格) → EXIT=0 绿（不误伤，文件级跳过）
  ② 守护测试真存在 + 变异自证（rule-0009）：scripts/test-cases-audit.test.sh #25–#28 覆盖上述四象限，
     与正向锚（#26）仅差一格；评委 neuter 矩阵报错行（print "X …"）后复跑 → pass=27 fail=2
     （#25 失败格空着、#28 占位符 翻红），还原后恢复 pass=29 fail=0 → 非虚构保证。
  ③ test-cases-audit.test 全过：make verify 内 pass=29 fail=0；独立复跑 EXIT=0。
  ④ covers 唯一真相源：脚本以 covers 行解析覆盖关系，无另存手维护映射表；模板/总纲均标「covers 为准」。
  ⑤ 不碰执行结果：skill ①/⑥、qa 正文「只写不跑(rule-0014)」、模板抬头均显式划清「不管过没过」。
  ⑥ reviewer 语义盲区（AC「5 次锁定」测成「3 次」）作为典型偏差写进 .md/.toml 正文 ②块——补机检盲区到位。
```

```yaml
prompt: "011"   # 改架构/接口须回顾 skill（rule-0007）
verdict: pass
severity: warn
reason: >
  属大改（写了 ADR-0014 + 新建 skill + 双栈子 agent + 扩机检 + 删并）。ADR-0014「受影响的 skill」栏
  逐条列名实相符：test-case=删（职责并入）、hc-test=新建、hc-e2e-qa/reviewer=新增双栈、
  test-cases-audit=扩、rule-0014/考题015=不变。栏目齐、需更新的更新、不需要的写了说明。
  唯一瑕疵：ADR 多处宣告「ADR-0008 superseded」但 0008 自身 frontmatter + index.yaml 仍 status: accepted
  ——状态未落地（见 yellow，warn 级、非 011 判失败口径，故不翻 fail）。
evidence: |
  - ADR-0014:38-45 受影响栏逐条对照实物全部命中（评委逐项核验）。
  - skills/README.md 索引仅 hc-test、无 test-case 残留；触发词不与 hc-test 双命中。
  - 演进栏（SKILL.md ⑦）写明：编排/场景/worker/防线/门禁变化时连同双栈 + 模板 + 机检一并回顾。
  - 瑕疵：templates/adr.md:3 规定 status 含 superseded 合法值；ADR-0008 frontmatter status: accepted、
    index.yaml status: accepted（未跟 ADR-0014 宣告改为 superseded）。
```

```yaml
prompt: "010"   # 收尾综合评审（rule-0005）
verdict: pass
severity: blocker
reason: >
  核心交付（双栈 e2e 线 + 双层防线 + 机检硬闸 + 守护测试 + ADR/总纲）完整且亲验全绿；
  闸门(001)立了 ADR、验证(002/003)结论如实有真实运行证据、断言(009/012)锚定产出方信号且有守护测试 + 变异自证、
  档位读取合理、skill 回顾(011)到位。两项 yellow 为删 skill 后的活引用残留 + ADR 状态未落地，均 warn 级文档卫生，
  不阻断收尾。
evidence: |
  - make verify EXIT=0「✓ 控制面自检通过」（评委亲跑）；docs-audit 绿（40 篇带 frontmatter 文档）。
  - 全部子自测绿：hook-policy 6/6、turn-backstop 4/4、correction-nudge 7/7、lessons-promote 3/3、
    stop-check 10/10、test-cases-audit.test 29/29。
  - skills 目录无漂移、rules 索引无漂移、.claude/agents 目录索引无漂移、decisions/features 索引一致。
  - 双栈齐：.md(无 model=会话模型) + .toml(model_reasoning_effort=high，非指定模型/非 haiku，与其余 9 个风格一致)
    + config.toml 注册 [agents.hc-e2e-qa]/[agents.hc-e2e-reviewer]；qa 有 Write、reviewer 无 Write。
  - 约束写进 agent 上下文正文（非只靠模板）：qa .md/.toml「怎么写」7 条 + reviewer 4 块均在 system prompt 正文逐条载。
  - 真账本未污染：docs/test-cases/index.yaml 仍空账本(test-cases: [])，实测伪账本全在 scratchpad、TC_DIR= 隔离。
```

```yaml
prompt: "002/003"   # blocked≠pass / 不许假完成（综合套用）
verdict: pass
severity: blocker
reason: 候选未声称用例已跑/已通过；机检 + 守护测试结论均评委亲跑复现，无假完成。
evidence: make verify / test-cases-audit.test / 矩阵四象限实测 / neuter 变异 全部评委本机复跑取证。
```

```yaml
prompt: "009"   # 验收断言锚定唯一真实证据 + 守护测试（rule-0009）
verdict: pass
severity: blocker
reason: >
  矩阵硬闸（本批 BLOCKER 修复点）的保证有守护测试背书，且 neuter 报错行 → 对应守护用例翻红，
  证明断言锚定真实信号、非牵强通过。qa/reviewer 正文均要求「预期锚唯一真实信号、不靠脆弱 toast」。
evidence: neuter print "X …" → test #25/#28 翻红(pass=27 fail=2)，还原恢复 29/29；模板 TC-1/2/3 示例预期锚 URL/DOM/落库。
```

---

## 综合分档：green（带 2 项 yellow，可收尾）

核心交付完整、亲验全绿、本批 BLOCKER 修复点（矩阵硬闸）真生效且有变异自证背书，无 blocker 级 fail。两项 yellow 均为删 test-case skill 后的收尾卫生瑕疵，不阻断收尾，建议后续小批补齐。

### yellow（warn 级，非 blocker）

- **删 skill 后留 2 处指向已删 `test-case` skill 的活引用残留**（候选声称「修 9 处引用」未扫净）：
  1. `templates/test-case.md:4` — 正文引用 `.agents/skills/test-case/SKILL.md`（该 skill 已删 → 悬空指针）。该模板无 frontmatter，docs-audit 的依赖链查不到、机检漏网。
  2. `docs/test-cases/index.yaml:1` — 注释「由 test-case skill 产出」仍指已删 skill。
  - 影响：文档卫生 / 引用一致性，不影响 e2e 核心功能；但与 rule-0011「决策落文档不漂移」精神相悖。修法：旧通用模板 `templates/test-case.md` 的 skill 指针改指 `hc-test/SKILL.md`（或随其定位决定保留/退役）、index.yaml 注释改「由 hc-test 产出」。

- **ADR-0008 superseded 状态未落地**：ADR-0014 背景/决策第 2 条/受影响栏/影响栏多处宣告「ADR-0008 被本 ADR 取代（superseded）」，但 `docs/decisions/0008-test-case-skill.md` frontmatter `status: accepted`、`docs/decisions/index.yaml` 内 0008 亦 `status: accepted`，均未改为 `superseded`。`templates/adr.md:3` 明确 `superseded` 是合法状态值，全仓亦无 superseded 落地先例。
  - 影响：名实不符（ADR 文字宣告 vs 实际状态），warn 级；非 011 判失败口径（受影响栏已填且需要的已更新）。修法：把 0008 自身 frontmatter + index.yaml status 改 `superseded`（本身是状态标记、不属「历史正文改写」）。

### 给用户的一句话提示

e2e 这一期 build 质量过硬、可收尾：双栈子 agent 齐、约束逐条写进 agent 正文、矩阵硬闸四象限实测全对且有守护测试 + 变异自证背书（评委亲 neuter 验证非虚构）、make verify 全绿。收尾前建议顺手扫掉两处删 skill 残留（`templates/test-case.md:4` + `docs/test-cases/index.yaml:1` 指向已删 test-case skill）、把 ADR-0008 status 落成 superseded——都是 warn 级文档卫生、不阻断。
