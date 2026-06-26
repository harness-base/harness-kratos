# kratos-base

首个被管工程：从 0 搭的 Kratos v2 微服务 monorepo 地基。
目标是用标准件组合（库自愈连接池 + Kratos 中间件 + k8s 探针 + 薄 provider）做到**依赖懒加载、
不宕服务、故障快速失败、恢复自动续上**——而非自研大框架。

- 模块路径：`github.com/z-mate/kratos-base`
- 设计与版本矩阵：`../../docs/decisions/0002-kratos-base-architecture-and-resilience.md`
- 实施计划（分片 T1…T9）：`../../tasks/kratos-base-s0-plan.md`

> 当前进度：**S0~S6 全 done**（需求包 F-0001~F-0006）。demo 服务（proto/biz/data/service）完整、
> PG/Redis/MQ 弹性闭环跑通；sandbox 起七服务（postgres + redis + etcd + rabbitmq + nacos +
> rmqnamesrv + rmqbroker）；`test/resilience/run_all.sh` 覆盖 24 个 AC（S0~S6）全绿，`make verify` 绿。
> 唯一如实 blocked：k8s e2e（本地无集群，另片）。

## 目录结构

```
projects/kratos-base/
├── go.mod / Makefile / buf.yaml / buf.gen.yaml / .golangci.yaml / .gitignore
├── api/demo/v1/        服务契约 .proto + 生成的 *.pb.go（入库）
├── app/demo/           demo 服务：cmd 入口 + internal（biz/data/service/server/conf）
├── third_party/        vendored 的 codegen 导入依赖（Kratos/google 注解 proto）
├── pkg/                共享标准件（resource provider / mq / errs / backends / confcenter…）
├── configs/            bootstrap.yaml（选配置源/注册中心/MQ 后端）+ runtime 配置
├── deploy/             sandbox docker-compose（七服务）+ migrations
└── test/resilience/    弹性 e2e 场景 + run_all.sh（24 AC 验收矩阵）
```

## 工具链

本机已具备：Go 1.26、buf（CLI v1，用 v2 配置）、protoc-gen-go / -go-grpc / -go-http / -go-errors、
golangci-lint v2、atlas、docker/compose。

一键安装 go 系 codegen / DI 工具（protoc 插件 + wire）：

```bash
make init
```

`make init` **不装** buf CLI 与 atlas CLI（用系统包管理器或官方脚本装）：

```bash
# buf（任选其一）
brew install bufbuild/buf/buf
# atlas
curl -sSf https://atlasgo.sh | sh
```

## 常用命令

```bash
make generate    # buf 生成 proto Go 代码 + ent 生成代码；产物入库
make wire        # 生成 wire 依赖注入代码（app/demo/cmd/wire_gen.go）
make build       # 编译 service 二进制到 bin/
make test        # go test -race ./...
make lint        # golangci-lint
make vet         # go vet
make verify      # 收口质量门：build + vet + lint + test
make migrate     # 生成 atlas 迁移（受 ent schema 在 internal 包限制，见 deploy/migrations/README.md）
make sandbox-up / sandbox-down   # 本地 sandbox（postgres+redis+etcd+rabbitmq+nacos+rmqnamesrv+rmqbroker 七服务）
```

## codegen 说明（重要）

- proto 生成走 **buf v2 配置**：`buf.yaml` 把 `api`（产出目标）与 `third_party`（仅导入解析）登记为两个
  module；`make generate` 实际跑 `buf generate api`，`.pb.go` 落在 `.proto` 旁、入库。
- **不使用 buf BSR（buf.build）依赖**：本环境访问不到 BSR，故 `google/api`、Kratos `errors` 等注解 proto
  整套 vendored 在 `third_party/`（来自 `kratos/v2@v2.9.2/third_party`）。升级随 Kratos 版本走，
  见 `third_party/README.md`。
- 生成代码与 ent gen 一律**入库**（Kratos 惯例），不要手改。

## 验证

最小收口：

```bash
go build ./...
make generate    # 应无错，且生成物与入库一致（幂等）
make verify      # build + vet + lint + test 全绿
```

控制面侧的验证路由见 `../../workspace/verification.yaml`。
