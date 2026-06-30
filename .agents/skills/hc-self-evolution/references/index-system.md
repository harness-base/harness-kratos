# 索引体系（横切）审查手册

> 横切维度：不审某一个资产区，审「全仓的索引这件事」——每个资产区是否有索引、索引是不是机器生成防漂移、有没有进 `make verify` 的 `--check`、引用是否悬空。索引是自检/自进化的**地图**；地图烂了，上层所有维度都在盲查。

## 规范（健康长什么样 / 不变量）

- **每个资产区都有索引。** 一个区（skills / rules / decisions / eval / features / prds / 各 docs 子目录 / templates / agents）必须有一份「区内有什么」的清单，否则自检没地图。
- **能自动生成的必须自动生成。** 手维护的索引一定漂——本仓只接受三类生成器：`rules-index.sh`（扫 AGENTS.md 的 `<!-- rule: -->` 标记）、`skills-index.sh`（扫 SKILL.md frontmatter）、`dir-index.sh`（扫目录下 `*.md` 标题）。新增「区」要么复用 `dir-index.sh`，要么仿写一个，**不许新增纯手维护的索引**。
- **每份生成的索引都有 `--check` 且进 `make verify`。** 生成器和校验是一对：`--check` 只比对、漂移则非零退出，挂进 `verify-control-plane.sh`。没进 verify 的索引 = 没人替你发现漂移。
- **生成的索引头部写明「自动生成、禁手改」**，并标注「默认不加载；自检 / 自进化时当地图查」——索引是按需的地图，不是常驻上下文。
- **引用不悬空。** 索引里的指针（eval id → prompt 文件、index.yaml 的 `file:`/`dir:` → 真实文件/目录）必须落到真实存在物；反向（目录存在却没登记）也要拦。
- **生成器自身忠实。** scanner 如实反映源标记/源文件，不漏不脏；但**忠实地继承错误源** ≠ 源正确——源头标记写错，索引会忠实地把错误放大（见漏洞模式）。

## 怎么检索现状（索引 / 文件 / 机器检查入口——可直接跑）

```bash
cd "$(git rev-parse --show-toplevel)"

# 1. 列全部 index.yaml 与 README 索引（盘点有哪些区、各自的索引文件）
find . -name index.yaml -not -path './.git/*' | sort
find . -name README.md -not -path './.git/*' -not -path '*/node_modules/*' | sort

# 2. 三个生成器（看注释即知扫什么源、怎么 --check）
cat scripts/rules-index.sh      # rules → docs/rules/index.yaml（扫 AGENTS.md 标记）+ eval 指针校验
cat scripts/skills-index.sh     # skills → .agents/skills/README.md（扫 SKILL.md frontmatter）
cat scripts/dir-index.sh        # 通用：<dir>/*.md → <dir>/README.md（context/harness/templates/.claude/agents）

# 3. 哪些索引进了 verify 的 --check（唯一真实闸门入口）
grep -nE 'index|--check|audit' scripts/verify-control-plane.sh

# 4. 跑一遍全量自检，看索引部分是否绿
make verify   # 含「skills 无漂移 / rules 索引无漂移 / 目录索引无漂移 / PRD 账本」

# 5. 单点验漂移（不写、只比对）
bash scripts/skills-index.sh --check
bash scripts/rules-index.sh  --check
for d in docs/context docs/harness templates .claude/agents; do bash scripts/dir-index.sh "$d" --check; done
bash scripts/prds-audit.sh

# 6. 判某个区的索引是「自动」还是「手维护」：有没有脚本引用它
for idx in rules eval decisions features prds; do
  echo -n "$idx/index.yaml <- "; grep -rl "docs/$idx/index.yaml" scripts/ | tr '\n' ' '; echo
done
```

现状速查（盘点结果，核到 2026-06-26）：

