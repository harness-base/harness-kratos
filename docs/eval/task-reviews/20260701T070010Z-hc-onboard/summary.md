---
task: hc-onboard
level: green
prompts:
  - "010"
  - "011"
  - "014"
  - "rule-0015"
generated_at: 2026-07-01T07:00:10Z
evaluator: hc-eval
verdict_summary: |
  green（有一条 warn 级 follow-up）。全部相关考题 pass；make verify(exit=0) / docs-audit(48) /
  verification-audit.test(17/0) 三条硬证据评委亲手复现全绿。机检真进 make verify pipeline
  （verify-control-plane.sh:103）。评委独立造 fixture 双向验证：fail-closed 对危险占位真堵住
  （带注释空PENDING / 裸N/A / 全角省略号 / 混合大小写ToDo / FIXME / 尖括号 / 三工程末位静默空
  全判红），且不误杀真命令（命令含 pending/TODO 子串正确保持绿）；多工程逐工程核成立。
  唯一 follow-up：F-1 单引号 YAML 空值（'  '）绕过 fail-closed（clean() 只剥双引号）——
  边界 warn，现仓库全用双引号故不影响真实守护、不阻断收尾，记后续补 clean() 单引号剥离 + 自测。
---

# summary — hc-onboard build eval

- **level**: green（可收尾；一条 warn follow-up）
- **prompts**: 010（收尾综合）、011（skill 同步 rule-0007）、014（索引不硬编码 rule-0012）、rule-0015 隔离侧检
- **generated_at**: 2026-07-01T07:00:10Z

逐题 verdict + 独立复核证据见 decision.md，候选副本见 candidate.md。
