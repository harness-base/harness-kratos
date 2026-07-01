# 接口契约：<design-id> <标题>

> 由 `hc-tech-design`（研发方案/技术设计阶段）产出，与 `design.md` 同目录落 `docs/designs/<id>/api-contract.md`。是 `design.md` ④接口设计 的展开正文（那里只放链接 + 一句话概述 + 设计原则，字段级细节都在这里）。
> **下游消费**：api 测试用例**逐字段对照测试**——请求参数表、成功响应字段表、错误码表都是 api 用例的「覆盖矩阵行」+断言来源，所以这里写歪 / 漏字段，下游就测歪 / 漏测。**设计阶段确认每个字段·每个 JSON 结构**，把理解偏差消解在落码之前（rule-0008）。
>
> **填写约定**：① 顶部端点索引表是总览，每加 / 删一个接口都同步它。② 每个接口一块，字段表**逐字段**写全（名 / 位置 / 类型 / 必填 / 约束）。③ 成功响应必给 **Mock 样例**（真实可解析的 JSON，给前端 mock + api 用例当夹具）。④ 错误响应**只列约定内的错误**（业务码 / 校验码，及**约定的服务态**如 503 `DB_UNAVAILABLE`）；**未约定的未预期故障**（裸 500 / panic）不进契约。**别按状态码段一刀切**——契约边界是「约定 / 未约定」，不是「4xx / 5xx」。⑤ 返回结构统一（同一套 envelope / 错误外壳，别一个接口一个样：本模板统一用 `{ "data": ... }`——列表 `data` 为数组、单资源 `data` 为对象）。
> **协议逃生口**：本模板默认 **HTTP-REST 视角**（Method / Path / HTTP 状态码 / Bearer / query·body）；**gRPC / 消息 / 异步**项目按实际换——`Method/Path` 换成 rpc 名 / topic，HTTP 状态码换成对应错误模型（如 gRPC status / enum reason），请求参数「位置」可为 `grpc-field` / `header` / `path`。字段名是通用占位，按工程实际形态填。
> 本模板示例用**中性占位端点**（`/v1/items` 列表 + 创建），**不带任何具体项目领域词**；`hc-tech-design`：照此为真实接口逐块重填，删掉占位说明。

---

## 通用约定

> 把全接口共用的约定写一次，各接口块不重复。`hc-tech-design` 按项目实际填；没有的项删掉、别留空壳。

- **Base URL / 版本**：`/v1`（版本放路径前缀，下方端点 path 均省略 base）。
- **鉴权**：默认 `Authorization: Bearer <token>`（按项目实际填；公开接口在该接口块标「鉴权：无」）。
- **统一返回外壳**：成功 / 失败用同一套结构（下例为占位约定，按项目定，全接口一致）：
  - 成功：统一包一层 `{ "data": ... }`——列表 `data` 为数组、单资源 `data` 为对象（本模板取此一套，全局一致；项目若另有既有外壳则全接口照其改、勿一接口一样）。
  - 失败：`{ "code": <业务码字符串>, "message": <可读说明>, "details": <可选，校验级字段错误> }`。
- **分页约定**（列表类接口共用）：query 传 `page`（页码，默认 1）/ `page_size`（每页条数，默认 20、上限按项目定）；响应把分页元数据归到 `meta`（`meta.total` 总数 / `meta.page` / `meta.page_size`），不散落到顶层。
- **时间 / ID 格式**：时间用 ISO-8601 UTC 字符串（如 `2026-06-30T08:00:00Z`）；ID 类型按项目定（示例用字符串）。

## 端点索引

> 总览全部接口，也给 api 用例当「覆盖矩阵」的行清单（每行一个端点 × 其成功/错误分支由用例覆盖）。新增 / 删除接口先动这张表。
> **索引是总览、下方明细块是真相源**；加 / 删接口两边同步（别只改一处，rule-0012 防双维护漂移）。

| # | 协议 | Method / RPC / Topic | Path / 目标 | 用途 | 鉴权 |
|---|---|---|---|---|---|
| 1 | HTTP-REST | GET | `/v1/items` | 分页列出资源（占位示例） | 需要 |
| 2 | HTTP-REST | POST | `/v1/items` | 创建一个资源（占位示例） | 需要 |
<!-- hc-tech-design：按真实接口逐行重填，协议/method/path/用途/鉴权对齐下方各接口块。协议列取值：HTTP-REST / gRPC / MQ-event / async -->

---

## 接口明细

