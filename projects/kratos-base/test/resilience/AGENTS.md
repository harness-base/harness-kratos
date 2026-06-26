# test/resilience 就近规约（test/resilience）

弹性验收 e2e（scen_*.sh + run_all.sh）。本文件是本包红线；工程全局红线见 `../../AGENTS.md`，断言总则见 rule-0009。

## 红线
- **日志断言用 BEFORE/AFTER 计数对比（grep -cF），禁裸 grep 一次**：单次 grep 可能命中恢复前旧行，掩盖热更/配置变更未真落地；计数增量才证明新增。（scen_conf_etcd.sh:25、scen_cc_runtime_down.sh:330-342；lessons 2026-06-23） <!-- rule: kratos/test-before-after-count | sev: blocker -->
- **避免共因污染：一个脚本别同时"推坏配置"和"停依赖容器"**：两动作共用同一信号（如 503）就无法区分"热更落地"vs"容器宕"；改单变量做唯一路径证明。（scen_cc_runtime_down.sh 注释；lessons 2026-06-23） <!-- rule: kratos/test-avoid-common-cause | sev: blocker -->
- **断言锚定产出方结构化字段，禁裸 payload 全文 grep**：请求日志会回显参数，裸 grep 可能命中 HTTP 访问日志而非真实消费；用 key==id + consumer:received 配对。（scen_mq_drop.sh:104-122；lessons 2026-06-12） <!-- rule: kratos/test-producer-evidence | sev: blocker -->
- **快速失败设硬数值边界，WARN ≠ PASS**：rule-0009 §C 禁 WARN/continue 通过已声称的保证；结果必须明确 PASS/FAIL，WARN 只能是旁证。（scen_mq_drop.sh:168-177；lessons 2026-06-24） <!-- rule: kratos/test-warn-not-pass | sev: blocker -->
- **scen_*.sh 开头立即 cd 工程根，CWD 无关**：脚本从 harness 根调用，假设 CWD=项目根会让相对路径失效；硬写 `cd "$(dirname "$0")/../.."`。（run_all.sh:9-15；lessons 2026-06-02） <!-- rule: kratos/test-cwd-invariant | sev: warn -->

## 指针
- 断言总则（rule-0009 锚定）：`../../AGENTS.md`
- 验收信号设计：`../../../../docs/decisions/0002-kratos-base-architecture-and-resilience.md`
- 蒙混完整复盘：`../../../../docs/eval/task-reviews/20260612T041709Z-kratos-base-s3/`
- 错题本：`../../../../tasks/lessons.md`
