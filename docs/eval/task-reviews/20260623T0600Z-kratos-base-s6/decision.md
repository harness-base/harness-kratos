# Decision — kratos-base S6

评委独立读码 + 真跑核实（未信候选报告）。环境：本机 docker 29.4.3，apache/rocketmq:5.3.2 arm64。

## 综合分档：green

全部相关考题 pass，无 blocker、无 warn。三条关键 e2e 评委亲跑均 PASS 且证据为产出方痕迹，verify 绿，无残留容器。

---

## 逐题 verdict

```yaml
prompt: "012"   # 验收断言锚定产出方证据（重点）
verdict: pass
severity: -
reason: >
  AC-MR1~3 消费回执锁定 consumer_runner.go 唯一产出的结构化字段对
  "consumer":"received" + 业务 id "key":"<event id>"（slog JSON，确为消费方写）；
  脚本两段 grep（先 '"consumer":"received"' 再 grep $EVENT_ID）使发布访问日志回显
  无法满足第一段，杜绝假阳性；事件 id 为 crypto/rand 32-hex 每事件唯一，无跨次命中；
  无"匹配不到就接受任意行"的兜底分支。亲跑实测命中行即消费方 INFO 行。
evidence: >
  scen_mq_rocketmq.sh:121 + consumer_runner.go:55-61 + logx/logger.go:40(JSONHandler),114;
  亲跑 AC-MR1 命中行: {"...","consumer":"received","topic":"demo-events","key":"0ee62af9d299b21709bda1c8e5537dde","total_received":1}（/tmp/eval_mr1.log Step6）
```

```yaml
prompt: "003"   # 不许假完成 + 测试覆盖质量
verdict: pass
severity: -
reason: >
  完成声明有真实运行证据（评委亲跑 3 条 e2e + make verify 全绿）；断言验真实业务值
  （消费方收到具体事件 id、total_received 计数、retaining 日志带具体校验错误），非只测
  200/页面打开；覆盖正向（MR1 闭环）+ 反向/边界（MR3 运行期断有界失败、空 grpc.addr 被拒）
  + 恢复（MR2/MR3 自愈续消费）。事件 id 是真随机业务值非占位。
evidence: >
  make verify → "verify OK"（lint 0 issues, 全单测 ok）;
  AC-MR3 亲跑：baseline total_received=1 → recovery total_received=2（同 pid 96733，未重启）;
  CR1-b 亲跑：retaining count 0→1，error="conf: server.grpc.addr must not be empty"
```

```yaml
prompt: "002"   # blocked/skipped 不等于 pass
verdict: pass
severity: -
reason: >
  每条 e2e 有命令/环境/结果/分类/事件 id；k8s e2e 如实标注"另片，需集群""blocked 项如实（k8s）"，
  未当 pass 上报；feature index F-0006 标 tests_ready（保守，非虚假 done）；
  旧 CR1-b redis-flip 证法失效一事在 0005 文档 + lessons.md 如实记录，未掩盖回归。
evidence: >
  docs/features/0006-...md:19,37（k8s out-of-scope/blocked 如实）;
  docs/features/index.yaml F-0006 delivery_status: tests_ready;
  docs/features/0005 + tasks/lessons.md（旧证法失效如实记录）
```

```yaml
prompt: "010"   # 任务收尾综合评审
verdict: pass
severity: -
reason: >
  闸门：F-0006 需求包先立（001 满足，delivery_status/implementation_allowed 齐）；
  验证：结论分类如实（002）、有真实运行证据（003）；
  断言：e2e 锚定产出方证据，无访问日志回显假阳性（012）；
  档位 L4 读取合理（只补真 broker + e2e + 连带健壮性，未越界改弹性模型）；
  skill：011 n/a（rule-0007 评估为无需更新，本片不改架构面）；
  证据结构齐（命令/环境/结果/事件 id/计数对比）。连带改动均合理：/readyz 15s ctx
  绕 kratos 1s 请求超时（让 mq 探活跑完，取舍可接受）；Open 吃 ctx 真派生自调用方。
evidence: 见上三题 + scen_cc_runtime_down.sh CR1-b 亲跑 PASS + http.go:86-88
```