> 每个接口一块，顺序与端点索引一致。块内五段固定：**描述 + 鉴权 → 请求 → 成功响应（含 Mock 样例）→ 错误响应 → 关联**。

### GET /v1/items

> 占位示例（列表 / 分页 / 查询参数形态）。分页列出资源集合，支持按状态过滤与关键字搜索。

- **鉴权**：需要（`Authorization: Bearer <token>`）。

**请求**

| 参数 | 位置 | 类型 | 必填 | 约束 / 说明 |
|---|---|---|---|---|
| `page` | query | integer | 否 | ≥1，默认 1 |
| `page_size` | query | integer | 否 | 1–100，默认 20 |
| `status` | query | string | 否 | 枚举 `active` / `archived`；不传=全部 |
| `q` | query | string | 否 | 关键字模糊匹配，长度 ≤ 64 |

**成功响应** — `200 OK`

> 统一外壳：列表接口 `data` 为数组（与单资源接口 `data` 为对象同属一套 `{ "data": ... }` 外壳）；分页元数据归到 `meta`，不散落到顶层。

| 字段 | 类型 | 约束 / 枚举 / 格式 | 说明 |
|---|---|---|---|
| `data` | array | 0–`page_size` 个元素 | 资源对象列表，元素结构见下 |
| `data[].id` | string | 非空；格式 `itm_` + 4 位数字 | 资源唯一标识 |
| `data[].name` | string | 长度 1–64 | 资源名称 |
| `data[].status` | string | 枚举 `active` / `archived` | 状态 |
| `data[].created_at` | string | ISO-8601 UTC（如 `2026-06-30T08:00:00Z`） | 创建时间 |
| `meta.total` | integer | ≥0 | 满足条件的总条数（用于分页） |
| `meta.page` | integer | ≥1 | 当前页码 |
| `meta.page_size` | integer | 1–100 | 当前每页条数 |

Mock 样例：

```json
{
  "data": [
    {
      "id": "itm_0001",
      "name": "示例资源 A",
      "status": "active",
      "created_at": "2026-06-30T08:00:00Z"
    },
    {
      "id": "itm_0002",
      "name": "示例资源 B",
      "status": "archived",
      "created_at": "2026-06-29T12:30:00Z"
    }
  ],
  "meta": {
    "total": 2,
    "page": 1,
    "page_size": 20
  }
}
```

**错误响应**（只列**约定内**的错误：业务码 / 校验码 / 约定的服务态；未约定的裸 500·panic 不进契约）

| HTTP 状态 | 业务码 | 含义 | 触发条件 |
|---|---|---|---|
| 400 | `INVALID_QUERY` | 查询参数非法 | `page` / `page_size` 越界，或 `status` 非枚举值 |
| 401 | `UNAUTHENTICATED` | 未认证 | 缺 token 或 token 失效 |
| 403 | `FORBIDDEN` | 无权限 | 已认证但无权访问该资源集合 |

**关联**

- 用户故事 / AC：US-NN / AC-NN（占位，填真实来源）
- 数据模型：`design.md` ③ 的「资源」实体 / 表

### POST /v1/items

> 占位示例（创建 / 请求体校验 / 资源冲突形态）。创建一个资源，成功返回新建资源对象。

- **鉴权**：需要（`Authorization: Bearer <token>`）。

**请求**

| 参数 | 位置 | 类型 | 必填 | 约束 / 说明 |
|---|---|---|---|---|
| `name` | body | string | 是 | 长度 1–64，去首尾空白后非空；同集合内唯一 |
| `status` | body | string | 否 | 枚举 `active` / `archived`，默认 `active` |
| `description` | body | string | 否 | 长度 ≤ 500 |

请求体 Mock 样例：

```json
{
  "name": "示例资源 C",
  "status": "active",
  "description": "可选描述"
}
```

**成功响应** — `201 Created`

> 统一外壳：单资源接口 `data` 为对象（与列表接口 `data` 为数组同属一套 `{ "data": ... }` 外壳）。

| 字段 | 类型 | 约束 / 枚举 / 格式 | 说明 |
|---|---|---|---|
| `data` | object | — | 新建资源对象，结构见下 |
| `data.id` | string | 非空；格式 `itm_` + 4 位数字（服务端生成） | 新建资源唯一标识 |
| `data.name` | string | 长度 1–64 | 资源名称 |
| `data.status` | string | 枚举 `active` / `archived` | 状态 |
| `data.description` | string\|null | 长度 ≤ 500；未传则为 `null` | 描述 |
| `data.created_at` | string | ISO-8601 UTC（如 `2026-06-30T09:15:00Z`） | 创建时间 |

