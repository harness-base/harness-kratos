---
title: 当前真实状态
status: active
owner: harness
last_updated: 2026-06-27
source_files: []
related_docs:
  - PROJECT_BRIEF.md
---

# 当前真实状态

状态口径：`done`(已建实) / `skeleton`(空壳已就位) / `planned`(暂未建)。

## 控制面

| 模块 | 状态 | 说明 |
|---|---|---|
| 入口（AGENTS/CLAUDE/README/Makefile） | done | |
| `tasks/`（todo / lessons / archive） | done | |
| `docs/context/` | done | 简报 / 状态 / 按需加载（含就近 AGENTS.md 加载规则） |
| `docs/rules/` | done | 规则**分布化入驻各 AGENTS.md + catalog 自动生成**（`rules-index.sh`，`--check` 进 make verify 防漂）；全局 + kratos 就近规则**清单/计数以 `docs/rules/index.yaml` 为准**（不硬编码枚举，rule-0012） |
| `docs/decisions/` | done | ADR 架构决策记录；**清单以 `docs/decisions/index.yaml` 为准**（index-audit 守，不硬编码枚举，rule-0012） |
| `docs/eval/` | done | 考题 / rubric / 评委 / 产出目录 |
| `docs/harness/` | done | 验证路由 / CI / hooks（含 Stop hook）说明 |
| `scripts/` | done | verify / docs-audit / run-eval / verify-eval / install-hooks / hook-policy(+test) / skills-index / **rules-index / dir-index / index-audit / prds-audit / test-cases-audit(+test)** / stop-check / **turn-backstop(+test)** |
| `.githooks/` + `.github/workflows/` | done | 带测试的 hook policy + CI |
| `.agents/skills/` | done | 技能集**以 `.agents/skills/README.md` 为准**（`skills-index` 从各 `SKILL.md` 自动生成、`--check` 进 `make verify` 防漂移，故此处不再硬编码枚举）；含 hc-prd（编排式，ADR-0010：产品总监 + 6 worker 双栈 subagent + 外部调研走 deep-research skill）、hc-self-evolution（带 references 审查手册）等 |
| `.claude/` | done | settings（PreToolUse + Stop hook）+ 子 agent（**以 `.claude/agents/README.md` 为准**，自动索引；如 hc-eval / hc-code-reviewer）+ skills 软链 |
| `.codex/` | done(部分) | 子 agent 与 `.claude/agents/` **一一双栈对齐**（各有 `.toml` + `config.toml` 注册，行为一致）+ config；其余按需 |
| `docs/features/` | done | F-0001~0006（kratos-base 6 个需求包） |
| `workspace/verification.yaml` | done | kratos-base 路由已填全（verify/unit/e2e/sandbox + 20 AC 弹性矩阵），含示例模板 |
| `projects/` | done | 挂载点，已挂 kratos-base（详见被管工程表） |
| `docs-maintainer` skill | planned | 待接入（写文档 / 管文档） |
| `docs/prds/` | done | 需求产出账本（hc-prd skill 产物 + prds-audit）；architecture 暂未建（drift 区已弃，见 ADR-0006） |
| `docs/test-cases/` | done | 测试用例账本（hc-test skill 产物（ADR-0014） + `test-cases-audit` 硬闸校 AC/FP 覆盖闭合，ADR-0008）；空账本待实战 |
| 自进化（① 落文档提醒 + ② hc-self-evolution） | done | `turn-backstop.sh`（机械触发落文档提醒，写 `- [ ]` 状态）+ `correction-nudge` 下一轮反馈待处理 + 文档漂移判据 `docs/harness/doc-sync-checklist.md` + `hc-doc-sync-reviewer` 子 agent（ADR-0012）+ `hc-self-evolution` skill/references + `hc-self-optimize` 子 agent |
| sandbox / E2E 环境 | done | kratos-base 已建实（`projects/kratos-base/deploy/sandbox` + `verification.yaml` 路由，20 AC 弹性 e2e 跑通）；新工程随接随建 |

## eval 怎么跑（免 key）

