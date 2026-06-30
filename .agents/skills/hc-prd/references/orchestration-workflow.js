// hc-prd 编排模板（reference）。产品总监(主 agent)拿它当总谱、按需改后用 Workflow 跑。
// 确认门是人在环断点——Workflow 跑不了交互。实际用法 = 总监【分两段跑】：
//   段一 Gather+Stories(轻审收敛) → 返回 user-stories.md 给用户 approved；
//   段二 Produce+Review → 返回整套给用户收尾确认。本模板写在一起、用注释标确认门。
// 输入 args = { id:'<prd-id>', brief:'<采集到的原始材料/对话摘要>', needResearch:bool, needPrototype:bool }
// 注：本文件由 Workflow 工具跑——`export const meta` + 顶层 await/return 是 Workflow 约定（运行时把脚本体包进 async 函数）。
//     拿 node 当裸 ESM 模块 `--check` 会报"顶层 return 非法"=误报，非真语法错（包成 async 函数即过）。

export const meta = {
  name: 'prd-orchestration',
  description: 'hc-prd 编排模板：产品总监调度 worker 产出 用户故事→PRD→原型，带权重/确认门/review loop',
  phases: [
    { title: 'Gather', detail: '需求采集 + 可选外部调研' },
    { title: 'Stories', detail: '用户故事+AC + 轻审 loop' },
    { title: 'Produce', detail: '并行产出 PRD/功能点/原型' },
    { title: 'Review', detail: 'PRD 审稿重审 loop' },
  ],
}

const REVIEW = {
  type: 'object', additionalProperties: false,
  required: ['clean', 'findings'],
  properties: {
    clean: { type: 'boolean' },
    findings: { type: 'array', items: {
      type: 'object', additionalProperties: false,
      required: ['worker', 'severity', 'issue', 'fix'],
      properties: {
        worker: { type: 'string', enum: ['hc-user-story-writer', 'hc-prd-writer', 'hc-feature-point-writer', 'hc-prototype-builder'] },
        severity: { type: 'string', enum: ['blocker', 'major', 'minor'] },
        issue: { type: 'string' }, fix: { type: 'string' },
      },
    } },
  },
}

const id = (args && args.id) || '<prd-id>'
const dir = 'docs/prds/' + id

// ── 必选：需求采集 ──
phase('Gather')
const reqs = await agent(
  '你是需求采集员。把下面原始材料理成结构化需求摘要(JTBD/页面/数据/四态/边界/验收/非目标)+待确认清单。不静默假设。\n材料：' + ((args && args.brief) || ''),
  { agentType: 'hc-requirements-gatherer', label: 'gather' })

// ── 可选·权重低：外部调研(用户要查 / 有市场 SOP 且摸不透才跑) ──
//    走 deep-research skill(可用的 research skill)：通用 subagent 用 Skill 工具调它，不另建专属 worker。
let research = ''
if (args && args.needResearch) {
  research = await agent('你做外部调研：用 deep-research skill(Skill 工具)查市场 SOP/事实，过 rule-0008 验收后给带来源的要点。需求：' + reqs,
    { label: 'external-research' })
}

// ── 必选：用户故事+AC，轻审 loop(≤2 轮，只盯地基四项) ──
phase('Stories')
let stories = await agent(
  '你是用户故事+AC 员。把需求摘要写成 ' + dir + '/user-stories.md(US-NN + 可观测 AC)。\n摘要：' + reqs + (research ? '\n外部要点：' + research : ''),
  { agentType: 'hc-user-story-writer', label: 'write-stories' })

for (let round = 0; round < 2; round++) {
  const rv = await agent(
    '你是 PRD 审稿员【轻审地基模式】。审 ' + dir + '/user-stories.md 四项：AC 可观测/故事完整/内部一致/对齐采集。clean=true 表示无问题。',
    { agentType: 'hc-prd-reviewer', label: 'light-review:' + round, schema: REVIEW })
  if (rv.clean || !rv.findings.length) break
  stories = await agent(
    '你是用户故事+AC 员。按审稿发现修 ' + dir + '/user-stories.md：\n' + JSON.stringify(rv.findings),
    { agentType: 'hc-user-story-writer', label: 'fix-stories:' + round })
}
log('用户故事+AC 轻审收敛 → 确认门：总监把 user-stories.md 交用户 approved 再往下')

