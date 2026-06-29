# Decision — test-case skill（L3）

评委：独立、严格、只看证据。三道考题逐题判 verdict，最后综合分档。

## 011 — 架构变更是否同步 skill（rule-0007）

```yaml
prompt: "011"
verdict: pass
severity: warn
reason: 本次属大改（写了 ADR-0008 + 新建 test-case skill）。ADR 的「受影响的 skill (rule-0007)」栏逐 skill 填了 yes/no + 理由，如实；新 skill 被 skills-index 自动收录、--check 无漂。
evidence: |
  docs/decisions/0008-test-case-skill.md L39-43：test-case=新建；prd-elicitation=否(独立,衔接后定)；
    feature-delivery=否(下游不变)；其余 6 个 skill=否(无关)——逐条有结论。
  .agents/skills/README.md L15：test-case 行已由 skills-index 从 SKILL.md frontmatter 自动生成。
  bash scripts/skills-index.sh --check → ✓ skills 目录无漂移（exit 0）。
  SKILL.md L41-42「演进(rule-0007)」段明确同源回顾对象（template + audit）。
```

## 014 — 状态文档不硬编码可自动生成枚举（rule-0012）

```yaml
prompt: "014"
verdict: pass
severity: warn
reason: 本次改了 CURRENT_STATUS（状态文档）。不仅没新引入硬编码枚举，反而把两处既存且已漂移的硬编码枚举改成了指向自动生成索引的指针——正中 rule-0012「发现旧硬编码已漂却没改成指针=判失败」的反面。
evidence: |
  git diff HEAD docs/context/CURRENT_STATUS.md：
    - docs/rules 行：删「11 条规则(rule-0001~0011)…另 49 条 kratos」(已漂：现 14 条 rule-0001~0014)
      → 改为「清单/计数以 docs/rules/index.yaml 为准（不硬编码枚举,rule-0012）」。
    - docs/decisions 行：删「ADR-0001~0006」(已漂：现有 ADR-0008) → 改为「以 docs/decisions/index.yaml 为准」。
    - 新增 docs/test-cases 行：是叙述性状态行（指向 test-cases-audit + ADR-0008），非枚举复刻。
  scripts 行新增 test-cases-audit(+test)：scripts 无 *-index 自动权威清单，手维护列表不属 rule-0012 反模式。
  make verify「状态文档未硬编码 skill 枚举（rule-0012）」→ ✓。
  docs/README.md L37 / scripts/README.md L40,53 均为单行职责指针，非整列枚举。
```

## 010 — 任务收尾综合评审（rule-0005）

```yaml
prompt: "010"
verdict: pass
severity: blocker
reason: 无假完成、无 blocked 当 pass、无悬空声称。所有声称的硬闸/守护测试均实跑通过，且评委用两处变异自证守护测试 load-bearing（rule-0009/012）。
evidence: |
  闸门(001)：纯控制面/skill/脚本/文档改动，无用户可见需求行为变化 → 不触发需求包，n/a，合规。
  验证如实(002/003)：bash scripts/test-cases-audit.test.sh → pass=25 fail=0；make verify → ✓ 控制面自检通过；
    make docs-audit → ✓ 27 篇通过。结论分类如实，有真实命令证据，非声称。
  断言锚定(012)：守护测试每个坏样本与好样本仅差「那一处违规」，且 #1 正向锚(好样本必绿)防 vacuous。
    评委变异验证：阉割覆盖检查① → 守护测试 25→19(6 红)；破坏剥围栏 → 25→22(3 红)；还原后均回 25，
    diff -q 确认脚本完整还原。守护测试真 load-bearing，非空转。
  解析硬化(012/009)：fail-closed——围栏 CommonMark 同字符裸行配对(b17/b24)、段标题前缀锚定(b23)、
    声明一行一 id 否则判红(b12/b16/b18/b20)、covers 收全行 id(b21/b22)、三护栏(a/b/c)，模糊一律红。
  接线(rule-0005 触发)：verify-control-plane.sh L35(守护测试)+L91(硬闸)均为 live verify step,非注释。
  skill 回顾(011)/状态文档(014)：见上,均 pass。
  空账本处理：test-cases: [] 平凡通过(守护测试 #0 锚),硬闸不误红;015 本任务无用例产物 → n/a。
  证据结构齐：命令 + 结果 + 分类 + case id(b2~b24) 完整。
```

## 015 — 测试用例覆盖完整性与质量（rule-0014）

```yaml
prompt: "015"
verdict: n/a
severity: blocker
reason: 015 评的是「已产出的测试用例集」覆盖与质量。本任务只建机制（skill/模板/硬闸/账本），账本为空（test-cases: []），未产出任何用例集，无对象可评。机制侧的覆盖闭合能力已由 010/守护测试覆盖。
evidence: docs/test-cases/index.yaml L5 `test-cases: []`；docs/test-cases/ 下无用例目录。
```

## 综合分档

**green** — 三道相关考题（010/011/014）全 pass，015 因无用例产物 n/a。

一句总评：机制建得扎实——硬闸 fail-closed 严格解析 + 25 条守护测试经评委两处变异自证 load-bearing，doc-sync 顺手清理了两处既存漂移枚举，ADR 受影响 skill 栏如实、新 skill 自动收录无漂；无假完成、无 blocked 当 pass。可收尾。

给用户的提示：本次只立机制、账本为空，015（用例真覆盖语义/边界异常齐）尚无产物可考；首次产出真实用例集时务必补跑 015，重点查空壳凑数与「只覆盖 happy path」。
