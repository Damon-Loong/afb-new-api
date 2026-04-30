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
    draft: '草稿',
    published: '已发布',
    ended: '已结束',
    archived: '已下架',
    pending: '待审核',
    approved: '已通过',
    rejected: '已驳回',
  };
  return map[status] || status;
}

function categoryText(category) {
  const map = {
    music: '音乐',
    video: '视频',
    text: '文档(文本)',
    document: '文档(文本)',
    image: '图片',
    mixed: '综合',
  };
  return map[category] || '作品';
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
    throw new Error(res.data?.message || '上传失败');
  }
  return res.data.data;
}

export default function MarketAdmin() {
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
      showError('活动标题不能为空');
      return;
    }
    setSaving(true);
    try {
      const res = editing
        ? await API.put(`/api/market/admin/activities/${editing.id}`, payload)
        : await API.post('/api/market/admin/activities', payload);
      if (res.data.success) {
        showSuccess('保存成功');
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
      const res = await API.patch(`/api/market/admin/activities/${record.id}/status`, {
        status,
      });
      if (res.data.success) {
        showSuccess('状态已更新');
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
        showSuccess('已删除');
        fetchActivities();
      } else {
        showError(res.data.message);
      }
    } catch (error) {
      showError(error);
    }
  };

  const updateSubmissionStatus = async (record, status) => {
    const rejectReason = status === 'rejected' ? window.prompt('请输入驳回原因', record.reject_reason || '') || '' : '';
    try {
      const res = await API.patch(`/api/market/admin/submissions/${record.id}/status`, {
        status,
        reject_reason: rejectReason,
      });
      if (res.data.success) {
        showSuccess('投稿状态已更新');
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
      const res = await API.patch(`/api/market/admin/submissions/${record.id}/feature`, {
        is_featured: !record.is_featured,
        sort_weight: record.sort_weight || 0,
      });
      if (res.data.success) {
        showSuccess(record.is_featured ? '已取消精选' : '已设为精选');
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
      const res = await API.delete(`/api/market/admin/submissions/${record.id}`);
      if (res.data.success) {
        showSuccess('投稿已删除');
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
        title: '活动',
        dataIndex: 'title',
        render: (text, record) => (
          <Space vertical align='start' spacing={2}>
            <Text strong>{text}</Text>
            <Text type='secondary' size='small'>
              {record.subtitle || '未填写副标题'}
            </Text>
          </Space>
        ),
      },
      {
        title: '状态',
        dataIndex: 'status',
        width: 110,
        render: (status) => <Tag>{statusText(status)}</Tag>,
      },
      {
        title: '类型',
        dataIndex: 'category',
        width: 120,
        render: (category) => <Tag>{categoryText(category)}</Tag>,
      },
      {
        title: '投稿',
        dataIndex: 'submission_count',
        width: 90,
      },
      {
        title: '排序',
        dataIndex: 'sort_weight',
        width: 90,
      },
      {
        title: '操作',
        width: 300,
        render: (_, record) => (
          <Space>
            <Button size='small' onClick={() => openEdit(record)}>
              编辑
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
              {record.status === 'published' ? '下架' : '发布'}
            </Button>
            <Button size='small' onClick={() => updateActivityStatus(record, 'ended')}>
              结束
            </Button>
            <Popconfirm title='确认删除该活动？投稿也会删除。' onConfirm={() => removeActivity(record)}>
              <Button size='small' type='danger'>
                删除
              </Button>
            </Popconfirm>
          </Space>
        ),
      },
    ],
    [],
  );

  const submissionColumns = useMemo(
    () => [
      {
        title: '作品',
        dataIndex: 'title',
        render: (text, record) => (
          <Space vertical align='start' spacing={2}>
            <Text strong>{text}</Text>
            <Text type='secondary' size='small'>
              {record.activity?.title || `活动 #${record.activity_id}`}
            </Text>
          </Space>
        ),
      },
      {
        title: '状态',
        dataIndex: 'status',
        width: 110,
        render: (status) => <Tag>{statusText(status)}</Tag>,
      },
      {
        title: '精选',
        dataIndex: 'is_featured',
        width: 90,
        render: (featured) => (featured ? <Tag color='amber'>精选</Tag> : '-'),
      },
      {
        title: '链接',
        dataIndex: 'work_url',
        render: (url) =>
          url ? (
            <a href={url} target='_blank' rel='noreferrer'>
              查看作品
            </a>
          ) : (
            '-'
          ),
      },
      {
        title: '提交时间',
        dataIndex: 'created_at',
        width: 170,
        render: (value) => (value ? new Date(value * 1000).toLocaleString() : '-'),
      },
      {
        title: '操作',
        width: 320,
        render: (_, record) => (
          <Space>
            <Button size='small' onClick={() => updateSubmissionStatus(record, 'approved')}>
              通过
            </Button>
            <Button size='small' type='warning' onClick={() => updateSubmissionStatus(record, 'rejected')}>
              驳回
            </Button>
            <Button size='small' onClick={() => updateSubmissionFeature(record)}>
              {record.is_featured ? '取消精选' : '设为精选'}
            </Button>
            <Popconfirm title='确认删除该投稿？' onConfirm={() => removeSubmission(record)}>
              <Button size='small' type='danger'>
                删除
              </Button>
            </Popconfirm>
          </Space>
        ),
      },
    ],
    [submissions],
  );

  return (
    <div className='px-2'>
      <Card style={{ borderRadius: 8 }}>
        <Tabs activeKey={activeKey} onChange={setActiveKey}>
          <Tabs.TabPane tab='活动管理' itemKey='activities'>
            <Space vertical align='start' spacing='loose' style={{ width: '100%' }}>
              <Space style={{ width: '100%', justifyContent: 'space-between' }}>
                <div>
                  <Title heading={3} style={{ margin: 0 }}>
                    需求市场
                  </Title>
                  <Text type='secondary'>发布活动征稿，用户完成创作后提交作品。</Text>
                </div>
                <Space>
                  <Button icon={<RefreshCw size={16} />} onClick={fetchActivities}>
                    刷新
                  </Button>
                  <Button theme='solid' icon={<Plus size={16} />} onClick={openCreate}>
                    新增活动
                  </Button>
                </Space>
              </Space>
              <Table rowKey='id' loading={activityLoading} columns={activityColumns} dataSource={activities} pagination={false} style={{ width: '100%' }} />
            </Space>
          </Tabs.TabPane>
          <Tabs.TabPane tab='投稿管理' itemKey='submissions'>
            <Space vertical align='start' spacing='loose' style={{ width: '100%' }}>
              <Space wrap>
                <Select
                  style={{ width: 220 }}
                  placeholder='按活动筛选'
                  value={submissionActivityId}
                  onChange={setSubmissionActivityId}
                  optionList={[
                    { label: '全部活动', value: '' },
                    ...activities.map((item) => ({ label: item.title, value: String(item.id) })),
                  ]}
                />
                <Select
                  style={{ width: 160 }}
                  placeholder='按状态筛选'
                  value={submissionStatus}
                  onChange={setSubmissionStatus}
                  optionList={[
                    { label: '全部状态', value: '' },
                    { label: '待审核', value: 'pending' },
                    { label: '已通过', value: 'approved' },
                    { label: '已驳回', value: 'rejected' },
                  ]}
                />
                <Button icon={<RefreshCw size={16} />} onClick={fetchSubmissions}>
                  刷新
                </Button>
              </Space>
              <Table rowKey='id' loading={submissionLoading} columns={submissionColumns} dataSource={submissions} pagination={false} style={{ width: '100%' }} />
            </Space>
          </Tabs.TabPane>
        </Tabs>
      </Card>
      <Modal
        title={editing ? '编辑活动' : '新增活动'}
        visible={visible}
        onCancel={() => setVisible(false)}
        onOk={save}
        confirmLoading={saving}
        width={820}
      >
        <Space vertical spacing='medium' style={{ width: '100%' }}>
          <FieldInput label='标题' value={form.title} onChange={(v) => updateField('title', v)} />
          <FieldInput label='副标题' value={form.subtitle} onChange={(v) => updateField('subtitle', v)} />
          <FieldInput
            label='奖池摘要'
            value={form.prize_summary}
            onChange={(v) => updateField('prize_summary', v)}
            placeholder='例如：本期活动设置 20万 奖金池'
          />
          <UploadUrlField label='封面' value={form.cover_url} onChange={(v) => updateField('cover_url', v)} usageType='activity_cover' />
          <FieldShell label='活动类型'>
            <Select style={{ width: '100%' }} value={form.category} onChange={(v) => updateField('category', v)}>
              <Select.Option value='image'>图片</Select.Option>
              <Select.Option value='video'>视频</Select.Option>
              <Select.Option value='music'>音乐</Select.Option>
              <Select.Option value='text'>文档(文本)</Select.Option>
              <Select.Option value='mixed'>综合</Select.Option>
            </Select>
          </FieldShell>
          <FieldShell label='状态'>
            <Select style={{ width: '100%' }} value={form.status} onChange={(v) => updateField('status', v)}>
              <Select.Option value='draft'>草稿</Select.Option>
              <Select.Option value='published'>已发布</Select.Option>
              <Select.Option value='ended'>已结束</Select.Option>
              <Select.Option value='archived'>已下架</Select.Option>
            </Select>
          </FieldShell>
          <FieldShell label='活动时间'>
            <Space wrap style={{ width: '100%' }}>
              <DatePicker type='dateTime' placeholder='开始时间' value={toDate(form.start_time)} onChange={(v) => updateField('start_time', fromDate(v))} />
              <DatePicker type='dateTime' placeholder='结束时间' value={toDate(form.end_time)} onChange={(v) => updateField('end_time', fromDate(v))} />
              <Input type='number' value={form.sort_weight} onChange={(v) => updateField('sort_weight', v)} placeholder='排序权重' style={{ width: 140 }} />
            </Space>
          </FieldShell>
          <FieldShell label='投稿时间'>
            <Space wrap style={{ width: '100%' }}>
              <DatePicker type='dateTime' placeholder='投稿开始' value={toDate(form.submission_start_time)} onChange={(v) => updateField('submission_start_time', fromDate(v))} />
              <DatePicker type='dateTime' placeholder='投稿结束' value={toDate(form.submission_end_time)} onChange={(v) => updateField('submission_end_time', fromDate(v))} />
            </Space>
          </FieldShell>
          <RichDetailField
            label='活动详情'
            value={form.detail_content}
            onChange={(v) => updateField('detail_content', v)}
          />
          <FieldTextArea
            label='政策标签'
            value={form.policy_lines}
            onChange={(v) => updateField('policy_lines', v)}
            placeholder={'一行一个：区域|政策名|说明\n海淀区|OPC补贴|政策展示用'}
          />
        </Space>
      </Modal>
    </div>
  );
}

function FieldShell({ label, children }) {
  return (
    <div style={{ display: 'flex', gap: 12, width: '100%' }}>
      <div style={{ width: 100, paddingTop: 8, color: 'var(--semi-color-text-1)' }}>{label}</div>
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
      <TextArea autosize={{ minRows: 3, maxRows: 8 }} value={value} onChange={onChange} placeholder={placeholder} />
    </FieldShell>
  );
}

function RichDetailField({ label, value, onChange }) {
  const [uploading, setUploading] = useState(false);
  const inputId = 'market-admin-detail-content-image';

  const handleFileChange = async (event) => {
    const file = event.target.files?.[0];
    if (!file) return;
    setUploading(true);
    try {
      const upload = await uploadMarketFile(file, 'activity_detail_image');
      const imageMarkdown = `\n\n![${upload.file_name || '活动详情图'}](${upload.file_url})\n\n`;
      onChange(`${value || ''}${imageMarkdown}`);
      showSuccess('图片已插入详情');
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
          placeholder={detailContentPlaceholder}
        />
        <Space wrap>
          <Button
            icon={<ImagePlus size={16} />}
            loading={uploading}
            onClick={() => document.getElementById(inputId)?.click()}
          >
            上传并插入图片
          </Button>
          <Text type='tertiary'>支持 Markdown，详情图也建议插入到正文中。</Text>
        </Space>
        <input id={inputId} type='file' accept='image/*' hidden onChange={handleFileChange} />
      </Space>
    </FieldShell>
  );
}

function UploadUrlField({ label, value, onChange, usageType }) {
  const [uploading, setUploading] = useState(false);
  const inputId = `market-admin-${usageType}`;

  const handleFileChange = async (event) => {
    const file = event.target.files?.[0];
    if (!file) return;
    setUploading(true);
    try {
      const upload = await uploadMarketFile(file, usageType);
      onChange(upload.file_url);
      showSuccess('上传成功');
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
          <Input value={value} onChange={onChange} placeholder={`${label} URL，或上传本地文件`} style={{ width: 420 }} />
          <Button loading={uploading} icon={<ImagePlus size={16} />} onClick={() => document.getElementById(inputId)?.click()}>
            上传
          </Button>
          {value ? <Button type='tertiary' onClick={() => onChange('')}>清除</Button> : null}
        </Space>
        <input id={inputId} type='file' hidden onChange={handleFileChange} />
        {value ? <img src={value} alt={label} style={{ width: 260, aspectRatio: '16 / 9', objectFit: 'cover', borderRadius: 8 }} /> : null}
      </Space>
    </FieldShell>
  );
}
