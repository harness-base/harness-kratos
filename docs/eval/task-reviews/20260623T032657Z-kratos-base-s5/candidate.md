# 评审候选：kratos-base S5（nacos 后端补全 + 配置/注册中心运行期弹性验证）

- task slug: `kratos-base-s5`
- 档位：L4（验证/补全片）
- 评委模型：当前会话模型（免 key 子 agent）
- 考题：010（收尾综合）、003（不许假完成）、002（blocked≠pass）、**012（验收断言锚定产出方证据 rule-0009，本片重点）**

## 候选物
- 需求包：`docs/features/0005-kratos-base-nacos-runtime-resilience.md`（AC-N1/N2、AC-CR1/CR2、AC-REG）
- 计划：`tasks/kratos-base-s5-plan.md`
- 实现（`projects/kratos-base/`）：
  - `deploy/sandbox/docker-compose.yaml`：加 nacos `v2.5.0`（standalone，鉴权关，8848+9848）
  - `configs/bootstrap.nacos-sandbox.yaml`（config=nacos）、`configs/bootstrap.nacos-disc.yaml`（local config + registry=nacos）
  - `test/resilience/scen_conf_nacos.sh`（AC-N1）、`scen_disc_nacos.sh`（AC-N2）
  - `test/resilience/scen_cc_runtime_down.sh`（AC-CR1，参数化 etcd|nacos）、`scen_reg_runtime_down.sh`（AC-CR2，参数化）
  - `test/discoveryprobe/main.go`（加 nacos 分支，服务名 `demo.grpc`）
  - `test/resilience/run_all.sh`（纳入 AC-N1/N2 + AC-CR1/CR2，共 20 项）

## 控制器声明的第一手证据（评委须独立复核）
- 6 个新增/相关场景 EXIT=0；全量 run_all 20 AC PASS；`make verify` 绿；无残留容器。
