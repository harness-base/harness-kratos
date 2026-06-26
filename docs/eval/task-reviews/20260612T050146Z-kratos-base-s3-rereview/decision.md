# eval 决定：kratos-base S3（MQ）red 后复评

- 任务档位：L4（同首轮：T2 监督循环 + 双 MQ 适配器 + demo 发布/消费闭环 + e2e）
- 候选：`docs/features/0003-kratos-base-mq.md`（复修后版本）+ `projects/kratos-base/`（pkg/mq/rabbitmq、pkg/resource、scen_mq_* 修复）
- 上轮评审：`docs/eval/task-reviews/20260612T041709Z-kratos-base-s3/`（red：①Publish 路由键=随机事件 id 被默认交换机静默丢弃→消息 100% 丢失；②e2e 消费断言命中发布侧 HTTP 访问日志→假阳性）
- 评审时间：2026-06-12（UTC 20260612T050146Z）
- 评审方式：独立复核——逐项读修复代码 + 亲跑 `make verify`、`go test ./pkg/mq/... ./pkg/resource/... -race -count=1`、`scen_mq_recover.sh`（EXIT=0）、`scen_mq_drop.sh`（EXIT=0）+ 独立活体探针（publish→rabbitmqctl 队列侧验证→消费回执 key==id）+ run_all 日志真伪鉴定（内嵌 wall-clock 时间戳 vs 修复文件 mtime）

## 上轮必修项逐项核销（先于考题 verdict）

1. **Publish 路由（必修①）：已修，代码+活体双重确认。**
   - 代码：`pkg/mq/rabbitmq/rabbitmq.go:218-229` — exchange=""、routingKey=`m.Topic`、`MessageId: m.Key`；`adaptDeliveries`（:345）对称把 `d.MessageId` 映射回 `Key`；空 topic fail-loud（:174）。`Config` 只剩 URL+DialTimeout（:41-46），Exchange/Queue 死字段在 go/yaml/测试全树零残留（grep 核实，conf.go/runtime*.yaml 无旋钮，sandbox yaml 注释明示"无 exchange/queue 旋钮"）；上轮的空操作 exchange-bind 死代码块已消失。
   - 活体（评委亲测 12:58 +08:00）：POST /v1/events → `{"id":"39dd8b0ec5e125def4468e36d650afe4"}`；3 秒内消费者结构化日志 `"consumer":"received","topic":"demo.events","key":"39dd8b0ec5e125def4468e36d650afe4","body_len":24`（key 与 publish 返回 id 逐字符相等，body_len=24=len("evaluator-probe-rereview")）；`rabbitmqctl list_queues name messages messages_unacknowledged consumers` → `demo.events 0 0 1`（消息已消费已 ack，非丢失）。上轮同款探针的结论（队列 0 条、0 回执）已反转。
2. **e2e 断言 id 锚定（必修②）：已修，无兜底残留。**
   - `scen_mq_recover.sh:183-201`：id 提取失败→硬 FAIL；断言=双重 grep `'"consumer":"received"'` AND `<事件id>`。
   - `scen_mq_drop.sh:97-122`（baseline）+ `:258-288`（recovery）：同款双锚定，baseline 同改。两脚本通读无任何"任意 consumer 行算过"分支。
   - 锚定有效性核实：检查真实访问日志行（component:http，args 回显 payload）——既无 `"consumer":"received"` 标记也无事件 id（响应体不入访问日志），双锚定在结构上不可能再被回显满足。
3. **重跑（必修③）：评委亲跑全过，run_all 日志鉴定为真。**
   - `scen_mq_recover.sh` EXIT=0：消费回执 key=`7f144417d3db14429a5764c040f78c81`==该次 publish 返回 id（评委在脚本轮询期间直接从 /tmp/demo_mq_recover.log 第一手捕获该行，12:53:03）。
   - `scen_mq_drop.sh` EXIT=0：baseline 回执 `1c11219c...`、恢复回执 `4cc28e8d...` 均 key==id；快速失败 0.006890s（熔断开）；全程同 pid 17490；demo 日志仅 2 条 consumer 行、key 各对应其 publish。
   - `/tmp/run_all_final.log` 真伪鉴定：单次流式跑（birth 12:45:08、内嵌 compose 时间戳 12:45:17→12:47:51、mtime 12:47），晚于最后一次代码修改（rabbitmq.go mtime 12:43:48）→ 全量 14 AC 用最终代码跑出，AC-M2/M3 回执行同样 id 锚定（如 key=3f1ada9f... 12:47:40）。"2.5 分钟跑完 14 AC"经评委实测交叉验证成立（评委亲跑 drop 场景 sandbox-up 12:53:41→PASSED 12:53:54，本机确实这么快）。

