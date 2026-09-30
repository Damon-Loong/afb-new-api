import { useTranslation } from 'react-i18next';
import i18n from '../../i18n/i18n';
import React, { useEffect, useMemo, useState } from 'react';
import {
  Button,
  Card,
  DatePicker,
  Input,
  Modal,
  Popconfirm,
  Select,
  Space,
  Tabs,
  Table,
  Tag,
  TextArea,
  Typography,
} from '@douyinfe/semi-ui';
import { ImagePlus, Plus, RefreshCw } from 'lucide-react';
import { API, showError, showSuccess } from '../../helpers';

const { Title, Text } = Typography;

const blankActivity = {
  title: '',
  subtitle: '',
  prize_summary: '',
  detail_content: '',
  cover_url: '',
  status: 'draft',
  category: 'image',
  sort_weight: 0,
  start_time: 0,
  end_time: 0,
  submission_start_time: 0,
  submission_end_time: 0,
  policy_lines: '',
};

const detailContentPlaceholder = `支持 Markdown，把活动详情图、活动介绍、投稿要求、奖励说明、评选规则放在同一份正文里。

## 活动介绍
写清楚活动主题、创作方向和参考风格。

## 投稿要求
- 作品需为原创
- 提交作品链接或附件
- 写明尺寸、格式、时长等要求

## 奖励说明
一等奖：...
二等奖：...
优秀作品：...

## 评选规则
1. 初筛：完整度与合规性
2. 复评：创意、完成度、传播潜力
3. 结果公布：...`;

function toDate(value) {
  return value ? new Date(value * 1000) : null;
}

function fromDate(value) {
  return value ? Math.floor(new Date(value).getTime() / 1000) : 0;
}

function statusText(status) {
  const map = {
    draft: i18n.t('草稿'),
    published: i18n.t('已发布'),
    ended: i18n.t('已结束'),
    archived: i18n.t('已下架'),
    pending: i18n.t('待审核'),
    approved: i18n.t('已通过'),
    rejected: i18n.t('已驳回'),
  };
  return map[status] || status;
}

function categoryText(category) {
  const map = {
    music: i18n.t('音乐'),
    video: i18n.t('视频'),
    text: i18n.t('文档(文本)'),
    document: i18n.t('文档(文本)'),
    image: i18n.t('图片'),
    mixed: i18n.t('综合'),
  };
  return map[category] || i18n.t('作品');
}

function parsePolicies(lines) {
  return String(lines || '')
    .split('\n')
    .map((line) => line.trim())
    .filter(Boolean)
    .map((line, index) => {
      const [regionName, policyName, description] = line.split('|');
      return {
        region_name: regionName || '',
        policy_name: policyName || regionName || '',
        description: description || '',
        sort_order: index,
      };
    });
}

function activityToForm(item) {
  return {
    ...blankActivity,
    ...item,
    policy_lines: (item.policies || [])
      .map((policy) =>
        [policy.region_name, policy.policy_name, policy.description]
          .filter((value) => value !== undefined && value !== null)
          .join('|'),
      )
      .join('\n'),
  };
}

async function uploadMarketFile(file, usageType) {
  const formData = new FormData();
  formData.append('file', file);
  formData.append('usage_type', usageType);
  const res = await API.post('/api/market/uploads', formData);
  if (!res.data?.success) {
    throw new Error(res.data?.message || i18n.t('上传失败'));
  }
  return res.data.data;
}

