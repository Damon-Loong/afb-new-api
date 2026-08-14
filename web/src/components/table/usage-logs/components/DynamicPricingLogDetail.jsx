import React, { useMemo } from 'react';
import { Space, Tag, Typography } from '@douyinfe/semi-ui';

const { Text } = Typography;

const decodeExpr = (value) => {
  if (!value) return '';
  try {
    return decodeURIComponent(
      Array.prototype.map
        .call(
          atob(value),
          (char) => `%${char.charCodeAt(0).toString(16).padStart(2, '0')}`,
        )
        .join(''),
    );
  } catch {
    return '';
  }
};

export default function DynamicPricingLogDetail({ other, t }) {
  const expression = useMemo(
    () => decodeExpr(other?.expr_b64),
    [other?.expr_b64],
  );
  const rules = Array.isArray(other?.request_rules) ? other.request_rules : [];

  return (
    <Space vertical align='start' spacing='tight' style={{ maxWidth: 760 }}>
      <Space wrap>
        <Tag color='orange'>{t('动态计费')}</Tag>
        {other?.matched_tier ? (
          <Tag color='blue'>
            {t('命中档位')}：{other.matched_tier}
          </Tag>
        ) : null}
      </Space>
      {rules.map((rule, index) => (
        <Space key={`${rule.cond}-${index}`} wrap>
          <Tag color={rule.matched ? 'green' : 'grey'}>
            {rule.matched ? t('已命中') : t('未命中')}
          </Tag>
          <Text code>{rule.cond}</Text>
          <Text>× {rule.multiplier}</Text>
        </Space>
      ))}
      {expression ? (
        <div style={{ width: '100%' }}>
          <Text type='tertiary'>{t('计费表达式')}</Text>
          <pre
            style={{
              margin: '6px 0 0',
              padding: 10,
              whiteSpace: 'pre-wrap',
              overflowWrap: 'anywhere',
              background: 'var(--semi-color-fill-0)',
              border: '1px solid var(--semi-color-border)',
              borderRadius: 6,
              fontSize: 12,
            }}
          >
            {expression}
          </pre>
        </div>
      ) : null}
    </Space>
  );
}
