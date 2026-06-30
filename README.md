# Agent Harness（harness-kratos）

一套通用、内容轻的 **agent 控制面（harness）**：用「最小内核 + 可挂载模块」治理 AI agent 在被管工程上做开发——把"怎么干活、谁来把关"沉淀成可执行的规则、技能、评分与机检，让 agent 的产出**可控、可验证、可追溯**。**不绑定语言 / 框架**；当前已挂载一个真实被管工程 `kratos-base`（Go / Kratos 微服务地基）。

**双运行时**：同一套控制面同时支持 **Claude Code** 与 **Codex**——子 agent / 技能在 `.claude/` 与 `.codex/` 双栈对齐、行为一致。

## 是什么 / 不是什么

- **是**：入口规则、任务状态、文档路由、规则库、验证路由、评分体系、可执行技能、hooks / CI —— 这套「脚手架」。
- **不是**：业务代码。被管工程的代码与测试在它自己的 `projects/<name>/` 里；控制面只做路由、执行、评分、收证据，**不存放业务测试本体**。

## 核心机制（凭什么靠谱）

- **规矩能机检**：密钥 / 危险命令由带测试的 `hook-policy` + git hooks 自动拦；删旧建新的引用漂移由各 `*-index --check` 进 `make verify` 防住——不靠自觉。
- **质量有评委**：L2+ 任务收尾由 **hc-eval 子 agent** 按 rubric 打分挑刺（用会话模型，**免 API key**），不靠 agent 自评。
- **挑刺有对手**：写代码 / 产需求走对抗式 review（`hc-code-reviewer` / `hc-prd-reviewer` 子 agent：多视角找问题 + 独立证伪 + 修复循环）。
- **文档能自检**：每篇文档头声明依赖，`docs-audit` 查"引用的文件 / 链接还在不在"。
- **agent 有错题本**：`tasks/lessons.md` 三段式记坑，反复出现升级成规则 / 技能。
- **按需加载**：按任务档位（L0–L6）+ 就近 `AGENTS.md` 决定读多少，不全量通读。
- **流程即技能**：流程写成可执行 skill（`.agents/skills/`），不是没人点开的说明文。

## 已落地

- **被管工程 `kratos-base`**：Go / Kratos 微服务地基（脚手架 + 配置中心 + 服务发现 + PG / Redis / MQ + 可观测 + 运行期弹性），验收点经多轮对抗评审硬化、全 PASS——详见 [`docs/context/CURRENT_STATUS.md`](docs/context/CURRENT_STATUS.md)。
- **技能集**：`hc-dev`（写代码统一入口）、`hc-test`（编排式产测试）、`hc-prd`（编排式产需求）、`hc-self-evolution` 等——清单以 [`.agents/skills/README.md`](.agents/skills/README.md) 自动索引为准。
- **子 agent**：`hc-eval`（评委）、`hc-code-reviewer` / `hc-prd-reviewer`（挑刺）、prd 编排 worker 等，双运行时——以 [`.claude/agents/README.md`](.claude/agents/README.md) 自动索引为准。
- **决策记录**：ADR 见 [`docs/decisions/`](docs/decisions/)（清单以 `index.yaml` 为准）。

## 结构

| 路径 | 职责 |
|---|---|
| `AGENTS.md` | 唯一权威入口：硬规则红线 + 启动顺序 + 验证命令 |
| `tasks/` | 当前任务（`todo.md`）+ 错题本（`lessons.md`）+ 归档 |
| `docs/context/` | 项目简报、真实状态、按需加载档位 |
| `docs/rules/` | 带编号的规则库（eval / ADR 按号引用） |
| `docs/decisions/` | ADR 架构决策记录 |
| `docs/eval/` | 评分体系：考题、rubric、评委、评审产出 |
| `docs/features/` · `docs/prds/` · `docs/test-cases/` | 需求工作包 / 需求产出 / 测试用例账本 |
| `docs/harness/` | 验证路由、CI、hooks、工程接入指南 |
| `.agents/skills/` | 自包含技能（流程即技能） |
| `.claude/` + `.codex/` | 双运行时 agent 配置：子 agent + hooks 接线 |
| `templates/` | feature / plan / ADR / doc / skill / eval-rubric 模板 |
| `scripts/` | 验证 / 文档自检 / eval / 索引 / hook 脚本（bash） |
| `workspace/verification.yaml` | 各被管工程怎么验证 |
| `projects/` | 被管工程挂载点（已挂 `kratos-base`） |

## 起步

```bash
make help          # 列出所有命令
make verify        # 控制面自检：结构 + 文档 + hook policy 测试 + 索引一致性
make docs-audit    # 文档自检：frontmatter 依赖文件在不在、链接通不通
make hooks         # 安装 git hooks（core.hooksPath → .githooks）
make eval          # 跑 task eval review（可选；CI / headless 用，需 EVAL_API_*）
```

- **eval 免 key**：默认用 hc-eval 子 agent（`.claude/agents/hc-eval.md` / `.codex/agents/hc-eval.toml`），用会话模型打分、无需 API key；`make eval` 是可选的 CI / headless 路径。
- **读多少**：看 [`docs/context/CONTEXT_LOADING.md`](docs/context/CONTEXT_LOADING.md)（L0–L6 档位 + 就近 `AGENTS.md`），默认少读、按需升档。

## 接入一个工程

把工程放进 `projects/<name>/`、写工程级 `AGENTS.md`、在 `workspace/verification.yaml` 填验证路由，即可纳管。详见 [`docs/harness/PROJECT_ONBOARDING.md`](docs/harness/PROJECT_ONBOARDING.md)。

## 索引

- 文档地图：[`docs/README.md`](docs/README.md)
- 当前真实状态：[`docs/context/CURRENT_STATUS.md`](docs/context/CURRENT_STATUS.md)
- 设计依据：[`docs/decisions/0001-harness-skeleton-design.md`](docs/decisions/0001-harness-skeleton-design.md)

## 参与贡献

欢迎 issue 与 PR。外部贡献走 **fork → 改 → 开 PR → review 合入**（直接 push 分支仅限协作者，`main` 受 PR + CI 保护）；动控制面 / 业务前先看 `AGENTS.md` 的硬规则红线，提交前过 `make verify`。

## 协议

[MIT](LICENSE) © 2026 harness-base
