---
title: hc-design build 收尾评审 — decision
status: active
owner: hc-eval
task: hc-design
generated_at: 2026-06-30T07:26:02Z
---

# 逐题 verdict

## 考题 011 — 架构变更是否同步 skill（rule-0007）

```yaml
prompt: "011"
verdict: pass
severity: warn
reason: ADR-0015「受影响的 skill」栏 5 项逐条填写且与仓库现状名实相符；新建/解锁/不变三态清楚，无空栏。
evidence: |
  docs/decisions/0015-hc-design.md L34-40 + git status 现状交叉核：
  - hc-design ／是·新建 → ?? .agents/skills/hc-design/ 新目录（git status 确为 untracked 新建）
  - hc-design-reviewer ／新增双栈 → ?? .claude/agents/hc-design-reviewer.md + ?? .codex/agents/hc-design-reviewer.toml
  - hc-test(api 用例线) ／是·前置解锁 → docs/harness/testing-flow.md L96「接口契约来源：由 hc-design 产出…ADR-0015…现已可产出」(git status: M testing-flow.md)
  - hc-prd / hc-dev ／否·不变 → git status 无任何 hc-prd/hc-dev skill 改动（grep 现状为空），与「不变」相符
  - docs/designs 产物区 ／新增 → ?? docs/designs/（README + index.yaml 已建）
  ADR 已登记 docs/decisions/index.yaml L74-78（id: ADR-0015 / file: 0015-hc-design.md）。
```

## 考题 010 — 任务收尾综合评审（rule-0005）

```yaml
prompt: "010"
verdict: pass
severity: blocker
reason: make verify + docs-audit 双绿；控制面↔项目隔离命根守住；硬原则逐条落地；双栈对称+注册齐；模板可用、无空壳、各 index 无漂移。无 blocker 级缺陷。
evidence: 见下「010 取证明细」。
```

# 010 取证明细（亲跑）

## A. 验证真绿（rule-0002/0003）
- `make verify` → 「✓ 控制面自检通过」。关键 --check 全过：hook-policy 6/6、turn-backstop 6/6、correction-nudge 7/7、lessons-promote 3/3、stop-check 10/10、test-cases-audit 29/29；skills 目录无漂移、rules 索引无漂移、4 个 dir-index（含 templates、.claude/agents）无漂移、decisions/features 索引一致、CLAUDE.md shim 齐、PRD/测试用例账本一致。
- `make docs-audit` → 「✓ docs-audit 通过（检查了 43 篇带 frontmatter 的文档）」——related_docs / 链接无悬空。

## B. 控制面 ↔ 项目隔离（命根，blocker 维度）
- 隔离 grep（绝对路径，覆盖 skill + design.md + api-contract.md + reviewer .md + reviewer .toml，词表：多租户/tenant_id/tenant/kratos/订单退款/payment/coupon/优惠券）：
  - 共 6 行命中，**逐行核为禁止性守护文本**，无一处真把项目领域内容掺入：
    - templates/design.md:107「不预设任何具体项目模型（如不预设多租户隔离），项目真有再写」
    - SKILL.md:57「不预设多租户 / tenant_id / 某域名词…模板示例用中性占位」
    - hc-design-reviewer.md:36 / .toml:31「是否预设了项目没有的模型（如凭空塞多租户/tenant_id…）= 过度设计」
    - hc-design-reviewer.md:72 / .toml:65「方案里有没有混进 harness 控制面概念…反向越界」
  - 结论：命根隔离 OK，无 blocker。

## C. 硬原则落地（skill 正文 + reviewer 上下文）
- skill 正文逐条命中：参考项目代码（②「必读 projects/<工程>/ 真实代码与资产、基于现状、不凭空」）/ 不确定查+问（③.3 rule-0008）/ 决策点用户拍（③.4 + ⑧）/ 全明确才落零 TBD（④「定稿必须可执行、零 TBD/待确认」）/ 用户审核门（④「用户点头才算定稿」）/ 对抗评审（⑥ + ⑧「派 hc-design-reviewer 回改到零缺陷」）。末段「门槛（全明确/零 TBD/用户审核/对抗评审）不省」收口。
- reviewer 上下文「零 TBD 硬闸」双栈都有：.md L39-40 + .toml L34-35「定稿出现任何一个 = blocker」；七块判断标题双栈齐全（①–⑦）。

## D. 双栈齐 + 注册 + 对称
- .md frontmatter：tools = Read, Glob, Grep, Bash（无 Write）；无 model 行（用会话模型）。L68「tools 无 Write，绝不动产物」。
- .toml：model_reasoning_effort = "high"（L3）。
- 注册：.codex/config.toml L65「[agents.hc-design-reviewer]」+ L67 config_file 指向 .toml。
- 约束写进 agent 正文（非只靠模板）：双栈均含七块可执行抓法本体 + 工作步骤 + 原则；两栈内容对称（七块标题、零 TBD 闸、rubric、只评不改原则一一对应）。

## E. 模板可用性
- design.md：9 段齐（## ①–⑨ 实测枚举到位：背景&范围/业务流程/数据模型/接口设计/技术要点/关键决策+备选/影响范围/异常/安全&风险）。
- api-contract.md：端点索引表（2 中性占位端点 /v1/items）+ 每端点五段（描述+鉴权/请求/成功响应+Mock/错误响应/关联）；反向核「不列 500」——错误码表无任何 5xx 行；占位中性、无项目领域词。
- frontmatter source/related_docs 为模板尖括号占位（docs-audit 不报，正常模板形态）。

## F. 无冗余空壳 + 索引登记
- .codex/agents/ 下无多余 README.md（仅各 reviewer .toml）。
- skills-index README 含 hc-design 行；.claude/agents README 含 hc-design-reviewer；templates dir-index 含 design.md + api-contract.md；以上经 make verify 的对应 --check 确认无漂移。
- docs/designs/ 产物区：README + index.yaml（designs: [] 空账本 + 注释样例），结构干净。

# 综合

- 分档：**green**（010、011 均 pass，无 blocker / warn 级未决问题）。
- 总评：双栈对称、命根隔离、硬原则落地、模板可用、验证双绿，达到 L2+ 收尾门槛，可收尾。
