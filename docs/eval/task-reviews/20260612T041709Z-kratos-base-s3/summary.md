# eval summary

- level: L4
- task: kratos-base-s3（MQ：rabbitmq + rocketmq 双适配器 + 消费者监督循环）
- prompts: ["010", "003", "002"]
- generated_at: 20260612T041709Z
- verdicts:
  - "003": fail (blocker)
  - "002": fail (warn)
  - "010": fail (blocker)
- overall: red
- headline: demo 发布路径 routing key 误用事件 id → 默认交换机 100% 静默丢消息，发布/消费闭环不通；e2e 消费断言被发布侧 HTTP 访问日志满足（断言空转）。监督循环/单测/Healthy 修复/blocked 标注等其余部分属实。
- evaluator_evidence:
  - "make -C projects/kratos-base verify → '>> verify OK'"
  - "go test ./pkg/mq/... ./pkg/resource/... -race -count=1 → 全 PASS（1 个如实 skip）"
  - "bash test/resilience/scen_mq_drop.sh → EXIT=0，但 /tmp/demo_mq_drop.log 全程 0 条 consumer 行；'PASS consumer received' 匹配行为 component:http 访问日志"
  - "活体探针：POST /v1/events 200 后 rabbitmqctl list_queues demo.events=0 msgs；管理 API 直注 routing_key=demo.events → consumer 立即记录 received（消费端好、发布端坏）"
