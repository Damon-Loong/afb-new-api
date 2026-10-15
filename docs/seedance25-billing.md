# Seedance 2.5 视频计费（本地验收版）

基线：`develop` 的 `5a22e601`。仅后端和管理后台；不修改 OPC 网站、不自动修改现有模型价格。

## 官方口径

核对日期：2026-10-15。

- [火山方舟模型价格](https://docs.volcengine.com/docs/ark/model-pricing?lang=zh)
- [Seedance 2.5 教程](https://docs.volcengine.com/docs/ark/seedance-2-5?lang=zh)
- [查询视频生成任务](https://docs.volcengine.com/docs/ark/get-video-generation-task-api?lang=zh)

方舟 `doubao-seedance-2-5-260628` 的标准价按百万视频 Token：480p/720p 无视频输入 ¥70、有视频输入 ¥42；1080p 无视频输入 ¥77、有视频输入 ¥46。此配置不使用活动价，也不混用 LAS 产品的按秒报价。

最终用量取成功任务的 `usage.completion_tokens`。分辨率选择单价；时长、参考视频等影响上游计算的 Token 数，不另外重复按秒收费。响应中的小数秒时长可保留用于审计。

## 后台配置

1. 系统设置 → 分组与模型定价设置 → 模型定价设置。
2. 新增或选择该模型（渠道仍需原有配置流程），选择“视频计费”。
3. 点击“填入 Seedance 2.5 标准价模板”，确认四档人民币单价。
4. 确认计费汇率。本模板为 `1 USD = 7 CNY`，它独立于全站显示汇率；全站若按人民币显示，两者应保持一致。
5. 设置预扣 Token 数，默认 300000。该数是可配置预扣量，不代表实际时长、实际用量或账单上限。由于自动时长及参考视频长度不能从请求可靠确定，本版不伪造精确时长估算；完成后按实际用量结算，可能补扣。
6. 应用更改并刷新验证。未选择视频计费的文本、图片、旧视频模型保持原算法。

内部额度计算：`completion_tokens / 1,000,000 × 人民币档位价 / 计费汇率 × 实际分组倍率 × QuotaPerUnit`。仅在最后一步取整，价格不提前四舍五入。用户组特殊倍率沿用原系统选择逻辑，提交时一并快照。

配置保存在 `billing_setting.billing_mode` 与 `billing_setting.video_pricing`。此结构化视频配置不通过旧“上游倍率同步”扁平化导入，避免丢失档位或变成零价。

## 支持与保护

- 火山原生 `POST /api/v3/contents/generations/tasks`，以及豆包视频渠道的转换接口 `POST /v1/videos`。
- 转换接口通过 `metadata.resolution`、`metadata.content`、`metadata.duration` 等传递视频参数。原生接口可直接传官方参数。
- 请求时预扣；上游成功后根据实际返回分辨率和用量补扣/退差额；failed、cancelled、expired 退还预扣。
- 任务终态与资金、令牌、用量统计在同一数据库事务内更新。重复轮询不重复结算；数据库失败回滚并保留重试机会。
- 成功但缺少有效用量/分辨率：后台标记“视频已完成 · 计费待核对”，保留预扣并重试，不默认为零元，也不按本地超时自动退款。上游永久缺失或任务查询超出有效期时需管理方核对账单，本版未提供人工补账按钮。
- 价格、汇率、分组倍率、内部额度单位使用提交时快照。修改设置不追溯影响在途任务。
- 日志区分提交预扣与最终差额；任务记录的额度保存最终总额。展开视频计费过程可查看用量、单价、汇率和倍率。
- Draft 样片以 480p 单独计费。Draft 转正式视频通过原生接口提交，引用同用户、同模型、同渠道且已成功的原生 Draft 任务；继承是否含视频输入的分类，目标为 1080p。不能引用系统外或无快照的 Draft。
- 本版只配置标准 service_tier；非 default 或带额外 tools 的请求明确拒绝，避免用不适用的价目默默收费。
- 不支持直接把 LAS 按秒报价、其他供应商模型或其他产品套餐套入本模板。

## 隔离本地预览

先构建前端（该项目依赖较大，构建时需要提高 Node 内存上限），再构建 Go 后端：

```sh
cd web
bun install --frozen-lockfile
NODE_OPTIONS=--max-old-space-size=8192 bun run build
cd ..
go build -o /tmp/seedance25-new-api .
```

创建专用临时目录作为运行目录和 SQLite 存储位置。启动时明确清空 `SQL_DSN`、`LOG_SQL_DSN`、`REDIS_CONN_STRING`，设置独立 `SQLITE_PATH` 和 `SESSION_SECRET`，关闭 `BATCH_UPDATE_ENABLED`，以 `--host 127.0.0.1 --port 13881` 启动。不要在含线上 `.env` 的目录启动。

另一个终端启动 `node scripts/seedance25-local-mock.mjs`（只监听 127.0.0.1:13882），再设置 `LOCAL_VIDEO_ADMIN_PASSWORD` 后运行 `node scripts/seedance25-local-setup.mjs`。初始化脚本只允许空测试库，创建 `video_admin` 和本地模拟渠道，不写线上数据。

访问 `http://127.0.0.1:13881/console/setting?tab=ratio`。运行 `node scripts/seedance25-local-smoke.mjs` 可验证四档价格、失败/取消、缺失用量、Draft 和转换接口的账户扣费；该脚本只允许唯一渠道指向本地 mock，无真实付费调用。

## 发布前验收边界

已具备单元测试、SQLite 事务测试和本地模拟端到端测试。真实火山返回、生产 MySQL/PostgreSQL、Redis/批量更新组合仍需发布前验证。后台应用设置采用原系统多请求保存机制，并非整套定价的原子批量更新。

当前本地验证：10 个端到端模拟场景通过，钱包总扣费与任务最终额度/待核对预扣额度一致；后台配置保存刷新通过；Go 应用与前端生产构建通过。`service`、`model`、`controller`、豆包适配器、`types` 包测试及 `relay/helper` 视频计费专项通过。

不能宣称全项目测试全绿：原基线 DeepSeek 测试直接填写已移入嵌套结构的 `ChannelBaseUrl`，`scripts` 的两个旧 Go 工具有重复 `main`，导致 `go test ./... -run '^$'` 失败；helper 全包首轮还出现流式文本测试的计时器问题（该单测单独运行通过）。这些文件未随本功能修改。

先完成本地验收，再将本功能提交同步至 `develop`，随后拣选相同功能提交至 `tianyue` 并单独构建回归；两分支存在品牌等差异，不能整分支覆盖。部署前备份配置与数据库；有在途视频任务时不可直接回退到不识别快照的旧版本，应先停止新任务并完成结算/核对。
