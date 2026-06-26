# 当前任务

> 只记手头这一件事；干完清空、旧的 roll 进 `archive/`。保持轻。
> 元：level: L1 ｜ task: kratos-base-coordination
>
> **已完成（L2）harness-rules-distribution**：规则分布化——规则入驻各 `AGENTS.md`（编号保留）、catalog 自动生成（`scripts/rules-index.sh`，含 eval 指针校验 + 变异自证）、`CLAUDE.md` `@import` shim、eval 题库独立；ADR-0004；eval yellow→评后即修 findings 清零（`docs/eval/task-reviews/20260626T014408Z-harness-rules-distribution/`）。本会话另完成 prd-elicitation skill + 门禁（rule-0010/考题013，ADR-0003）。下面 kratos-base 段为暂挂的标准上下文。
> （切片间歇期。已完成切片：S0=L5 green、S1=L3 green、S4=L4 done、S3=L4 red→复修→复评 yellow 置 done、**S5=L4 yellow→复修→第一手复验 done**（nacos v2.5.0 真后端补全 + 配置/注册运行期弹性 AC-CR1/CR2）、**S6=L4 green**（rocketmq 本机 2 容器 e2e 销账 + 有界失败修复，全量 24 AC PASS，评审 `20260623T0600Z-kratos-base-s6`）。下一片立项时升档本行。）
> 下一步候选：k8s configmap/secret 配置源 + k8s 服务发现（需集群，kind/k3d 本机可起）；S2 obs 打磨 / 熔断限流打磨 / MQ backlog 按需开。

## kratos-base 基建（进行中）

### 已完成
- [x] S0 地基 + PG 弹性闭环（F-0001 done，eval green，`docs/eval/task-reviews/20260602T105017Z-kratos-base-s0/`）
- [x] S1 Redis（F-0002 done，eval green，`.../20260602T122049Z-kratos-base-s1/`）
- [x] S4 配置中心+服务发现四后端（F-0004 done，eval yellow→warn 修平，`.../20260611T100401Z-kratos-base-s4/`）
- [x] S3 MQ（F-0003 done：rabbitmq 闭环 e2e + rocketmq 适配器；**eval red 揭穿假阳性→复修 5 项→复评 yellow blocker 全核销**，`.../20260612T041709Z-kratos-base-s3/` + `.../20260612T050146Z-kratos-base-s3-rereview/`）

- [x] S5 nacos 后端补全 + 配置/注册运行期弹性（F-0005 done，eval yellow→复修→第一手复验，`.../20260623T032657Z-kratos-base-s5/`）
- [x] S6 rocketmq 本机轻量环境 + e2e 销账（F-0006 done，eval green，`.../20260623T0600Z-kratos-base-s6/`；含有界失败/消费者重建/ctx 修复）

### 余下切片（待用户定）
- [ ] k8s configmap/secret 配置源 + k8s 服务发现（需集群，kind/k3d 本机可起）；S2 obs 打磨 / 熔断限流打磨 / MQ backlog（胶水单测）按需开

## Review
- 升档理由补记（当时漏写，process miss 记入 lessons）：S0 按 **L5**（架构级新建工程：技术选型 + 弹性架构 + 全套地基）、S1 按 **L3**（标准功能切片，沿既有脊柱）。触发依据：用户要的产物（整套微服务基建）+ 目标文件（新建工程全量）。
- S0/S1 收尾 eval 均 green（rule-0005 已满足）；S0 遗留（atlas 正规化、AC5 trace_id 硬断言、metrics service_name）记录在 feature/ADR，未粉饰。
- git 仍未提交（用户选择"先不提交"）；工作区含 S0+S1+S3 备料共 ~10 项改动。
