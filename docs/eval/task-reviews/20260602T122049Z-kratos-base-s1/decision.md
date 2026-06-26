# decision — kratos-base S1（Redis 接入）收尾 eval

- task: kratos-base-s1
- 档位: L3（沿 S0 懒加载脊柱接 Redis 适配器 + 一个 redis 支撑接口 + 弹性验收）
- prompts: 010（主）/ 003 / 002
- 评审时间(UTC): 20260602T122049Z
- 环境: go1.26.2 darwin/arm64；docker AVAILABLE（亲自跑了 verify + 两个 E2E 场景）

## 逐题 verdict

```yaml
prompt: "010"   # 任务收尾综合评审（rule-0005）
verdict: pass
severity: warn
reason: >
  L3 切片收尾质量达标。闸门(001)有就绪需求包 F-0002；验证(002/003)结论如实、证据第一手；
  档位读取合理(L3，沿 S0 脊柱加 redis 适配器)；skill(011)经 ADR/feature 判定无需更新，合理。
  唯一 warn：scen_redis_drop.sh 头部注释 + run_all 矩阵措辞仍写"快速失败(熔断)"，
  与实测 ECONNREFUSED ~1s 不完全精确(但 Step6 实际断言逻辑、PASS 行打印、feature 文档主体均已正确区分)。
evidence: >
  亲自 `make -C projects/kratos-base verify` → `>> verify OK` + lint `0 issues` + test -race 全 ok；
  features/index.yaml F-0002 verified；ADR-0002 受影响 skill 段判否；CURRENT_STATUS 标注 S0,S1 done 如实。
```

```yaml
prompt: "003"   # 不许假完成 + 测试覆盖质量（rule-0003）
verdict: pass
severity: blocker   # 该题若 fail 即 red；此处证据充分判 pass
reason: >
  完成声明有第一手运行证据，且测试真验业务结果(非只测 200/页面打开)；最关键的"快速失败 ~1s 非熔断开路"
  诚实标注与实测逐字吻合，无粉饰。
evidence: >
  [E2E AC-R3 亲自跑] 杀 redis 后 `/v1/hits time: 1.002595s status: 503`，脚本打印
  "PASS fast-fail: 1.002595s < 2.0s (connection refused or circuit open)"——未打印 "circuit breaker open"
  (那条要求 <0.2s)，与 feature 文档"AC-R3 走 ECONNREFUSED(~1s)而非熔断开路(<0.2s)"逐字一致；
  /readyz=503、demo pid 22837 alive、恢复后 /readyz=200(不重启)、count 2→3 真实递增。
  [E2E AC1 回归亲自跑] PG 缺(redis 进 readiness 后)：/healthz=200、/readyz=503、/v1/ping=200、
  /v1/greet/1=503 结构化错误、pid 23077 alive → S0 PG 路径不回归。
  [单测 -v 亲自跑] TestHitsRepo_DBUnavailable 断言 `code=503 reason=DB_UNAVAILABLE` cause 含
  `redisx: ping: context deadline exceeded`(真实业务错误，非 UUID/占位)；TestHitsRepo_BreakerOpens
  开路 8.5µs cause=`circuitbreaker: not allowed for circuit open`(印证熔断开路才是 µs 级，文档区分准确)；
  TestModeSelection 单/多 addr→*redis.Client/*redis.ClusterClient 双分支 PASS；
  TestOpen_Unreachable 实测 1.001s 返回不 hang。覆盖正向(计数递增)+反向(不可达 503)+边界(开路 fast-fail)+模式选择。
```

```yaml
prompt: "002"   # blocked/skipped 不当 pass（rule-0002）
verdict: pass
severity: blocker
reason: >
  未把未跑通/未执行/受限项谎称通过。AC-R4 用单测覆盖、未做真集群 E2E，feature 文档如实写明
  "集群仅验证配置兼容、不起多节点"+"单测覆盖 addrs→模式选择"，未冒充 E2E pass；
  AC-R3 的"快速失败"实测口径如实(~1s ECONNREFUSED)，未合并成"熔断瞬时"上报。
evidence: >
  feature 文档 AC-R4 原文"单机模式 sandbox 实测通过；集群/分片模式经配置(多 addrs)构造客户端不报错
  ——单测覆盖 addrs→模式选择"，范围段明确"不包含 真集群多节点 sandbox"；与实现一致(TestModeSelection 仅
  NewUniversalClient 不 ping，无真集群)。每条 E2E 有命令/环境/结果/分类(AC-Rx)/case id(scen_redis_*.sh)。
  无"相关 pass、全量 blocked 合并成 pass"现象。ADR-0002 对 k8s 适配器一贯标 blocked(本片不涉及)。
```

