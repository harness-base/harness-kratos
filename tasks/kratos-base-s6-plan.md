# 实施计划：kratos-base S6（rocketmq 本机轻量环境 + e2e 销账）

> 验证/补全片。设计源：`docs/features/0006-...md` + ADR-0002（MQ 监督循环弹性）。断言遵守 rule-0009。

## 1. 概述

- rocketmq adapter + 单测 S3 已落（`pkg/mq/rocketmq`、`app/demo` 的 SelectMQ kind 分支）。本片只补"真 broker + e2e"。
- `apache/rocketmq:5.3.2` arm64 已 pull。最轻量完整 = namesrv + broker(`--enable-proxy` 内嵌 proxy) 2 容器，JVM 堆压到 256m/512m。

## 2. 步骤

### S6-T1 rocketmq 进 sandbox + bootstrap + 起得来
- `deploy/sandbox/docker-compose.yaml` 加 `rmqnamesrv` + `rmqbroker`：
  - namesrv：`command: sh mqnamesrv`，`JAVA_OPT_EXT=-server -Xms256m -Xmx256m -Xmn128m`，端口 9876。
  - broker：`command: sh mqbroker -n rmqnamesrv:9876 --enable-proxy -c <broker.conf>`，`JAVA_OPT_EXT=-server -Xms512m -Xmx512m -Xmn256m`，端口 10911+8081，depends_on namesrv。
  - `deploy/sandbox/rocketmq/broker.conf`：`brokerClusterName=DefaultCluster`、`brokerName=broker-a`、`brokerId=0`、`namesrvAddr=rmqnamesrv:9876`、`brokerIP1=127.0.0.1`、`autoCreateTopicEnable=true`、`autoCreateSubscriptionGroup=true`。
  - healthcheck：namesrv 查 9876、broker 查 8081 端口可连（proxy 起）。`sandbox-up` 等 broker healthy（rocketmq 起较慢，retries 给足）。
  - **JVM 坑**：JAVA_OPT_EXT 追加在默认 `-Xmx8g` 之后、靠后者覆盖；若仍 OOM 起不来，sed 改 runbroker.sh 的硬编码堆。
- `configs/bootstrap.rocketmq-sandbox.yaml`：mq.kind=rocketmq、rocketmq.endpoint=127.0.0.1:8081、topic、consumer_group、await/request_timeout 合理值。
- 验证：sandbox-up 后 broker healthy，golang v5 客户端能连 8081（先 smoke：起 demo mode=rocketmq /readyz 200）。

### S6-T2 rocketmq e2e（AC-MR1~MR3）+ 回归 + eval
- 复用 rabbitmq 的 `scen_mq_*.sh` 结构，做 rocketmq 版（参数化或新脚本）：
  - **AC-MR1** publish→consume：HTTP 触发发布 → 消费方结构化回执含事件 id（rule-0009，不裸串/不取访问日志回显）。
  - **AC-MR2** 启动期宕→不崩+本地接口照常+readyz 反映 → 起 broker 自愈续上。
  - **AC-MR3** 运行期断→快速失败不崩 → 恢复监督循环重连续消费。
- topic/group：autocreate 应够；若消费拿不到消息，用 `mqadmin updateTopic -t <topic> -c DefaultCluster` + `updateSubGroup -g <group>` 兜底。
- run_all.sh 纳入 AC-MR1~MR3；**全量回归 S0~S6**；收尾 eval（slug `kratos-base-s6`，过考题 012）。

## 3. 验证 runbook（映射 AC）

- [ ] `make -C projects/kratos-base verify` 绿
- [ ] AC-MR1 publish→consume id 锚定
- [ ] AC-MR2 启动期宕→不崩→自愈
- [ ] AC-MR3 运行期断→快速失败→恢复续消费
- [ ] AC-REG run_all 全量（S0~S6）PASS
- [ ] 无残留容器；eval green/yellow

## 4. 失败模式与回滚

- broker JVM OOM 起不来 → 压堆（JAVA_OPT_EXT / sed runbroker.sh）。
- 客户端连不上 proxy → 核 brokerIP1=127.0.0.1、端口 8081 映射、endpoint 配置。
- 消费拿不到消息 → mqadmin 手建 topic/subgroup。
- 回滚：删 rocketmq compose 段 + broker.conf + 新场景脚本即回滚（不动既有代码语义，除非修适配器暗坑）。

## 5. 受影响 skill / 文档（rule-0007）

- 无需更新。收尾把 rocketmq 从 blocked 销账、CURRENT_STATUS/feature 对齐。
