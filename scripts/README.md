# scripts/

控制面的全部 shell 脚本。按**使用场景**分四类——先看下面四节挑对工具，再去看脚本本身。

> 本 README 不自动加载。根 `AGENTS.md` 启动顺序第 5 条要求：进入某目录想读 / 动其下文件前，若该目录有 README，**先读一下**。

## 1. 开发者手动跑（通过 Makefile）

| 命令 | 脚本 | 干什么 |
|---|---|---|
| `make verify` | `verify-control-plane.sh` | 控制面总自检：结构 + 文档自检 + hook policy 自测 + 自进化兜底自测 + eval 资产 + 各类索引漂移检查。**日常一键体检走这个。** |
| `make docs-audit` | `docs-audit.sh` | 校验各 `.md` frontmatter 里 `source_files` / `related_docs` 指向的目标是否存在。 |
| `make eval` | `run-eval.sh` | eval 的**可选** CI/headless 路径：拼 evaluator + rubric + 考题 + 候选 → 调外部 LLM API → 写 task-review。需要 `EVAL_API_BASE` / `EVAL_API_KEY`。交互时默认走 eval 子 agent（`.claude/agents/eval.md`，免 key）。 |
| `make verify-eval` | `verify-eval-materials.sh` | 检查 eval 资产结构：考题文件、`index.yaml` 登记一致。 |
| `make hooks` | `install-hooks.sh` | 安装 git hooks（`core.hooksPath` 指向 `.githooks/`）。 |

`make help` 看 Makefile 帮助。

## 2. 自动触发的 hook（不要手动跑）

| 脚本 | 触发时机 | 干什么 |
|---|---|---|
| `stop-check.sh` | Claude Code Stop 事件 | 若 todo 声明 L2+，必须有 eval 评审产出（rule-0005）；并提醒记 lessons。`exit 2` 拦住收尾。 |
| `turn-backstop.sh` | 每 K 轮 / commit 边界 / 变更文件数增量 | 触发 headless Haiku 复查最近对话，把"做了决策 / 学了偏好 / 有知识却没写进文档"捞进 `tasks/optimization-log.md`。**全程 best-effort，任何失败一律 exit 0**，不阻断收尾。 |
| `hook-policy.sh` | PreToolUse（在 `.claude/settings.json` 里挂上） | 扫描传入内容里的疑似密钥（Bearer / api_key / secret 等）与高危命令（`git reset --hard` / `rm -rf`），命中非零退出。 |

## 3. 索引 / 账本生成器（写产物 + 防漂移）

这几个脚本都支持 `--check` 模式——只比对、不写；漂移则非零退出，被 `make verify` 统一调用。**手改它们生成的产物会被 verify 当成漂移挡下**。

| 脚本 | 产物 | 来源 |
|---|---|---|
| `rules-index.sh` | `docs/rules/index.yaml` | 扫各 `AGENTS.md` 里的 `<!-- rule: rule-00NN \| sev: ... \| eval: ... -->` 标记。 |
| `dir-index.sh <dir>` | `<dir>/README.md` | 该目录下所有 `*.md` 的 frontmatter title（缺则首个 `#` 标题）。用于决策 / 需求等"账本式"目录。 |
| `skills-index.sh` | `.agents/skills/README.md` | 各 SKILL.md frontmatter（name / description）。 |
| `index-audit.sh <dir>` | （只校验）| `<dir>/index.yaml` 登记的 `file:` 都存在，且 `<dir>` 下每个 `NNNN-*.md` 都被登记。 |
| `prds-audit.sh` | （只校验）| `docs/prds/index.yaml` 与 `docs/prds/*/prd.md` 一致 + `prd.md` 含必备章节。质量（验收可观测 / 原型可点通等）由 eval 考题 013 / rule-0010 判，不在这里。 |

## 4. 脚本自测

改对应脚本时**必须同步改 / 跑通**自测。

| 自测脚本 | 测哪个 |
|---|---|
| `hook-policy.test.sh` | `hook-policy.sh` |
| `turn-backstop.test.sh` | `turn-backstop.sh` |

两个自测都被 `make verify` 调用。

---

**新增脚本时**：归到上面四类之一并在本 README 登记；写产物的脚本同时实现 `--check` 模式，并接进 `verify-control-plane.sh`；hook 类脚本同步加自测；本 README 是手写的（不是 dir-index.sh 自动生成），不要忘了同步。
