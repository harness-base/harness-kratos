# 优化 / 自进化 log

> harness 自进化闭环的记录。两类来源：
> - `落文档`（①，capture）：Stop hook 机械触发（K 轮 / commit 边界 / 变更增量）→ headless Haiku 复查最近对话，捞"做了决策/学了东西却没写进文档"的，提醒落文档（`scripts/turn-backstop.sh`）。
> - `judgment`（②，自进化审查）：`self-evolution` 规范检查 / `self-optimize` 子 agent 的发现。
>
> 重要条目应**晋升到对应的家**：决策→ADR、踩坑→`lessons`、知识→就近 `AGENTS.md`/规则、用户偏好→memory。本 log 是中转站，不是终点。

---

## 2026-06-26 `judgment`（self-evolution 构建时捞到的缺口 + "都做"处理结果）
> ✅=已修　⏳=待办/立项。

- ✅ [docs] CURRENT_STATUS.md 漂移 → 已同步（规则/ADR/技能/scripts/prds/自进化行 + last_updated）。
- ✅ [skills] skills README 假命令 `make skills-index` → 改 `scripts/skills-index.sh` 头部为 `bash scripts/skills-index.sh`，重生成。
- ✅ [templates] ADR-0001/0003 "受影响 skill"栏 → 已补。
- ✅ [index-system] decisions/features 索引漂移 → 新写 `scripts/index-audit.sh`（双向一致性）进 verify；eval 索引本就由 `verify-eval-materials.sh` 守。
- ✅ [project-onboarding] verify 路由盲区 → 加"路由工程路径可达"检查进 verify（命令真能跑仍由各工程 e2e 负责）。
- ✅ [Codex 对等-部分] `self-optimize` 子 agent → 镜像 `.codex/agents/self-optimize.toml` + 注册 config。
- ✅ [process-coverage-部分] bugfix → 新建 `.agents/skills/bugfix/` skill。
- ✅ [① / 自身] ① 落文档提醒正名拆分（backstop/rule-0011/log/ADR-0005 订正）；老 self-optimize skill 退役；子 agent 对齐为②深审执行器。
- ⏳ [Codex 对等-剩余] Codex 原生 hooks（PreToolUse/Stop 等价）未接——`config.toml` 已 `hooks=true`，但需 Codex hook schema，研究性，立项。硬层(git/CI)已对等兜底。
- ⏳ [process-coverage-剩余] 迁移 / loop-engineering 流程——无具体案例，待真触发时按 plan 设计立项（别凭空造低质 skill）。
- ⏳ [templates-小] eval task-review 三联（candidate/decision/summary）无模板——低优先。