// ════ 确认门(人在环)：用户 approved user-stories 后才继续(总监在此停 / 本段作段一返回)。 ════

// ── 必选：PRD 本体先出(FP 单一权威：正文不造 FP 号)——功能点/原型都依赖成品 PRD ──
phase('Produce')
const prd = await agent(
  '你是 PRD 本体员。按已确认 ' + dir + '/user-stories.md 写 ' + dir + '/prd.md(套 templates/prd.md，四态/范围闭合/验收可观测)。正文按功能描述写，【不要自造 FP 号，FP 编号由功能点清单员统一造】。',
  { agentType: 'hc-prd-writer', label: 'prd' })
// ── 再并行：功能点(FP 唯一权威，读成品 PRD 抽号+锚映射) ∥ 可选原型(待确认默认全披露) ──
const produced = await parallel([
  () => agent('你是功能点清单员(FP 编号唯一权威)。读成品 ' + dir + '/prd.md 与 user-stories.md，抽功能点编号 FP-NN + 建 US↔FP↔正文 三级映射(只有你造 FP 号、正文列锚到 prd.md 段落)。',
    { agentType: 'hc-feature-point-writer', label: 'fp' }),
  ...((args && args.needPrototype) ? [() => agent('你是原型员。按 ' + dir + ' 的故事/成品 PRD 产可点原型到 ' + dir + '/prototype/。对任何待确认点取的默认须全部显式披露(页内+文件头)。',
    { agentType: 'hc-prototype-builder', label: 'prototype' })] : []),
])

// ── 必选：PRD 审稿重审 loop(框住并行产出；只重跑被审出问题的 worker；有轮数上限) ──
phase('Review')
// 本轮真跑过的 worker(用户跳过原型则不含 hc-prototype-builder)——审稿提示与重跑都只认这些，兑现"已留痕跳过不算缺口"
const ran = new Set(['hc-user-story-writer', 'hc-prd-writer', 'hc-feature-point-writer'])
if (args && args.needPrototype) ran.add('hc-prototype-builder')
const protoNote = (args && args.needPrototype) ? '含原型(查可点/四态)' : '本次用户未要原型，勿把"缺原型"当缺口'
let dry = 0
const MAX_ROUNDS = 4                     // 防审稿员持续挑刺不收敛：到顶即停并提示总监人工裁
for (let r = 0; r < MAX_ROUNDS && dry < 1; r++) {
  const rv = await agent(
    '你是 PRD 审稿员【重审下游模式】。审整套(' + dir + ' 的 stories+prd+功能点' + ((args && args.needPrototype) ? '+原型' : '') + ')：PRD 合故事/FP 映射齐/四态/范围闭合；' + protoNote + '。clean=true 表示无问题。',
    { agentType: 'hc-prd-reviewer', label: 'heavy-review:' + r, schema: REVIEW })
  if (rv.clean || !rv.findings.length) { dry++; continue }
  dry = 0
  const byWorker = {}
  for (const f of rv.findings) { if (ran.has(f.worker)) { (byWorker[f.worker] = byWorker[f.worker] || []).push(f) } }  // 只重跑真跑过的 worker
  if (!Object.keys(byWorker).length) { dry++; continue }   // 发现全落在没跑的 worker(如被跳过的原型)→ 视作无效、收敛
  await parallel(Object.keys(byWorker).map(w => () =>       // 回原 worker 角色重跑，只跑有问题的
    agent('你是 ' + w + '。按审稿发现修你产出的部分(' + dir + ')：\n' + JSON.stringify(byWorker[w]),
      { agentType: w, label: 'fix:' + w })))
}
if (!dry) { log('⚠ 重审到 ' + MAX_ROUNDS + ' 轮仍未收敛 → 交总监人工裁') }
log('整套 PRD 重审收敛 → 收尾确认：总监把整套交用户最终确认')

// ════ 收尾确认(人在环)：总监把整套交用户最终确认。 ════
return { id, stories, prd, produced }
