# 摘要：kratos-base S5 eval

- 分档：**yellow → 复修第一手复验 → 置 done**
- 重点考题 012（rule-0009）抓出两处弱锚定，均已修：
  1. AC-CR1「热更续上」共因污染（坏 DSN+停 pg→503 区分不了热更 vs pg 宕）→ 改 **redis-addr flip**（pg/redis 不停、503 只可能来自 watch 落地后重建失败，正向无共因）。
  2. AC-CR2「自动重注册」启动期旧日志假命中（Runner 无重注册循环）→ 删该断言，恢复只认 **CR2-d 真 pong**，如实标注恢复靠注册中心 SDK 租约 keepalive。
- 010/003/002 pass(warn)；nacos 从误判 blocked 翻为真后端验过（v2.5.0 arm64）；全量 run_all 20 AC PASS。
- 教训：`tasks/lessons.md` 2026-06-23 共因污染断言。
- rule-0009/考题 012 首次实战即抓出同型问题，规则有效。
