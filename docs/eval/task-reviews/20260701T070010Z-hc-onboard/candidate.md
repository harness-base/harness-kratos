# candidate — hc-onboard（新项目接入 skill）build

任务档位：L3（收尾闸，rule-0005）。task slug = `hc-onboard`。

## 候选产物清单

- ADR：`docs/decisions/0017-hc-onboard-project-onboarding.md`（+ `docs/decisions/index.yaml` 第 84 行登记 ADR-0017）
- skill：`.agents/skills/hc-onboard/SKILL.md`（进 `.agents/skills/README.md` 索引第 10 行）
- reviewer 双栈：`.claude/agents/hc-onboard-reviewer.md` + `.codex/agents/hc-onboard-reviewer.toml`（`.codex/config.toml` 第 77 行 `[agents.hc-onboard-reviewer]` 登记）
- 模板：`templates/project-agents.md`
- 机检：`scripts/verification-audit.sh` + `scripts/verification-audit.test.sh`（进 `scripts/verify-control-plane.sh` 第 103 行 → `make verify`）
- 接线：`.codex/config.toml`、`workspace/verification.yaml`、`docs/harness/VERIFICATION_ROUTING.md`（第 31 行三态）、`docs/harness/PROJECT_ONBOARDING.md`（瘦身 40 行、0 流程步）、`docs/context/CURRENT_STATUS.md`（第 33 行指向 hc-onboard、清 docs-maintainer）

## 设计要点（ADR-0017）

引导式 skill、本期只做「新项目」分支（7 步、每步先确认再落、只搭壳不越界、占位守三态、派 reviewer 对抗评审）；create-sandbox 拆下一轮；接入点占位机检 fail-closed；支持多工程共存。

## 候选自称的证据（评委已独立复跑，见 decision.md）

- `make verify` ✓（exit=0）
- `make docs-audit` ✓（48 篇）
- `verification-audit.test` 17/0（含 fail-closed 假绿守护 + 多工程逐工程测试）
- 两轮对抗评审 + 用户追加「多工程兼容」后补：机检改 fail-closed、PROJECT_ONBOARDING 真瘦身、CURRENT_STATUS 清 docs-maintainer、多工程隔离。
