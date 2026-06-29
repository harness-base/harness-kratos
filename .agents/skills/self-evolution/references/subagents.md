# subagents 审查手册

子 agent = 独立、隔离上下文、可并行的执行体（评分 / 审查 / 重判断）。本维度审「该独立的有没有独立、定义对不对、跟 skill 分工乱没乱、Codex 侧对不对等」。

## 规范（健康长什么样 / 不变量）

- **独立任务用子 agent**：评分 / 审查 / 重判断这类「与主线隔离、可并行、重上下文」的任务，拆给子 agent；让主 agent 上下文保持干净（AGENTS.md「工作方式」）。
- **定义齐全**：每个 `.claude/agents/<name>.md` 有 frontmatter `name` / `description` / `tools`（最小工具范围），正文给清晰 system prompt（角色 + 步骤 + 原则 + 产出落点）。
- **免 key**：用当前会话模型，不依赖 `EVAL_API_*` 等外部 key（`run-eval.sh` 那种 curl 外部 LLM 是**可选** CI 路径，不是默认）。
- **职责与 skill 分清**：`skill = 流程`（主 agent 读了照着走），`subagent = 独立执行体`（被 spawn 去干一件隔离的事）。skill 与 subagent 是**入口 vs 引擎**关系，不是两份重复逻辑——如 `self-evolution` skill（深审入口、列总览索引、晋升归档）调 `self-optimize` 子 agent（多维判断引擎）。
- **软触发 vs 硬触发分清**：子 agent 靠主 agent 记得 spawn = **软**；必须机械可靠的兜底走 hook headless（`claude -p`）= **硬**。别把「忘了就失效」的东西做成子 agent（ADR-0005 决策 4）。
- **进自动索引防漂移**：`.claude/agents/README.md` 由 `dir-index.sh` 生成、禁手改，`--check` 进 `make verify`。
- **Codex 对等**：双运行时的子 agent 在 `.codex/agents/*.toml` 有等价定义并在 `.codex/config.toml` 注册，行为与 `.claude/agents/` 一致。

## 怎么检索现状（命令可直接跑）

```bash
# 现有子 agent 定义（Claude 侧）
ls .claude/agents/ ; cat .claude/agents/eval.md .claude/agents/self-optimize.md .claude/agents/code-reviewer.md

# Codex 侧对等定义 + 注册
ls .codex/agents/ ; sed -n '/\[agents/,$p' .codex/config.toml

# 谁 spawn 它们 / 何时 spawn（事实源）
grep -rn "子 agent\|子代理\|subagent\|spawn" AGENTS.md docs/ scripts/ .agents/skills/

# 自动索引现状 + 是否漂移（与 verify 同一入口）
cat .claude/agents/README.md
bash scripts/dir-index.sh .claude/agents --check

# 兜底走的是 headless（不是子 agent）——确认触发是机械的
grep -n "claude -p\|MODEL\|HARNESS_TRIAGE" scripts/turn-backstop.sh
```

事实锚点（核对过）：现有三个 `.claude/agents/`：`eval.md`（rule-0005 收尾评委）、`self-optimize.md`（self-evolution 的 ② 深审执行器）、`code-reviewer.md`（`dev` skill 的挑刺引擎，对抗式 review；ADR-0009）；前二者 `tools: Read, Glob, Grep, Write, Bash`，`code-reviewer` 是 `tools: Read, Glob, Grep, Bash`（**无 Write，只评不改**）；均免 key。每轮兜底 `scripts/turn-backstop.sh` 跑的是 **headless Haiku**（`claude -p --model`），**不是** spawn 子 agent——重判断子 agent 由主 agent / `self-evolution` / `dev` skill 按需 spawn（`dev` 在 workflow 里 `agentType:'code-reviewer'`）。

## 怎么判（逐条可判定）