Mock 样例：

```json
{
  "data": {
    "id": "itm_0003",
    "name": "示例资源 C",
    "status": "active",
    "description": "可选描述",
    "created_at": "2026-06-30T09:15:00Z"
  }
}
```

**错误响应**（只列**约定内**的错误：业务码 / 校验码 / 约定的服务态；未约定的裸 500·panic 不进契约）

| HTTP 状态 | 业务码 | 含义 | 触发条件 |
|---|---|---|---|
| 422 | `VALIDATION_FAILED` | 字段校验不通过 | `name` 缺失 / 超长 / 空白，或 `status` 非枚举值；`details` 列出逐字段错误 |
| 401 | `UNAUTHENTICATED` | 未认证 | 缺 token 或 token 失效 |
| 403 | `FORBIDDEN` | 无权限 | 已认证但无创建权限 |
| 409 | `CONFLICT` | 资源冲突 | 同集合内 `name` 已存在 |
| 503 | `DB_UNAVAILABLE` | 依赖不可用（**约定的服务态**示例） | 数据库 / 关键下游不可用——**若工程把它当契约级服务态**（如 resilience 测试硬断言）就列进来；纯未预期故障则不列 |

`422` 错误体 Mock 样例：

```json
{
  "code": "VALIDATION_FAILED",
  "message": "请求参数校验失败",
  "details": [
    { "field": "name", "reason": "不能为空" }
  ]
}
```

**关联**

- 用户故事 / AC：US-NN / AC-NN（占位，填真实来源）
- 数据模型：`design.md` ③ 的「资源」实体 / 表

---

<!-- 协议逃生口·骨架（仅 gRPC / MQ-event / async 项目用，REST 项目删掉本注释）：
     上面两块是 HTTP-REST 视角；非-REST 项目照下面极简骨架替换每个端点块的字段名，五段结构不变。

  【gRPC 端点块骨架】
    ### CreateItem (rpc)
    - **协议**：gRPC ｜ **service.method**：`ItemService/CreateItem` ｜ **鉴权**：metadata `authorization: Bearer <token>`
    **请求**（proto message `CreateItemRequest`）
    | 字段 | 位置 | 类型 | 必填 | 约束 / 说明 |  → 位置填 `grpc-field`
    **成功响应**（proto message `CreateItemResponse`）
    | 字段 | 类型 | 约束 / 枚举 / 格式 | 说明 |  → 字段表同 REST，给 Mock（JSON 或 textproto）
    **错误响应**（只列约定内）
    | gRPC status (code) | 业务 reason(enum) | 含义 | 触发条件 |  → 如 INVALID_ARGUMENT / UNAUTHENTICATED / PERMISSION_DENIED / ALREADY_EXISTS / UNAVAILABLE；HTTP 状态码列换成 gRPC status

  【MQ-event 端点块骨架】
    ### item.created (event)
    - **协议**：MQ-event ｜ **topic**：`item.created` ｜ **鉴权**：N/A（信道级，非逐消息）
    **请求/载荷**（event schema）
    | 字段 | 位置 | 类型 | 必填 | 约束 / 说明 |  → 位置填 `payload` / `header`（含 `event_id` / `occurred_at` / 幂等键）
    **成功响应**：异步无同步响应——写"消费成功语义"（ack / 落库 / 下游事件），给一条 payload Mock 样例
    **错误响应**（只列约定内）：消费失败处置（重试 / 进 DLQ / 幂等去重），列约定的失败 reason 枚举，无 HTTP 状态
-->

<!-- hc-tech-design：以上两块为「列表 + 创建」占位示例。照此为每个真实接口补齐五段（描述+鉴权 / 请求 / 成功响应+Mock / 错误响应[只列约定内的错误] / 关联），删掉占位说明与中性示例。错误码表须与 design.md ⑧『约定内错误 + 业务 code』对齐（⑧=各端点错误表的并集、本端点表是其子集，非逐行相等）（同名同 HTTP 状态），覆盖业务码 / 校验码(400/422) / 鉴权(401/403) / 约定服务态(503)——按「约定 / 未约定」口径核闭合，别用窄的「业务码一一对应」(会把合法的 401/403/503 误判为多余)。 -->

> 本模板为通用指导，可按接口数量与复杂度增删；但「端点索引 + 每接口五段 + 成功响应给 Mock + 错误码只列约定内的错误」是 api 用例逐字段对照的硬依赖，勿省。
