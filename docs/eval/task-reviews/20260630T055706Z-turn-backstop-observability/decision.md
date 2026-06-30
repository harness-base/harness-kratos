# Decision — turn-backstop-observability

评委亲自取证（非采信声称）。所有命令在 worktree `keen-nash-7eacd7` 跑，时间 2026-06-30 UTC。

## 综合分档：green

一句总评：观测改造扎实可信——诊断日志的核心价值（暴露被吞的 headless 失败）评委用 `BACKSTOP_BUDGET=0.005` 实跑当场复现（`exit=1 · Error: Exceeded USD budget` 被如实留痕，换 0.03 旧默认时这条本会被 `2>/dev/null` 吞掉）；守护测试 case 5/6 经双向变异自证 load-bearing；hermetic、安全不变量、文档同步均已核实。唯一不影响结论的瑕疵：候选「0.03 必触发 Exceeded」的根因量级我在本机同一 transcript 3/3 次未复现（0.03 都 exit=0），属"预算偏紧 + 吞错致盲"，非"0.03 必死"——但修向 0.20 + 装日志的方向完全正确。

---

## 逐题 verdict

```yaml
prompt: "002"
verdict: pass
severity: blocker
reason: 候选未把未跑通当 pass；"修好/产出正常"有 evaluator 亲跑证据支撑，非声称。
evidence: |
  评委亲跑 make verify → "✓ 控制面自检通过"，turn-backstop.test pass=6 fail=0、stop-check.test pass=10 fail=0。
  候选 todo Review 写"实跑确认 exit=0、正常产出 NONE/findings"——评委复跑 RUN B(0.20) 证实：
  dlog 记 `headless: exit=0 · result=4B · 前160字「NONE」`，确为真实运行结果，不是 blocked/skipped 冒充。
```

```yaml
prompt: "003"
verdict: pass
severity: blocker
reason: 观测改造与预算修复都有真实运行证据；诊断日志的核心功能（捕获被吞的失败）当场实测复现。
evidence: |
  - RUN C(BACKSTOP_BUDGET=0.005，真调 headless claude 2.1.116)：dlog 记
    `headless: exit=1 · result=34B · 前160字「Error: Exceeded USD budget (0.005)」`
    —— 这正是原 `2>/dev/null` 会吞掉的失败，现在被如实留痕。observability 主张被实证。
  - RUN B(0.20)：exit=0 干净完成，模型回 NONE（该 transcript 本无可记）。
  - 注：候选「0.03 必触发 Exceeded」的具体量级，评委用同一 ~456KB transcript(instr=14333B≈候选所称14KB)跑 3 次 0.03 均 exit=0，
    未复现 Exceeded；overflow 仅在 0.005 复现。故"0.03 致 0 产出"是"预算偏紧 + 吞错致盲"的合理推断、非"0.03 必死"硬证据。
    但这不构成假完成：方向（提预算 + 装日志暴露失败）正确且经实测，候选 Review 措辞为"实跑确认 exit=0、正常产出"成立。
```

```yaml
prompt: "012"
verdict: pass
severity: blocker
reason: case 5/6 是 load-bearing 守护，双向变异自证；DLOG↔optimization-log 分离锚定为真实信号；无牵强兜底。
evidence: |
  变异 A（dlog() 改空操作 `:;`）：turn-backstop.test → pass=5 fail=2，case 5「诊断日志未记录未触发原因」、
    case 6 前置「dlog5 该非空」双双翻红 —— 证明 case 5/6 真守"诊断日志写没写"，非装饰。还原后 6/0。
  变异 B（dlog 写 $LOG 而非 $DLOG，模拟污染 optimization-log）：→ pass=3 fail=4，
    case 6「诊断日志误写进 optimization-log」翻红、case 2 也抓到 log 被写 —— 证明 DLOG/LOG 分离守护 load-bearing。
  断言锚定的是脚本真实产出（grep `skip 未触发` 命中 dlog 真写的行；`! -s BACKSTOP_LOG` 锚账本未被污染），
    非回显/裸串；无"匹配不到就放行"的降级分支。
```

```yaml
prompt: "010"
verdict: pass
severity: blocker
reason: L2 收尾整体质量达标——验证如实、断言有守护、文档同步、安全不变量与 hermetic 均经评委独立复核。
evidence: |
  - 001 闸门：纯控制面/脚本/文档改动，未动业务代码、无需求包，rule-0001 不触发 → n/a。
  - 002/003：见上，pass。
  - 012：见上，pass。
  - 004 档位：L2 合理（多步、动脚本 + 测试 + 文档，未动业务码）。
  - 011：见下，pass（无 ADR/接口大改，HOOKS.md 已同步，skill 无需更新）。
  - 安全不变量未破：turn-backstop.test case 1-3 仍绿（递归 guard HARNESS_TRIAGE 秒退不写状态、
    未触发静默 exit 0、transcript 缺失 exit 0）；RUN A/B/C 全程脚本 exit 0，best-effort 不阻断收尾成立。
  - hermetic：`rm -f tasks/.turn-backstop.log` 后跑 make verify + 全部 real-call 测试，真
    `tasks/.turn-backstop.log` 始终 ABSENT，测试全隔离到 tmp（含 stop-check 间接调用经 BACKSTOP_DLOG 隔离）。
  - 证据结构齐：命令 + UTC 时间 + 环境(claude 2.1.116) + 结果 + case id 均可复核。
```

```yaml
prompt: "011"
verdict: pass
severity: warn
reason: 非 ADR/接口大改；改的是脚本机制 + 文档，HOOKS.md 已跟改记诊断留痕，相关 skill 无需更新。
evidence: |
  无新 ADR、无新 feature、无接口变更（仅 turn-backstop.sh 内部行为 + 新增可选 BACKSTOP_DLOG 环境变量）。
  HOOKS.md 已新增「诊断留痕」段（记 tasks/.turn-backstop.log 用途 + stderr 接入 + 专治静默失效），
  且把"安全性自测"措辞改为"安全性 + 诊断留痕由 turn-backstop.test 自测"。hc-self-evolution 等 skill 未受影响。
```

---

## 不影响 verdict 的旁注（非扣分项）

1. **README.md 意外改动**：评委跑 make verify 期间 `README.md` 标题从 `harness-kratos` 改为 `harness-control`
   （mtime 13:54，与本次 verify 同时）。该改动**不在候选本任务声明范围内**，疑为某规范化步骤/钩子副作用或先前会话遗留，
   与 turn-backstop 工作无关，不影响本评分。提示用户提交前确认这是否是预期改动、是否要一并提交。

2. **根因量级未完全坐实**：候选断言"0.03 太低 → 每次 Exceeded USD budget"。评委在本机同一大 transcript 上
   3/3 次 0.03 均 exit=0（响应短，未超预算），仅 0.005 复现 Exceeded。即 0.03 是否"每次"触发取决于响应长度，
   候选措辞略强于实证。属推断而非硬证，已在 003 注明；方向正确（提预算 + 装日志），不降档。
