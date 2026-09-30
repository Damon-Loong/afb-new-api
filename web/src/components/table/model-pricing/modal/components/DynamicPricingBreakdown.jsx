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

import React, { useState, useEffect } from 'react';
import { Avatar, Tag, Table, Typography, Select } from '@douyinfe/semi-ui';
import { IconPriceTag } from '@douyinfe/semi-icons';
import { calculateModelPrice } from '../../../../../helpers';
import { BILLING_PRICING_VARS } from '../../../../../constants';
import {
  SOURCE_TIME,
  MATCH_RANGE,
  MATCH_EQ,
  MATCH_GTE,
  MATCH_LT,
  MATCH_CONTAINS,
  MATCH_EXISTS,
} from '../../../../../pages/Setting/Ratio/components/requestRuleExpr';

const { Text } = Typography;

const TIME_FUNC_LABELS = {
  hour: '小时',
  minute: '分钟',
  weekday: '星期',
  month: '月份',
  day: '日期',
};

function describeCondition(cond, t) {
  if (cond.source === SOURCE_TIME) {
    const fn = t(TIME_FUNC_LABELS[cond.timeFunc] || cond.timeFunc);
    const tz = cond.timezone || 'UTC';
    if (cond.mode === MATCH_RANGE) {
      return `${fn} ${cond.rangeStart}:00~${cond.rangeEnd}:00 (${tz})`;
    }
    const opMap = { [MATCH_EQ]: '=', [MATCH_GTE]: '≥', [MATCH_LT]: '<' };
    return `${fn} ${opMap[cond.mode] || '='} ${cond.value} (${tz})`;
  }
  const src = cond.source === 'header' ? t('请求头') : t('请求参数');
  const path = cond.path || '';
  if (cond.mode === MATCH_EXISTS) return `${src} ${path} ${t('存在')}`;
  if (cond.mode === MATCH_CONTAINS)
    return `${src} ${path} ${t('包含')} "${cond.value}"`;
  const opMap = { eq: '=', gt: '>', gte: '≥', lt: '<', lte: '≤' };
  return `${src} ${path} ${opMap[cond.mode] || '='} ${cond.value}`;
}

function describeGroup(group, t) {
  const parts = (group.conditions || []).map((c) => describeCondition(c, t));
  return parts.join(' && ');
}

export default function DynamicPricingBreakdown({
  billingExpr,
  modelData,
  groupRatio,
  selectedGroup,
  tokenUnit,
  displayPrice,
  t,
}) {
  const [detailGroup, setDetailGroup] = useState(selectedGroup || 'all');
  useEffect(() => {
    setDetailGroup(selectedGroup || 'all');
  }, [selectedGroup, modelData.model_name]);
  const priceData = calculateModelPrice({
    record: modelData,
    selectedGroup: detailGroup,
    groupRatio,
    tokenUnit,
    displayPrice,
  });
  const tiers = priceData.tiers;
  const ruleGroups = priceData.rules;
  const hasTiers = tiers && tiers.length > 0;
  const hasRules = ruleGroups && ruleGroups.length > 0;

  if (!hasTiers && !hasRules) {
    return (
      <div>
        <div className='flex items-center mb-3'>
          <Avatar size='small' color='amber' className='mr-2 shadow-md'>
            <IconPriceTag size={16} />
          </Avatar>
          <Text className='text-lg font-medium'>{t('动态计费')}</Text>
        </div>
        <div className='text-sm text-gray-500'>
          <code style={{ fontSize: 12, wordBreak: 'break-all' }}>
            {t('按规则计算')}：{billingExpr}
          </code>
        </div>
      </div>
    );
  }

  const priceFields = BILLING_PRICING_VARS.map((v) => [v.field, v.shortLabel]);

  const tierColumns = [
    {
      title: t('档位'),
      dataIndex: 'label',
      render: (text, record) => (
        <div>
          <Tag color='blue' size='small'>
            {text || t('默认')}
          </Tag>
          {record.condSummary && (
            <div className='text-xs text-gray-500 mt-1'>
              {record.condSummary}
            </div>
          )}
        </div>
      ),
    },
    ...priceFields
      .filter(
        ([field]) =>
          hasTiers && tiers.some((tier) => tier[field] !== undefined),
      )
      .map(([field, label]) => ({
        title: `${t(label)} / 1${priceData.unitLabel} tokens`,
        dataIndex: field,
        render: (v) =>
          v !== undefined ? <Text strong>{priceData.format(v)}</Text> : '-',
      })),
  ];

  const tierData = hasTiers
    ? tiers.map((tier, i) => ({
        key: `tier-${i}`,
        label: tier.label,
        condSummary: tier.conditions
          .join(' && ')
          .replace(/\blen\b/g, t('上下文长度'))
          .replace(/\bp\b/g, t('输入'))
          .replace(/\bc\b/g, t('输出')),
        ...Object.fromEntries(
          priceFields.map(([field]) => [field, tier[field] ?? 0]),
        ),
      }))
    : [];

  return (
    <div>
      <div className='flex items-center mb-4'>
        <Avatar size='small' color='amber' className='mr-2 shadow-md'>
          <IconPriceTag size={16} />
        </Avatar>
        <div>
          <Text className='text-lg font-medium'>{t('动态计费')}</Text>
          <div className='text-xs text-gray-600'>
            {t('价格根据用量档位和请求条件动态调整')}
          </div>
        </div>
      </div>

      <div className='flex items-center gap-2 mb-4'>
        <Text>{t('分组')}</Text>
        <Select
          value={detailGroup}
          onChange={setDetailGroup}
          style={{ minWidth: 180 }}
          optionList={[
            { label: t('全部（最低价格）'), value: 'all' },
            ...(modelData.enable_groups || [])
              .filter((g) => groupRatio[g] !== undefined)
              .map((g) => ({ label: g, value: g })),
          ]}
        />
        <Text>
          {priceData.usedGroup} · {t('分组倍率')} {priceData.usedGroupRatio}x
        </Text>
      </div>
      {hasTiers && (
        <div style={{ marginBottom: 16 }}>
          <Text
            strong
            className='text-sm'
            style={{ display: 'block', marginBottom: 8 }}
          >
            {t('分档价格表')}
          </Text>
          <Table
            dataSource={tierData}
            columns={tierColumns}
            pagination={false}
            size='small'
            bordered={false}
            className='!rounded-lg'
          />
        </div>
      )}

      {hasRules && (
        <div style={{ marginBottom: 16 }}>
          <Text
            strong
            className='text-sm'
            style={{ display: 'block', marginBottom: 8 }}
          >
            {t('条件乘数')}
          </Text>
          {ruleGroups.map((group, gi) => (
            <div
              key={`group-${gi}`}
              style={{
                display: 'flex',
                justifyContent: 'space-between',
                alignItems: 'center',
                padding: '8px 12px',
                borderRadius: 6,
                background: 'var(--semi-color-fill-0)',
                marginBottom: 4,
              }}
            >
              <Text size='small'>{describeGroup(group, t)}</Text>
              <Tag color='orange' size='small'>
                {group.multiplier}x
              </Tag>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
