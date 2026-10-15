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
import React, { useEffect, useState } from 'react';
import {
  Banner,
  Button,
  Checkbox,
  DatePicker,
  Radio,
  RadioGroup,
  Select,
  SideSheet,
  Space,
  Typography,
} from '@douyinfe/semi-ui';
import { API, showError, semiSelectPortalProps } from '../../../../helpers';
import { DATE_RANGE_PRESETS } from '../../../../constants/console.constants';
import { useIsMobile } from '../../../../hooks/common/useIsMobile';

const { Text } = Typography;
const DIMENSIONS = [
  ['user', '按用户'],
  ['model', '按模型'],
  ['day', '按天'],
  ['token', '按令牌'],
  ['group', '按分组'],
  ['channel', '按渠道'],
];

const BillExportModal = ({
  visible,
  onClose,
  initialFilters,
  isAdminUser,
  t,
}) => {
  const isMobile = useIsMobile();
  const endpoint = isAdminUser ? '/api/log/export' : '/api/log/self/export';
  const storageKey = `bill-export-job-${isAdminUser ? 'admin' : 'self'}`;
  const [options, setOptions] = useState(null);
  const [job, setJob] = useState(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [pollingError, setPollingError] = useState('');
  const [pollAttempt, setPollAttempt] = useState(0);
  const [template, setTemplate] = useState('standard');
  const [filterChoices, setFilterChoices] = useState({ users: [], models: [] });
  const [choicesLoading, setChoicesLoading] = useState(false);
  const [choicesError, setChoicesError] = useState('');
  const [choicesAttempt, setChoicesAttempt] = useState(0);
  const running = job?.status === 'running';

  useEffect(() => {
    try {
      const id = sessionStorage.getItem(storageKey);
      if (id) setJob({ id, status: 'running' });
    } catch {
      // Export still works when browser storage is unavailable.
    }
  }, [storageKey]);

  useEffect(() => {
    if (!visible || !initialFilters || (running && options)) return;
    setOptions({
      start_timestamp: Math.floor(
        new Date(initialFilters.start_timestamp).getTime() / 1000,
      ),
      end_timestamp: Math.floor(
        new Date(initialFilters.end_timestamp).getTime() / 1000,
      ),
      user_ids: [],
      model_names: [],
      format: 'xlsx',
      timezone: Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC',
      dimensions: ['user', 'model', 'day'],
    });
    setTemplate('standard');
    setError('');
    // Only inherit the date range; hidden page filters must not narrow the bill.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [initialFilters]);

  useEffect(() => {
    if (!running || !job?.id) return;
    let active = true;
    let timer;
    const poll = async () => {
      try {
        const { data } = await API.get(`${endpoint}/${job.id}`, {
          skipErrorHandler: true,
        });
        if (!active) return;
        if (!data.success)
          throw new Error(data.message || t('账单任务查询失败'));
        setPollingError('');
        setJob(data.data);
        if (data.data.status === 'running') timer = setTimeout(poll, 2000);
      } catch (err) {
        if (!active) return;
        if (err.response?.status === 404) {
          setJob({
            ...job,
            status: 'failed',
            message: t('账单任务已过期或无权访问'),
          });
          try {
            sessionStorage.removeItem(storageKey);
          } catch {
            /* optional storage */
          }
        } else {
          setPollingError(t('账单状态查询失败，可重试查询已有任务'));
        }
      }
    };
    poll();
    return () => {
      active = false;
      clearTimeout(timer);
    };
  }, [job?.id, running, endpoint, storageKey, pollAttempt, t]);

  useEffect(() => {
    if (!visible || !options) return;
    const start = options.start_timestamp;
    const end = options.end_timestamp;
    if (
      !Number.isFinite(start) ||
      !Number.isFinite(end) ||
      start <= 0 ||
      end < start
    ) {
      setChoicesLoading(false);
      return;
    }
    let active = true;
    setChoicesLoading(true);
    setChoicesError('');
    const timer = setTimeout(async () => {
      try {
        const { data } = await API.get(`${endpoint}/options`, {
          params: { start_timestamp: start, end_timestamp: end },
          skipErrorHandler: true,
        });
        if (!active) return;
        if (!data.success)
          throw new Error(data.message || t('加载账单筛选项失败'));
        setFilterChoices(data.data);
      } catch (err) {
        if (active)
          setChoicesError(
            err.response?.data?.message ||
              err.message ||
              t('加载账单筛选项失败'),
          );
      } finally {
        if (active) setChoicesLoading(false);
      }
    }, 250);
    return () => {
      active = false;
      clearTimeout(timer);
    };
  }, [
    visible,
    options?.start_timestamp,
    options?.end_timestamp,
    endpoint,
    choicesAttempt,
    t,
  ]);

  const update = (key, value) =>
    setOptions((current) => ({ ...current, [key]: value }));
  const selectTemplate = (value) => {
    setTemplate(value);
    const dimensions = {
      standard: ['user', 'model', 'day'],
      user: ['user', 'model'],
      model: ['model', 'day'],
    }[value];
    if (dimensions) update('dimensions', dimensions);
  };

  const generate = async () => {
    if (!options) return;
    if (
      !Number.isFinite(options.start_timestamp) ||
      !Number.isFinite(options.end_timestamp) ||
      options.start_timestamp <= 0 ||
      options.end_timestamp < options.start_timestamp
    ) {
      setError(t('请选择有效的账单起止时间'));
      return;
    }
    if (options.end_timestamp - options.start_timestamp > 366 * 86400) {
      setError(t('单次账单周期不能超过 366 天'));
      return;
    }
    setBusy(true);
    setError('');
    try {
      const { data } = await API.post(endpoint, options, {
        skipErrorHandler: true,
      });
      if (!data.success) throw new Error(data.message || t('账单生成失败'));
      setJob(data.data);
      setPollingError('');
      try {
        sessionStorage.setItem(storageKey, data.data.id);
      } catch {
        /* optional storage */
      }
    } catch (err) {
      setError(err.response?.data?.message || err.message || t('账单生成失败'));
    } finally {
      setBusy(false);
    }
  };

  const download = async () => {
    setBusy(true);
    try {
      const response = await API.get(`${endpoint}/${job.id}/file`, {
        responseType: 'blob',
        skipErrorHandler: true,
      });
      const url = URL.createObjectURL(response.data);
      const link = document.createElement('a');
      link.href = url;
      link.download = job.filename;
      document.body.appendChild(link);
      link.click();
      link.remove();
      setTimeout(() => URL.revokeObjectURL(url), 1000);
    } catch {
      showError(t('账单下载失败，文件可能已过期，请重新生成'));
    } finally {
      setBusy(false);
    }
  };

  return (
    <SideSheet
      title={t('导出消费账单')}
      visible={visible}
      onCancel={onClose}
      width={Math.min(
        600,
        typeof window === 'undefined' ? 600 : window.innerWidth,
      )}
      footer={
        <div className='flex justify-end gap-2'>
          <Button onClick={onClose}>{t('关闭')}</Button>
          {job?.status === 'ready' && (
            <Button onClick={download} loading={busy}>
              {t('下载账单')}
            </Button>
          )}
          <Button
            theme='solid'
            onClick={generate}
            loading={running || busy}
            disabled={
              !options || running || busy || choicesLoading || !!choicesError
            }
          >
            {running ? t('账单生成中') : t('生成账单')}
          </Button>
        </div>
      }
    >
      <div className='flex flex-col gap-5'>
        <Banner
          type='info'
          description={t(
            '导出筛选条件下的全部消费和额度退回，不限当前页；不含充值、市场奖励或支付退款。',
          )}
        />
        {options && (
          <fieldset
            disabled={running || busy}
            className='flex flex-col gap-5 border-0 p-0 m-0 min-w-0'
          >
            <div className='flex flex-col gap-2'>
              <Text strong>{t('时间范围（浏览器本地时间）')}</Text>
              {isMobile ? (
                <div className='grid grid-cols-1 gap-2'>
                  {[
                    ['start_timestamp', '开始时间'],
                    ['end_timestamp', '结束时间'],
                  ].map(([key, label]) => (
                    <label key={key} className='flex flex-col gap-1'>
                      <Text type='tertiary'>{t(label)}</Text>
                      <DatePicker
                        type='dateTime'
                        className='w-full'
                        value={
                          options[key] ? new Date(options[key] * 1000) : null
                        }
                        onChange={(date) =>
                          update(
                            key,
                            date
                              ? Math.floor(new Date(date).getTime() / 1000)
                              : 0,
                          )
                        }
                      />
                    </label>
                  ))}
                </div>
              ) : (
                <DatePicker
                  type='dateTimeRange'
                  className='w-full'
                  value={[
                    new Date(options.start_timestamp * 1000),
                    new Date(options.end_timestamp * 1000),
                  ]}
                  onChange={(dates) =>
                    setOptions((current) => ({
                      ...current,
                      start_timestamp: dates?.[0]
                        ? Math.floor(new Date(dates[0]).getTime() / 1000)
                        : 0,
                      end_timestamp: dates?.[1]
                        ? Math.floor(new Date(dates[1]).getTime() / 1000)
                        : 0,
                    }))
                  }
                  presets={DATE_RANGE_PRESETS.map((preset) => ({
                    text: t(preset.text),
                    start: preset.start(),
                    end: preset.end(),
                  }))}
                />
              )}
            </div>
            <label className='flex flex-col gap-2'>
              <Text strong>{t('账单展示与日汇总时区')}</Text>
              <Select
                {...semiSelectPortalProps}
                value={options.timezone}
                onChange={(value) => update('timezone', value)}
                className='w-full'
              >
                {[
                  ...new Set([
                    Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC',
                    'Asia/Shanghai',
                    'UTC',
                  ]),
                ].map((zone) => (
                  <Select.Option key={zone} value={zone}>
                    {zone}
                  </Select.Option>
                ))}
              </Select>
            </label>
            <div className='grid grid-cols-1 gap-3'>
              {isAdminUser && (
                <label className='flex flex-col gap-2'>
                  <Text strong>{t('用户（可多选）')}</Text>
                  <Select
                    {...semiSelectPortalProps}
                    multiple
                    filter
                    showClear
                    maxTagCount={3}
                    loading={choicesLoading}
                    disabled={running || busy || choicesLoading}
                    className='w-full'
                    placeholder={t('不选择则包含全部用户')}
                    value={options.user_ids}
                    onChange={(values) => update('user_ids', values || [])}
                    optionList={filterChoices.users.map((user) => ({
                      value: user.user_id,
                      label: `${user.username || t('用户')} (#${user.user_id})`,
                    }))}
                  />
                </label>
              )}
              <label className='flex flex-col gap-2'>
                <Text strong>{t('模型名称（可多选）')}</Text>
                <Select
                  {...semiSelectPortalProps}
                  multiple
                  filter
                  showClear
                  maxTagCount={3}
                  loading={choicesLoading}
                  disabled={running || busy || choicesLoading}
                  className='w-full'
                  placeholder={t('不选择则包含全部模型')}
                  value={options.model_names}
                  onChange={(values) => update('model_names', values || [])}
                  optionList={filterChoices.models.map((model) => ({
                    value: model,
                    label: model,
                  }))}
                />
              </label>
              <Text type='tertiary'>
                {t('下拉选项来自所选时间范围内的消费和额度退回日志。')}
              </Text>
              {choicesError && (
                <Banner
                  type='danger'
                  description={
                    <Space>
                      {choicesError}
                      <Button
                        onClick={() => setChoicesAttempt((value) => value + 1)}
                      >
                        {t('重试')}
                      </Button>
                    </Space>
                  }
                />
              )}
            </div>
            <div className='flex flex-col gap-2'>
              <Text strong>{t('导出模板')}</Text>
              <RadioGroup
                value={template}
                onChange={(event) => selectTemplate(event.target.value)}
              >
                <Radio value='standard'>{t('标准对账包')}</Radio>
                <Radio value='user'>{t('用户对账')}</Radio>
                <Radio value='model'>{t('模型用量')}</Radio>
                <Radio value='custom'>{t('自定义汇总')}</Radio>
              </RadioGroup>
              <Checkbox.Group
                value={options.dimensions}
                onChange={(values) => {
                  setTemplate('custom');
                  update('dimensions', values);
                }}
              >
                <Space wrap>
                  {DIMENSIONS.filter(
                    ([key]) => isAdminUser || key !== 'channel',
                  ).map(([key, label]) => (
                    <Checkbox key={key} value={key}>
                      {t(label)}
                    </Checkbox>
                  ))}
                </Space>
              </Checkbox.Group>
              <Text type='tertiary'>
                {t('始终包含账单概览和逐笔明细，所选维度分别生成汇总表。')}
              </Text>
            </div>
            <div className='flex flex-col gap-2'>
              <Text strong>{t('文件格式')}</Text>
              <RadioGroup
                value={options.format}
                onChange={(event) => update('format', event.target.value)}
              >
                <Radio value='xlsx'>Excel (.xlsx)</Radio>
                <Radio value='csv'>{t('CSV 压缩包（每张表一个文件）')}</Radio>
              </RadioGroup>
            </div>
          </fieldset>
        )}
        <Text type='tertiary'>
          {t(
            '钱包、订阅及来源未记录额度分栏；USD 仅按导出时额度单位折算，不代表实付。单次最多 10 万条、366 天，超过上限请缩小范围。',
          )}
        </Text>
        <Text type='tertiary'>
          {t(
            '后台生成，可关闭面板后重新打开查看。文件保留 30 分钟，服务重启后需重新生成。',
          )}
        </Text>
        {error && <Banner type='danger' description={error} />}
        {running && (
          <Banner type='info' description={t('账单正在后台生成，请稍候。')} />
        )}
        {pollingError && (
          <Banner
            type='warning'
            description={
              <Space>
                {pollingError}
                <Button
                  onClick={() => {
                    setPollingError('');
                    setPollAttempt((value) => value + 1);
                  }}
                >
                  {t('重试')}
                </Button>
              </Space>
            }
          />
        )}
        {job?.status === 'failed' && (
          <Banner type='danger' description={job.message} />
        )}
        {job?.status === 'ready' && (
          <Banner
            type='success'
            description={t(
              '账单已生成，共 {{count}} 条记录，文件保留 30 分钟。',
              { count: job.rows },
            )}
          />
        )}
      </div>
    </SideSheet>
  );
};

export default BillExportModal;
