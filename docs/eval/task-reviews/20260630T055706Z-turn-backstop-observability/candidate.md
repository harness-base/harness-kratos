# Candidate — turn-backstop 诊断日志 + 修「① capture 0 产出」静默失效

## 任务
给 `scripts/turn-backstop.sh`（落文档提醒 ① capture 通道）装诊断日志，并修「长期 0 产出」的静默失效。
档位：L2（todo `level: L2 ｜ task: turn-backstop-observability`）。

## 改动清单（git diff HEAD）

- `scripts/turn-backstop.sh`（+27/-8）
  - 新增 `DLOG="${BACKSTOP_DLOG:-$ROOT/tasks/.turn-backstop.log}"` + `dlog()` 写函数；超 800 行自裁到末 400。
  - 记录：未触发原因（skip）、FIRE 触发因、输入备妥（slice/checkmap 大小）、headless 调用参数、headless `exit` 码 + result 字节数 + 前 160 字、写没写 optimization-log。
  - headless claude 的 stderr 由 `2>/dev/null`（吞错）改为 `2>>"$DLOG"`（接进诊断日志）。
  - 默认预算 `BACKSTOP_BUDGET` 0.03 → 0.20。
- `scripts/turn-backstop.test.sh`（+15/-3）
  - case 5：未触发也写 DLOG（记 `skip 未触发`），dlog 失效则翻红（变异自证）。
  - case 6：诊断日志写 DLOG 不污染 optimization-log（未触发时 BACKSTOP_LOG 仍空）。
- `scripts/stop-check.test.sh`（+3/-1）：`run()` 补 `BACKSTOP_DLOG="$tmp/dl"`，隔离间接调用的诊断日志。
- `docs/harness/HOOKS.md`（+3/-1）：新增「诊断留痕」段，记 `tasks/.turn-backstop.log` 用途、stderr 接入、专治静默失效。
- `.gitignore`（+1）：加 `tasks/.turn-backstop.log`。
- `tasks/lessons.md`（+5）：2026-06-30 三段式教训（best-effort 全程 2>/dev/null 吞错 → 总失败伪装成静默 0 产出）。
- `tasks/todo.md`：当前任务块 + Review 段。

## 候选声称的根因
headless claude `--max-budget-usd 0.03` 对 ~14KB prompt 太低 → 每次 `Exceeded USD budget` 报错退出、0 产出，被 `2>/dev/null || true` 吞掉 → 静默失效；提到 0.20 后正常产出。
