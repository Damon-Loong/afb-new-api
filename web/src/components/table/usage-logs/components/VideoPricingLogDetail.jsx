import React from 'react';
import { renderQuota } from '../../../../helpers';

export default function VideoPricingLogDetail({ other, t }) {
  const s = other.video_pricing_snapshot;
  if (!s) return null;
  const settled = other.actual_quota !== undefined;
  const resolution = other.resolution || s.resolution;
  const key = `${resolution === '1080p' ? 'high' : 'low'}_${s.has_video_input ? 'with' : 'no'}_video`;
  return (
    <div className='space-y-1'>
      <div>
        {settled ? t('已结算') : t('提交预扣，非最终费用')} · {resolution} ·{' '}
        {s.has_video_input ? t('有视频输入') : t('无视频输入')}
      </div>
      <div>
        {t('单价')}：¥{s.pricing[key]} / 1M video tokens；{t('计费汇率')}：1 USD
        = {s.pricing.exchange_rate} CNY；{t('分组倍率')}：{s.group_ratio}
      </div>
      <div>
        {t('用量')}：
        {settled ? other.completion_tokens : s.pricing.reserve_tokens} video
        tokens
        {settled
          ? `；${t('视频时长')}：${other.duration_seconds ?? '-'} s`
          : ''}
      </div>
      <div>
        {t('计算规则')}：video tokens ÷ 1,000,000 × {t('人民币档位价')} ÷{' '}
        {t('汇率')} × {t('分组倍率')} × {t('内部额度单位')}
      </div>
      {settled && (
        <div>
          {t('预扣')}：{renderQuota(other.pre_consumed_quota, 6)}；
          {t('最终总费用')}：{renderQuota(other.actual_quota, 6)}。
          {t('本条日志金额仅为补扣或退款差额。')}
        </div>
      )}
    </div>
  );
}
