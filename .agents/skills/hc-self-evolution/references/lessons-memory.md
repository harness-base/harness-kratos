# lessons / memory 审查手册

错题本、用户偏好、自进化 log 三件套的健康度。核心红线：**踩坑当场记、反复出现晋升成规则、log 是中转站不是终点。**

## 规范（健康长什么样 / 不变量）

- 踩坑当场记进 `tasks/lessons.md`，**三段式**（Mistake / Prevention / Earlier signal），按日期倒序，新的在上（格式见文件头 1-12 行）。
- **反复出现的 lesson 必须晋升成规则**（走 `hc-add-rule`），不许同型坑反复记不收口（rule-0007）。
- `tasks/optimization-log.md` 是**中转站**：兜底（Stop hook 机械触发）和 judgment（hc-self-optimize 子 agent）捞到的候选落这里，**重要的必须晋升到家**——决策→ADR、踩坑→`lessons`、知识→就近 `AGENTS.md`/规则、偏好→memory（rule-0011）。log 里不许烂着不动。
- memory（`~/.claude/projects/<proj>/memory/MEMORY.md` + 同目录条目）只存**跨会话的用户偏好/工作方式**，有 index、条目不悬空、不过期。
- 只认**预期格式**入库：兜底只把形如 `[类别] ...` 的行写进 log；LLM 报错（`Prompt is too long` / `overloaded` / `budget`）、`NONE`、空一律不记（lessons 2026-06-26 那条就是被这个坑过）。

## 怎么检索现状（命令可直接跑）

```bash
ROOT="$(git rev-parse --show-toplevel)"

# 错题本：看条数、最近日期、是否三段式
grep -c '^## 20' "$ROOT/tasks/lessons.md"
grep '^## 20'  "$ROOT/tasks/lessons.md" | head            # 标题倒序看新鲜度

# 找"同型反复"——同主题词出现 ≥2 次 = 晋升信号（例：空转测试 / 共因 / 超时竞态 / CWD）
grep -niE '空转|共因|超时|竞态|CWD|牵强|假完成|裸 ?grep' "$ROOT/tasks/lessons.md"

# log 是否空转：有没有 judgment/兜底 条目，最后一条是什么时候
sed -n '1,12p' "$ROOT/tasks/optimization-log.md"          # 头部约定（中转站语义）
grep -nE '`兜底`|`judgment`' "$ROOT/tasks/optimization-log.md"   # 现状：仅有表头说明、0 条 ⇒ 闭环尚未跑过

# memory：index 与条目文件（路径基于主仓工程根，与本 worktree 无关；本仓即下方路径）
cat ~/.claude/projects/-Users-zhouhaiyin-project-harness-kratos/memory/MEMORY.md
ls   ~/.claude/projects/-Users-zhouhaiyin-project-harness-kratos/memory/

# 兜底机制本体（WHEN 在脚本、WHAT 给 Haiku）+ 它的自测
sed -n '46,78p' "$ROOT/scripts/turn-backstop.sh"
bash "$ROOT/scripts/turn-backstop.test.sh"
```

- 这三类**没有专门的 `--check`**（不像 rules/skills 有索引校验）——健康度靠上面 grep + 人判，是已知的弱机器化点。

## 怎么判（逐条可判定）

- **符合**：lessons 全三段式；同型坑出现 ≥2 次的都已有对应规则；log 里每条候选要么已晋升（家里能查到）、要么明确标注待办；memory 条目都在 index 里且仍真。
- **缺口**：lessons 有条目但缺 Earlier signal / Prevention 段；log 有候选但躺着没晋升、也没标待办；memory index 指向不存在的条目文件。
- **漏洞**：同一主题 lesson 反复记 ≥2 次却没规则（晋升断链，rule-0007 失守）；log 长期堆积无人晋升（空转，rule-0011 失守）；把 LLM 错误串/`NONE` 当发现写进 log（污染）；memory 存了过期/错误的偏好仍被当事实加载。

## 常见漏洞模式（本仓真实案例）

- **log 空转 / 污染——本仓刚踩过**：`tasks/lessons.md` 2026-06-26「兜底把超长 transcript 喂给 Haiku，"Prompt is too long" 被当发现记进 log」。`turn-backstop` 按**行**截 transcript，JSONL 单行含工具输出可能极大→prompt 超限→Haiku 回报错串→脚本没识别，当成发现追加进 `optimization-log` 还提交了一条。修法已固化：改按**字节**截（`TAIL_BYTES`，脚本 21/55 行），入库只认 `^\[`（脚本 73 行）。
- **同型坑反复未晋升**：2026-06-12「池类依赖掩盖探活失败不重建」与多条 e2e/共因/超时竞态 lesson 同主题（断言锚错信号）反复出现——这类一旦 ≥2 次就该考虑晋升，对应已沉淀为 rule-0009（验收断言锚定唯一真实证据）。判 lessons 健康时，专盯"同主题第 2 次还在记 lesson、却没规则"。
- **晋升断链被 eval 抓**：2026-06-26「rule-0007 改了 skill 却没在 ADR 记录 = 判失败」——做了没记 = 没履行（eval-011 判 blocker fail）。
- **log 闭环要看真跑没跑**：`② judgment` 已有条目；`① capture`（turn-backstop）实测**仍 0 产出**（根因待查，ADR-0012 列 backlog）。doc-drift 现写 `- [ ]` 状态、经 `correction-nudge` 下一轮反馈，处理后标 `- [x]`。审查时区分"没东西要记"与"机制没启动"，并查 `- [ ]` 有没有堆积（待处理没人理）。

## 修复用哪个操作 skill / 脚本

- **反复 lesson → 晋升成规则**：用 `hc-add-rule` skill（`.agents/skills/hc-add-rule/SKILL.md`）——定范围→写进就近 `AGENTS.md`+登记→挂执行/考题。
- **处理 log 候选 / 主动深审**：用 `hc-self-evolution` skill（`.agents/skills/hc-self-evolution/SKILL.md`，规范检查层入口）；复杂时 spawn `hc-self-optimize` 子 agent（`.claude/agents/hc-self-optimize.md`）。把候选搬到 ADR/lessons/就近 AGENTS.md/规则/memory，不许烂在 log（rule-0011）。
- **写/整理用户偏好**：写 memory 文件 + 维护 index；整理用 `anthropic-skills:consolidate-memory`（合并重复、修过期、剪 index）。
- **兜底机制本身的修复**：`scripts/turn-backstop.sh`（触发与入库逻辑）+ `scripts/turn-backstop.test.sh`（改了必须红得起来，mutation 自证）。
