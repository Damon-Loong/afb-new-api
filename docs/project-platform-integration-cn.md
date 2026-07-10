# 第三方项目接入 OPC 平台说明

OPC 平台地址：

- 前端地址：`https://opc.mbmzone.com`
- 后端接口地址：`https://afb-api.mbmzone.com`

平台提供统一登录、用户信息、Token 余额、充值入口和项目扣费接口。

## 1. 项目配置

项目接入前，需要先在 OPC 后台“项目管理”中配置：

| 配置项 | 说明 |
| --- | --- |
| 项目标识 | 项目唯一 key，例如 `ota` |
| 项目名称 | 平台展示名称 |
| 官网 URL | 项目前端地址，同时作为登录回跳和跨域白名单 |
| 扣费密钥 | 项目后端调用扣费接口时用于生成签名 |
| 启用项目 | 开启后项目可用 |
| 允许 SSO | 开启后允许统一登录 |

## 2. 统一登录

项目需要登录时，跳转到：

```text
https://opc.mbmzone.com/sso/login?project={project_key}&return_to={return_to}
```

示例：

```text
https://opc.mbmzone.com/sso/login?project=ota&return_to=https%3A%2F%2Fota.xxx.com
```

登录成功后，OPC 会跳回项目，并追加：

```text
?project=ota&ticket=xxx&state=xxx
```

项目用 `ticket` 换取用户登录信息：

```http
POST https://afb-api.mbmzone.com/api/sso/exchange
Content-Type: application/json
```

```json
{
  "project": "ota",
  "ticket": "xxx",
  "state": "xxx"
}
```

返回：

```json
{
  "success": true,
  "data": {
    "id": 123,
    "username": "user",
    "display_name": "User",
    "phone": "13800000000",
    "quota": 99000000,
    "access_token": "opc-user-access-token"
  }
}
```

后续接口使用返回的 `id` 和 `access_token`。

## 3. 查询用户信息和剩余 Token

```http
GET https://afb-api.mbmzone.com/api/user/self
Authorization: Bearer {access_token}
New-API-User: {user_id}
```

返回中的 `quota` 是用户剩余 Token。

## 4. 跳转充值

项目可打开 OPC 钱包页：

```text
https://opc.mbmzone.com/wallet?project={project_key}&access_token={access_token}&user_id={user_id}
```

示例：

```text
https://opc.mbmzone.com/wallet?project=ota&access_token=xxx&user_id=123
```

## 5. 项目扣费

```http
POST https://afb-api.mbmzone.com/api/projects/billing/charge
Authorization: Bearer {access_token}
New-API-User: {user_id}
Content-Type: application/json
X-Project-Key: {project_key}
X-Project-Timestamp: {unix_timestamp}
X-Project-Signature: {signature}
Idempotency-Key: {idempotency_key}
```

请求体：

```json
{
  "project_key": "ota",
  "token_amount": 1000000,
  "idempotency_key": "ota-calendar-123",
  "description": "OTA 日历价格查询",
  "metadata": {
    "endpoint": "calendar_prices",
    "hotel_id": 123
  }
}
```

请求体说明：

| 字段 | 说明 |
| --- | --- |
| `project_key` | 项目标识 |
| `token_amount` | 本次扣除的 Token 数量 |
| `idempotency_key` | 幂等键，同一次业务扣费保持一致 |
| `description` | 扣费说明 |
| `metadata` | 扩展信息，可记录业务 ID、接口名等 |

返回：

```json
{
  "success": true,
  "data": {
    "project_key": "ota",
    "idempotency_key": "ota-calendar-123",
    "token_amount": 1000000,
    "remaining_quota": 99000000,
    "replayed": false
  }
}
```

返回说明：

| 字段 | 说明 |
| --- | --- |
| `success` | 是否请求成功 |
| `project_key` | 项目标识 |
| `idempotency_key` | 本次扣费使用的幂等键 |
| `token_amount` | 本次扣除的 Token 数量 |
| `remaining_quota` | 扣费后的剩余 Token |
| `replayed` | 是否为幂等重放结果 |

请求头说明：

| 请求头 | 说明 |
| --- | --- |
| `X-Project-Key` | 项目标识，例如 `ota` |
| `X-Project-Timestamp` | 当前 Unix 秒级时间戳，用于防止旧请求重复使用 |
| `X-Project-Signature` | 项目扣费签名，用后台配置的扣费密钥生成 |
| `Idempotency-Key` | 幂等键，同一次业务扣费必须保持一致，避免重复扣费 |

签名原文：

```text
{project_key}
{timestamp}
{idempotency_key}
{token_amount}
```

签名算法：

```text
HMAC-SHA256(secret, payload)
```

其中 `secret` 是项目管理里配置的扣费密钥。

## 6. 常见错误

| 状态码 | 含义 |
| --- | --- |
| `401` | 用户未登录或 access token 无效 |
| `402` | 用户剩余 Token 不足 |
| `403` | 项目不可用、密钥未配置或签名错误 |
| `429` | 请求过于频繁 |
| `500` | 平台内部错误 |
