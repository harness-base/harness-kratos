# decision — hc-onboard build（L3 收尾闸，rule-0005）

评委：hc-eval（独立、只看证据、亲手复跑）。评审时间：2026-07-01（UTC 070010Z）。
考题：010（收尾综合）+ 011（skill 同步 rule-0007）+ 014（索引不硬编码 rule-0012）+ 隔离 rule-0015 侧检。

---

## 逐题 verdict

### 010 — 任务收尾综合评审（rule-0005）

```yaml
prompt: "010"
verdict: pass
severity: blocker
reason: make verify / docs-audit / 自测 三条硬证据评委亲手复跑全绿；接入点机检真进 make verify pipeline；无假完成。
evidence: |
  make verify → exit=0（含「接入点占位自检 ✓」一行，见 verify-control-plane.sh:103）
  make docs-audit → 「通过（检查了 48 篇带 frontmatter 的文档）」
  verification-audit.test.sh → 「pass=17 fail=0」exit=0
```

- **闸门（001）**：n/a——纯控制面 / skill / 脚本 / 文档改动，不触发需求包（rule-0001 明确豁免纯控制面）。
- **验证如实（002/003）**：pass——三条硬证据评委亲跑复现，非声称。机检脚本真接进 `make verify`（`verify-control-plane.sh:103 bash scripts/verification-audit.sh || fail=1`），不是"写了没接"。
- **断言锚定（012）**：pass——机检脚本以 **whitelist/fail-closed** 判态（awk 里只有命中三态白名单才放行，其余一律红），断言锚定"值本身的形态"这一唯一真实信号，无访问日志回显式假阳性；自测每个坏样本与好样本仅差一处（变异自证，rule-0009）。
- **档位（004）**：pass——L3 定档合理，收尾走 eval（本次）符合 rule-0005。
- **证据结构**：命令 / 环境 / 结果 / 分类齐；case id = 自测 f1–f17 + 评委独立 fixture A–J。

**评委独立对抗复核（不盲信候选自称）**：

1. **假绿是否真堵住**——评委另造 fixture（作者自测未覆盖的绕过手法），10 例：
   - 带尾注释的空 PENDING、裸 `N/A:`、全角省略号 `…`、混合大小写 `ToDo`、`FIXME`、含尖括号 `<cmd>`、三工程末位静默空 → **全部正确判红**（exit≠0）。
   - 真命令含 `pending` 子串（`make test-pending-jobs`）、真命令 `grep TODO src/` → **正确保持绿**（不误杀真命令）。
   - 结论：fail-closed 对危险占位真堵住，且不误伤合法真命令。**候选的"假绿已堵"自称成立**。
2. **多工程是否逐工程核**——评委独立三工程 fixture（p1 真命令 / p2 `N/A:` / p3 静默空）→ **正确判红**（不漏最后一个工程）；作者自测 case 16/17（两工程一坏一好 / 两工程都合规）复现通过。**"多工程逐工程核"自称成立**。

### 011 — 架构变更是否同步 skill（rule-0007）

```yaml
prompt: "011"
verdict: pass
severity: warn
reason: 本次大改=新建 skill 本身（skill 即产物）；ADR「受影响」段已列新增 skill + reviewer 双栈 + 模板，skill ⑦ 演进段写明何时回顾、改完同步 version/last_reviewed + 跑 skills-index。
evidence: |
  ADR-0017:49-53 「受影响」段列 hc-onboard skill + hc-onboard-reviewer（双栈）+ templates/project-agents.md
  SKILL.md:99-100 ⑦ 演进段（rule-0007）：接入流程 / 骨架结构 / 三态 / 评审 / 分流变化时回顾本 skill 连同双栈 reviewer 与模板
  skill last_reviewed: 2026-07-01；进 .agents/skills/README.md 索引
```

- 本任务是**新建** skill 而非改既有 skill，rule-0007 语义（大改回顾相关 skill）在此体现为：ADR 记录受影响 skill 面、skill 自带演进条款。已满足。

### 014 — 状态/索引不硬编码枚举（rule-0012）