## 追加两缺陷修复核销

4. **死句柄自失效 getLiveConn：已修。** `rabbitmq.go:84-97`（IsClosed→`p.Close()` 失效缓存→立即重 Get 一次），发布路径 :181 与消费 ConnectFn :281 双路径生效。功能性证据：drop 场景 broker 弹后**不重启**恢复发布+消费（旧逻辑消费端会永卡死连接）。
5. **发布方幂等 QueueDeclare + Persistent：已修。** `rabbitmq.go:200-210`（durable 队列、与消费侧同参）+ `:226`（DeliveryMode=Persistent）；rabbitmqctl 确认 `demo.events durable=true`。recover 场景（全新 broker、消费者重连窗口）EXIT=0 即该路径的端到端行使。

## 上轮 warn 项复核

- rocketmq 注释：已修。`rocketmq.go:13-18` 包注释与 README"Architecture"段均改为源码核实结论（Start 对不可达端点无限阻塞→goroutine+RequestTimeout+GracefulStop 兜底），旧"无网络 I/O"说法零残留。
- provider 单测：已补。`TestProvider_HealthFail_ClosesAndRebuilds`（provider_test.go:523）真实断言"探活失败→关闭死句柄（close 计数=1、handle.closed）→下次 Get 重建（build 计数=2、新句柄）"，-race 亲跑 PASS；且 `Healthy()` 失败路径现关闭死句柄（provider.go:117-124），上轮指出的"每故障周期泄漏一个句柄"一并修掉。
- adaptDeliveries 停机滞留：**未修**（ctx 取消+out 缓冲满时发送阻塞）。上轮已定级"不阻塞、影响极小（停机路径）"，维持 warn。

## 逐题 verdict

```yaml
prompt: "003"   # 不许假完成 + 测试覆盖质量（rule-0003）
verdict: pass
severity: warn   # pass 但附 warn 备注
reason: 完成声明全部有评委可复现的第一手运行证据；上轮假阳性断言已改为结构上无法被访问日志满足的双锚定；发布→消费闭环经三个独立通道证实（recover 场景、drop 场景、评委手工探针+队列侧验证）。
evidence: |
  - make -C projects/kratos-base verify → ">> verify OK"（评委亲跑）。
  - go test ./pkg/mq/... ./pkg/resource/... -race -count=1 → 4 包全 ok，含新增
    TestProvider_HealthFail_ClosesAndRebuilds；仅 1 个如实 skip（rocketmq Receive 循环退避需真 broker）。
  - scen_mq_recover.sh EXIT=0 / scen_mq_drop.sh EXIT=0（评委亲跑）；消费回执行均为消费者
    结构化日志且 key==该次 publish 返回 id；访问日志行（component:http）实测不含标记与 id，
    双锚定无法假阳性。
  - 评委独立探针：publish id 39dd8b0e... → 消费回执 key 同值、body_len 与探针 payload 等长；
    rabbitmqctl：demo.events durable=true、消费后 0 messages 0 unacked 1 consumer。
  - warn 备注（不阻塞）：①新增薄胶水行为（空 topic 守卫、MessageId↔Key 映射、getLiveConn、
    发布方声明）无专门单测，仅靠 e2e 覆盖——AMQP 薄层靠真 broker e2e 可接受，但空 topic
    守卫目前无任何测试行使；②监督循环重试在运行时零日志（见 010 warn）。
```

