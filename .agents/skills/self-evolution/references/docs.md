# 文档 审查手册

审查 harness 的文档层是否健康：事实源单一、规则全文就近、shim 不缺、路由准、context 不漂移、引用不悬空。

## 规范（健康长什么样 / 不变量）

- **`AGENTS.md` 是常驻规则源、机制加载**：Claude Code 经同级 `CLAUDE.md` 的 `@import` 自动加载；Codex 原生按目录层级读 `AGENTS.md`。**加载是机制**——审查时遇到"提示 AI 去读 AGENTS.md"这类话术 = 加载链路没接通，断环。
- **事实源单一**：常驻规则全文只在 `AGENTS.md`（根 + 就近）；其它文档只引用、不复刻规则正文。事实源 = 正式文档 + 工程当前代码（rule-0008）。
- **根 `AGENTS.md` 红线精简、全文就近**：harness 全局红线（rule-00NN）全文留在根；**项目专属规则下沉到 `projects/**/AGENTS.md` 就近生效**，不堆在根。
- **凡 `AGENTS.md` 必有同级 `CLAUDE.md` shim**：shim 内容就一行 `@AGENTS.md`（Claude Code 靠 `@import` 加载，机制不靠"自然语言叫你读"）。
- **`docs/README.md` 路由准**：阅读顺序、目录职责表与磁盘实际目录一致；指到的文件真存在。
- **`docs/context/` 反映真实状态**：`CURRENT_STATUS.md` 的模块/规则/工程状态与代码现状一致；`CONTEXT_LOADING.md` 档位规则可用。
- **frontmatter 不悬空**：每篇带 frontmatter 的 `.md`，其 `source_files` / `related_docs` 指向的目标都存在（`docs-audit` 兜底）。
- **README 类资产健康**：每份 `README.md` 要么机器自动生成且有 `--check` 漂移检测进 `make verify`（如 `dir-index.sh` / `skills-index.sh`），要么手写但有明确"装什么 / 由谁同步"约定。**没归属的手写 README = 漂移源**。README 不在 `@import` 加载机制内、装的是按需查阅的地图，靠根 `AGENTS.md` 启动顺序第 5 条规则触发读取。

## 怎么检索现状（直接可跑）

```bash
ROOT="$(git rev-parse --show-toplevel)"

# 路由总入口 + 阅读顺序
cat "$ROOT/docs/README.md"

# 全部 AGENTS.md / CLAUDE.md（看就近分布、配对是否齐）
find "$ROOT" -name AGENTS.md -not -path '*/.git/*'
find "$ROOT" -name CLAUDE.md -not -path '*/.git/*'

# 机器检查：related_docs/source_files 悬空
bash "$ROOT/scripts/docs-audit.sh"          # 或 make docs-audit

# 机器检查：shim 漏配 / 未 @import（含在控制面自检里）
bash "$ROOT/scripts/verify-control-plane.sh"  # 看 "AGENTS.md ↔ CLAUDE.md shim" 段；或 make verify

# 真实状态 vs 代码
cat "$ROOT/docs/context/CURRENT_STATUS.md"

# 规则全文位置（确认就近、不堆根）
grep -rn '<!-- rule:' "$ROOT" --include=AGENTS.md

# 非 docs/ 区的全部 README（多为手写散文）
find "$ROOT" -name README.md -not -path '*/.git/*' -not -path '*/node_modules/*' \
  -not -path '*/docs/*' -not -path '*/.agents/*'

# 这些 README 是否被某 *-index / *-audit 守（在 verify-control-plane.sh 里能找到）
grep -nE 'README\.md|coverage-audit' "$ROOT/scripts/verify-control-plane.sh"
```

## 怎么判（逐条可判定）

- **事实源单一？** 同一条规则正文是否在两处出现（根又复刻到某 doc）= 违反，收敛到 `AGENTS.md`。
- **根红线精简、就近？** 根 `AGENTS.md` 是否混入了只对某工程成立的规则 = 该下沉到 `projects/**/AGENTS.md`。
- **shim 齐？** `find AGENTS.md` 的每个目录下都有 `CLAUDE.md` 且含 `@AGENTS.md`？`verify-control-plane.sh` shim 段报 `✗` = 缺/未 import。
- **路由准？** `docs/README.md` 目录职责表的每一项在磁盘上存在；阅读顺序里的文件 `docs-audit` 不报悬空。
- **context 不漂移？** `CURRENT_STATUS.md` 写的状态与代码对得上（例：规则条数、工程进度、skills 个数）。机器查不了，逐条对现状判。
- **引用不悬空？** `docs-audit.sh` 退出 0 且打印 `✓ docs-audit 通过`；任何 `✗ ... → 引用不存在` = 悬空。
- **知识就近？** 新沉淀的工程/目录级规矩是否写进了**最近**的 `AGENTS.md`，还是堆在根/某大文档里。
- **手写 README 有没有人守？** 在 `scripts/verify-control-plane.sh` 找不到守它的命令、自身也没标"由谁维护、何时同步" = 缺口。
- **README 是不是复刻了红线规则正文？** grep 该 README 是否含 `<!-- rule: rule-00NN` 或与 AGENTS.md 红线高度相似的整段——纯说明扫描格式时用的示例字符串可豁免。复刻 = 违反"事实源单一"。