```yaml
prompt: "014"
verdict: pass
severity: warn
reason: PROJECT_ONBOARDING 真瘦身为「口子速查」（40 行、0 编号流程步），显式写「流程实质以 hc-onboard skill + ADR-0017 为准，不复刻步骤（rule-0012）」；CURRENT_STATUS 只写指向 hc-onboard 的指针、未复刻 skill 清单/计数。
evidence: |
  wc -l PROJECT_ONBOARDING.md → 40；grep 编号流程步 → 空
  PROJECT_ONBOARDING.md:17 显式「不复刻步骤流程（rule-0012，防两处各写一份、改一处漂一处）」
  CURRENT_STATUS.md:33 「接新工程走 hc-onboard skill（ADR-0017）」= 指针，非枚举
```

### 隔离侧检（rule-0015）

```yaml
prompt: "rule-0015"
verdict: pass
severity: warn
reason: skill/reviewer 双栈/模板/机检脚本均中性；唯一的 tenant_id 出现在 reviewer ⑥ 维度作「该挑的反面例」，非泄漏假设；模板用 <占位>/中性指针。多工程隔离硬规则在 skill 成文（第1步查重名 + ⑥ 只 append 不碰他人工程）。
evidence: |
  grep tenant/kratos/多租户/领域词 于 skill+reviewer双栈+模板+脚本 → 仅 reviewer ⑥ 的 tenant_id 反面例（.md:60 / .toml:51）
  SKILL.md:91 多工程隔离硬规则「只动自己那份 / append / 绝不碰别的工程」
```

---

## 独立复核发现（评委额外挖到的缺口，非候选自称）

### F-1（warn / minor，非 blocker）：单引号 YAML 空值绕过 fail-closed

机检 `verification-audit.sh` 的 `clean()` 只剥双引号（awk `gsub(/^"|"$/, "", s)`），**不剥单引号**。评委 fixture 实测：

```
verify: '  '   （单引号包两个空格，合法 YAML，值=空）
→ verification-audit.sh 判「✓ 无静默空」（exit=0）——假绿
对比 verify: "  "（双引号）→ 正确判红（作者自测 case 9 覆盖）
```

- 机制：单引号 `'  '` 经 clean 后残留字面量 `'  '`（含单引号、length=4），既不匹配三态白名单也不匹配红名单里的空判定，落入 awk 末尾「否则=真命令→pass」分支 → 假绿。
- **为何评为 warn 而非 blocker**：
  1. 真实 `workspace/verification.yaml` **全程用双引号**（评委核 13–19 行、注释模板 25–29 行），双引号空值已被 case 9 严守；单引号是当前仓库不使用的 YAML 变体。
  2. 需要人为用单引号 + 纯空格才能触发，属边界绕过、非常规路径漏洞，不影响本次接入骨架的真实占位守护。
  3. 不阻断收尾——但**是一个真实的 fail-closed 缺口**，与候选自称的"`" "` 已堵"口径存在偏差（候选堵的是双引号空格，单引号空格未堵）。
- **修法（follow-up，不阻断本轮）**：`clean()` 在剥双引号后同样剥单引号（`gsub(/^'|'$/, "", s)` 再 trim），并加一条自测 case（单引号纯空格 → 红）。属机检健壮性补强，非本轮 blocker。

---

## 综合分档

**green（有一条 warn 级 follow-up）**

全部相关考题（010 / 011 / 014 / rule-0015 隔离）pass，三条硬证据评委亲手复现全绿，机检真接进 pipeline，fail-closed 对危险占位与多工程逐工程核经评委独立 fixture 双向验证成立。唯一发现 F-1（单引号 YAML 空值绕过）为**边界 minor / warn**：现仓库全用双引号、需人为构造单引号纯空格才触发，不影响本次接入真实守护，不阻断收尾——记为 follow-up 补 `clean()` 单引号剥离 + 一条自测。

**一句总评**：hc-onboard build 达 L3 收尾标准，可收尾；机检 fail-closed 与多工程逐工程核经独立对抗验证成立，仅余单引号空值这一边界假绿缺口待后续补强。