| 区 | 索引文件 | 生成器 | 进 verify `--check`? |
|---|---|---|---|
| skills | `.agents/skills/README.md` | `skills-index.sh`（自动） | ✅ |
| rules | `docs/rules/index.yaml` | `rules-index.sh`（自动）+ eval 指针校验 | ✅ |
| context/harness/templates/agents | 各自 `README.md` | `dir-index.sh`（自动，本仓刚补） | ✅（4 个 dir 循环） |
| prds | `docs/prds/index.yaml` | 手维护，但有 `prds-audit.sh` 校验一致性 | ✅（audit，非 --check 重生成） |
| **decisions** | `docs/decisions/index.yaml` | **手维护，无生成器** | ❌ 仅查存在，**不查内容漂移** |
| **eval** | `docs/eval/index.yaml` | **手维护，无生成器** | ❌ 仅查存在 + `verify-eval-materials` 部分覆盖 |
| **features** | `docs/features/index.yaml` | **手维护，无生成器** | ❌ 连存在检查都没进结构闸 |
| docs/eval/* 子目录 README | 多份 | 手写散文，非清单 | ❌ |
| projects/kratos-base/** README | 多份 | 手写 | ❌（被管工程域，另算） |

注意：`make skills-index` / `rules-index` / `dir-index` **不是 Makefile 目标**，重生成只能直接 `bash scripts/<x>.sh`（无参=重生成，`--check`=校验）。

## 怎么判（逐条可判定）

- **区无索引** → 缺口。`find` 出的资产区没有对应 index.yaml/README 清单，自检对该区没有地图。
- **手维护漂移** → 漏洞（blocker 级别看影响）。判据：该区索引**没有生成器**（第 6 步 grep 为 NONE 或仅指向 verify 自身），且内容是人手敲的 → 一定会漂。本仓 **decisions / eval / features 三区 index.yaml 命中**。
- **有索引但没进 verify** → 漏洞。生成器存在但 `--check` 没挂进 `verify-control-plane.sh` → 漂了也无人知。判据：第 3 步 grep 不到该索引的 `--check`。
- **引用悬空** → 漏洞。索引里的指针落空：eval id 指向不存在的 prompt、index.yaml 的 `file:`/`dir:` 指向不存在的文件/目录、related_docs/source_files 悬空。判据：跑 `rules-index.sh --check`（含 `check_eval_pointers`）、`prds-audit.sh`、`docs-audit.sh`。
- **生成器忠实但源错** → 索引「无漂移」却内容错。判据：`--check` EXIT=0 **不代表内容对**——还要核源头标记/源文件本身是否正确（见下方真实案例）。`--check` 绿 ≠ 收敛。
- **索引头缺「自动生成/禁手改」声明** 或被人当常驻上下文全量加载 → 违反「默认不加载、自检时查」。

## 常见漏洞模式（本仓真实案例）

- **手编索引偷偷漂 + 凭空造悬空指针**（`docs/eval/task-reviews/20260626T014408Z-harness-rules-distribution/decision.md`）：规则分布化那轮，候选在 `AGENTS.md` 标记里把 rule-0005 的 `eval: 010` 手写成 `eval: 005`、给 rule-0006/0008 凭空加 `eval: 006/008`——而 `docs/eval/prompts/` 根本没有 005/006/008。catalog 生成器**忠实地**把这 3 个悬空指针落进了 `index.yaml`，`--check` 还是 EXIT=0「无漂移」。同时 rule-0007 的 severity 被 warn→blocker 偷改，ADR 却白纸黑字写「severity / eval 映射全保留」。教训：**生成器忠实 ≠ 源正确；`--check` 绿 ≠ 内容对**。修复时直接把「eval 标记必须指向存在的 prompt 文件」固化进 `rules-index.sh` 的 `check_eval_pointers`，并**变异自证**（注入 `eval:999` → `--check` FAIL → 还原 PASS）——这正是「补生成器校验 + 进 verify」的标准动作。
- **「`--check` 单轮零」当收敛**（`tasks/lessons.md` 2026-06-12 条「假收敛」）：跑机器校验出 0 就判通过、想停，被追问后换怀疑式视角又挖出 36+ 真问题。映射到索引维度：**索引绿只证明「索引与源一致」，不证明「源是对的」**，必须另核源头。
- **登记的检查入口自身是坏的**（`tasks/lessons.md` 2026-06-02「e2e 脚本隐含 CWD 假设」）：同理，索引生成器/`--check` 若有 CWD 假设或平台假设（macOS 无 `timeout`，见 2026-06-26 条），从登记的路由亲跑才暴露——**别信子代理在「对的目录」里跑过**。

## 修复用哪个操作 skill / 脚本

- **补一个目录的自动索引**（最省事）：`bash scripts/dir-index.sh <dir>` 生成，把 `<dir> --check` 加进 `scripts/verify-control-plane.sh` 的目录索引循环。decisions/eval/features 这种结构化 index.yaml 不适合 `dir-index`（它扫 `*.md` 标题），需仿 `rules-index.sh`/`prds-audit.sh` 另写「目录 ↔ index.yaml 一致性」校验器。
- **补/改规则索引**：改 `AGENTS.md` 的 `<!-- rule: -->` 标记后 `bash scripts/rules-index.sh` 重生成；走 `hc-add-rule` skill 保证「写下来 + 登记 + 挂执行」三步齐。
- **补/改技能索引**：改 SKILL.md frontmatter 后 `bash scripts/skills-index.sh` 重生成。
- **写新生成器/校验器的范式**：照 `scripts/rules-index.sh`（gen + `--check` 非零退出 + 指针校验 + 变异自证）或 `scripts/prds-audit.sh`（正向：目录必登记+必备章节；反向：登记必存在），并把 `--check`/audit 挂进 `verify-control-plane.sh`。
- **统筹与登记缺口**：用 `hc-self-evolution` skill 把发现晋升归档；机器查不了的判断写 `tasks/optimization-log.md`，捞到的须晋升、不许烂在 log 里（rule-0011）。
