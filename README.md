# Agent Harness

一套通用、内容轻的 **agent 控制面（harness）**：用「最小内核 + 可挂载模块」治理 AI agent 在被管工程上的开发。控制面先行，被管工程以后挂进 `projects/`。**不绑定语言 / 框架，工程进来即用。**

## 是什么 / 不是什么

- **是**：入口规则、任务状态、文档路由、规则库、验证路由、评分体系、hooks / CI —— 这套「怎么干活、谁来把关」的脚手架。
- **不是**：业务代码。被管工程的代码与测试都在它自己的 `projects/<name>/` 里；控制面只做路由、执行、评分、收证据，**不存放业务测试本体**。

## 核心机制（它凭什么靠谱）

- **规矩能机检**：密钥 / 危险命令等由带测试的 `hook-policy` + git hooks 自动拦，不靠自觉。
- **质量有评委**：L2+ 任务收尾由 **eval 子 agent** 按 rubric 打分挑刺（用会话模型，**免 API key**），不靠 agent 自评。
- **文档能自检**：每篇文档头声明依赖，`docs-audit` 查"引用的文件 / 链接还在不在"。
- **agent 有错题本**：`tasks/lessons.md` 三段式记坑，反复出现升级成规则 / skill。
- **按需加载**：按任务档位（L0-L6）+ 就近 `AGENTS.md` 决定读多少，不全量通读。
- **流程即技能**：流程写成可执行 skill（`.agents/skills/`），不是没人点开的说明文。

## 结构

| 路径 | 职责 |
|---|---|
| `AGENTS.md` | 唯一权威入口：硬规则红线 + 启动顺序 + 验证命令 |
| `tasks/` | 当前任务（`todo.md` 轻量）+ 错题本（`lessons.md`）+ 归档 |
| `docs/context/` | 项目简报、真实状态、按需加载档位 |
| `docs/rules/` | 带编号的规则库（eval / ADR 按号引用） |
| `docs/decisions/` | ADR 架构决策记录 |
| `docs/eval/` | 评分体系：考题、rubric、评委、评审产出 |
| `docs/harness/` | 验证路由、CI、hooks、**工程接入指南** |
| `docs/features/` | 需求 / 工作包（随被管工程填；已挂 kratos-base 的需求包） |
| `.agents/skills/` | 自包含技能（流程即技能） |
| `.claude/` + `.codex/` | agent 配置：eval 子 agent（双运行时）+ hooks 接线 |
| `templates/` | feature / plan / ADR / doc / skill / eval-rubric 模板 |
| `scripts/` | 验证 / 文档自检 / eval / 装 hook 脚本（bash） |
| `.githooks/` + `.github/` | git hooks（提交 / 推送拦截）+ CI |
| `workspace/verification.yaml` | 各被管工程怎么验证 |
| `projects/` | 被管工程挂载点（已挂 kratos-base） |

## 起步

```bash
make help          # 看所有命令
make verify        # 控制面自检（结构 + 文档 + hook + eval 资产 + skills）
make hooks         # 安装 git hooks（需先 git init）
```

- **eval 免 key**：默认用 eval 子 agent（`.claude/agents/eval.md` / `.codex/agents/eval.toml`）；CI / headless 可选 `make eval`（需 `EVAL_API_*`）。

## 接入一个工程

照 **[`docs/harness/PROJECT_ONBOARDING.md`](docs/harness/PROJECT_ONBOARDING.md)** 走：放代码 → 写工程 `AGENTS.md` → 填验证路由 → 立需求包 → 加工程级规则。

## 索引

- 文档地图：`docs/README.md`
- 当前真实状态：`docs/context/CURRENT_STATUS.md`
- 设计依据：`docs/decisions/0001-harness-skeleton-design.md`

## 参与贡献

欢迎 issue 与 PR。外部贡献走 **fork → 改 → 开 PR → review 合入**（直接 push 分支仅限协作者）；动控制面 / 业务前先看 `AGENTS.md` 的硬规则红线，提交前过 `make verify`。

## 协议

[MIT](LICENSE) © 2026 harness-base
