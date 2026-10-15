import React from 'react';
import { Banner, Button, InputNumber } from '@douyinfe/semi-ui';

export const SEEDANCE25_PRICING = {
  exchange_rate: 7,
  low_no_video: 70,
  low_with_video: 42,
  high_no_video: 77,
  high_with_video: 46,
  reserve_tokens: 300000,
};

export default function VideoPricingEditor({ value = {}, onChange, t }) {
  const fields = [
    ['low_no_video', '480p / 720p · 无视频输入'],
    ['low_with_video', '480p / 720p · 有视频输入'],
    ['high_no_video', '1080p · 无视频输入'],
    ['high_with_video', '1080p · 有视频输入'],
  ];
  return (
    <div>
      <Banner
        type='info'
        closeIcon={null}
        description={t(
          '适用于火山方舟 Seedance 2.5 标准价。按上游 completion_tokens 结算，时长和分辨率已反映在视频 Token 中，不再额外按秒计费。仅支持豆包视频任务接口。',
        )}
      />
      <div className='my-4'>
        <Button onClick={() => onChange({ ...SEEDANCE25_PRICING })}>
          {t('填入 Seedance 2.5 标准价模板')}
        </Button>
        <a
          className='ml-3'
          href='https://docs.volcengine.com/docs/ark/model-pricing?lang=zh'
          target='_blank'
          rel='noreferrer'
        >
          {t('查看官方计费文档')}
        </a>
      </div>
      {fields.map(([key, label]) => (
        <div key={key} className='mb-4'>
          <div className='mb-2 font-medium'>{t(label)}</div>
          <InputNumber
            value={value[key]}
            min={0.000001}
            max={1000000}
            suffix='¥ / 百万视频 Token'
            style={{ width: '100%' }}
            onChange={(v) => onChange({ ...value, [key]: v })}
          />
        </div>
      ))}
      <div className='mb-2 font-medium'>
        {t('计费汇率：1 USD 等于多少 CNY')}
      </div>
      <InputNumber
        value={value.exchange_rate}
        min={0.01}
        max={1000000}
        onChange={(v) => onChange({ ...value, exchange_rate: v })}
      />
      <p className='my-2 text-xs text-gray-500'>
        {t(
          '人民币单价只除以该汇率一次，转换为内部美元额度，再乘实际分组倍率。此处不会修改全站显示汇率，请保持一致。提交后的任务使用原价格与汇率快照。',
        )}
      </p>
      <div className='mb-2 mt-4 font-medium'>
        {t('提交时预扣视频 Token 数（不是最终用量）')}
      </div>
      <InputNumber
        value={value.reserve_tokens}
        min={1}
        max={100000000}
        precision={0}
        onChange={(v) => onChange({ ...value, reserve_tokens: v })}
      />
      <p className='my-2 text-xs text-gray-500'>
        {t(
          '无法从请求可靠获取参考视频长度和自动时长，因此使用固定预扣用量。成功后按实际用量多退少补；失败、取消退还。上游缺少用量或分辨率时保留待核对，不按零元结算。',
        )}
      </p>
      <p className='my-2 text-xs text-gray-500'>
        {t(
          'Draft 样片按 480p 独立计费；基于 Draft 的正式视频通过火山原生接口提交，并沿用样片任务是否含视频输入的分类。暂不支持非标准 service_tier。',
        )}
      </p>
    </div>
  );
}
