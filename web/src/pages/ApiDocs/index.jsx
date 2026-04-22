/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

import React, { useMemo, useState } from 'react';
import { Button } from '@douyinfe/semi-ui';
import {
  Bot,
  ChevronDown,
  Copy,
  CreditCard,
  KeyRound,
  LogIn,
  Mail,
  MessageCircle,
  Sparkles,
  UserPlus,
  Users,
} from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { showError, showSuccess } from '../../helpers';

const ApiDocs = () => {
  const { t } = useTranslation();

  const baseUrl = useMemo(() => {
    if (typeof window === 'undefined') {
      return 'http://localhost:3000';
    }
    return window.location.origin;
  }, []);

  const endpointCards = useMemo(
    () => {
      const apiKeyHeaderParam = {
        name: 'Authorization',
        required: true,
        location: 'header',
        description: t('Bearer sk-xxxxxx，使用令牌管理中创建的 API Token。'),
      };
      const jsonContentTypeParam = {
        name: 'Content-Type',
        required: true,
        location: 'header',
        description: t('固定为 application/json。'),
      };
      const apiKeyNotes = [
        t('可用模型范围会受分组、渠道映射和令牌模型限制影响。'),
      ];
      const genericAiResponse = `{
  "id": "response-xxxxxxxx",
  "object": "response",
  "created": 1710000000,
  "model": "MODEL_NAME",
  "data": {}
}`;

      const createAiEndpoint = ({
        key,
        title,
        method = 'POST',
        path,
        summary,
        params = [],
        body,
        responseExample = genericAiResponse,
        notes = [],
      }) => ({
        key,
        icon: <MessageCircle size={18} />,
        title: t(title),
        method,
        path,
        summary: t(summary),
        notes: [...apiKeyNotes, ...notes.map((note) => t(note))],
        params:
          method === 'GET'
            ? [apiKeyHeaderParam, ...params]
            : [apiKeyHeaderParam, jsonContentTypeParam, ...params],
        requestExample:
          method === 'GET'
            ? `curl "${baseUrl}${path}" \\
  -H "Authorization: Bearer sk-YOUR_API_TOKEN"`
            : `curl -X ${method} "${baseUrl}${path}" \\
  -H "Authorization: Bearer sk-YOUR_API_TOKEN" \\
  -H "Content-Type: application/json" \\
  -d '${body}'`,
        responseExample,
      });

      const modelParam = {
        name: 'model',
        required: true,
        location: 'body',
        description: t('模型 ID，需要在当前 API Token 的可用范围内。'),
      };
      const taskIdParam = {
        name: 'task_id',
        required: true,
        location: 'path',
        description: t('异步任务 ID，由提交任务接口返回。'),
      };

      return [
      {
        key: 'verification',
        icon: <Mail size={18} />,
        title: t('发送验证码'),
        method: 'GET',
        path: '/api/verification',
        summary: t('向指定邮箱发送注册验证码，用于后续注册校验。'),
        notes: [
          t('如果邮箱已被占用，接口会直接返回失败。'),
        ],
        params: [
          {
            name: 'email',
            required: true,
            location: 'query',
            description: t('目标邮箱地址，例如 vvs3912@outlook.com。'),
          },
        ],
        requestExample: `curl "${baseUrl}/api/verification?email=vvs3912@outlook.com"`,
        responseExample: `{
  "success": true,
  "message": ""
}`,
      },
      {
        key: 'register',
        icon: <UserPlus size={18} />,
        title: t('注册接口'),
        method: 'POST',
        path: '/api/user/register',
        summary: t('创建普通用户账号，不会自动登录，成功后只返回 success。'),
        notes: [
          t('注册成功后，后端会自动为该用户生成用户 access_token。'),
          t(
            '如果开启邮箱验证，email 和 verification_code 必填；否则这两个字段不是必需。',
          ),
        ],
        params: [
          {
            name: 'username',
            required: true,
            location: 'body',
            description: t('用户名。'),
          },
          {
            name: 'password',
            required: true,
            location: 'body',
            description: t('密码，8 到 20 位。'),
          },
          {
            name: 'email',
            required: false,
            location: 'body',
            description: t('开启邮箱验证时必须传入。'),
          },
          {
            name: 'verification_code',
            required: false,
            location: 'body',
            description: t('开启邮箱验证时必须传入，通过发送验证码接口获取。'),
          },
          {
            name: 'aff_code',
            required: false,
            location: 'body',
            description: t('邀请码，可选。'),
          },
        ],
        requestExample: `curl -X POST "${baseUrl}/api/user/register" \\
  -H "Content-Type: application/json" \\
  -d '{
    "username": "testuser",
    "password": "12345678",
    "email": "test@example.com",
    "verification_code": "123456"
  }'`,
        responseExample: `{
  "success": true,
  "message": ""
}`,
      },
      {
        key: 'login',
        icon: <LogIn size={18} />,
        title: t('登录接口'),
        method: 'POST',
        path: '/api/user/login',
        summary: t(
          '使用用户名和密码登录。正常情况下会写入 session 并返回用户信息；如果用户开启了 2FA，会先返回 require_2fa。',
        ),
        notes: [
          t('如果账号开启 2FA，这一步不会直接完成登录，而是返回 require_2fa: true。'),
          t('成功登录后服务端会写入 session，并返回用户 access_token。'),
        ],
        params: [
          {
            name: 'username',
            required: true,
            location: 'body',
            description: t('用户名。后端也允许用邮箱登录该字段。'),
          },
          {
            name: 'password',
            required: true,
            location: 'body',
            description: t('登录密码。'),
          },
        ],
        requestExample: `curl -X POST "${baseUrl}/api/user/login" \\
  -H "Content-Type: application/json" \\
  -d '{
    "username": "testuser",
    "password": "12345678"
  }'`,
        responseExample: `// 正常登录
{
  "success": true,
  "message": "",
  "data": {
    "id": 1,
    "username": "testuser",
    "display_name": "testuser",
    "role": 1,
    "status": 1,
    "group": "default",
    "access_token": "USER_ACCESS_TOKEN"
  }
}

// 如果开启了 2FA
{
  "success": true,
  "message": "需要进行两步验证",
  "data": {
    "require_2fa": true
  }
}`,
      },
      {
        key: 'self-user',
        icon: <Users size={18} />,
        title: t('获取当前用户信息'),
        method: 'GET',
        path: '/api/user/self',
        summary: t(
          '返回当前登录用户的基础信息、余额额度、已用额度、请求次数、分组和权限信息。',
        ),
        notes: [
          t('余额字段 quota 表示当前用户剩余额度；used_quota 表示已使用额度；request_count 表示累计请求次数。'),
        ],
        params: [
          {
            name: 'Cookie / Session',
            required: false,
            location: 'header',
            description: t('浏览器登录后可使用 session/cookie 作为认证方式。'),
          },
          {
            name: 'Authorization',
            required: false,
            location: 'header',
            description: t(
              '可传 Bearer USER_ACCESS_TOKEN。这里是用户 access_token，不是 sk-... API Token。',
            ),
          },
          {
            name: 'New-API-User',
            required: true,
            location: 'header',
            description: t(
              '当前用户 ID，后端会用它和 session 或 access_token 中的用户 ID 做一致性校验。',
            ),
          },
        ],
        requestExample: `# 浏览器登录后的 session/cookie 方式
curl "${baseUrl}/api/user/self" \\
  -H "Cookie: session=YOUR_SESSION_COOKIE" \\
  -H "New-API-User: 1"

# 用户 access_token 方式，不是 sk-... API Token
curl "${baseUrl}/api/user/self" \\
  -H "Authorization: Bearer USER_ACCESS_TOKEN" \\
  -H "New-API-User: 1"`,
        responseFields: [
          {
            name: 'success',
            type: 'boolean',
            description: t('请求是否成功。'),
          },
          {
            name: 'message',
            type: 'string',
            description: t('接口消息，成功时通常为空字符串。'),
          },
          {
            name: 'data.id',
            type: 'number',
            description: t('当前用户 ID。'),
          },
          {
            name: 'data.username',
            type: 'string',
            description: t('登录用户名。'),
          },
          {
            name: 'data.display_name',
            type: 'string',
            description: t('用户显示名称。'),
          },
          {
            name: 'data.role',
            type: 'number',
            description: t('用户角色值，普通用户和管理员会有不同角色等级。'),
          },
          {
            name: 'data.status',
            type: 'number',
            description: t('用户状态值，用于判断账号是否可用。'),
          },
          {
            name: 'data.email',
            type: 'string',
            description: t('用户绑定邮箱。'),
          },
          {
            name: 'data.github_id / discord_id / oidc_id',
            type: 'string',
            description: t('第三方 OAuth 绑定标识，未绑定时通常为空。'),
          },
          {
            name: 'data.wechat_id / telegram_id / linux_do_id',
            type: 'string',
            description: t('其他第三方账号绑定标识，未绑定时通常为空。'),
          },
          {
            name: 'data.group',
            type: 'string',
            description: t('用户所属分组，会影响模型、倍率和渠道可用性。'),
          },
          {
            name: 'data.quota',
            type: 'number',
            description: t(
              '用户当前剩余额度，原始 quota 单位。换算公式：data.quota / quota_per_unit，默认 quota_per_unit 为 500000。',
            ),
          },
          {
            name: 'data.used_quota',
            type: 'number',
            description: t(
              '用户历史已使用额度，原始 quota 单位。换算公式：used_quota / quota_per_unit。',
            ),
          },
          {
            name: 'data.request_count',
            type: 'number',
            description: t('用户累计请求次数。'),
          },
          {
            name: 'data.aff_code',
            type: 'string',
            description: t('用户邀请码。'),
          },
          {
            name: 'data.aff_count',
            type: 'number',
            description: t('通过该用户邀请码注册的用户数量。'),
          },
          {
            name: 'data.aff_quota',
            type: 'number',
            description: t(
              '当前可转入余额的邀请奖励额度，原始 quota 单位。换算公式：aff_quota / quota_per_unit。',
            ),
          },
          {
            name: 'data.aff_history_quota',
            type: 'number',
            description: t(
              '历史累计邀请奖励额度，原始 quota 单位。换算公式：aff_history_quota / quota_per_unit。',
            ),
          },
          {
            name: 'data.inviter_id',
            type: 'number',
            description: t('邀请人用户 ID，没有邀请人时通常为空或 0。'),
          },
          {
            name: 'data.setting',
            type: 'string/object',
            description: t('用户个性化设置，后端按用户设置结构保存。'),
          },
          {
            name: 'data.stripe_customer',
            type: 'string',
            description: t('Stripe 客户标识，未配置支付或未绑定时通常为空。'),
          },
          {
            name: 'data.sidebar_modules',
            type: 'array',
            description: t('该用户可见的侧边栏模块配置。'),
          },
          {
            name: 'data.permissions',
            type: 'object',
            description: t('后端根据当前用户角色计算出的权限信息。'),
          },
        ],
        responseExample: `{
  "success": true,
  "message": "",
  "data": {
    "id": 1,
    "username": "admin",
    "display_name": "admin",
    "role": 100,
    "status": 1,
    "email": "admin@example.com",
    "group": "default",
    "quota": 200000,
    "used_quota": 1000,
    "request_count": 10,
    "aff_quota": 0,
    "aff_history_quota": 0,
    "permissions": {},
    "sidebar_modules": []
  }
}`,
      },
      {
        key: 'subscription-plans',
        icon: <Users size={18} />,
        title: t('获取平台订阅套餐'),
        method: 'GET',
        path: '/api/subscription/plans',
        summary: t(
          '返回当前平台已启用、可供用户购买的订阅套餐列表。',
        ),
        notes: [
          t('该接口只返回 enabled = true 的套餐；管理员查看全部套餐需要使用 /api/subscription/admin/plans。'),
          t('返回结果按 sort_order desc、id desc 排序。'),
          t('每一项结构为 { plan: {...} }，实际套餐信息在 plan 字段内。'),
          t('total_amount 是套餐总额度，0 通常表示不限额度。'),
          t('duration_unit 和 duration_value 共同决定套餐有效期。'),
          t('upgrade_group 非空时，购买后用户分组会升级到该分组。'),
        ],
        params: [
          {
            name: 'Cookie / Session',
            required: false,
            location: 'header',
            description: t('浏览器登录后可使用 session/cookie 作为认证方式。'),
          },
          {
            name: 'Authorization',
            required: false,
            location: 'header',
            description: t(
              '可传 Bearer USER_ACCESS_TOKEN。这里是用户 access_token，不是 sk-... API Token。',
            ),
          },
          {
            name: 'New-API-User',
            required: true,
            location: 'header',
            description: t(
              '当前用户 ID，后端会用它和 session 或 access_token 中的用户 ID 做一致性校验。',
            ),
          },
        ],
        requestExample: `# 浏览器登录后的 session/cookie 方式
curl "${baseUrl}/api/subscription/plans" \\
  -H "Cookie: session=YOUR_SESSION_COOKIE" \\
  -H "New-API-User: 1"

# 用户 access_token 方式，不是 sk-... API Token
curl "${baseUrl}/api/subscription/plans" \\
  -H "Authorization: Bearer USER_ACCESS_TOKEN" \\
  -H "New-API-User: 1"`,
        responseFields: [
          {
            name: 'success',
            type: 'boolean',
            description: t('请求是否成功。'),
          },
          {
            name: 'message',
            type: 'string',
            description: t('接口消息，成功时通常为空字符串。'),
          },
          {
            name: 'data',
            type: 'array',
            description: t('套餐列表，每一项包含一个 plan 对象。'),
          },
          {
            name: 'data[].plan',
            type: 'object',
            description: t('套餐项对象，实际套餐信息位于 plan 内。'),
          },
          {
            name: 'data[].plan.id',
            type: 'number',
            description: t('套餐 ID，购买订阅时作为 plan_id 传入。'),
          },
          {
            name: 'data[].plan.title',
            type: 'string',
            description: t('套餐标题。'),
          },
          {
            name: 'data[].plan.subtitle',
            type: 'string',
            description: t('套餐副标题或补充说明。'),
          },
          {
            name: 'data[].plan.price_amount',
            type: 'number',
            description: t('套餐价格金额。'),
          },
          {
            name: 'data[].plan.currency',
            type: 'string',
            description: t('价格币种。'),
          },
          {
            name: 'data[].plan.duration_unit',
            type: 'string',
            description: t('有效期单位，常见值包括 year、month、day、hour、custom。'),
          },
          {
            name: 'data[].plan.duration_value',
            type: 'number',
            description: t('有效期数值，与 duration_unit 配合使用。'),
          },
          {
            name: 'data[].plan.custom_seconds',
            type: 'number',
            description: t('自定义有效期秒数，仅 duration_unit 为 custom 时使用。'),
          },
          {
            name: 'data[].plan.enabled',
            type: 'boolean',
            description: t('套餐是否启用；该接口只返回启用套餐。'),
          },
          {
            name: 'data[].plan.sort_order',
            type: 'number',
            description: t('排序值，越大越靠前。'),
          },
          {
            name: 'data[].plan.max_purchase_per_user',
            type: 'number',
            description: t('每个用户最多购买次数，0 表示不限。'),
          },
          {
            name: 'data[].plan.total_amount',
            type: 'number',
            description: t(
              '套餐总额度，原始 quota 单位，0 通常表示不限额度。换算公式：total_amount / quota_per_unit。',
            ),
          },
          {
            name: 'data[].plan.upgrade_group',
            type: 'string',
            description: t('购买后升级到的用户分组，未配置时为空。'),
          },
          {
            name: 'data[].plan.quota_reset_period',
            type: 'string',
            description: t('额度重置周期，常见值包括 never、daily、weekly、monthly、custom。'),
          },
          {
            name: 'data[].plan.quota_reset_custom_seconds',
            type: 'number',
            description: t('自定义额度重置周期秒数，仅 quota_reset_period 为 custom 时使用。'),
          },
          {
            name: 'data[].plan.stripe_price_id / creem_product_id',
            type: 'string',
            description: t('第三方订阅支付产品标识，未配置对应支付方式时通常为空。'),
          },
          {
            name: 'data[].plan.created_at / updated_at',
            type: 'number',
            description: t('套餐创建和更新时间，Unix 秒级时间戳。'),
          },
        ],
        responseExample: `{
  "success": true,
  "message": "",
  "data": [
    {
      "plan": {
        "id": 3,
        "title": "Pro 月度套餐",
        "subtitle": "适合稳定使用的个人用户",
        "price_amount": 19.9,
        "currency": "default",
        "duration_unit": "month",
        "duration_value": 1,
        "custom_seconds": 0,
        "enabled": true,
        "sort_order": 100,
        "stripe_price_id": "",
        "creem_product_id": "",
        "max_purchase_per_user": 0,
        "upgrade_group": "premium",
        "total_amount": 500000,
        "quota_reset_period": "monthly",
        "quota_reset_custom_seconds": 0,
        "created_at": 1710000000,
        "updated_at": 1710000000
      }
    }
  ]
}`,
      },
      {
        key: 'user-subscriptions',
        icon: <Users size={18} />,
        title: t('获取用户订阅套餐'),
        method: 'GET',
        path: '/api/subscription/self',
        summary: t(
          '返回当前用户的订阅扣费偏好、生效订阅列表，以及包含过期/作废记录的全部订阅列表。',
        ),
        notes: [
          t('subscriptions 只包含当前生效中的订阅套餐实例。'),
          t('all_subscriptions 包含当前用户所有订阅实例，包括生效、过期和作废。'),
          t(
            '该接口里的订阅对象只直接返回 plan_id；如果要显示套餐名称，需要再调用 /api/subscription/plans，用 plan_id 匹配 plan.id。',
          ),
          t('amount_total 和 amount_used 分别表示订阅总额度和已用额度。'),
        ],
        params: [
          {
            name: 'Cookie / Session',
            required: false,
            location: 'header',
            description: t('浏览器登录后可使用 session/cookie 作为认证方式。'),
          },
          {
            name: 'Authorization',
            required: false,
            location: 'header',
            description: t(
              '可传 Bearer USER_ACCESS_TOKEN。这里是用户 access_token，不是 sk-... API Token。',
            ),
          },
          {
            name: 'New-API-User',
            required: true,
            location: 'header',
            description: t(
              '当前用户 ID，后端会用它和 session 或 access_token 中的用户 ID 做一致性校验。',
            ),
          },
        ],
        requestExample: `# 浏览器登录后的 session/cookie 方式
curl "${baseUrl}/api/subscription/self" \\
  -H "Cookie: session=YOUR_SESSION_COOKIE" \\
  -H "New-API-User: 1"

# 用户 access_token 方式，不是 sk-... API Token
curl "${baseUrl}/api/subscription/self" \\
  -H "Authorization: Bearer USER_ACCESS_TOKEN" \\
  -H "New-API-User: 1"`,
        responseFields: [
          {
            name: 'success',
            type: 'boolean',
            description: t('请求是否成功。'),
          },
          {
            name: 'message',
            type: 'string',
            description: t('接口消息，成功时通常为空字符串。'),
          },
          {
            name: 'data.billing_preference',
            type: 'string',
            description: t(
              '扣费偏好，常见值包括 subscription_first、wallet_first、subscription_only、wallet_only。',
            ),
          },
          {
            name: 'data.subscriptions',
            type: 'array',
            description: t('当前生效中的订阅实例列表。'),
          },
          {
            name: 'data.all_subscriptions',
            type: 'array',
            description: t('全部订阅实例列表，包含已过期和已作废记录。'),
          },
          {
            name: 'data.subscriptions[].subscription.id',
            type: 'number',
            description: t('用户订阅实例 ID。'),
          },
          {
            name: 'data.subscriptions[].subscription.user_id',
            type: 'number',
            description: t('订阅所属用户 ID。'),
          },
          {
            name: 'data.subscriptions[].subscription.plan_id',
            type: 'number',
            description: t('订阅套餐 ID，可用于匹配 /api/subscription/plans 返回的 plan.id。'),
          },
          {
            name: 'data.subscriptions[].subscription.amount_total',
            type: 'number',
            description: t(
              '订阅总额度，原始 quota 单位，0 通常表示不限额度。换算公式：amount_total / quota_per_unit。',
            ),
          },
          {
            name: 'data.subscriptions[].subscription.amount_used',
            type: 'number',
            description: t(
              '订阅已使用额度，原始 quota 单位。换算公式：amount_used / quota_per_unit。',
            ),
          },
          {
            name: 'data.subscriptions[].subscription.start_time',
            type: 'number',
            description: t('订阅开始时间，Unix 秒级时间戳。'),
          },
          {
            name: 'data.subscriptions[].subscription.end_time',
            type: 'number',
            description: t('订阅结束时间，Unix 秒级时间戳。'),
          },
          {
            name: 'data.subscriptions[].subscription.status',
            type: 'string',
            description: t('订阅状态，常见值包括 active、expired、cancelled。'),
          },
          {
            name: 'data.subscriptions[].subscription.source',
            type: 'string',
            description: t('订阅来源，例如 order 表示支付订单生成，admin 表示管理员手动绑定。'),
          },
          {
            name: 'data.subscriptions[].subscription.last_reset_time',
            type: 'number',
            description: t('上次额度重置时间，Unix 秒级时间戳；未启用周期重置时通常为 0。'),
          },
          {
            name: 'data.subscriptions[].subscription.next_reset_time',
            type: 'number',
            description: t('下次额度重置时间，Unix 秒级时间戳；未启用周期重置时通常为 0。'),
          },
          {
            name: 'data.subscriptions[].subscription.upgrade_group',
            type: 'string',
            description: t('该套餐生效后升级到的用户分组，未配置时为空。'),
          },
          {
            name: 'data.subscriptions[].subscription.prev_user_group',
            type: 'string',
            description: t('套餐升级分组前的用户分组，用于订阅失效后的回退。'),
          },
          {
            name: 'data.subscriptions[].subscription.created_at / updated_at',
            type: 'number',
            description: t('订阅实例创建和更新时间，Unix 秒级时间戳。'),
          },
          {
            name: 'data.all_subscriptions[].subscription.id',
            type: 'number',
            description: t('用户订阅实例 ID。'),
          },
          {
            name: 'data.all_subscriptions[].subscription.user_id',
            type: 'number',
            description: t('订阅所属用户 ID。'),
          },
          {
            name: 'data.all_subscriptions[].subscription.plan_id',
            type: 'number',
            description: t('订阅套餐 ID，可用于匹配 /api/subscription/plans 返回的 plan.id。'),
          },
          {
            name: 'data.all_subscriptions[].subscription.amount_total',
            type: 'number',
            description: t(
              '订阅总额度，原始 quota 单位，0 通常表示不限额度。换算公式：amount_total / quota_per_unit。',
            ),
          },
          {
            name: 'data.all_subscriptions[].subscription.amount_used',
            type: 'number',
            description: t(
              '订阅已使用额度，原始 quota 单位。换算公式：amount_used / quota_per_unit。',
            ),
          },
          {
            name: 'data.all_subscriptions[].subscription.start_time',
            type: 'number',
            description: t('订阅开始时间，Unix 秒级时间戳。'),
          },
          {
            name: 'data.all_subscriptions[].subscription.end_time',
            type: 'number',
            description: t('订阅结束时间，Unix 秒级时间戳。'),
          },
          {
            name: 'data.all_subscriptions[].subscription.status',
            type: 'string',
            description: t('订阅状态，常见值包括 active、expired、cancelled。'),
          },
          {
            name: 'data.all_subscriptions[].subscription.source',
            type: 'string',
            description: t('订阅来源，例如 order 表示支付订单生成，admin 表示管理员手动绑定。'),
          },
          {
            name: 'data.all_subscriptions[].subscription.last_reset_time',
            type: 'number',
            description: t('上次额度重置时间，Unix 秒级时间戳；未启用周期重置时通常为 0。'),
          },
          {
            name: 'data.all_subscriptions[].subscription.next_reset_time',
            type: 'number',
            description: t('下次额度重置时间，Unix 秒级时间戳；未启用周期重置时通常为 0。'),
          },
          {
            name: 'data.all_subscriptions[].subscription.upgrade_group',
            type: 'string',
            description: t('该套餐生效后升级到的用户分组，未配置时为空。'),
          },
          {
            name: 'data.all_subscriptions[].subscription.prev_user_group',
            type: 'string',
            description: t('套餐升级分组前的用户分组，用于订阅失效后的回退。'),
          },
          {
            name: 'data.all_subscriptions[].subscription.created_at / updated_at',
            type: 'number',
            description: t('订阅实例创建和更新时间，Unix 秒级时间戳。'),
          },
        ],
        responseExample: `{
  "success": true,
  "message": "",
  "data": {
    "billing_preference": "subscription_first",
    "subscriptions": [
      {
        "subscription": {
          "id": 12,
          "user_id": 1,
          "plan_id": 3,
          "amount_total": 500000,
          "amount_used": 12000,
          "start_time": 1710000000,
          "end_time": 1712592000,
          "status": "active",
          "source": "order",
          "last_reset_time": 1710000000,
          "next_reset_time": 1710086400,
          "upgrade_group": "premium",
          "prev_user_group": "default",
          "created_at": 1710000000,
          "updated_at": 1710000000
        }
      }
    ],
    "all_subscriptions": [
      {
        "subscription": {
          "id": 12,
          "user_id": 1,
          "plan_id": 3,
          "amount_total": 500000,
          "amount_used": 12000,
          "start_time": 1710000000,
          "end_time": 1712592000,
          "status": "active",
          "source": "order"
        }
      }
    ]
  }
}`,
      },
      {
        key: 'user-topups',
        icon: <CreditCard size={18} />,
        title: t('获取用户充值记录'),
        method: 'GET',
        path: '/api/user/topup/self',
        summary: t(
          '分页获取当前用户最近 30 天的充值与订阅支付流水。该接口与钱包管理里的账单弹窗使用同一条用户侧接口。',
        ),
        notes: [
          t('只返回当前用户自己的记录。'),
          t('p 表示页码，page_size 表示每页数量；本地分页工具也兼容 ps 和 size。'),
          t('keyword 可按订单号 trade_no 搜索。'),
          t('用户侧查询被限制为最近 30 天；管理员充值记录接口不受该时间窗口限制。'),
          t('余额充值和订阅购买都会写入充值流水；订阅购买记录通常 amount 为 0，money 为支付金额。'),
        ],
        params: [
          {
            name: 'Cookie / Session',
            required: false,
            location: 'header',
            description: t('浏览器登录后可使用 session/cookie 作为认证方式。'),
          },
          {
            name: 'Authorization',
            required: false,
            location: 'header',
            description: t(
              '可传 Bearer USER_ACCESS_TOKEN。这里是用户 access_token，不是 sk-... API Token。',
            ),
          },
          {
            name: 'New-API-User',
            required: true,
            location: 'header',
            description: t(
              '当前用户 ID，后端会用它和 session 或 access_token 中的用户 ID 做一致性校验。',
            ),
          },
          {
            name: 'p',
            required: false,
            location: 'query',
            description: t('页码，从 1 开始；不传默认为 1。'),
          },
          {
            name: 'page_size',
            required: false,
            location: 'query',
            description: t('每页数量，不传使用系统默认值，最大 100。'),
          },
          {
            name: 'keyword',
            required: false,
            location: 'query',
            description: t('订单号搜索关键字，对 trade_no 做模糊搜索。'),
          },
        ],
        requestExample: `# 获取最近 30 天充值/订阅支付流水
curl "${baseUrl}/api/user/topup/self?p=1&page_size=10" \\
  -H "Authorization: Bearer USER_ACCESS_TOKEN" \\
  -H "New-API-User: 1"

# 按订单号搜索
curl "${baseUrl}/api/user/topup/self?p=1&page_size=10&keyword=USR1NO" \\
  -H "Authorization: Bearer USER_ACCESS_TOKEN" \\
  -H "New-API-User: 1"`,
        responseFields: [
          {
            name: 'success',
            type: 'boolean',
            description: t('请求是否成功。'),
          },
          {
            name: 'message',
            type: 'string',
            description: t('接口消息，成功时通常为空字符串。'),
          },
          {
            name: 'data.page',
            type: 'number',
            description: t('当前页码。'),
          },
          {
            name: 'data.page_size',
            type: 'number',
            description: t('每页数量。'),
          },
          {
            name: 'data.total',
            type: 'number',
            description: t('总记录数。'),
          },
          {
            name: 'data.items',
            type: 'array',
            description: t('充值和订阅支付流水列表。'),
          },
          {
            name: 'data.items[].id',
            type: 'number',
            description: t('流水 ID。'),
          },
          {
            name: 'data.items[].user_id',
            type: 'number',
            description: t('用户 ID。'),
          },
          {
            name: 'data.items[].amount',
            type: 'number',
            description: t(
              '充值到账数量，原始 quota 单位；订阅支付流水通常为 0。换算公式：amount / quota_per_unit。',
            ),
          },
          {
            name: 'data.items[].money',
            type: 'number',
            description: t(
              '实际支付金额，不是 quota 单位；它表示订单支付侧金额，币种取决于对应支付配置。',
            ),
          },
          {
            name: 'data.items[].trade_no',
            type: 'string',
            description: t('支付订单号。'),
          },
          {
            name: 'data.items[].payment_method',
            type: 'string',
            description: t('支付方式，例如 alipay、wxpay、stripe、creem。'),
          },
          {
            name: 'data.items[].status',
            type: 'string',
            description: t('订单状态，常见值包括 pending、success、expired。'),
          },
          {
            name: 'data.items[].create_time / complete_time',
            type: 'number',
            description: t('创建时间和完成时间，Unix 秒级时间戳。'),
          },
        ],
        responseExample: `{
  "success": true,
  "message": "",
  "data": {
    "page": 1,
    "page_size": 10,
    "total": 2,
    "items": [
      {
        "id": 101,
        "user_id": 1,
        "amount": 100,
        "money": 10,
        "trade_no": "USR1NOabc1231710000000",
        "payment_method": "alipay",
        "create_time": 1710000000,
        "complete_time": 1710000060,
        "status": "success"
      },
      {
        "id": 102,
        "user_id": 1,
        "amount": 0,
        "money": 29.9,
        "trade_no": "SUBUSR1NOxyz7891710000100",
        "payment_method": "stripe",
        "create_time": 1710000100,
        "complete_time": 1710000160,
        "status": "success"
      }
    ]
  }
}`,
      },
      {
        key: 'list-api-tokens',
        icon: <KeyRound size={18} />,
        title: t('获取API令牌列表'),
        method: 'GET',
        path: '/api/token/',
        summary: t(
          '分页获取当前用户创建的 API 令牌列表。返回的 key 为打码后的展示值，不会直接泄露完整 key。',
        ),
        notes: [
          t('该接口只返回当前登录用户自己的令牌，不返回其他用户令牌。'),
          t('p 表示页码，page_size 表示每页数量；本地代码也兼容 ps 和 size。'),
          t('page_size 最大会被限制为 100。'),
          t('items 中的 key 是打码值；如需完整 key，请调用 POST /api/token/{id}/key。'),
        ],
        params: [
          {
            name: 'Cookie / Session',
            required: false,
            location: 'header',
            description: t('浏览器登录后可使用 session/cookie 作为认证方式。'),
          },
          {
            name: 'Authorization',
            required: false,
            location: 'header',
            description: t(
              '可传 Bearer USER_ACCESS_TOKEN。这里是用户 access_token，不是 sk-... API Token。',
            ),
          },
          {
            name: 'New-API-User',
            required: true,
            location: 'header',
            description: t(
              '当前用户 ID，后端会用它和 session 或 access_token 中的用户 ID 做一致性校验。',
            ),
          },
          {
            name: 'p',
            required: false,
            location: 'query',
            description: t('页码，从 1 开始；不传默认为 1。'),
          },
          {
            name: 'page_size',
            required: false,
            location: 'query',
            description: t('每页数量，不传使用系统默认值，最大 100。'),
          },
        ],
        requestExample: `# 浏览器登录后的 session/cookie 方式
curl "${baseUrl}/api/token/?p=1&page_size=10" \\
  -H "Cookie: session=YOUR_SESSION_COOKIE" \\
  -H "New-API-User: 1"

# 用户 access_token 方式，不是 sk-... API Token
curl "${baseUrl}/api/token/?p=1&page_size=10" \\
  -H "Authorization: Bearer USER_ACCESS_TOKEN" \\
  -H "New-API-User: 1"`,
        responseFields: [
          {
            name: 'success',
            type: 'boolean',
            description: t('请求是否成功。'),
          },
          {
            name: 'message',
            type: 'string',
            description: t('接口消息，成功时通常为空字符串。'),
          },
          {
            name: 'data.page',
            type: 'number',
            description: t('当前页码。'),
          },
          {
            name: 'data.page_size',
            type: 'number',
            description: t('每页数量。'),
          },
          {
            name: 'data.total',
            type: 'number',
            description: t('当前用户令牌总数。'),
          },
          {
            name: 'data.items',
            type: 'array',
            description: t('令牌列表。'),
          },
          {
            name: 'data.items[].id',
            type: 'number',
            description: t('令牌 ID，可用于获取指定令牌或完整 key。'),
          },
          {
            name: 'data.items[].name',
            type: 'string',
            description: t('令牌名称。'),
          },
          {
            name: 'data.items[].key',
            type: 'string',
            description: t('打码后的令牌 key，不是完整可用 key。'),
          },
          {
            name: 'data.items[].status',
            type: 'number',
            description: t('令牌状态，1 通常表示启用，2 通常表示禁用。'),
          },
          {
            name: 'data.items[].expired_time',
            type: 'number',
            description: t('过期时间，Unix 秒级时间戳；-1 表示永不过期。'),
          },
          {
            name: 'data.items[].remain_quota',
            type: 'number',
            description: t(
              '令牌剩余额度，原始 quota 单位。换算公式：remain_quota / quota_per_unit；无限额度令牌可为 0。',
            ),
          },
          {
            name: 'data.items[].unlimited_quota',
            type: 'boolean',
            description: t('是否无限额度。'),
          },
          {
            name: 'data.items[].used_quota',
            type: 'number',
            description: t(
              '令牌已用额度，原始 quota 单位。换算公式：used_quota / quota_per_unit。',
            ),
          },
          {
            name: 'data.items[].model_limits_enabled',
            type: 'boolean',
            description: t('是否启用模型限制。'),
          },
          {
            name: 'data.items[].model_limits',
            type: 'string',
            description: t('允许模型列表，通常为逗号分隔字符串。'),
          },
          {
            name: 'data.items[].group',
            type: 'string',
            description: t('令牌所属分组。'),
          },
          {
            name: 'data.items[].created_time / accessed_time',
            type: 'number',
            description: t('创建时间和最近访问时间，Unix 秒级时间戳。'),
          },
        ],
        responseExample: `{
  "success": true,
  "message": "",
  "data": {
    "page": 1,
    "page_size": 10,
    "total": 1,
    "items": [
      {
        "id": 12,
        "user_id": 1,
        "key": "abcd**********wxyz",
        "status": 1,
        "name": "test-key",
        "created_time": 1710000000,
        "accessed_time": 1710000000,
        "expired_time": -1,
        "remain_quota": 0,
        "unlimited_quota": true,
        "model_limits_enabled": false,
        "model_limits": "",
        "allow_ips": "",
        "used_quota": 0,
        "group": "",
        "cross_group_retry": false
      }
    ]
  }
}`,
      },
      {
        key: 'get-api-token-key',
        icon: <KeyRound size={18} />,
        title: t('获取完整API令牌'),
        method: 'POST',
        path: '/api/token/{id}/key',
        summary: t(
          '根据令牌 ID 获取当前用户某个 API 令牌的完整值。列表和详情接口只返回打码值，真正可用于模型调用的完整令牌需要通过该接口获取。',
        ),
        notes: [
          t('只能获取当前登录用户自己名下的完整令牌。'),
          t('id 可以从 GET /api/token/ 的 data.items[].id 中获得。'),
          t('返回的 data.key 是未加 sk- 前缀的完整令牌值；模型接口调用时通常使用 Authorization: Bearer sk-完整令牌。'),
          t('GET /api/token/{id} 只返回打码值，不会返回完整令牌。'),
        ],
        params: [
          {
            name: 'Cookie / Session',
            required: false,
            location: 'header',
            description: t('浏览器登录后可使用 session/cookie 作为认证方式。'),
          },
          {
            name: 'Authorization',
            required: false,
            location: 'header',
            description: t(
              '可传 Bearer USER_ACCESS_TOKEN。这里是用户 access_token，不是 sk-... API Token。',
            ),
          },
          {
            name: 'New-API-User',
            required: true,
            location: 'header',
            description: t(
              '当前用户 ID，后端会用它和 session 或 access_token 中的用户 ID 做一致性校验。',
            ),
          },
          {
            name: 'id',
            required: true,
            location: 'path',
            description: t('令牌 ID，可从令牌列表接口返回的 data.items[].id 获取。'),
          },
        ],
        requestExample: `# 先通过 GET /api/token/ 拿到令牌 ID，再获取完整 key
curl -X POST "${baseUrl}/api/token/12/key" \\
  -H "Authorization: Bearer USER_ACCESS_TOKEN" \\
  -H "New-API-User: 1"`,
        responseFields: [
          {
            name: 'success',
            type: 'boolean',
            description: t('请求是否成功。'),
          },
          {
            name: 'message',
            type: 'string',
            description: t('接口消息，成功时通常为空字符串。'),
          },
          {
            name: 'data.key',
            type: 'string',
            description: t('完整 API 令牌值。模型接口调用时通常需要拼接为 Bearer sk-完整令牌。'),
          },
        ],
        responseExample: `{
  "success": true,
  "message": "",
  "data": {
    "key": "abcd1234efgh5678ijkl9012mnop3456"
  }
}`,
      },
      {
        key: 'create-api-token',
        icon: <KeyRound size={18} />,
        title: t('创建API令牌'),
        method: 'POST',
        path: '/api/token/',
        summary: t(
          '为当前用户创建一个用于模型调用的 API 令牌。创建成功后，可在令牌列表中通过令牌 ID 获取完整令牌。',
        ),
        notes: [
          t('创建出来的令牌用于 AI 模型接口调用，实际使用时 Authorization 格式为 Bearer sk-完整令牌。'),
          t('创建接口成功时不会直接返回完整令牌，只返回 success。'),
          t('需要先通过 /api/token/?p=1&size=10 获取令牌 ID，再调用 POST /api/token/{id}/key 获取完整令牌。'),
          t('name 最长 50 个字符。'),
          t('expired_time 为 Unix 秒级时间戳，-1 表示永不过期。'),
          t('unlimited_quota 为 false 时，remain_quota 必须大于 0。'),
          t('model_limits 需要传逗号分隔字符串，例如 gpt-4o-mini,gpt-4.1。'),
          t('group 可为空；可用分组可以通过 /api/user/self/groups 获取。'),
        ],
        params: [
          {
            name: 'Cookie / Session',
            required: false,
            location: 'header',
            description: t('浏览器登录后可使用 session/cookie 作为认证方式。'),
          },
          {
            name: 'Authorization',
            required: false,
            location: 'header',
            description: t(
              '可传 Bearer USER_ACCESS_TOKEN。这里是用户 access_token，不是 sk-... API Token。',
            ),
          },
          {
            name: 'New-API-User',
            required: true,
            location: 'header',
            description: t(
              '当前用户 ID，后端会用它和 session 或 access_token 中的用户 ID 做一致性校验。',
            ),
          },
          {
            name: 'Content-Type',
            required: true,
            location: 'header',
            description: t('固定为 application/json。'),
          },
          {
            name: 'name',
            required: true,
            location: 'body',
            description: t('令牌名称，最长 50 个字符。'),
          },
          {
            name: 'expired_time',
            required: true,
            location: 'body',
            description: t('过期时间，Unix 秒级时间戳；-1 表示永不过期。'),
          },
          {
            name: 'unlimited_quota',
            required: true,
            location: 'body',
            description: t('是否无限额度。true 时 remain_quota 可为 0。'),
          },
          {
            name: 'remain_quota',
            required: false,
            location: 'body',
            description: t(
              '有限额度时传入，单位是原始 quota；unlimited_quota 为 false 时必须大于 0。',
            ),
          },
          {
            name: 'model_limits_enabled',
            required: false,
            location: 'body',
            description: t('是否启用模型限制；后端会按该字段和 model_limits 判断。'),
          },
          {
            name: 'model_limits',
            required: false,
            location: 'body',
            description: t('允许的模型列表，逗号分隔字符串；为空表示不限制。'),
          },
          {
            name: 'allow_ips',
            required: false,
            location: 'body',
            description: t('IP 白名单，可为空；多个 IP 通常按换行分隔。'),
          },
          {
            name: 'group',
            required: false,
            location: 'body',
            description: t('令牌使用的分组，可为空。'),
          },
          {
            name: 'cross_group_retry',
            required: false,
            location: 'body',
            description: t('跨分组重试，仅 auto 分组场景下有意义。'),
          },
        ],
        requestExample: `curl -X POST "${baseUrl}/api/token/" \\
  -H "Content-Type: application/json" \\
  -H "Authorization: Bearer USER_ACCESS_TOKEN" \\
  -H "New-API-User: 1" \\
  -d '{
    "name": "test-key",
    "expired_time": -1,
    "unlimited_quota": true,
    "remain_quota": 0,
    "model_limits_enabled": false,
    "model_limits": "",
    "allow_ips": "",
    "group": "",
    "cross_group_retry": false
  }'`,
        responseFields: [
          {
            name: 'success',
            type: 'boolean',
            description: t('请求是否成功。'),
          },
          {
            name: 'message',
            type: 'string',
            description: t('接口消息，成功时通常为空字符串。'),
          },
        ],
        responseExample: `{
  "success": true,
  "message": ""
}`,
      },
      createAiEndpoint({
        key: 'list-models',
        title: '获取可用模型',
        method: 'GET',
        path: '/v1/models',
        summary: '获取当前 API Token 可用的模型对象列表。',
        notes: [
          '返回结果会受渠道、分组、模型映射和令牌模型限制影响。',
          'OpenAI 格式响应中会包含 supported_endpoint_types，可用来区分文本、绘图、Embedding、Rerank、视频等能力。',
        ],
        params: [],
        responseExample: `{
  "object": "list",
  "data": [
    {
      "id": "gpt-4o-mini",
      "object": "model",
      "created": 1710000000,
      "owned_by": "openai",
      "supported_endpoint_types": ["openai"]
    },
    {
      "id": "gpt-image-1",
      "object": "model",
      "created": 1710000000,
      "owned_by": "openai",
      "supported_endpoint_types": ["image-generation"]
    }
  ]
}`,
      }),
      createAiEndpoint({
        key: 'retrieve-model',
        title: '检索模型',
        method: 'GET',
        path: '/v1/models/{model}',
        summary: '根据模型 ID 获取单个模型信息。',
        notes: ['把路径里的 {model} 替换成实际模型名，例如 gpt-4o-mini。'],
        params: [
          {
            name: 'model',
            required: true,
            location: 'path',
            description: t('模型 ID，例如 gpt-4o-mini。'),
          },
        ],
        responseExample: `{
  "id": "gpt-4o-mini",
  "object": "model",
  "created": 1710000000,
  "owned_by": "system"
}`,
      }),
      {
        key: 'chat-completions',
        icon: <MessageCircle size={18} />,
        title: t('创建聊天补全'),
        method: 'POST',
        path: '/v1/chat/completions',
        summary: t(
          'OpenAI Chat Completions 兼容接口，根据对话消息创建模型响应，支持流式和非流式。',
        ),
        notes: [
          t('stream 为 true 时返回 Server-Sent Events 流式数据。'),
          t('tools、tool_choice、response_format、reasoning_effort 等高级参数是否生效取决于上游模型能力。'),
        ],
        params: [
          {
            name: 'Authorization',
            required: true,
            location: 'header',
            description: t('Bearer sk-xxxxxx，使用令牌管理中创建的 API Token。'),
          },
          {
            name: 'Content-Type',
            required: true,
            location: 'header',
            description: t('固定为 application/json。'),
          },
          {
            name: 'model',
            required: true,
            location: 'body',
            description: t('模型 ID，例如 gpt-4o-mini、gpt-4.1、claude-3-7-sonnet 等。'),
          },
          {
            name: 'messages',
            required: true,
            location: 'body',
            description: t(
              '对话消息数组，常见 role 包括 system、user、assistant、tool。',
            ),
          },
          {
            name: 'temperature',
            required: false,
            location: 'body',
            description: t('采样温度，通常 0 到 2，默认 1。'),
          },
          {
            name: 'top_p',
            required: false,
            location: 'body',
            description: t('核采样参数，通常 0 到 1，默认 1。'),
          },
          {
            name: 'n',
            required: false,
            location: 'body',
            description: t('生成数量，默认 1。'),
          },
          {
            name: 'stream',
            required: false,
            location: 'body',
            description: t('是否使用流式响应，默认 false。'),
          },
          {
            name: 'stream_options',
            required: false,
            location: 'body',
            description: t('流式响应配置，例如 include_usage。'),
          },
          {
            name: 'stop',
            required: false,
            location: 'body',
            description: t('停止序列，字符串或字符串数组。'),
          },
          {
            name: 'max_tokens',
            required: false,
            location: 'body',
            description: t('最大生成 Token 数。'),
          },
          {
            name: 'max_completion_tokens',
            required: false,
            location: 'body',
            description: t('最大补全 Token 数，部分新模型优先使用该字段。'),
          },
          {
            name: 'presence_penalty',
            required: false,
            location: 'body',
            description: t('存在惩罚，通常 -2 到 2，默认 0。'),
          },
          {
            name: 'frequency_penalty',
            required: false,
            location: 'body',
            description: t('频率惩罚，通常 -2 到 2，默认 0。'),
          },
          {
            name: 'logit_bias',
            required: false,
            location: 'body',
            description: t('调整指定 token 的采样偏置。'),
          },
          {
            name: 'user',
            required: false,
            location: 'body',
            description: t('终端用户标识，可用于上游风控或追踪。'),
          },
          {
            name: 'tools',
            required: false,
            location: 'body',
            description: t('工具定义数组，常用于 function calling。'),
          },
          {
            name: 'tool_choice',
            required: false,
            location: 'body',
            description: t('工具调用策略，支持字符串或对象形式。'),
          },
          {
            name: 'response_format',
            required: false,
            location: 'body',
            description: t('响应格式约束，例如 JSON object / JSON schema。'),
          },
          {
            name: 'seed',
            required: false,
            location: 'body',
            description: t('随机种子，部分模型支持更稳定的复现。'),
          },
          {
            name: 'reasoning_effort',
            required: false,
            location: 'body',
            description: t('推理强度，支持 low、medium、high 的模型可使用。'),
          },
          {
            name: 'modalities',
            required: false,
            location: 'body',
            description: t('多模态输出类型数组，例如 text、audio。'),
          },
          {
            name: 'audio',
            required: false,
            location: 'body',
            description: t('音频输出配置，需配合支持音频输出的模型和 modalities 使用。'),
          },
        ],
        requestExample: `curl -X POST "${baseUrl}/v1/chat/completions" \\
  -H "Authorization: Bearer sk-YOUR_API_TOKEN" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "gpt-4o-mini",
    "messages": [
      {
        "role": "system",
        "content": "你是一个简洁可靠的助手。"
      },
      {
        "role": "user",
        "content": "用一句话介绍这个接口。"
      }
    ],
    "temperature": 0.7,
    "stream": false
  }'`,
        responseExample: `{
  "id": "chatcmpl-xxxxxxxx",
  "object": "chat.completion",
  "created": 1710000000,
  "model": "gpt-4o-mini",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "这是一个兼容 OpenAI 的聊天补全接口，可根据 messages 生成模型回复。"
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 24,
    "completion_tokens": 18,
    "total_tokens": 42
  }
}

// stream: true 时返回 text/event-stream，形如：
data: {"id":"chatcmpl-xxxxxxxx","object":"chat.completion.chunk","choices":[{"delta":{"content":"你好"},"index":0}]}

data: [DONE]`,
      },
      createAiEndpoint({
        key: 'claude-messages',
        title: 'Claude Messages',
        path: '/v1/messages',
        summary: 'Claude Messages 兼容接口，用 messages 创建 Claude 风格模型回复。',
        notes: ['请求结构接近 Anthropic Messages，通常包含 model、messages、max_tokens。'],
        params: [
          modelParam,
          {
            name: 'messages',
            required: true,
            location: 'body',
            description: t('Claude 对话消息数组。'),
          },
          {
            name: 'max_tokens',
            required: true,
            location: 'body',
            description: t('最大输出 Token 数。'),
          },
          {
            name: 'system',
            required: false,
            location: 'body',
            description: t('系统提示词，可传字符串或内容块。'),
          },
          {
            name: 'stream',
            required: false,
            location: 'body',
            description: t('是否流式返回。'),
          },
        ],
        body: `{
    "model": "claude-3-7-sonnet",
    "max_tokens": 1024,
    "messages": [
      {
        "role": "user",
        "content": "你好"
      }
    ]
  }`,
      }),
      createAiEndpoint({
        key: 'responses',
        title: '创建 Responses',
        path: '/v1/responses',
        summary: 'OpenAI Responses 兼容接口，适合统一文本、多模态和工具调用响应。',
        notes: ['支持 input、instructions、tools、response_format 等字段，具体能力取决于模型和渠道。'],
        params: [
          modelParam,
          {
            name: 'input',
            required: true,
            location: 'body',
            description: t('输入内容，可以是字符串或结构化输入数组。'),
          },
          {
            name: 'instructions',
            required: false,
            location: 'body',
            description: t('系统级指令。'),
          },
          {
            name: 'stream',
            required: false,
            location: 'body',
            description: t('是否流式返回。'),
          },
        ],
        body: `{
    "model": "gpt-4.1",
    "input": "用一句话介绍你自己"
  }`,
      }),
      createAiEndpoint({
        key: 'responses-compact',
        title: '压缩 Responses',
        path: '/v1/responses/compact',
        summary: 'Responses 压缩接口，用于将对话或上下文压缩为更短的响应输入。',
        notes: ['本地路由会以 OpenAI Responses Compact 模式转发。'],
        params: [
          modelParam,
          {
            name: 'input',
            required: true,
            location: 'body',
            description: t('需要压缩的上下文内容。'),
          },
        ],
        body: `{
    "model": "gpt-4.1",
    "input": "需要压缩的长上下文内容"
  }`,
      }),
      createAiEndpoint({
        key: 'completions',
        title: '创建文本补全',
        path: '/v1/completions',
        summary: 'OpenAI Completions 兼容接口，适合旧版文本补全模型。',
        notes: ['新模型通常建议优先使用 /v1/chat/completions 或 /v1/responses。'],
        params: [
          modelParam,
          {
            name: 'prompt',
            required: true,
            location: 'body',
            description: t('补全文本提示词。'),
          },
          {
            name: 'max_tokens',
            required: false,
            location: 'body',
            description: t('最大生成 Token 数。'),
          },
        ],
        body: `{
    "model": "gpt-3.5-turbo-instruct",
    "prompt": "写一句欢迎语",
    "max_tokens": 64
  }`,
      }),
      createAiEndpoint({
        key: 'embeddings',
        title: '创建 Embeddings',
        path: '/v1/embeddings',
        summary: '将文本输入转换为向量表示，用于检索、聚类和相似度计算。',
        params: [
          modelParam,
          {
            name: 'input',
            required: true,
            location: 'body',
            description: t('待向量化的文本、文本数组或 token 数组。'),
          },
        ],
        body: `{
    "model": "text-embedding-3-small",
    "input": "这是一段需要向量化的文本"
  }`,
        responseExample: `{
  "object": "list",
  "data": [
    {
      "object": "embedding",
      "index": 0,
      "embedding": [0.0123, -0.0456]
    }
  ],
  "model": "text-embedding-3-small",
  "usage": {
    "prompt_tokens": 8,
    "total_tokens": 8
  }
}`,
      }),
      createAiEndpoint({
        key: 'engine-embeddings',
        title: 'Engine Embeddings',
        path: '/v1/engines/{model}/embeddings',
        summary: '兼容旧版 OpenAI engines 路径的 Embeddings 接口。',
        notes: ['把路径里的 {model} 替换为实际 embedding 模型名。'],
        params: [
          {
            name: 'model',
            required: true,
            location: 'path',
            description: t('Embedding 模型 ID。'),
          },
          {
            name: 'input',
            required: true,
            location: 'body',
            description: t('待向量化内容。'),
          },
        ],
        body: `{
    "input": "hello world"
  }`,
      }),
      createAiEndpoint({
        key: 'rerank',
        title: '重排序',
        path: '/v1/rerank',
        summary: '对候选文档按 query 相关性重新排序。',
        params: [
          modelParam,
          {
            name: 'query',
            required: true,
            location: 'body',
            description: t('查询文本。'),
          },
          {
            name: 'documents',
            required: true,
            location: 'body',
            description: t('候选文档数组。'),
          },
          {
            name: 'top_n',
            required: false,
            location: 'body',
            description: t('返回前 N 条结果。'),
          },
        ],
        body: `{
    "model": "rerank-model",
    "query": "什么是 API 网关",
    "documents": ["文档 A", "文档 B"]
  }`,
      }),
      createAiEndpoint({
        key: 'moderations',
        title: '内容审查',
        path: '/v1/moderations',
        summary: '对输入内容进行安全审查和分类。',
        params: [
          modelParam,
          {
            name: 'input',
            required: true,
            location: 'body',
            description: t('需要审查的文本或文本数组。'),
          },
        ],
        body: `{
    "model": "omni-moderation-latest",
    "input": "需要审查的内容"
  }`,
      }),
      createAiEndpoint({
        key: 'image-generations',
        title: '图像生成',
        path: '/v1/images/generations',
        summary: '根据文本提示词生成图片。',
        params: [
          modelParam,
          {
            name: 'prompt',
            required: true,
            location: 'body',
            description: t('图片生成提示词。'),
          },
          {
            name: 'size',
            required: false,
            location: 'body',
            description: t('图片尺寸，例如 1024x1024。'),
          },
          {
            name: 'n',
            required: false,
            location: 'body',
            description: t('生成图片数量。'),
          },
        ],
        body: `{
    "model": "gpt-image-1",
    "prompt": "一张未来科技风的 API 控制台海报",
    "size": "1024x1024"
  }`,
      }),
      createAiEndpoint({
        key: 'image-edits',
        title: '图像编辑',
        path: '/v1/images/edits',
        summary: '基于输入图片和提示词编辑图片。',
        notes: ['该接口通常使用 multipart/form-data；页面示例仅展示核心字段。'],
        params: [
          {
            name: 'image',
            required: true,
            location: 'form',
            description: t('需要编辑的图片文件。'),
          },
          {
            name: 'prompt',
            required: true,
            location: 'form',
            description: t('编辑提示词。'),
          },
          {
            name: 'model',
            required: false,
            location: 'form',
            description: t('图像编辑模型。'),
          },
        ],
        body: `{
    "model": "gpt-image-1",
    "prompt": "把背景改成蓝色霓虹城市"
  }`,
      }),
      createAiEndpoint({
        key: 'legacy-edits',
        title: '旧版编辑',
        path: '/v1/edits',
        summary: '兼容旧版 edits 路径的编辑接口。',
        notes: ['新项目通常优先使用 /v1/responses 或图像编辑接口。'],
        params: [
          modelParam,
          {
            name: 'input',
            required: true,
            location: 'body',
            description: t('需要编辑的输入内容。'),
          },
          {
            name: 'instruction',
            required: true,
            location: 'body',
            description: t('编辑指令。'),
          },
        ],
        body: `{
    "model": "text-davinci-edit-001",
    "input": "待编辑文本",
    "instruction": "改写得更简洁"
  }`,
      }),
      createAiEndpoint({
        key: 'audio-gemini',
        title: '原生Gemini音频',
        path: '/v1beta/models/{model}:generateContent',
        summary: 'Gemini 原生格式的音频/多模态内容生成接口。',
        notes: ['把路径里的 {model} 替换成实际 Gemini 多模态模型名。'],
        params: [
          {
            name: 'model',
            required: true,
            location: 'path',
            description: t('Gemini 多模态模型名。'),
          },
          {
            name: 'contents',
            required: true,
            location: 'body',
            description: t('Gemini contents 数组，可包含文本、音频等 part。'),
          },
        ],
        body: `{
    "contents": [
      {
        "parts": [
          {
            "text": "请分析这段音频"
          }
        ]
      }
    ]
  }`,
      }),
      createAiEndpoint({
        key: 'audio-speech',
        title: '语音合成',
        path: '/v1/audio/speech',
        summary: '将文本转换为语音音频。',
        params: [
          modelParam,
          {
            name: 'input',
            required: true,
            location: 'body',
            description: t('需要合成语音的文本。'),
          },
          {
            name: 'voice',
            required: true,
            location: 'body',
            description: t('声音名称。'),
          },
          {
            name: 'response_format',
            required: false,
            location: 'body',
            description: t('音频格式，例如 mp3、wav。'),
          },
        ],
        body: `{
    "model": "tts-1",
    "input": "你好，欢迎使用 API。",
    "voice": "alloy"
  }`,
        responseExample: t('返回音频二进制数据，Content-Type 取决于 response_format。'),
      }),
      createAiEndpoint({
        key: 'audio-transcriptions',
        title: '语音转写',
        path: '/v1/audio/transcriptions',
        summary: '将音频文件转写为文本。',
        notes: ['该接口通常使用 multipart/form-data。'],
        params: [
          {
            name: 'file',
            required: true,
            location: 'form',
            description: t('音频文件。'),
          },
          {
            name: 'model',
            required: true,
            location: 'form',
            description: t('转写模型，例如 whisper-1。'),
          },
          {
            name: 'language',
            required: false,
            location: 'form',
            description: t('音频语言。'),
          },
        ],
        body: `{
    "model": "whisper-1",
    "file": "@audio.mp3"
  }`,
        responseExample: `{
  "text": "转写后的文本"
}`,
      }),
      createAiEndpoint({
        key: 'audio-translations',
        title: '语音翻译',
        path: '/v1/audio/translations',
        summary: '将音频翻译为目标语言文本，常见为翻译成英文。',
        notes: ['该接口通常使用 multipart/form-data。'],
        params: [
          {
            name: 'file',
            required: true,
            location: 'form',
            description: t('音频文件。'),
          },
          {
            name: 'model',
            required: true,
            location: 'form',
            description: t('翻译模型，例如 whisper-1。'),
          },
        ],
        body: `{
    "model": "whisper-1",
    "file": "@audio.mp3"
  }`,
        responseExample: `{
  "text": "Translated text"
}`,
      }),
      createAiEndpoint({
        key: 'realtime',
        title: '实时会话',
        method: 'GET',
        path: '/v1/realtime',
        summary: 'OpenAI Realtime 兼容 WebSocket 接口，用于低延迟实时交互。',
        notes: ['本地路由为 GET /v1/realtime，实际使用时通常通过 WebSocket 建连。'],
        params: [
          {
            name: 'Authorization',
            required: true,
            location: 'header',
            description: t('Bearer sk-xxxxxx。'),
          },
        ],
        responseExample: t('升级为 WebSocket 连接后收发实时事件。'),
      }),
      createAiEndpoint({
        key: 'gemini-list-models',
        title: 'Gemini 列出模型',
        method: 'GET',
        path: '/v1beta/models',
        summary: 'Gemini API 兼容的模型列表接口。',
        notes: ['也可以通过 /v1beta/openai/models 获取 OpenAI 兼容模型列表。'],
        params: [],
      }),
      createAiEndpoint({
        key: 'gemini-openai-models',
        title: 'Gemini OpenAI 模型列表',
        method: 'GET',
        path: '/v1beta/openai/models',
        summary: 'Gemini OpenAI 兼容路径下的模型列表接口。',
        params: [],
      }),
      createAiEndpoint({
        key: 'gemini-generate-content',
        title: 'Gemini 生成内容',
        path: '/v1beta/models/{model}:generateContent',
        summary: 'Gemini generateContent 兼容接口。',
        notes: ['把路径里的 {model} 替换成实际 Gemini 模型名。'],
        params: [
          {
            name: 'model',
            required: true,
            location: 'path',
            description: t('Gemini 模型名。'),
          },
          {
            name: 'contents',
            required: true,
            location: 'body',
            description: t('Gemini 内容数组。'),
          },
        ],
        body: `{
    "contents": [
      {
        "parts": [
          {
            "text": "你好"
          }
        ]
      }
    ]
  }`,
      }),
      createAiEndpoint({
        key: 'gemini-stream-generate-content',
        title: 'Gemini 流式生成内容',
        path: '/v1beta/models/{model}:streamGenerateContent',
        summary: 'Gemini streamGenerateContent 兼容接口，返回流式内容。',
        notes: ['把路径里的 {model} 替换成实际 Gemini 模型名。'],
        params: [
          {
            name: 'model',
            required: true,
            location: 'path',
            description: t('Gemini 模型名。'),
          },
          {
            name: 'contents',
            required: true,
            location: 'body',
            description: t('Gemini 内容数组。'),
          },
        ],
        body: `{
    "contents": [
      {
        "parts": [
          {
            "text": "流式讲个短故事"
          }
        ]
      }
    ]
  }`,
      }),
      createAiEndpoint({
        key: 'files-list',
        title: '列出文件（未实现）',
        method: 'GET',
        path: '/v1/files',
        summary: '列出文件占位接口，当前本地路由返回未实现。',
        notes: ['该接口在本地路由中接入 RelayNotImplemented。'],
        params: [],
        responseExample: `{
  "error": {
    "message": "not implemented"
  }
}`,
      }),
      createAiEndpoint({
        key: 'files-create',
        title: '上传文件（未实现）',
        path: '/v1/files',
        summary: '上传文件占位接口，当前本地路由返回未实现。',
        notes: ['该接口通常使用 multipart/form-data；当前本地路由未实现。'],
        params: [
          {
            name: 'file',
            required: true,
            location: 'form',
            description: t('待上传文件。'),
          },
          {
            name: 'purpose',
            required: false,
            location: 'form',
            description: t('文件用途。'),
          },
        ],
        body: `{
    "purpose": "assistants",
    "file": "@file.jsonl"
  }`,
      }),
      createAiEndpoint({
        key: 'files-retrieve',
        title: '获取文件信息（未实现）',
        method: 'GET',
        path: '/v1/files/{id}',
        summary: '获取文件元信息占位接口，当前本地路由返回未实现。',
        params: [
          {
            name: 'id',
            required: true,
            location: 'path',
            description: t('文件 ID。'),
          },
        ],
      }),
      createAiEndpoint({
        key: 'files-delete',
        title: '删除文件（未实现）',
        method: 'DELETE',
        path: '/v1/files/{id}',
        summary: '删除文件占位接口，当前本地路由返回未实现。',
        params: [
          {
            name: 'id',
            required: true,
            location: 'path',
            description: t('文件 ID。'),
          },
        ],
      }),
      createAiEndpoint({
        key: 'files-content',
        title: '获取文件内容（未实现）',
        method: 'GET',
        path: '/v1/files/{id}/content',
        summary: '下载文件内容占位接口，当前本地路由返回未实现。',
        params: [
          {
            name: 'id',
            required: true,
            location: 'path',
            description: t('文件 ID。'),
          },
        ],
      }),
      createAiEndpoint({
        key: 'video-generations',
        title: '视频生成',
        path: '/v1/video/generations',
        summary: '提交视频生成任务。',
        params: [
          modelParam,
          {
            name: 'prompt',
            required: true,
            location: 'body',
            description: t('视频生成提示词。'),
          },
        ],
        body: `{
    "model": "video-model",
    "prompt": "未来城市中的自动驾驶飞车"
  }`,
      }),
      createAiEndpoint({
        key: 'video-generation-result',
        title: '查询视频生成任务',
        method: 'GET',
        path: '/v1/video/generations/{task_id}',
        summary: '根据任务 ID 查询视频生成任务状态和结果。',
        params: [taskIdParam],
      }),
      createAiEndpoint({
        key: 'openai-videos',
        title: '创建视频',
        path: '/v1/videos',
        summary: 'OpenAI 兼容的视频创建接口。',
        params: [
          modelParam,
          {
            name: 'prompt',
            required: true,
            location: 'body',
            description: t('视频提示词。'),
          },
        ],
        body: `{
    "model": "video-model",
    "prompt": "一只猫在月球上跳舞"
  }`,
      }),
      createAiEndpoint({
        key: 'openai-video-result',
        title: '获取视频任务',
        method: 'GET',
        path: '/v1/videos/{task_id}',
        summary: '查询 OpenAI 兼容视频任务结果。',
        params: [taskIdParam],
      }),
      createAiEndpoint({
        key: 'openai-video-content',
        title: '下载视频内容',
        method: 'GET',
        path: '/v1/videos/{task_id}/content',
        summary: '下载视频任务生成的内容文件。',
        notes: ['该路由支持 TokenOrUserAuth，API 客户端仍建议使用 sk-... API Token。'],
        params: [taskIdParam],
        responseExample: t('返回视频二进制内容。'),
      }),
      createAiEndpoint({
        key: 'openai-video-remix',
        title: '视频 Remix',
        path: '/v1/videos/{video_id}/remix',
        summary: '基于已有视频创建 remix 任务。',
        params: [
          {
            name: 'video_id',
            required: true,
            location: 'path',
            description: t('已有视频 ID。'),
          },
          {
            name: 'prompt',
            required: false,
            location: 'body',
            description: t('Remix 提示词。'),
          },
        ],
        body: `{
    "prompt": "改成夜晚霓虹风格"
  }`,
      }),
      createAiEndpoint({
        key: 'suno-submit',
        title: 'Suno 提交任务',
        path: '/suno/submit/{action}',
        summary: '提交 Suno 音乐生成相关任务。',
        notes: ['把 {action} 替换成具体动作，具体字段取决于上游 Suno 渠道。'],
        params: [
          {
            name: 'action',
            required: true,
            location: 'path',
            description: t('任务动作名称。'),
          },
        ],
        body: `{
    "prompt": "一首未来感电子音乐"
  }`,
      }),
      createAiEndpoint({
        key: 'suno-fetch',
        title: 'Suno 查询任务',
        path: '/suno/fetch',
        summary: '批量查询 Suno 任务结果。',
        params: [
          {
            name: 'ids',
            required: true,
            location: 'body',
            description: t('任务 ID 数组。'),
          },
        ],
        body: `{
    "ids": ["task-id"]
  }`,
      }),
      createAiEndpoint({
        key: 'suno-fetch-id',
        title: 'Suno 查询单个任务',
        method: 'GET',
        path: '/suno/fetch/{id}',
        summary: '根据任务 ID 查询单个 Suno 任务。',
        params: [
          {
            name: 'id',
            required: true,
            location: 'path',
            description: t('Suno 任务 ID。'),
          },
        ],
      }),
      ];
    },
    [baseUrl, t],
  );

  const [activeEndpointKey, setActiveEndpointKey] = useState(
    endpointCards[0]?.key || '',
  );
  const [expandedGroups, setExpandedGroups] = useState({
    user: false,
    ai: false,
  });
  const [expandedCategories, setExpandedCategories] = useState({});
  const [expandedDocSections, setExpandedDocSections] = useState({});

  const activeEndpoint =
    endpointCards.find((endpoint) => endpoint.key === activeEndpointKey) ||
    endpointCards[0];

  const parameterGroups = useMemo(() => {
    const order = ['path', 'query', 'header', 'body'];
    const grouped = (activeEndpoint?.params || []).reduce((acc, param) => {
      const key = param.location || 'other';
      if (!acc[key]) acc[key] = [];
      acc[key].push(param);
      return acc;
    }, {});
    return [
      ...order.filter((key) => grouped[key]),
      ...Object.keys(grouped).filter((key) => !order.includes(key)),
    ].map((key) => ({
      key,
      title:
        {
          path: 'Path',
          query: 'Query',
          header: 'Header',
          body: 'Body',
        }[key] || key,
      items: grouped[key],
    }));
  }, [activeEndpoint]);

  const responseFieldTree = useMemo(() => {
    const createNode = (name, path) => ({
      key: path,
      name,
      path,
      field: null,
      children: [],
      childMap: new Map(),
    });

    const root = createNode('__root__', '__root__');
    (activeEndpoint?.responseFields || []).forEach((field) => {
      const rawName = String(field.name || '').trim();
      if (!rawName) return;

      const parts = rawName.split('.').map((part) => part.trim()).filter(Boolean);
      let cursor = root;
      let path = '';

      parts.forEach((rawPart, index) => {
        const isArrayPart = rawPart.endsWith('[]');
        const part = isArrayPart ? rawPart.slice(0, -2) : rawPart;
        path = path ? `${path}.${part}` : part;
        if (!cursor.childMap.has(part)) {
          const node = createNode(part, path);
          cursor.childMap.set(part, node);
          cursor.children.push(node);
        }

        cursor = cursor.childMap.get(part);
        if (isArrayPart && !cursor.field) {
          cursor.field = {
            name: path,
            type: 'array',
            description: t('数组项。'),
          };
        }
        if (index === parts.length - 1) {
          cursor.field = field;
        }
      });
    });

    const countLeaves = (node) =>
      (node.field ? 1 : 0) +
      node.children.reduce((sum, child) => sum + countLeaves(child), 0);

    const stripMaps = (node) => ({
      key: node.key,
      name: node.name,
      path: node.path,
      field: node.field,
      count: countLeaves(node),
      children: node.children.map(stripMaps),
    });

    return root.children.map(stripMaps);
  }, [activeEndpoint]);

  const endpointGroups = useMemo(
    () => [
      {
        key: 'user',
        title: t('用户接口'),
        description: t('注册、登录、验证码、用户可用资源'),
        icon: <Users size={18} />,
        categories: [
          {
            key: 'user-auth',
            title: t('用户认证'),
            description: t('登录、注册和邮箱验证码'),
            endpointKeys: ['verification', 'register', 'login'],
          },
          {
            key: 'user-management',
            title: t('用户管理'),
            description: t('用户可用资源、订阅和模型列表'),
            endpointKeys: ['self-user', 'user-subscriptions', 'user-models'],
          },
          {
            key: 'subscription-market',
            title: t('订阅套餐'),
            description: t('平台可购买的订阅套餐列表'),
            endpointKeys: ['subscription-plans'],
          },
          {
            key: 'payment-billing',
            title: t('支付与账单'),
            description: t('充值记录和支付流水'),
            endpointKeys: ['user-topups'],
          },
          {
            key: 'api-token',
            title: t('API 令牌'),
            description: t('创建和管理模型调用 API Key'),
            endpointKeys: ['list-api-tokens', 'get-api-token-key', 'create-api-token'],
          },
        ],
      },
      {
        key: 'ai',
        title: t('AI 模型接口'),
        description: t('完全按官方文档侧边栏层级归类'),
        icon: <Bot size={18} />,
        categories: [
          {
            key: 'ai-audio',
            title: t('音频（Audio）'),
            description: t('语音识别和语音合成接口'),
            children: [
              {
                key: 'audio-gemini-format',
                title: t('原生Gemini格式'),
                description: t('Gemini 多模态音频格式'),
                endpointKeys: ['audio-gemini'],
              },
              {
                key: 'audio-openai-format',
                title: t('原生OpenAI格式'),
                description: t('Speech / Transcription / Translation'),
                endpointKeys: [
                  'audio-speech',
                  'audio-transcriptions',
                  'audio-translations',
                ],
              },
            ],
          },
          {
            key: 'ai-chat',
            title: t('聊天（Chat）'),
            description: t('对话补全接口'),
            children: [
              {
                key: 'chat-claude-format',
                title: t('原生Claude格式'),
                description: t('Messages API'),
                endpointKeys: ['claude-messages'],
              },
              {
                key: 'chat-gemini-format',
                title: t('原生Gemini格式'),
                description: t('generateContent / streamGenerateContent'),
                endpointKeys: [
                  'gemini-generate-content',
                  'gemini-stream-generate-content',
                ],
              },
              {
                key: 'chat-openai-format',
                title: t('原生OpenAI格式'),
                description: t('ChatCompletions / Responses'),
                endpointKeys: ['chat-completions', 'responses'],
              },
            ],
          },
          {
            key: 'ai-completions',
            title: t('补全（Completions）'),
            description: t('传统文本补全接口'),
            children: [
              {
                key: 'completions-openai-format',
                title: t('原生OpenAI格式'),
                description: t('Completions API'),
                endpointKeys: ['completions'],
              },
            ],
          },
          {
            key: 'ai-embeddings',
            title: t('嵌入（Embeddings）'),
            description: t('文本嵌入向量生成接口'),
            children: [
              {
                key: 'embeddings-openai-format',
                title: t('原生OpenAI格式'),
                description: t('Embeddings API'),
                endpointKeys: ['embeddings'],
              },
              {
                key: 'embeddings-gemini-format',
                title: t('原生Gemini格式'),
                description: t('Engine Embeddings 兼容路径'),
                endpointKeys: ['engine-embeddings'],
              },
            ],
          },
          {
            key: 'ai-images',
            title: t('图像（Images）'),
            description: t('AI 图像生成接口'),
            children: [
              {
                key: 'images-openai-format',
                title: t('原生OpenAI格式'),
                description: t('生成图像 / 编辑图像'),
                endpointKeys: ['image-generations', 'image-edits'],
              },
            ],
          },
          {
            key: 'ai-models',
            title: t('模型（Models）'),
            description: t('获取可用的模型列表'),
            children: [
              {
                key: 'models-openai-format',
                title: t('原生OpenAI格式'),
                description: t('OpenAI 模型列表'),
                endpointKeys: ['list-models'],
              },
              {
                key: 'models-gemini-format',
                title: t('原生Gemini格式'),
                description: t('Gemini 模型列表'),
                endpointKeys: ['gemini-list-models'],
              },
            ],
          },
          {
            key: 'ai-moderations',
            title: t('审查（Moderations）'),
            description: t('内容安全审核接口'),
            children: [
              {
                key: 'moderations-openai-format',
                title: t('原生OpenAI格式'),
                description: t('Moderations API'),
                endpointKeys: ['moderations'],
              },
            ],
          },
          {
            key: 'ai-realtime',
            title: t('实时语音（Realtime）'),
            description: t('实时音频流接口'),
            children: [
              {
                key: 'realtime-openai-format',
                title: t('原生OpenAI格式'),
                description: t('Realtime Session'),
                endpointKeys: ['realtime'],
              },
            ],
          },
          {
            key: 'ai-rerank',
            title: t('重排序（Rerank）'),
            description: t('文档重排序接口'),
            endpointKeys: ['rerank'],
          },
          {
            key: 'ai-unimplemented',
            title: t('未实现（Unimplemented）'),
            description: t('占位接口，暂未实现'),
            children: [
              {
                key: 'unimplemented-files',
                title: t('文件（Files）'),
                description: t('文件相关占位接口'),
                endpointKeys: [
                  'files-list',
                  'files-create',
                  'files-retrieve',
                  'files-delete',
                  'files-content',
                ],
              },
            ],
          },
          {
            key: 'ai-videos',
            title: t('视频（Videos）'),
            description: t('AI 视频生成接口'),
            children: [
              {
                key: 'videos-generation',
                title: t('视频生成任务'),
                description: t('创建和查询视频生成任务'),
                endpointKeys: ['video-generations', 'video-generation-result'],
              },
              {
                key: 'videos-sora',
                title: t('Sora'),
                description: t('创建视频、查询状态、获取内容'),
                endpointKeys: [
                  'openai-videos',
                  'openai-video-result',
                  'openai-video-content',
                ],
              },
            ],
          },
        ],
      },
    ],
    [t],
  );

  const toggleGroup = (groupKey) => {
    setExpandedGroups((groups) => ({
      ...groups,
      [groupKey]: !groups[groupKey],
    }));
  };

  const toggleCategory = (categoryKey) => {
    setExpandedCategories((categories) => ({
      ...categories,
      [categoryKey]: !categories[categoryKey],
    }));
  };

  const toggleDocSection = (sectionKey) => {
    setExpandedDocSections((sections) => ({
      ...sections,
      [sectionKey]: !sections[sectionKey],
    }));
  };

  const isDocSectionExpanded = (sectionKey, defaultOpen = false) =>
    expandedDocSections[sectionKey] ?? defaultOpen;

  const getCategoryEndpointKeys = (categories = []) =>
    categories.flatMap((category) => [
      ...(category.endpointKeys || []),
      ...getCategoryEndpointKeys(category.children || []),
    ]);

  const renderEndpointItem = (endpoint) => {
    const isActive = endpoint.key === activeEndpoint?.key;
    const compactSummary = (() => {
      const summary = String(endpoint.summary || '').trim();
      if (summary.length <= 24) {
        return summary;
      }
      const sentence = summary.split(/(?<=[。.!！?？])\s*/)[0]?.trim();
      if (sentence && sentence.length <= 28) {
        return sentence;
      }
      const phrase = summary.split(/[，,；;、]/)[0]?.trim();
      if (phrase && phrase.length >= 8 && phrase.length <= 28) {
        return phrase;
      }
      return `${summary.slice(0, 26)}...`;
    })();
    return (
      <button
        key={endpoint.key}
        type='button'
        className={`api-docs-sidebar__item ${isActive ? 'api-docs-sidebar__item--active' : ''}`}
        onClick={() => setActiveEndpointKey(endpoint.key)}
      >
        <span className='api-docs-sidebar__item-icon'>{endpoint.icon}</span>
        <span className='api-docs-sidebar__item-body'>
          <span className='api-docs-sidebar__item-row'>
            <span
              className={`api-docs-method api-docs-method--${endpoint.method.toLowerCase()}`}
            >
              {endpoint.method}
            </span>
            <code>{endpoint.path}</code>
          </span>
          <strong>{endpoint.title}</strong>
          <span title={endpoint.summary}>{compactSummary}</span>
        </span>
      </button>
    );
  };

  const renderCategory = (category, depth = 0) => {
    const categoryEndpointKeys = getCategoryEndpointKeys([category]);
    const categoryEndpoints = categoryEndpointKeys
      .map((key) => endpointCards.find((endpoint) => endpoint.key === key))
      .filter(Boolean);
    const isCategoryExpanded = expandedCategories[category.key] === true;
    const hasActiveCategoryEndpoint = categoryEndpoints.some(
      (endpoint) => endpoint.key === activeEndpoint?.key,
    );

    return (
      <div
        key={category.key}
        className={`api-docs-sidebar__category api-docs-sidebar__category--depth-${depth} ${hasActiveCategoryEndpoint ? 'api-docs-sidebar__category--active' : ''}`}
      >
        <button
          type='button'
          className='api-docs-sidebar__category-trigger'
          onClick={() => toggleCategory(category.key)}
          aria-expanded={isCategoryExpanded}
        >
          <span className='api-docs-sidebar__category-body'>
            <strong>{category.title}</strong>
            <span>{category.description}</span>
          </span>
          <span className='api-docs-sidebar__category-count'>
            {categoryEndpoints.length}
          </span>
          <ChevronDown
            size={15}
            className={`api-docs-sidebar__group-chevron ${isCategoryExpanded ? 'api-docs-sidebar__group-chevron--open' : ''}`}
          />
        </button>

        {isCategoryExpanded && (
          <div className='api-docs-sidebar__category-list'>
            {(category.children || []).map((child) =>
              renderCategory(child, depth + 1),
            )}
            {(category.endpointKeys || [])
              .map((key) =>
                endpointCards.find((endpoint) => endpoint.key === key),
              )
              .filter(Boolean)
              .map(renderEndpointItem)}
          </div>
        )}
      </div>
    );
  };

  const copyTextWithTextarea = (text) => {
    const textarea = document.createElement('textarea');
    textarea.value = text;
    textarea.setAttribute('readonly', '');
    textarea.style.position = 'fixed';
    textarea.style.top = '-9999px';
    textarea.style.left = '-9999px';
    textarea.style.opacity = '0';
    document.body.appendChild(textarea);
    textarea.focus();
    textarea.select();
    textarea.setSelectionRange(0, text.length);
    const copied = document.execCommand('copy');
    document.body.removeChild(textarea);
    return copied;
  };

  const copyText = async (value) => {
    const text = String(value ?? '');
    try {
      if (!text) {
        throw new Error('empty text');
      }
      try {
        if (!navigator.clipboard) {
          throw new Error('clipboard api unavailable');
        }
        await navigator.clipboard.writeText(text);
      } catch (clipboardError) {
        if (!copyTextWithTextarea(text)) {
          throw clipboardError;
        }
      }
      showSuccess(t('复制成功'));
    } catch (error) {
      showError(t('复制失败，请手动复制'));
    }
  };

  const renderParamItem = (param) => (
    <div key={param.name} className='api-docs-param-item'>
      <div className='api-docs-param-item__head'>
        <code>{param.name}</code>
        <span
          className={
            param.required ? 'api-docs-required' : 'api-docs-optional'
          }
        >
          {param.required ? t('必填') : t('可选')}
        </span>
      </div>
      <p>{param.description}</p>
    </div>
  );

  const renderFieldItem = (field) => (
    <div key={field.name} className='api-docs-param-item api-docs-field-item'>
      <div className='api-docs-param-item__head'>
        <code>{field.name}</code>
        <span className='api-docs-param-badge'>{field.type}</span>
      </div>
      <p>{field.description}</p>
    </div>
  );

  const renderFieldNode = (node, depth = 0) => {
    const hasChildren = node.children.length > 0;
    const sectionKey = `${activeEndpoint?.key || 'endpoint'}:field:${node.path}`;
    const expanded = isDocSectionExpanded(sectionKey, depth === 0);

    if (!hasChildren) {
      return (
        <div
          key={node.key}
          className='api-docs-field-tree__leaf'
          style={{ '--field-depth': depth }}
        >
          {renderFieldItem({
            ...node.field,
            name: node.name,
          })}
        </div>
      );
    }

    return (
      <div
        key={node.key}
        className='api-docs-field-tree__node'
        style={{ '--field-depth': depth }}
      >
        <button
          type='button'
          className='api-docs-field-tree__trigger'
          onClick={() => toggleDocSection(sectionKey)}
          aria-expanded={expanded}
        >
          <span className='api-docs-field-tree__name'>
            <code>{node.name}</code>
            {node.field?.type && (
              <span className='api-docs-param-badge'>{node.field.type}</span>
            )}
          </span>
          <span className='api-docs-accordion__meta'>
            {t('{{count}} 个字段', { count: node.count })}
          </span>
          <ChevronDown
            size={15}
            className={`api-docs-sidebar__group-chevron ${expanded ? 'api-docs-sidebar__group-chevron--open' : ''}`}
          />
        </button>
        {node.field?.description && (
          <p className='api-docs-field-tree__description'>
            {node.field.description}
          </p>
        )}
        {expanded && (
          <div className='api-docs-field-tree__children'>
            {node.children.map((child) => renderFieldNode(child, depth + 1))}
          </div>
        )}
      </div>
    );
  };

  const renderAccordion = ({
    key,
    title,
    count,
    children,
    defaultOpen = false,
    className = '',
  }) => {
    const sectionKey = `${activeEndpoint?.key || 'endpoint'}:${key}`;
    const expanded = isDocSectionExpanded(sectionKey, defaultOpen);
    return (
      <div className={`api-docs-accordion ${className}`}>
        <button
          type='button'
          className='api-docs-accordion__trigger'
          onClick={() => toggleDocSection(sectionKey)}
          aria-expanded={expanded}
        >
          <span>{title}</span>
          <span className='api-docs-accordion__meta'>{count}</span>
          <ChevronDown
            size={15}
            className={`api-docs-sidebar__group-chevron ${expanded ? 'api-docs-sidebar__group-chevron--open' : ''}`}
          />
        </button>
        {expanded && <div className='api-docs-accordion__body'>{children}</div>}
      </div>
    );
  };

  return (
    <div className='api-docs-shell px-3 md:px-6'>
      <section className='api-docs-layout'>
        <aside className='api-docs-sidebar'>
          <div className='api-docs-sidebar__head'>
            <div className='api-docs-sidebar__topbar'>
              <div>
                <span className='api-docs-sidebar__eyebrow'>{t('接口目录')}</span>
              </div>
            </div>
          </div>
          <div className='api-docs-sidebar__list'>
            {endpointGroups.map((group) => {
              const groupEndpointKeys = getCategoryEndpointKeys(group.categories);
              const groupEndpoints = groupEndpointKeys
                .map((key) => endpointCards.find((endpoint) => endpoint.key === key))
                .filter(Boolean);
              const isExpanded = expandedGroups[group.key];
              const hasActiveEndpoint = groupEndpoints.some(
                (endpoint) => endpoint.key === activeEndpoint?.key,
              );
              return (
                <div
                  key={group.key}
                  className={`api-docs-sidebar__group ${hasActiveEndpoint ? 'api-docs-sidebar__group--active' : ''}`}
                >
                  <button
                    type='button'
                    className='api-docs-sidebar__group-trigger'
                    onClick={() => toggleGroup(group.key)}
                    aria-expanded={isExpanded}
                  >
                    <span className='api-docs-sidebar__group-icon'>
                      {group.icon}
                    </span>
                    <span className='api-docs-sidebar__group-body'>
                      <strong>{group.title}</strong>
                      <span>{group.description}</span>
                    </span>
                    <span className='api-docs-sidebar__group-count'>
                      {groupEndpoints.length}
                    </span>
                    <ChevronDown
                      size={16}
                      className={`api-docs-sidebar__group-chevron ${isExpanded ? 'api-docs-sidebar__group-chevron--open' : ''}`}
                    />
                  </button>

                  {isExpanded && (
                    <div className='api-docs-sidebar__group-list'>
                      {group.categories.map((category) =>
                        renderCategory(category),
                      )}
                    </div>
                  )}
                </div>
              );
            })}
          </div>
        </aside>

        {activeEndpoint && (
          <article className='api-docs-card api-docs-card--detail'>
            <div className='api-docs-card__header'>
              <div className='api-docs-card__title-group'>
                <span className='api-docs-card__icon'>{activeEndpoint.icon}</span>
                <div>
                  <div className='api-docs-card__meta'>
                    <span
                      className={`api-docs-method api-docs-method--${activeEndpoint.method.toLowerCase()}`}
                      >
                      {activeEndpoint.method}
                    </span>
                    <code>{activeEndpoint.path}</code>
                  </div>
                  <h2 className='api-docs-card__title'>{activeEndpoint.title}</h2>
                </div>
              </div>
              <Button
                type='tertiary'
                icon={<Copy size={15} />}
                className='api-docs-inline-button'
                onClick={() => copyText(`${baseUrl}${activeEndpoint.path}`)}
              >
                {t('复制地址')}
              </Button>
            </div>

            <p className='api-docs-card__summary'>{activeEndpoint.summary}</p>

            <div className='api-docs-detail-grid'>
              <div className='api-docs-detail-main'>
                <div className='api-docs-section'>
                  <h3>{t('参数说明')}</h3>
                  <div className='api-docs-accordion-list'>
                    {parameterGroups.map((group, index) =>
                      renderAccordion({
                        key: `params:${group.key}`,
                        title: group.title,
                        count: t('{{count}} 个参数', { count: group.items.length }),
                        defaultOpen: index === 0,
                        children: (
                          <div className='api-docs-param-list'>
                            {group.items.map(renderParamItem)}
                          </div>
                        ),
                      }),
                    )}
                  </div>
                </div>

                {activeEndpoint.responseFields?.length > 0 && (
                  <div className='api-docs-section'>
                    <h3>{t('返回字段')}</h3>
                    <div className='api-docs-field-tree'>
                      {responseFieldTree.map((node) => renderFieldNode(node))}
                    </div>
                  </div>
                )}

                <div className='api-docs-section'>
                  <h3>{t('注意事项')}</h3>
                  <ul className='api-docs-note-list'>
                    {activeEndpoint.notes.map((note) => (
                      <li key={note}>{note}</li>
                    ))}
                  </ul>
                </div>
              </div>

              <aside className='api-docs-examples-rail'>
                <div className='api-docs-section api-docs-section--rail'>
                  <h3>{t('调用示例')}</h3>
                  <div className='api-docs-code-block'>
                    <button
                      type='button'
                      className='api-docs-code-copy'
                      aria-label={t('复制调用示例')}
                      title={t('复制调用示例')}
                      onClick={() => copyText(activeEndpoint.requestExample)}
                    >
                      <Copy size={14} />
                    </button>
                    <pre>{activeEndpoint.requestExample}</pre>
                  </div>
                </div>

                <div className='api-docs-section api-docs-section--rail'>
                  <h3>{t('返回示例')}</h3>
                  <div className='api-docs-code-block api-docs-code-block--response'>
                    <button
                      type='button'
                      className='api-docs-code-copy'
                      aria-label={t('复制返回示例')}
                      title={t('复制返回示例')}
                      onClick={() => copyText(activeEndpoint.responseExample)}
                    >
                      <Copy size={14} />
                    </button>
                    <pre>{activeEndpoint.responseExample}</pre>
                  </div>
                </div>
              </aside>
            </div>
          </article>
        )}
      </section>
    </div>
  );
};

export default ApiDocs;