默认用 **hc-eval 子 agent**（`.claude/agents/hc-eval.md` / `.codex/agents/hc-eval.toml`），用当前会话模型打分，无需 API key；`scripts/run-eval.sh` 是可选的 CI / headless 路径（需 `EVAL_API_*`）。

## 被管工程

| 工程 | 状态 | 说明 |
|---|---|---|
| `projects/kratos-base` | done(S0,S1,S3,S4,S5,S6) | Kratos 微服务地基。**S0**：脚手架+codegen、配置两段式(file)、日志/错误/可观测(otel+prom)、懒加载脊柱 `resource.Provider`+健康探针、PG/ent、demo+中间件链+熔断、HTTP 入口；AC1–6 验收过（eval green）。**S1 Redis**：同脊柱，`/v1/hits`+readiness 含 PG+Redis；AC-R1~3 过（eval green）。**S4 配置中心+服务发现**：四后端配置源(file/etcd/nacos/k8s，bootstrap/INFRA_MODE 选)、注册/发现(local/etcd/nacos/k8s)+非致命注册 Runner、`pkg/backends` 接入层；etcd e2e 全闭环（配置热更/config-flip/坏配置回滚 + 注册非致命/discovery 调通），全量 run_all 11 AC PASS；nacos e2e blocked(arm64 无镜像，脚本已备)、k8s e2e blocked(无集群)；eval yellow→warn 修平置 done（`20260611T100401Z-kratos-base-s4`）。**S3 MQ 已接**（F-0003 done：rabbitmq 全闭环 e2e（AC-M1~M3 id 锚定回执）、rocketmq 适配器+单测（E2E blocked 待 broker）、监督循环+死句柄自失效+发布方声明队列防丢；eval red→复修→复评 yellow blocker 全核销）。**S5-T2 运行期弹性**：新增 AC-CR1(etcd/nacos) 配置中心运行期宕→进程不崩→恢复热更续上、AC-CR2(etcd/nacos) 注册中心运行期宕→discovery 失败→恢复自动重注册续上；watch goroutine 实测非致命（kratos config 库错误回路 continue，不 panic）；全量弹性矩阵 **20 AC PASS**（含 S0~S5-T2 全量回归）。**S6 rocketmq**（F-0006 done，eval green）：本机 2 容器轻量 rocketmq（namesrv + broker 内嵌 proxy，JVM 压 256/512m）；AC-MR1~3 全闭环（publish→consume id 锚定回执、启动期宕自愈、运行期断**有界失败~1s**+监督循环重建 SimpleConsumer 续消费）；连带修 Publish goroutine 限时吃 ctx、pgxpool/redisx.Open 吃调用方 ctx、/readyz 15s 探测、CR1-b 证法重写为 confcenter `retaining` 计数对比；全量 **24 AC PASS**（S0~S6）。**评审硬化（**14 轮对抗评审，~70 真问题修平**）**：每轮多维并行出 findings + 每条独立 agent 对抗证伪 + 修复进下一轮直到零；全部新增测试 mutation 自证 load-bearing。**两段**：R1-R7 第一循环（覆盖/单测牵强/e2e牵强/正确性）收敛 11→17→4→1→1→1→0——但**单轮零=假收敛**（视角钝化）；R8 起换【怀疑式新视角】（并发 TOCTOU/优雅关停时序/热更一致性/可观测/中间件链/注册生命周期/安全/k8s/文档漂移/infra + **复查自身修复**）又挖出 36+ 真问题（含我方先前的 DLQ 假修与两条空转测试、registryx backoff 溢出）二次收敛 5→7→7→11→4→1→0。尽除 goroutine 泄漏(rocketmq Start 加 dialReachable 预检 / rabbitmq adaptDeliveries ctx-select)、EnableSsl 包全局 data race(set-once)、e2e 假阳性(共因/WARN放行/裸串/松正则全清，改 confcenter `config applied` 计数等产出方硬证)、牵强测试(断路器/重连/自愈/契约改硬断言 + mutation 自证 load-bearing)；覆盖率 conf 47→98.5 / data 57→81.7 / rabbitmq 31.6→45 / rocketmq 49.7→53.5 / confcenter→97.4 / registryx→81；`go test -race ./...` 干净 + 全量 24 AC PASS 复核。 |
