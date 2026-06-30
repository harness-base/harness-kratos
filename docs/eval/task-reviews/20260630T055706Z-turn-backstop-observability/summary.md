---
task: turn-backstop-observability
level: green
prompts: ["002", "003", "010", "011", "012"]
generated: 2026-06-30T05:57:06Z
evaluator: hc-eval
---

# Summary — turn-backstop-observability

- 分档: green
- 考题: 002 pass / 003 pass / 010 pass / 011 pass / 012 pass
- 取证方式: 评委亲跑 make verify + 双向变异自证 + 真调 headless claude（0.005/0.03/0.20 三档）+ hermetic 复核，未采信声称。
- 关键证据:
  - make verify 绿，turn-backstop.test 6/0、stop-check.test 10/0。
  - 诊断日志实证: BACKSTOP_BUDGET=0.005 真调 claude → dlog 留痕 `exit=1 · Error: Exceeded USD budget`（原会被吞）；0.20 → `exit=0 · NONE`。
  - 守护变异自证: dlog 改空操作 → case 5/6 翻红；dlog 改写 LOG → case 6 翻红。还原后均绿。
  - hermetic: 真 tasks/.turn-backstop.log 全程未被测试写。
- 旁注（不扣分）: README.md 标题在 verify 期间被改（疑钩子/副作用，非本任务范围）；"0.03 必触发 Exceeded" 根因量级未完全复现（本机 0.03 实测 3/3 exit=0），属合理推断，方向正确。
- 给用户: 可收尾/提交。提交前确认 README.md 那处标题改动是否一并带上。
