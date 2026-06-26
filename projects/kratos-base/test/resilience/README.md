# resilience 场景说明

## 场景矩阵

| 脚本 | AC | 描述 |
|---|---|---|
| scen_boot_dep_down.sh | AC1 | 启动期依赖宕（PG 未起）、进程不崩 |
| scen_recover.sh | AC2 | 按需连 + 自愈（不重启） |
| scen_runtime_drop.sh | AC3 | 运行中断连 → 快速失败 → 恢复 |
| scen_config_hot.sh | AC4 | 配置热更（file）+ 坏配置回滚 |
| scen_observability.sh | AC5+AC6 | 可观测 + 网关链路 |
| scen_redis_boot_down.sh | AC-R1 | Redis 启动期宕、readyz/hits/greet 正确 |
| scen_redis_recover.sh | AC-R2 | Redis 恢复自愈、hits 计数递增 |
| scen_redis_drop.sh | AC-R3 | Redis 运行中断→快速失败→恢复 |
| scen_conf_etcd.sh | AC-C2 | etcd 配置热更 + config-flip 闭环 |
| scen_disc_etcd.sh | AC-D | etcd 注册非致命 + 发现闭环 |

## 运行全量

```bash
# 从 harness 根跑（workspace/verification.yaml e2e 路由）
bash projects/kratos-base/test/resilience/run_all.sh
```

## nacos 场景（已接，默认进 run_all）

**状态：DONE**（S5 起本机真跑）。早期误判"arm64 无镜像"是基于旧的 `v2.3.2`；实测
`nacos/nacos-server:v2.5.0` **有 arm64 多平台镜像**，已进 `deploy/sandbox/docker-compose.yaml`
（standalone、鉴权关、8848 HTTP + 9848 gRPC），`make sandbox-up` 会一并拉起并等 healthy。

默认矩阵已含 nacos 四条：
- `scen_conf_nacos.sh`（AC-N1：配置加载/热更/坏配置回滚）
- `scen_disc_nacos.sh`（AC-N2：注册非致命 + discoveryprobe 真 pong）
- `scen_cc_runtime_down.sh nacos`（AC-CR1：配置中心运行期宕→不崩→恢复续上）
- `scen_reg_runtime_down.sh nacos`（AC-CR2：注册中心运行期宕→discovery 失败→恢复又通）

单独跑：
```bash
bash test/resilience/scen_conf_nacos.sh
bash test/resilience/scen_cc_runtime_down.sh nacos
```

nacos 2.5 关键点：即便 `NACOS_AUTH_ENABLE=false` 也要设 auth token 三件套（见 compose）；
config 格式靠 dataId 后缀识别（`runtime.yaml`→yaml）；注册服务名带 scheme 后缀（`demo.grpc`）。
