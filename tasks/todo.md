# 当前任务

> 只记手头这一件事；干完清空、旧的 roll 进 `archive/`。保持轻。
> 元：level: L3 ｜ task: hc-test-e2e（上一个 rename-hc-prefix 完成·待提交）

## 当前：hc-test 测试编排（ADR-0014；总监 + hc-e2e-qa/hc-e2e-reviewer，分阶段实现·整体发布）
- [x] 设计敲定：总监(默认 A + 用户覆盖) / hc-e2e-qa(输入优先级 AC>FP>US>PRD + 模板 + 覆盖矩阵) / hc-e2e-reviewer(主观覆盖率 + 引用源符合/理解偏差 + 质量) / 机检矩阵带"无·理由"逃生口 / 干掉 test-case / 会话模型(非 haiku)
- [x] 落文档：流程总纲 `docs/harness/testing-flow.md` + ADR-0014 + 登记 index + dir-index regen + `make verify` 绿
- [x] 建：hc-test skill 总谱 + `hc-e2e-qa`/`hc-e2e-reviewer` 双栈子 agent + `templates/e2e-test-case.md` + 扩 `test-cases-audit`(矩阵硬闸 + 逃生口 + 守护测试 29/0) + 删 test-case skill + 修引用 + 索引 regen（workflow 并行建 + 对抗复核）
- [x] 收尾：`make verify` 绿 + hc-eval **green**（015/011/010 pass）+ 补 eval 2 yellow（悬空指针 / 0008 superseded）
- [ ] 提交（待用户授权）
- 占位（本期不建、待前置）：hc-api-qa/hc-api-reviewer(待研发方案)、hc-script-impl/hc-script-reviewer(待 sandbox)、回归

## Review（hc-test）
- **任务**：测试做成编排式（ADR-0014）——hc-test 总监 + e2e 这一期（`hc-e2e-qa` 写 / `hc-e2e-reviewer` 审）+ 矩阵硬闸；干掉 test-case 并入。**分阶段实现、整体发布**。
- **做法**：流程总纲落 `docs/harness/testing-flow.md`（唯一真相源，skill/agent/ADR 引它不复制）；要求写进子 agent 上下文（非只靠模板）；两层覆盖防线——机检 `test-cases-audit` 扩矩阵完整性（带"无·理由"逃生口防误伤）查**结构** + `hc-e2e-reviewer` 4 块查**判断**（主观覆盖率 / 引用源符合·理解偏差 / 质量），不重叠。
- **质量**：workflow 并行建 + 对抗复核抓到 **BLOCKER**（矩阵闸当时只是文档承诺、脚本没扩 = rule-0009 虚构保证）→ 真建出来；建时又自照出 byte-awk 多字节误伤 bug → 修。矩阵闸 **29/0 + 变异自证 load-bearing**。
- **验证**：`make verify` + `docs-audit` 全绿；**hc-eval green**（考题 015/011/010 + 002/003/009 全 pass，亲测矩阵四象限），产物 `docs/eval/task-reviews/20260630T024219Z-hc-test-e2e/`；eval 2 yellow（悬空指针漏扫 / ADR-0008 未落 superseded）已补。
- **坑（已记 lessons 2026-06-30）**：① 要求得进 agent 上下文（非只靠模板）；② byte-awk 多字节进字符类误撞中文；③ 删 skill 后"全修无悬空"过度声称（排除了保留文件没扫其内容）。
- **占位**：api / 脚本 / 回归 待前置；整体发布。

## 完成·待提交：存量改名 hc- 前缀（项目定名 harness-control；5 skill + 10 子 agent 全改名）
- [x] P1 5 个 skill 目录改名 + frontmatter name + skills-index（prd-elicitation→**hc-prd** 缩短，其余加前缀）
- [x] P2 10 个子 agent 改名（.claude .md + .codex .toml + config.toml 注册 + dir-index）
- [x] P3 改活引用：workflow `agentType` 派活 / frontmatter `source_files`·`related_docs` 路径 / prose 互引（dev·eval·self-evolution 语义词逐处判）/ 顺带修被否旧名 `agent-harness→harness-control`
- [x] P4 make verify 绿 + 全量复扫 0 活残留 + lessons 记坑
- [x] P5 独立对抗复核：审计 workflow(5 面)3 绿 + hc-eval 评审 **green**；据结果修漏改(subagents.md `eval`→`hc-eval`)+ ADR-0013 两处 yellow(文件数 / §4 措辞补"正文路径锚点也跟改")
- [ ] P6 提交（待用户授权）—— commit + push PR #6，不加 Co-Authored-By

## Review
- **任务**：项目定名 `harness-control`、统一前缀 `hc-`；存量 5 skill（prd-elicitation→**hc-prd** 缩短、其余加前缀）+ 10 子 agent 全改名。用户选 **A**（全量改名，接受历史不改写 = 活引用 hc- / 历史旧名分裂）。
- **范围**：89 文件（40 git mv 重命名 + 48 改内容）。分 4 段、每段 verify。
- **做法**：改名靠 `git mv` + 锚定 sed。活引用分三类——① workflow `agentType` 派活（不改就断）② frontmatter 路径指针（不改 docs-audit 红）③ prose 互引（一致性）。distinct token 用 `[^-]` 守护 sed 防双前缀；`dev`/`eval`/`self-evolution` 是语义词（`make eval`·`docs/eval`·`/dev/null`·自进化闭环），逐处判、不盲 sed。
- **历史不改写**：ADR / lessons / eval-reviews / specs-plans **正文**保留旧名；但其 frontmatter `source_files`·`related_docs` **路径**指针跟改（文件真移了，不改 docs-audit 红）——改名元数据、非改决策正文。
- **验证**：`make verify` 全绿；全量复扫活文件 0 个 bare 旧名（唯一"残留" = ADR 文件名 `0003-prd-elicitation-and-prototype.md`，正确）；`/dev/null` 完好；无 `hc-hc-` 双前缀；agent registry 已重载认 hc- 名。
- **坑（已记 lessons 2026-06-30）**：缩短式改名（prd-elicitation→hc-prd）≠ 加前缀，blanket 前缀-sed 误产 `hc-prd-elicitation`（自查 grep 新名发现并修复清零）。
- **独立复核**：审计 workflow 5 面（派活/frontmatter 118 条/漏网/改过头/冒烟）——3 面零 finding，捞 1 漏改(已修)；**hc-eval 评审 green**（考题 010/011/014/002/003 全 pass，评委亲跑 verify/docs-audit 证真绿），产物 `docs/eval/task-reviews/20260630T002542Z-rename-hc-prefix/`。两审对"历史正文路径锚点"判断分歧——采纳评委：**名是叙述(留旧)、路径是指针(跟文件走)**，ADR-0013 §4 已据此补准。

## 已闭（已提交，下次清理滚 archive）
- doc-sync-redesign（L3，932ecef，ADR-0012，eval green）；demote-context-loading（L3，PR #6，eval green）；prd-orchestration（L4，PR #3/#5）；dev-skill（L4，7b6576d）；test-case-skill（L3，c0c94f6）。