export default function MarketAdmin() {
  const { t } = useTranslation();
  const [activeKey, setActiveKey] = useState('activities');
  const [activityLoading, setActivityLoading] = useState(false);
  const [submissionLoading, setSubmissionLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [activities, setActivities] = useState([]);
  const [submissions, setSubmissions] = useState([]);
  const [visible, setVisible] = useState(false);
  const [editing, setEditing] = useState(null);
  const [form, setForm] = useState(blankActivity);
  const [submissionStatus, setSubmissionStatus] = useState('');
  const [submissionActivityId, setSubmissionActivityId] = useState('');

  const fetchActivities = async () => {
    setActivityLoading(true);
    try {
      const res = await API.get('/api/market/admin/activities', {
        params: { page_size: 100 },
      });
      if (res.data.success) {
        setActivities(res.data.data.items || []);
      } else {
        showError(res.data.message);
      }
    } catch (error) {
      showError(error);
    } finally {
      setActivityLoading(false);
    }
  };

  const fetchSubmissions = async () => {
    setSubmissionLoading(true);
    try {
      const res = await API.get('/api/market/admin/submissions', {
        params: {
          page_size: 100,
          status: submissionStatus || undefined,
          activity_id: submissionActivityId || undefined,
        },
      });
      if (res.data.success) {
        setSubmissions(res.data.data.items || []);
      } else {
        showError(res.data.message);
      }
    } catch (error) {
      showError(error);
    } finally {
      setSubmissionLoading(false);
    }
  };

  useEffect(() => {
    fetchActivities();
  }, []);

  useEffect(() => {
    if (activeKey === 'submissions') {
      fetchSubmissions();
    }
  }, [activeKey, submissionStatus, submissionActivityId]);

  const openCreate = () => {
    setEditing(null);
    setForm(blankActivity);
    setVisible(true);
  };

  const openEdit = (record) => {
    setEditing(record);
    setForm(activityToForm(record));
    setVisible(true);
  };

  const updateField = (key, value) => {
    setForm((prev) => ({ ...prev, [key]: value }));
  };

  const save = async () => {
    const payload = {
      title: form.title,
      subtitle: form.subtitle,
      prize_summary: form.prize_summary,
      detail_content: form.detail_content,
      cover_url: form.cover_url,
      status: form.status,
      category: form.category,
      sort_weight: Number(form.sort_weight || 0),
      start_time: Number(form.start_time || 0),
      end_time: Number(form.end_time || 0),
      submission_start_time: Number(form.submission_start_time || 0),
      submission_end_time: Number(form.submission_end_time || 0),
      policies: parsePolicies(form.policy_lines),
    };
    if (!payload.title.trim()) {
      showError(t('活动标题不能为空'));
      return;
    }
    setSaving(true);
    try {
      const res = editing
        ? await API.put(`/api/market/admin/activities/${editing.id}`, payload)
        : await API.post('/api/market/admin/activities', payload);
      if (res.data.success) {
        showSuccess(t('保存成功'));
        setVisible(false);
        fetchActivities();
      } else {
        showError(res.data.message);
      }
    } catch (error) {
      showError(error);
    } finally {
      setSaving(false);
    }
  };

  const updateActivityStatus = async (record, status) => {
    try {
      const res = await API.patch(
        `/api/market/admin/activities/${record.id}/status`,
        {
          status,
        },
      );
      if (res.data.success) {
        showSuccess(t('状态已更新'));
        fetchActivities();
      } else {
        showError(res.data.message);
      }
    } catch (error) {
      showError(error);
    }
  };

  const removeActivity = async (record) => {
    try {
      const res = await API.delete(`/api/market/admin/activities/${record.id}`);
      if (res.data.success) {
        showSuccess(t('已删除'));
        fetchActivities();
      } else {
        showError(res.data.message);
      }
    } catch (error) {
      showError(error);
    }
  };

  const updateSubmissionStatus = async (record, status) => {
    const rejectReason =
      status === 'rejected'
        ? window.prompt(t('请输入驳回原因'), record.reject_reason || '') || ''
        : '';
    try {
      const res = await API.patch(
        `/api/market/admin/submissions/${record.id}/status`,
        {
          status,
          reject_reason: rejectReason,
        },
      );
      if (res.data.success) {
        showSuccess(t('投稿状态已更新'));
        fetchSubmissions();
      } else {
        showError(res.data.message);
      }
    } catch (error) {
      showError(error);
    }
  };

  const updateSubmissionFeature = async (record) => {
    try {
      const res = await API.patch(
        `/api/market/admin/submissions/${record.id}/feature`,
        {
          is_featured: !record.is_featured,
          sort_weight: record.sort_weight || 0,
        },
      );
      if (res.data.success) {
        showSuccess(record.is_featured ? t('已取消精选') : t('已设为精选'));
        fetchSubmissions();
      } else {
        showError(res.data.message);
      }
    } catch (error) {
      showError(error);
    }
  };

  const removeSubmission = async (record) => {
    try {
      const res = await API.delete(
        `/api/market/admin/submissions/${record.id}`,
      );
      if (res.data.success) {
        showSuccess(t('投稿已删除'));
        fetchSubmissions();
      } else {
        showError(res.data.message);
      }
    } catch (error) {
      showError(error);
    }
  };

  const activityColumns = useMemo(
    () => [
      {
        title: t('活动'),
        dataIndex: 'title',
        render: (text, record) => (
          <Space vertical align='start' spacing={2}>
            <Text strong>{text}</Text>
            <Text type='secondary' size='small'>
              {record.subtitle || t('未填写副标题')}
            </Text>
          </Space>
        ),
      },
      {
        title: t('状态'),
        dataIndex: 'status',
        width: 110,
        render: (status) => <Tag>{statusText(status)}</Tag>,
      },
      {
        title: t('类型'),
        dataIndex: 'category',
        width: 120,
        render: (category) => <Tag>{categoryText(category)}</Tag>,
      },
      {
        title: t('投稿'),
        dataIndex: 'submission_count',
        width: 90,
      },
      {
        title: t('排序'),
        dataIndex: 'sort_weight',
        width: 90,
      },
      {
        title: t('操作'),
        width: 300,
        render: (_, record) => (
          <Space>
            <Button size='small' onClick={() => openEdit(record)}>
              {t('编辑')}
            </Button>
            <Button
              size='small'
              onClick={() =>
                updateActivityStatus(
                  record,
                  record.status === 'published' ? 'archived' : 'published',
                )
              }
            >
              {record.status === 'published' ? t('下架') : t('发布')}
            </Button>
            <Button
              size='small'
              onClick={() => updateActivityStatus(record, 'ended')}
            >
              {t('结束')}
            </Button>
            <Popconfirm
              title={t('确认删除该活动？投稿也会删除。')}
              onConfirm={() => removeActivity(record)}
            >
              <Button size='small' type='danger'>
                {t('删除')}
              </Button>
            </Popconfirm>
          </Space>
        ),
      },
    ],
    [t],
  );

  const submissionColumns = useMemo(
    () => [
      {
        title: t('作品'),
        dataIndex: 'title',
        render: (text, record) => (
          <Space vertical align='start' spacing={2}>
            <Text strong>{text}</Text>
            <Text type='secondary' size='small'>
              {record.activity?.title ||
                t('活动 #{{id}}', { id: record.activity_id })}
            </Text>
          </Space>
        ),
      },
      {
        title: t('状态'),
        dataIndex: 'status',
        width: 110,
        render: (status) => <Tag>{statusText(status)}</Tag>,
      },
      {
        title: t('精选'),
        dataIndex: 'is_featured',
        width: 90,
        render: (featured) =>
          featured ? <Tag color='amber'>{t('精选')}</Tag> : '-',
      },
      {
        title: t('链接'),
        dataIndex: 'work_url',
        render: (url) =>
          url ? (
            <a href={url} target='_blank' rel='noreferrer'>
              {t('查看作品')}
            </a>
          ) : (
            '-'
          ),
      },
      {
        title: t('提交时间'),
        dataIndex: 'created_at',
        width: 170,
        render: (value) =>
          value ? new Date(value * 1000).toLocaleString() : '-',
      },
      {
        title: t('操作'),
        width: 320,
        render: (_, record) => (
          <Space>
            <Button
              size='small'
              onClick={() => updateSubmissionStatus(record, 'approved')}
            >
              {t('通过')}
            </Button>
            <Button
              size='small'
              type='warning'
              onClick={() => updateSubmissionStatus(record, 'rejected')}
            >
              {t('驳回')}
            </Button>
            <Button
              size='small'
              onClick={() => updateSubmissionFeature(record)}
            >
              {record.is_featured ? t('取消精选') : t('设为精选')}
            </Button>
            <Popconfirm
              title={t('确认删除该投稿？')}
              onConfirm={() => removeSubmission(record)}
            >
              <Button size='small' type='danger'>
                {t('删除')}
              </Button>
            </Popconfirm>
          </Space>
        ),
      },
    ],
    [submissions, t],
  );

  return (
    <div className='px-2'>
      <Card style={{ borderRadius: 8 }}>
        <Tabs activeKey={activeKey} onChange={setActiveKey} size='medium'>
          <Tabs.TabPane tab={t('活动管理')} itemKey='activities'>
            <Space
              vertical
              align='start'
              spacing='loose'
              style={{ width: '100%' }}
            >
              <Space style={{ width: '100%', justifyContent: 'space-between' }}>
                <div>
                  <Title heading={3} style={{ margin: 0 }}>
                    {t('需求市场')}
                  </Title>
                  <Text type='secondary'>
                    {t('发布活动征稿，用户完成创作后提交作品。')}
                  </Text>
                </div>
                <Space>
                  <Button
                    icon={<RefreshCw size={16} />}
                    onClick={fetchActivities}
                  >
                    {t('刷新')}
                  </Button>
                  <Button
                    theme='solid'
                    icon={<Plus size={16} />}
                    onClick={openCreate}
                  >
                    {t('新增活动')}
                  </Button>
                </Space>
              </Space>
              <Table
                rowKey='id'
                loading={activityLoading}
                columns={activityColumns}
                dataSource={activities}
                pagination={false}
                style={{ width: '100%' }}
              />
            </Space>
          </Tabs.TabPane>
          <Tabs.TabPane tab={t('投稿管理')} itemKey='submissions'>
            <Space
              vertical
              align='start'
              spacing='loose'
              style={{ width: '100%' }}
            >
              <Space wrap>
                <Select
                  style={{ width: 220 }}
                  placeholder={t('按活动筛选')}
                  value={submissionActivityId}
                  onChange={setSubmissionActivityId}
                  optionList={[
                    { label: t('全部活动'), value: '' },
                    ...activities.map((item) => ({
                      label: item.title,
                      value: String(item.id),
                    })),
                  ]}
                />
                <Select
                  style={{ width: 160 }}
                  placeholder={t('按状态筛选')}
                  value={submissionStatus}
                  onChange={setSubmissionStatus}
                  optionList={[
                    { label: t('全部状态'), value: '' },
                    { label: t('待审核'), value: 'pending' },
                    { label: t('已通过'), value: 'approved' },
                    { label: t('已驳回'), value: 'rejected' },
                  ]}
                />
                <Button
                  icon={<RefreshCw size={16} />}
                  onClick={fetchSubmissions}
                >
                  {t('刷新')}
                </Button>
              </Space>
              <Table
                rowKey='id'
                loading={submissionLoading}
                columns={submissionColumns}
                dataSource={submissions}
                pagination={false}
                style={{ width: '100%' }}
              />
            </Space>
          </Tabs.TabPane>
        </Tabs>
      </Card>
      <Modal
        title={editing ? t('编辑活动') : t('新增活动')}
        visible={visible}
        onCancel={() => setVisible(false)}
        onOk={save}
        confirmLoading={saving}
        width={820}
      >
        <Space vertical spacing='medium' style={{ width: '100%' }}>
          <FieldInput
            label={t('标题')}
            value={form.title}
            onChange={(v) => updateField('title', v)}
          />
          <FieldInput
            label={t('副标题')}
            value={form.subtitle}
            onChange={(v) => updateField('subtitle', v)}
          />
          <FieldInput
            label={t('奖池摘要')}
            value={form.prize_summary}
            onChange={(v) => updateField('prize_summary', v)}
            placeholder={t('例如：本期活动设置 20万 奖金池')}
          />
          <UploadUrlField
            label={t('封面')}
            value={form.cover_url}
            onChange={(v) => updateField('cover_url', v)}
            usageType='activity_cover'
          />
          <FieldShell label={t('活动类型')}>
            <Select
              style={{ width: '100%' }}
              value={form.category}
              onChange={(v) => updateField('category', v)}
            >
              <Select.Option value='image'>{t('图片')}</Select.Option>
              <Select.Option value='video'>{t('视频')}</Select.Option>
              <Select.Option value='music'>{t('音乐')}</Select.Option>
              <Select.Option value='text'>{t('文档(文本)')}</Select.Option>
              <Select.Option value='mixed'>{t('综合')}</Select.Option>
            </Select>
          </FieldShell>
          <FieldShell label={t('状态')}>
            <Select
              style={{ width: '100%' }}
              value={form.status}
              onChange={(v) => updateField('status', v)}
            >
              <Select.Option value='draft'>{t('草稿')}</Select.Option>
              <Select.Option value='published'>{t('已发布')}</Select.Option>
              <Select.Option value='ended'>{t('已结束')}</Select.Option>
              <Select.Option value='archived'>{t('已下架')}</Select.Option>
            </Select>
          </FieldShell>
          <FieldShell label={t('活动时间')}>
            <Space wrap style={{ width: '100%' }}>
              <DatePicker
                type='dateTime'
                placeholder={t('开始时间')}
                value={toDate(form.start_time)}
                onChange={(v) => updateField('start_time', fromDate(v))}
              />
              <DatePicker
                type='dateTime'
                placeholder={t('结束时间')}
                value={toDate(form.end_time)}
                onChange={(v) => updateField('end_time', fromDate(v))}
              />
              <Input
                type='number'
                value={form.sort_weight}
                onChange={(v) => updateField('sort_weight', v)}
                placeholder={t('排序权重')}
                style={{ width: 140 }}
              />
            </Space>
          </FieldShell>
          <FieldShell label={t('投稿时间')}>
            <Space wrap style={{ width: '100%' }}>
              <DatePicker
                type='dateTime'
                placeholder={t('投稿开始')}
                value={toDate(form.submission_start_time)}
                onChange={(v) =>
                  updateField('submission_start_time', fromDate(v))
                }
              />
              <DatePicker
                type='dateTime'
                placeholder={t('投稿结束')}
                value={toDate(form.submission_end_time)}
                onChange={(v) =>
                  updateField('submission_end_time', fromDate(v))
                }
              />
            </Space>
          </FieldShell>
          <RichDetailField
            label={t('活动详情')}
            value={form.detail_content}
            onChange={(v) => updateField('detail_content', v)}
          />
          <FieldTextArea
            label={t('政策标签')}
            value={form.policy_lines}
            onChange={(v) => updateField('policy_lines', v)}
            placeholder={t(
              '一行一个：区域|政策名|说明\n海淀区|OPC补贴|政策展示用',
            )}
          />
        </Space>
      </Modal>
    </div>
  );
}

function FieldShell({ label, children }) {
  return (
    <div style={{ display: 'flex', gap: 12, width: '100%' }}>
      <div
        style={{ width: 100, paddingTop: 8, color: 'var(--semi-color-text-1)' }}
      >
        {label}
      </div>
      <div style={{ flex: 1 }}>{children}</div>
    </div>
  );
}

function FieldInput({ label, value, onChange, placeholder }) {
  return (
    <FieldShell label={label}>
      <Input value={value} onChange={onChange} placeholder={placeholder} />
    </FieldShell>
  );
}

function FieldTextArea({ label, value, onChange, placeholder }) {
  return (
    <FieldShell label={label}>
      <TextArea
        autosize={{ minRows: 3, maxRows: 8 }}
        value={value}
        onChange={onChange}
        placeholder={placeholder}
      />
    </FieldShell>
  );
}

function RichDetailField({ label, value, onChange }) {
  const { t } = useTranslation();
  const [uploading, setUploading] = useState(false);
  const inputId = 'market-admin-detail-content-image';

  const handleFileChange = async (event) => {
    const file = event.target.files?.[0];
    if (!file) return;
    setUploading(true);
    try {
      const upload = await uploadMarketFile(file, 'activity_detail_image');
      const imageMarkdown = `\n\n![${upload.file_name || t('活动详情图')}](${upload.file_url})\n\n`;
      onChange(`${value || ''}${imageMarkdown}`);
      showSuccess(t('图片已插入详情'));
    } catch (error) {
      showError(error);
    } finally {
      setUploading(false);
      event.target.value = '';
    }
  };

  return (
    <FieldShell label={label}>
      <Space vertical align='start' spacing='medium' style={{ width: '100%' }}>
        <TextArea
          autosize={{ minRows: 12, maxRows: 24 }}
          value={value}
          onChange={onChange}
          placeholder={t(detailContentPlaceholder)}
        />
        <Space wrap>
          <Button
            icon={<ImagePlus size={16} />}
            loading={uploading}
            onClick={() => document.getElementById(inputId)?.click()}
          >
            {t('上传并插入图片')}
          </Button>
          <Text type='tertiary'>
            {t('支持 Markdown，详情图也建议插入到正文中。')}
          </Text>
        </Space>
        <input
          id={inputId}
          type='file'
          accept='image/*'
          hidden
          onChange={handleFileChange}
        />
      </Space>
    </FieldShell>
  );
}

function UploadUrlField({ label, value, onChange, usageType }) {
  const { t } = useTranslation();
  const [uploading, setUploading] = useState(false);
  const inputId = `market-admin-${usageType}`;

  const handleFileChange = async (event) => {
    const file = event.target.files?.[0];
    if (!file) return;
    setUploading(true);
    try {
      const upload = await uploadMarketFile(file, usageType);
      onChange(upload.file_url);
      showSuccess(t('上传成功'));
    } catch (error) {
      showError(error);
    } finally {
      setUploading(false);
      event.target.value = '';
    }
  };

  return (
    <FieldShell label={label}>
      <Space vertical align='start' spacing='medium' style={{ width: '100%' }}>
        <Space wrap>
          <Input
            value={value}
            onChange={onChange}
            placeholder={t('{{label}} URL，或上传本地文件', { label })}
            style={{ width: 420 }}
          />
          <Button
            loading={uploading}
            icon={<ImagePlus size={16} />}
            onClick={() => document.getElementById(inputId)?.click()}
          >
            {t('上传')}
          </Button>
          {value ? (
            <Button type='tertiary' onClick={() => onChange('')}>
              {t('清除')}
            </Button>
          ) : null}
        </Space>
        <input id={inputId} type='file' hidden onChange={handleFileChange} />
        {value ? (
          <img
            src={value}
            alt={label}
            style={{
              width: 260,
              aspectRatio: '16 / 9',
              objectFit: 'cover',
              borderRadius: 8,
            }}
          />
        ) : null}
      </Space>
    </FieldShell>
  );
}
