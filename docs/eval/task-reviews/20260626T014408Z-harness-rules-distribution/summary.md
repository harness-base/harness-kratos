---
level: yellow
task: harness-rules-distribution
prompts: ["010", "011"]
generated_at: 2026-06-26
remediated: true
---

# summary

规则分布化的**机制层扎实、诚实性达标**：scanner 忠实生成 catalog（`--check` 无漂移）、shim 三处 1:1 成立、rule-00NN 编号全保号（全仓 171 处引用未断）、`make verify` 与 `make docs-audit` 自跑均真绿（exit 0，非声称）；rule-0001 的 MUST STOP、rule-0009 的共因/竞态/守护测试三牙、rule-0010 的"不强制 PRD 存在"例外，正文层均无丢失。

评审时 yellow，三处问题：(1) **blocker** — ADR-0004 缺 `templates/adr.md` 强制的"受影响的 skill（rule-0007）"栏，且 `context-loading` 未回顾未声明，命中 eval-011 判失败口径；(2) **warn** — rule-0007 severity 被 warn→blocker 偷改，与 ADR"severity 全保留"矛盾；(3) **warn** — rule-0005/0006/0008 的 eval 标记指向不存在的 prompt 005/006/008，catalog 忠实继承了坏指针。

**评后即修（remediated）**：三项 + 1 处 minor 全部修平，并把"eval 指针必须指向存在考题"固化进 `rules-index.sh --check`（变异自证 load-bearing）。修后 `make verify`/`docs-audit` 真绿、catalog severity/eval 与 HEAD 一致。详见 decision.md 末"复修记录"。最终状态：findings 清零。