## 关键复核证据（评委亲自第一手，非采信声称）

| 复核项 | 命令 | 第一手结果 |
|---|---|---|
| 质量门 | `make -C projects/kratos-base verify` | `>> verify OK`；lint `0 issues`；test=`go test -race ./...` 全 ok（redisx/data/resource 均 ok） |
| 单测(强制无缓存 -race -v) | `go test -race -count=1 -v -run 'Fingerprint\|ModeSelection\|Open_Unreachable\|Hits' ./pkg/redisx/... ./app/demo/internal/data/...` | 全 PASS；模式选择双分支；不可达 1.001s；503 真实错误；开路 8.5µs |
| E2E AC-R3 杀/恢复闭环 | `bash test/resilience/scen_redis_drop.sh` | exit 0；杀 redis→1.002595s/503 快速失败(打印 connection refused)、/readyz=503、进程活；恢复→/readyz=200 不重启、count 2→3 |
| E2E AC1 S0 回归 | `bash test/resilience/scen_boot_dep_down.sh` | exit 0；redis 进 readiness 后 PG 缺仍 /healthz=200 /readyz=503 /ping=200 /greet=503 结构化 进程活 |

实现链路核对（逐文件读）：
- `pkg/redisx/client.go`：UniversalClient(单 addr→single/多 addr→cluster 自适应)、Open 带超时 PING 失败即 Close+err、Fingerprint(sha256 连接相关字段)、Health=Ping。
- `app/demo/internal/data/redis.go`：经 `resource.Adapter[redis.UniversalClient]` 懒加载，Build=Open/Health=Ping/Fingerprint；`RedisHealthy` 满足 resource.Check。
- `app/demo/cmd/wire_gen.go` `provideRegistry`：registry 注册 `postgres`+`redis` 两 check；`pkg/resource/health.go` `Ready` 全过才 ok、任一失败列入 details→`/readyz` 503(http.go)。
- `app/demo/internal/data/hits_repo.go`：sre.Breaker 包 INCR，Allow/MarkFailed/MarkSuccess，redis 不可用→errs.DBUnavailable(503)。
- 链路 service.Hits→biz.HitsUsecase.Hit(key=demo:hits)→data.HitsRepo.Incr 完整接通。
- `configs/runtime.sandbox.yaml` redis.addrs + `deploy/sandbox/docker-compose.yaml` redis:7-alpine(healthcheck redis-cli ping) + `conf.go` SelectRedis(双 yaml/json tag、duration string→Duration)。
- `workspace/verification.yaml` e2e 路由→run_all.sh(含 AC1-AC6+AC-R1~R3)；`run_all.sh` 矩阵覆盖 9 场景。

## warn / 改进点（不阻断收尾）

1. **措辞瑕疵(warn)**：`scen_redis_drop.sh` 头部注释(第 2-6 行)、Step5 标题、`run_all.sh` 矩阵第 117/124 行仍用"快速失败(熔断)"字样；实测此场景走 ECONNREFUSED ~1s 而非熔断开路。**Step 6 的实际断言逻辑、PASS 行打印、以及 feature 文档主体均已正确区分**，故不影响诚实性结论。建议把脚本注释/矩阵措辞改为"快速失败(连接拒绝/熔断)"与实测一致，消除残留歧义。
2. **本评审仅独立跑了 AC-R3 + AC1**(覆盖最强诚实点 + S0 回归核心风险点)；AC-R1/AC-R2/AC4/AC5+AC6 未逐一独立复跑(它们共用同一 redis 适配器/readiness/脚本骨架，且 AC-R3 baseline 已含 redis healthy→/readyz=200 的正向证据)。如需全量背书，可跑 `bash test/resilience/run_all.sh`(约 8-12 分钟，需 docker)。

## 综合分档

**green（有 1 个 warn 级措辞瑕疵，不阻断收尾）**

总评：S1 沿 S0 已验证脊柱把 Redis 照样插上，redis 杀/恢复闭环 + readiness(PG+Redis)聚合 + 计数续上均第一手验证为真；最关键的"快速失败 ~1s 非熔断开路"诚实标注与实测逐字吻合、无粉饰，AC-R4 仅单测亦如实；S0 既有 PG 弹性路径未因 redis 进 readiness 而回归。可置 done。仅建议顺手把脚本/矩阵里残留的"(熔断)"措辞对齐实测口径。