```yaml
prompt: "002"   # blocked / skipped 不等于 pass（rule-0002）
verdict: pass
severity: warn   # 口径合格；沿用上轮对 blocked 面的认定
reason: 上轮"未验证子结论并入 pass"的问题已消除——本轮候选的每条 pass 主张（EXIT=0、id 锚定回执、14 AC、queue 驻留）评委均独立复现或鉴定属实；rocketmq E2E 仍如实标 blocked（feature 状态段+README 写明触发方式），未冒充。
evidence: |
  - 候选状态段主张"AC-M1~M3 独立跑 EXIT=0（id 锚定）"与"全量 run_all 14 AC PASS"：
    评委重跑 M2/M3 复现；run_all 日志经 birth/mtime/内嵌时间戳鉴定为修复后单次真跑。
  - blocked 面：F-0003 范围预声明 rocketmq E2E 不包含；pkg/mq/rocketmq/README.md Status 段
    "E2E: BLOCKED — require namesrv+broker"+跑法；单测 skip 注明 "requires a live broker"。
    blocked 全程未被并入 pass。
  - 证据结构：verify/unit/e2e 命令按 workspace/verification.yaml 路由；场景输出含命令/结果/
    case id（AC-M*）；评委可据此完整重放。
```

```yaml
prompt: "010"   # 任务收尾综合评审（rule-0005）
verdict: pass
severity: warn
reason: 上轮 red 的两个 blocker（路由键、断言假阳性）+ 追加两缺陷均真修真验，核心交付"发布/消费闭环端到端证明"现成立且可独立复现；剩余为 warn 级（AC-M3"退避重试日志可见"子项未满足、若干小覆盖缺口），可有条件收尾。
evidence: |
  - 001 闸门：pass（沿上轮认定：F-0003 于 commit 0c7237a 先行入库，implementation_allowed:true；
    本轮为同一 feature 的缺陷修复，无新立项义务；index.yaml 仍 verified、按流程待 eval 后置 done——
    时序正确，没有抢跑置 done）。
  - 002 / 003：pass（见上）。
  - 004 档位：n/a 维持（调用方未列，主修复 ~1 文件+2 脚本，无超载迹象）。
  - 011 skill/架构回顾：pass。本轮无新架构层（修复轮）；tasks/lessons.md 新增两条高质量教训
    （"访问日志回显假阳性"含 Prevention：日志断言必须锚定产出方结构化字段+队列侧证据，
    与本次修复一一对应；"池类依赖掩盖探活缺口"沿上轮）。
  - warn（收尾条件，均小）：
    ① AC-M3 原文含"消费者监督循环退避重试（**日志可见**）"——RunSupervised 与 rabbitmq
       ConnectFn 均无日志，drop 窗口 demo 日志只有 http 503 访问日志，零条重试痕迹；
       重订阅事实由更强信号证明（同 pid 下恢复后回执 total_received:2），但该括号子项
       照原文未满足：要么给监督循环补一条 reconnect/attempt 日志，要么修订 AC 文本并留痕。
    ② 新增胶水行为缺单测（见 003 warn①）。
    ③ adaptDeliveries 停机滞留 goroutine 维持上轮 warn 未修（影响极小）。
```

## 综合分档：yellow（可有条件收尾）

**总评**：上轮 red 的全部必修项与追加缺陷都通过了"读代码 + 亲跑 + 队列侧对照"的独立复核——消息现在真实进队列、被消费、被 ack（rabbitmqctl 可见 durable 队列与 0 积压），断言在结构上不再可能被访问日志骗过，run_all 日志经时间戳鉴定确为修复后真跑；与上轮"皮肉是假的"相比，本轮闭环是真的。不足为 green 的只剩 warn：AC-M3 的"退避重试（日志可见）"子项照原文不成立（监督循环运行时零日志，重试不可观测），加上两处小覆盖缺口。

**收尾条件（满足其一即可处理 ①，②③ 留 backlog 即可）**：
1. AC-M3"日志可见"：给 RunSupervised（或 demo 的 Backoff 包装）补一条结构化 reconnect 日志并在 drop 场景顺手断言；**或**修订 F-0003 AC-M3 文本删去"（日志可见）"并注明"以恢复后回执+同 pid 为准"。
2. 空 topic 守卫、MessageId↔Key 映射的小单测（backlog）。
3. adaptDeliveries 停机滞留（backlog，维持上轮结论）。

**给用户的一句话**：S3 这轮是真修好了——发布→消费闭环三路独立验证全通、队列侧有据，处理掉 AC-M3"重试日志可见"这个一行级子项（补日志或改 AC 文本）即可置 done；rocketmq E2E 继续如实挂 blocked 待 broker 配置。
