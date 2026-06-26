# third_party protos (vendored)

这些 `.proto` 是 codegen 的**导入依赖**，从 Kratos v2.9.2 的 `third_party/` 整套复制而来
（`github.com/go-kratos/kratos/v2@v2.9.2/third_party`）。

为什么 vendor 而不是用 buf BSR（`buf.build/...`）依赖：
- 本环境**访问不到 BSR**（`buf dep update` 报 "the server hosted at that remote is unavailable"）。
- vendor 后 `buf generate` 完全离线可复现，不依赖外网与 buf.lock。

内容（导入闭包自洽）：
- `google/api/{annotations,http,field_behavior,httpbody,client}.proto` —— HTTP 注解（`protoc-gen-go-http`）。
- `errors/errors.proto` —— Kratos 错误模型注解（`protoc-gen-go-errors`，T2 用）。
- `validate/validate.proto`、`buf/validate/validate.proto` —— 校验注解（后续任务用）。
- `google/protobuf/descriptor.proto` —— 上述文件的根依赖。

**升级方式**：随 Kratos 版本走。bump `go.mod` 里 kratos 版本后，重新
`cp -R "$(go env GOMODCACHE)/github.com/go-kratos/kratos/v2@<ver>/third_party/." third_party/`，
再 `make generate` 确认无错。不要手改这些文件。

buf 配置（`buf.yaml`）把本目录登记为独立 module 仅用于**导入解析**；
`buf.gen.yaml` 对本目录 `managed.disable`（不重写它们的 `go_package`），且 codegen 只对 `api` module 产物
（`buf generate api`），故这里不会生成 `.pb.go`。