- **该独立却塞主 agent**：评分 / 审查 / 重判断仍在主 agent 上下文里硬做（污染上下文、不可并行、自评失独立性）→ 缺口，抽成子 agent。
- **定义过期**：`<name>.md` 的步骤 / 产出落点 / `tools` 与现实脱节（如步骤引用已改名的文件、`tools` 给多了或漏了 Write）→ 缺口。
- **与 skill 重叠空转**：skill 正文把子 agent 的判断逻辑又抄一遍（两份会漂），或 skill 与子 agent 边界含糊到不知道该读哪份 → 漏洞，收敛成「skill=入口/流程，subagent=引擎」单一事实源。
- **软硬错配**：把「漏了就失效」的兜底做成靠主 agent 记得 spawn 的子 agent（应走 hook headless）；或反过来把需要会话模型/隔离上下文的重判断硬塞进 headless → 缺口。
- **索引漂移 / 悬空**：`dir-index.sh .claude/agents --check` 报漂移；AGENTS.md「已有子代理：…」清单与 `.claude/agents/` 实际不符 → 缺口。
- **Codex 不对等**：`.claude/agents/` 每个子 agent 要在 `.codex/agents/*.toml` 有等价定义并在 `config.toml` 注册，否则 Codex 侧裸奔。当前 `eval`、`self-optimize`、`code-reviewer` 均已对等（`.codex/agents/{eval,self-optimize,code-reviewer}.toml` 且在 `.codex/config.toml` 注册 `[agents.<name>]`）；新增子 agent 须同步补 `.codex/` 的 toml **并注册**（漏注册 = Codex 侧调不到）。

## 常见漏洞模式（本仓真实案例）

- **职责与 skill 重叠**：早期 `self-optimize` 把 skill 和子 agent 混在一起——靠 ADR-0005 拆清为 `self-evolution` skill（入口）+ `self-optimize` 子 agent（引擎），老 self-optimize skill 已退役。维度种子点名的本仓老毛病。
- **triage 误做成子 agent**：ADR-0005 决策 4（`docs/decisions/0005-self-evolution-loop.md:41`）否决「兜底 triage 做成子 agent」——子 agent 要主 agent 记得 spawn=软，会忘记记的 agent 同样会忘记触发；故兜底走 hook headless 才硬，self-optimize 重判断仍是子 agent。
- **prompt 不自包含 → 流式超时 / 0 产出**：`tasks/lessons.md` 2026-06-02「子代理长任务流式超时」——prompt 不自包含导致先读一堆文件长时间静默触发 idle timeout；早期信号 = 子代理 tool_uses 不少但 0 token 输出、目标目录无产物。教训：派发前 prompt 自包含（接口/语义/测试全给，明示「别读文件直接写」）、预热依赖缓存。
- **子代理掩盖 CWD 假设**：`tasks/lessons.md` 2026-06-02「e2e 脚本隐含 CWD 假设」——子代理在工程目录跑全过，掩盖了「从 harness 根调用即全挂」的坏路由。子 agent 的工作目录假设要和真实调用点对齐。
- **eval 子 agent 抓出主线假收敛**：`tasks/lessons.md` 2026-06-26 + `docs/eval/task-reviews/20260626T014408Z-harness-rules-distribution/`——eval 子 agent 逐条 `git show HEAD` 对比，抓出主线「无损迁移/全保留」实为偷改、判 yellow。正面案例：独立子 agent 的隔离视角抓出主 agent 自评抓不到的。

## 修复用哪个操作 skill / 脚本

- 加 / 改子 agent：直接写 `.claude/agents/<name>.md`（frontmatter `name`/`description`/`tools` + system prompt）；Codex 对等同步 `.codex/agents/<name>.toml` 并在 `.codex/config.toml` `[agents.<name>]` 注册。
- 重生成索引：`scripts/dir-index.sh .claude/agents`（改完跑，禁手改 README）；`make verify` 收口（含 `--check`）。
- 职责与 skill 重叠 → 改对应 `.agents/skills/<name>/SKILL.md`，把判断逻辑留在子 agent、skill 只留入口/流程，并按 rule-0007 回顾。
- 软硬错配（该机械兜底的）→ 调 `scripts/turn-backstop.sh` / `scripts/stop-check.sh`（hook headless），别塞子 agent。