## 常见漏洞模式（本仓真实案例）

- **文档现状漂移**：commit `ee72ca4`（R11）专门"文档现状化"——文档描述与代码已脱节，靠对抗评审才照出。**活例（写本手册时实测）**：`docs/context/CURRENT_STATUS.md` 仍写 "8 条规则（rule-0001 ~ 0008）" 与 `.agents/skills` "4 个技能"，但实际已增至 7 个（含 `prd-elicitation`/`self-evolution`/`bugfix`）—— 典型 context 没跟代码同步（本批已修 CURRENT_STATUS）。
- **声称"全保留/不变"却偷改**（`tasks/lessons.md` 2026-06-26 规则分布化）：ADR 凭记忆写"severity/eval 映射全保留"，实际改了 rule-0007 severity、给规则编了不存在的 eval 指针 → eval 子 agent 逐条 `git show HEAD` 对比判 yellow。教训：凡"X 保留/不变"，必须对事实源机械核对，能机器查的固化成 `--check`。
- **知识堆在根、没就近**：本仓 kratos 早期工程规则曾缺就近 `AGENTS.md`（规则分布化前都堆控制面）；现已下沉到 `projects/kratos-base/**/AGENTS.md`（`pkg/*` · `app/demo/internal/*` · `test/resilience` 各层就近），根 `AGENTS.md` 明确"项目专属规则沉淀在 `projects/**/AGENTS.md`，不堆这里"。审查时查"该就近的有没有就近"。
- **shim 漏配**：新建 `AGENTS.md` 忘了同级 `CLAUDE.md` → Claude Code 加载不到该层规则（`verify-control-plane.sh` shim 段会拦，但漏跑就漏）。
- **related_docs 悬空**：frontmatter 引用的文件改名/移动后没同步 → `docs-audit` 红。
- **手写 README 自承"靠人记得"无机器兜底**：`scripts/README.md`（2026-06-26 新加这份 README 时埋下）末尾自标"手写，不要忘了同步"——scripts/ 新增 `.sh` 不更新 README，`make verify` 仍全绿。同型还有根 `README.md` 顶层结构表、`docs/README.md` 子目录职责表。
- **`docs/eval/README.md` 是散文型 README**（触发口径与流程）：不适合纯枚举式 audit，但其引用到的文件应被 `docs-audit` 兜（要求带 frontmatter）。判据：README 提到的具体文件 / 路径都该存在。

## 修复用哪个操作 skill / 脚本

- **机器闭环**：`scripts/docs-audit.sh`（悬空）、`scripts/verify-control-plane.sh` 的 shim 段（shim 缺配）、`make verify` / `make docs-audit` 统一入口。
- **改文档内容**（路由/context 现状化/就近下沉）：直接编对应 `.md` / `AGENTS.md`，改完跑 `make verify` 复核。
- **改了代码/配置/接口后主动同步文档**：用 `doc-sync` skill（对照 checklist 查 README / AGENTS / CURRENT_STATUS 等要不要跟改），是 `turn-backstop` 落文档提醒的主动版。
- **状态/索引文档别硬编码可自动生成的枚举**：能交给 `*-index`（如 `.agents/skills/README.md` by `skills-index`）的清单/计数，写"以该自动生成索引为准"，别在 `CURRENT_STATUS` 等处复刻——硬编码枚举是反复漂移源（`tasks/lessons.md` 2026-06-27）。
- **沉淀一条规则到就近 `AGENTS.md` 并挂执行**：`add-rule` skill。
- **决策类大改要留 ADR**：`templates/adr.md` 起草（rule-0007，别手搓省栏）。
- **新建 `AGENTS.md` 后**：必补同级 `CLAUDE.md`（`@AGENTS.md` 一行），靠 `verify-control-plane.sh` shim 段自证。
- **手写 README 缺漂移检测**：写或复用 `scripts/dir-coverage-audit.sh`（通用版，按目录配置 include/exclude glob 做双向覆盖检查：文件名 ↔ README 提及），挂进 `verify-control-plane.sh`。
- **新写 README 时**：不复刻规则正文（事实源在 AGENTS.md）；按需查阅型材料可以长，但要前重后轻——长会话上下文压缩后还能传递主要信息。