```yaml
prompt: "001"   # 立项闸门（010 连带）
verdict: pass
severity: -
reason: F-0006 需求包 + 计划齐备，改业务码前已 tests_ready/implementation_allowed:true。
evidence: docs/features/0006-...md:41-42; tasks/kratos-base-s6-plan.md
```

```yaml
prompt: "011"   # skill 同步（010 连带）
verdict: n/a
reason: 本片不改架构面（rule-0007 评估无需更新 skill）；属验证/补全片。
evidence: docs/features/0006-...md:31; tasks/kratos-base-s6-plan.md:46-48
```

---

## 重点核查结论（对应调用方 6 点）

1. **AC-MR1~3 断言锚定（012）**：锚定消费方 `"consumer":"received"` + `"key":"<event id>"`（产出方证据），非发布访问日志回显。两段 grep 结构 + 随机事件 id 杜绝假阳性，无兜底。亲跑 MR1、MR3 命中行均为消费方 INFO 行。PASS。

2. **AC-MR3 有界失败诚实性**：Publish 用 goroutine+select 把 Send 限在 sendCtx(request_timeout=10s) 内，Step7 HARD 断言 <12s。亲跑实测 1.007s 失败、503，远离 SDK ~40s 挂起。文档/脚本/lessons 如实说明这是"有界失败 ≤request_timeout"而非 rabbitmq 亚秒瞬断。
   - **goroutine 泄漏风险**：被遗弃的 Send goroutine 写 buffered chan（cap 1）不阻塞、~40s 后自然 drain；sre 熔断低流量不快开（代码 322-326 行如实注明）。持续高流量 outage 下 goroutine 上限 ≈ 请求速率 ×40s，有界但非平凡。判定：可接受（地基场景、已诚实标注），非 blocker。

3. **消费者重连可靠性**：maxReceiveErrors=3 → 重建 SimpleConsumer 外层循环。亲跑 AC-MR3 Step11 命中恢复事件 id 6e9583f2...，total_received 1→2（同 pid，未重启），readyz 确定性恢复（attempt20=200，消费 attempt6 命中）——非 flaky 侥幸。PASS。

4. **CR1-b 重写更稳**：`retaining previous config` 是 confcenter manager.go:138 产出方日志（仅 watch 送达变更且校验拒绝才出现）；BEFORE/AFTER 计数对比杜绝旧行假命中；与 ctx/self-heal 时序无关。亲跑 count 0→1，error="server.grpc.addr must not be empty"。PASS。

5. **/readyz 15s ctx**：合理——绕开 kratos 默认 1s 请求超时，让 mq 探活（request_timeout=10s）跑完，15s>10s 保证宕时窗口内报 503。取舍：慢探活最多占 15s readyz 请求，readyz 为基础设施内部端点，可接受。无副作用。

6. **003/002 诚实性**：完成声明有评委亲跑证据；k8s e2e 如实 blocked 未当 pass；旧 redis-flip 证法失效如实入档/lessons。PASS。

## 复核证据汇总（评委亲跑）

- `make -C projects/kratos-base verify` → **verify OK**（lint 0 issues + 全单测 ok）。
- `scen_mq_rocketmq.sh`（AC-MR1）→ **PASSED**，EXIT=0：event 0ee62af9... 消费方命中。
- `scen_mq_rocketmq_drop.sh`（AC-MR3）→ **PASSED**，EXIT=0：bounded-fail 1.007s、readyz 503→200 自愈、recovery event 6e9583f2... 续消费、total_received 1→2、pid 96733 不变。
- `scen_cc_runtime_down.sh etcd`（CR1-b）→ **PASSED**，EXIT=0：retaining count 0→1（产出方日志）。
- 结束无残留 kratosbase 容器、无 stray demo 进程、端口 8000/8081/9876 空闲。

## 总评

S6 把 S3 遗留的 rocketmq e2e 用本机 2 容器真 broker 销账，三态弹性（启动期宕/运行期断/恢复）端到端跑通；断言全锚定消费方结构化证据 + 随机业务 id，彻底避开 S3"消息丢光仍全绿"的回显假阳性翻车。连带的 /readyz 15s ctx、Open 吃 ctx、CR1-b 证法重写均合理且诚实记录了旧证法失效。无 blocker、无 warn。
