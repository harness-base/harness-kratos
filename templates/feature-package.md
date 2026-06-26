# Feature 需求包：&lt;feature-id&gt; &lt;标题&gt;

> 用本模板建需求包，登记进 `docs/features/index.yaml`。改业务代码前必须就绪（rule-0001）。

## 背景 / 目标
（为什么做，达成什么用户可见结果）

## 范围
- 包含：
- 不包含：

## 用户故事 / 验收目标
- As a ..., I want ..., so that ...
- 验收：可观察、可验证的结果（不是"做完了"）

## 影响面
- 被管工程：`projects/<name>`
- 接口 / 数据 / 权限：
- 受影响 skill（rule-0007）：

## 测试设计
- API：case_id / 路由 / 前置 / 权限·租户 / 请求 / 断言 / 反向 / 持久化
- E2E：case_id / 角色 / 入口 / 步骤 / 网络断言 / UI 断言 / 刷新后

## 状态
- delivery_status: draft | tests_ready | implementing | verified | done
- implementation_allowed: false
