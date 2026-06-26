# 实施计划：kratos-base S5（nacos 后端 + 配置/注册运行期弹性）

> 验证/补全片。设计源：`docs/features/0005-...md` + ADR-0002 弹性模型澄清。验收断言遵守 rule-0009（锚定产出方证据）。

## 1. 概述

- nacos arm64 实测可用（`nacos/nacos-server:v2.5.0`，本机已 pull）。contrib `config/nacos`+`registry/nacos` 在 S4 已进 go.mod、代码已接（`bootstrap.NewConfigSource` 的 nacos 分支、`registryx` 的 nacos 分支）。本片主要是**起真 nacos 跑 e2e** + **补运行期宕/恢复场景**，少量代码（多半是 sandbox/脚本/配置）。
- 不做：etcd 启动快照（用户否决）、k8s 集群、rocketmq。

## 2. 步骤

### S5-T1 nacos 进 sandbox + nacos 功能 e2e（AC-N1/N2）
- `deploy/sandbox/docker-compose.yaml` 加 `nacos/nacos-server:v2.5.0`：`MODE=standalone`、`NACOS_AUTH_ENABLE=false`（沙箱关鉴权省事）、端口 8848+9848（2.x gRPC 端口）；healthcheck 打 `/nacos/v1/console/health/readiness`。`sandbox-up` 等它 healthy（nacos 启动较慢，retries 给足）。
- 复核/修 `configs/bootstrap.nacos-sandbox.yaml`（S4 草稿）：mode=nacos、server_addrs=127.0.0.1:8848、data_id=`runtime.yaml`(以 .yaml 后缀利于格式识别)、group=DEFAULT_GROUP、registry kind=nacos（或单测配置中心时 kind=local）。
- `scen_conf_nacos.sh`（S4 已草拟，NACOS_E2E 门控）：本机真跑——发布配置到 nacos → demo mode=nacos 启动 /readyz 200 → 改配置热更 → 坏配置回滚。把它纳入默认 run_all（nacos 现在本机可起，不再是可选）。
- `scen_disc_nacos.sh`（新）：kind=nacos——nacos 未起时 demo 照起+注册重试 WARN（AC-N2 非致命）→ 起 nacos → 自动注册 → discoveryprobe（已存在，传 nacos discovery）得 pong。
- 验证：两脚本本机真跑 EXIT=0；`make verify` 绿。

### S5-T2 配置/注册中心运行期宕/恢复（AC-CR1/CR2，etcd+nacos 参数化）+ 回归 + eval
- `scen_cc_runtime_down.sh BACKEND`（etcd|nacos）：sandbox-up + demo(mode=BACKEND) → /readyz 200、/v1/greet 200 → `docker compose stop <backend>` → **断言进程活 + /v1/greet 200 + /v1/ping 200 + /readyz 200**（继续用内存配置）→ `start <backend>` → 推一个配置变更 → **断言热更续上**（confcenter 版本++/行为变化，锚定产出方，不裸 grep）。
- `scen_reg_runtime_down.sh BACKEND`：sandbox-up + demo(registry kind=BACKEND) + discoveryprobe 通 → `stop <backend>` → **断言 discoveryprobe 调用失败 + /v1/ping 200 + 进程活** → `start <backend>` → 断言自动重注册 + discoveryprobe 又通。
- **若发现后端宕导致 watch goroutine 崩进程**：修 `confcenter.BindKratosWatch`/相关，确保 watch 错误只记日志不致命（这正是要做实的暗坑）。
- run_all.sh 纳入新场景（etcd+nacos 各跑）；**亲自从 harness 根跑全量**，S0~S4 零回归。
- 收尾 eval（slug `kratos-base-s5`），eval 会按 rule-0009/考题 012 审断言锚定。

## 3. 验证 runbook（映射 AC）

- [ ] `make -C projects/kratos-base verify` 绿
- [ ] AC-N1 nacos 配置：加载/热更/回滚
- [ ] AC-N2 nacos 注册：非致命+发现调通
- [ ] AC-CR1 配置中心运行期宕（etcd & nacos）：进程活+本地接口照常+恢复热更续上
- [ ] AC-CR2 注册中心运行期宕（etcd & nacos）：跨服务报错+本地照常+恢复重注册
- [ ] AC-REG run_all 全量（S0~S5）PASS
- [ ] 无残留容器；eval green/yellow

## 4. 失败模式与回滚

- nacos 2.5 鉴权/gRPC 端口（9848）配置不对 → 起不来：核 docker 日志、确认 8848+9848 都映射、auth 关。
- 后端宕暴露 watch 崩进程 → 修非致命（属本片目标，不算意外）。
- 回滚：本片全在工程内 + 不动既有代码语义（除非修 watch 暗坑）；删 nacos compose 段+新场景脚本即回滚。

## 5. 受影响 skill / 文档（rule-0007）

- 无需更新。收尾把 nacos 从 blocked 销账、CURRENT_STATUS/feature 对齐。
