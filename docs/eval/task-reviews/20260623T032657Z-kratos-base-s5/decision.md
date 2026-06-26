# 评审结论：kratos-base S5

## 初评分档：yellow（有条件收尾，须先修两处 rule-0009 弱锚定）

恢复能力本身为真、e2e 真跑通；但本片重点考题 012 上有实打实的弱锚定 + 未如实标注，按"宁可误报不漏报"判 warn（非 blocker，恢复另有充分旁证）。评委独立复跑 `scen_cc_runtime_down.sh nacos`、`scen_reg_runtime_down.sh etcd`、`make verify` 核实，非空信报告。

## 逐题 verdict

| 考题 | verdict | severity | 理由 |
|---|---|---|---|
| 010 收尾综合 | pass | warn | 立项/计划/状态闸门齐、回归与 verify 真跑；断言层有 012 弱锚定故综合压 yellow |
| 003 不许假完成 | pass | warn | 完成声明有可复核证据（独立复跑+verify+无残留）；扣分点在 CR1「热更续上」证法不充分却表述为已证 |
| 002 blocked≠pass | pass | warn | blocked 项如实（etcd 启动快照不做/k8s/rocketmq）；nacos 从误判 blocked 翻为真验且复核成立 |
| **012 断言锚定** | **fail** | **warn** | 见下两处 |

### 012 命中两处弱锚定
1. **AC-CR1「恢复后热更续上」共因污染**：测法"推坏 DSN + 同时停 pg → /readyz 503"。pg 已停则即便坏 DSN 未经 watch 落地、503 照样出现——503 无法区分"热更落地"与"pg 宕"。且 confcenter 成功 reload 静默不打日志，demo 日志全程无任何 reload/连接痕迹，证据链里没有一行能证明坏 DSN 真落到运行中的 provider。同 S3 案的"非产出方痕迹满足断言"同型问题。
2. **AC-CR2-c「自动重注册」启动期旧日志假命中**：恢复后 `grep "service registered"` 命中的是**启动期**那行（时间戳在停机之前）；且 `pkg/registryx` Runner 注册一次即 break+park、**无重注册循环**。恢复"续上"真正成立靠 CR2-d（discoveryprobe 恢复后又得真 pong，靠注册中心 SDK 租约 keepalive 自愈）——这条干净充分；CR2-c 的措辞与证法错位。

### 锚定良好（确认无问题）
- discoveryprobe 的 "pong" 是真产出方证据（Ping handler 固定返回，经 discovery 真 RPC 才拿得到，不可能是入参回显）。
- "retaining previous config"、registryx WARN/INFO 均为真产出方日志行。
- 各脚本无"匹配不到就降级兜底"分支。

### warn（非 blocker）
- `scen_disc_nacos.sh` WARN grep 宽 alternation（建议收紧到精确串）。
- 工程根残留 34MB 构建产物 `discoveryprobe`（建议 gitignore）。
- docker-compose `version` 过时字段（无害）。

---

## 复修后复评（控制器修复 + 第一手复验，2026-06-23）

评委两处 012 findings 已全部修复并经控制器第一手复跑确认：

1. **AC-CR1 改 redis-addr flip（pg 无关正向证）**：配置中心恢复后，推一个把 `data.redis.addrs` 改成坏地址（`localhost:6390`，**redis 容器仍在 6379 运行、pg 不停**）的配置 → /readyz 翻 503。唯一触发路径=新配置经 watch 落地 → provider 按坏地址重建失败 → redis 探活失败；pg/redis 容器均活着，排除共因。再推回好地址 → 200（重建成功，热更又落地一次）。第一手复跑：etcd+nacos 均 EXIT=0，CR1-b 503 / CR1-c 200。
2. **AC-CR2 删 CR2-c，恢复证据只用 CR2-d**：删除"自动重注册"日志 grep（行为不存在）；恢复证据只认 discoveryprobe 恢复后真 pong（CR2-d），并如实标注恢复机制是注册中心 SDK 租约 keepalive、非 app 重注册。第一手复跑：etcd+nacos 均 EXIT=0，CR2-a 无 pong / CR2-d 真 pong。
3. 小项全修：disc_nacos grep 收紧为精确串；`discoveryprobe` 入 `.gitignore`（git status 不再跟踪）；删 compose `version` 行。
4. 教训入 `tasks/lessons.md`（2026-06-23 共因污染断言）。

**复修后状态：两处 012 弱锚定核销，断言改为产出方/正向无共因证据。最终分档 yellow（已修平）→ 置 done。**
