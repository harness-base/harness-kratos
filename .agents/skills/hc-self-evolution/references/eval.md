# eval 维度审查手册

审查 harness 的「质量闸门」是否真闸得住：L2+/关键点收尾该评必评、规则有考题守、考题↔规则不悬空、评审不走过场。

## 规范（健康长什么样 / 不变量）

- **该评必评（rule-0005）**：L2+ 任务与关键决策点（能不能开工 / 测试够不够 / 验证结论分类对不对）收尾前必过 eval；L0/L1 不触发。
- **独立评委，不靠自评**：评分由 `docs/eval/evaluator.md` 设定的独立评委按 `docs/eval/rubric.md` 打分，默认怀疑"已完成/已通过"，只认可复核证据。
- **题库独立、按号引规则**：考题在 `docs/eval/prompts/`，登记在 `docs/eval/index.yaml`，每道按 `rule-00NN` 关联规则（多数考题盯一条 rule）。
- **blocker 规则有考题守**：红线规则（尤其 blocker 级）应有对应考题钉死翻车点；规则与考题双向不悬空——index 登记的考题文件都在、prompts 里的考题都登记进 index、AGENTS.md 的 `eval:` 标记都指向存在的考题。
- **免 key 可跑**：默认走 hc-eval 子 agent（`.claude/agents/hc-eval.md`，Codex 用 `.codex/agents/hc-eval.toml`），免 API key；`make eval`（`scripts/run-eval.sh`）是可选 CI/headless 路径。两条路写同样的 `task-reviews/` 产出，Stop hook 只认产出在不在。
- **产出结构齐**：每次评审落 `docs/eval/task-reviews/<时间戳>-<task>/`，含 `candidate.md` / `decision.md`（逐题 verdict + 综合分档）/ `summary.md`。

## 怎么检索现状

```bash
# 资产结构 + 双向登记自检（核心入口；index↔prompts 不悬空）
bash scripts/verify-eval-materials.sh            # 或 make verify-eval

# AGENTS.md 的 eval: 标记是否指向存在的考题（防"凭空指针"，已进 make verify）
bash scripts/rules-index.sh --check

# 考题登记表（id ↔ rule ↔ file）
cat docs/eval/index.yaml

# 哪些规则有 eval 守、哪些 eval:[] 裸奔（按 severity 看 blocker 是否裸奔）
grep -nE 'id:|severity:|eval:' docs/rules/index.yaml

# 现有考题与评审产出
ls docs/eval/prompts/        ls docs/eval/task-reviews/
```

- 评委设定 / 评分口径：`docs/eval/evaluator.md`、`docs/eval/rubric.md`。
- 触发口径 / 跑法：`docs/eval/README.md`。

## 怎么判

- **缺口·规则无守护考题**：`docs/rules/index.yaml` 里 `severity: blocker` 的规则 `eval: []` 裸奔（无考题钉翻车点）→ 缺口，按号补考题。注意区分：rule-0006/0008 这类靠 hook/流程拦的可不配考题，但**行为类 blocker** 必须有。
- **漏洞·指针悬空**：`bash scripts/rules-index.sh --check` 或 `verify-eval-materials.sh` 报错 = 标记引用不存在的考题、或 prompts 未登记进 index → 立即修，别让闸门带病。
- **漏洞·考题↔规则对不上**：`index.yaml` 里某考题 `rule:` 指向的规则在 AGENTS.md 已删/改号/改义，考题却没跟 → 悬空，对齐或下线。
- **缺口·考题牵强/过时**：考题判据无法被证据证伪（"看着合理就 pass"）、或锚的实现已变（脚本/字段重命名）→ 收紧判据或更新（参考 012 的"锚定唯一真实信号"写法）。
- **缺口·该评没评**：L2+ 任务收尾 `task-reviews/` 无对应产出，或 `decision.md` 缺 verdict/证据/综合分档 → 走过场，补评。
- **符合**：两个机器检查全绿 + blocker 行为规则均有考题守 + 抽查一道考题判据能据证据判定且锚的实现仍在。

## 常见漏洞模式（本仓真实案例）

- **断言被"回显"骗过，eval 才揭穿**（`lessons.md` 2026-06-12 / `task-reviews/20260612T041709Z-kratos-base-s3/`）：e2e 用裸 payload 全文 grep，命中的是发布请求 HTTP 访问日志的回显入参，实际路由键错误致消息 100% 丢失。实现+复跑都被骗，独立 eval 评委 `rabbitmqctl` 查队列+注入对照才抓出 → 沉淀为考题 012，复评见 `20260612T050146Z-kratos-base-s3-rereview/` 反转结论。**启示：考题判据要锚"只能由处理方产出的结构化字段+业务 id"，裸串 grep 一律打回。**
- **指针凭记忆迁移、悬空**（`lessons.md` 2026-06-26）：规则分布化时给 rule-0005/0006/0008 编了不存在的 eval 指针（005/006/008）、rule-0007 severity 私改，hc-eval 子 agent 逐条 `git show HEAD` 对比判 yellow。**修复固化：把"eval 标记必须指向存在考题"加进 `rules-index.sh --check` 并变异自证。**
- **"假收敛"——绿了但机制脆弱**（`lessons.md` 2026-06-11 redis-flip / s3 系列 14 轮对抗）：断言虽 PASS 但靠"超时巧合"成立，非干净因果 → 催生考题 012 的共因污染/超时竞态/无守护测试三类假阳性专项清单。**启示：考题不仅判"绿没绿"，要判"绿得是不是真行为"。**
- **eval 走过场 / 大改没回顾**（`lessons.md` 2026-06-11 eval-011 blocker fail）：ADR 漏"受影响的 skill"栏、context-loading 没声明 → 011 直接判 blocker。**启示：收尾评审要逐项要 verdict+证据，不接受"应该没问题"。**

## 修复用哪个操作 skill / 脚本

- **加/改考题或补规则守护** → `hc-add-rule` skill（定范围→写下来+登记→挂执行），新增考题后跑 `verify-eval-materials.sh` 确认双向登记。
- **重生成规则索引 / 校验 eval 指针** → `bash scripts/rules-index.sh`（重生成）/ `--check`（只校验，进 `make verify`）。
- **补评 / 重评** → hc-eval 子 agent（`.claude/agents/hc-eval.md`，免 key）；CI/headless 用 `make eval ARGS="..."`。产出落 `docs/eval/task-reviews/`。
- **结构自检** → `bash scripts/verify-eval-materials.sh`（`make verify-eval`）；整体控制面自检 `make verify`。
- **发现晋升归档** → `hc-self-evolution` skill / `hc-self-optimize` 子 agent（把捞到的缺口/漏洞从 log 晋升到规则/考题/lessons，不许烂在 `tasks/optimization-log.md`）。
