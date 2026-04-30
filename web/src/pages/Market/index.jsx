import React, { useEffect, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import {
  Button,
  Card,
  Empty,
  Input,
  Modal,
  Space,
  Spin,
  Tag,
  TextArea,
  Typography,
} from '@douyinfe/semi-ui';
import { ArrowLeft, CalendarDays, FileUp, Gift, ImagePlus, Star } from 'lucide-react';
import { API, showError, showSuccess } from '../../helpers';
import MarkdownRenderer from '../../components/common/markdown/MarkdownRenderer';
import './style.css';

const { Title, Text, Paragraph } = Typography;

const pageStyle = {
  maxWidth: 1520,
  margin: '0 auto',
  padding: '24px 32px 64px',
};

function formatDate(value) {
  if (!value) return '长期有效';
  const date = new Date(value * 1000);
  const pad = (n) => String(n).padStart(2, '0');
  return `${date.getFullYear()}/${date.getMonth() + 1}/${date.getDate()} ${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

function activityStatusText(status) {
  const map = {
    draft: '草稿',
    published: '进行中',
    ended: '已结束',
    archived: '已下架',
  };
  return map[status] || status;
}

function displayActivityStatus(activity) {
  if (activity?.source_name === 'jimeng') {
    const map = {
      in_progress: '进行中',
      in_evaluation: '评奖中',
      awarded: '已开奖',
    };
    return map[activity.source_status] || activityStatusText(activity.status);
  }
  return activityStatusText(activity?.status);
}

function categoryText(category) {
  const map = {
    music: '音乐',
    video: '视频',
    text: '文档',
    document: '文档',
    image: '图片',
    mixed: '综合',
  };
  return map[category] || '作品';
}

function sourceText(sourceName) {
  const map = {
    jimeng: '即梦',
    official: '官方',
  };
  return map[sourceName] || '官方';
}

function canSubmitActivity(activity) {
  if (!activity) return false;
  if (typeof activity.can_submit === 'boolean') {
    return activity.can_submit;
  }
  if (activity.status && activity.status !== 'published') {
    return false;
  }
  if (activity.source_name && activity.source_name !== 'official') {
    return activity.source_status === 'in_progress';
  }
  return true;
}

function getSubmitUrl(activity) {
  if (!activity) return '';
  if (activity.submission_mode === 'external') {
    return activity.submit_url || activity.external_url || '';
  }
  if (activity.submission_mode === 'none') {
    return '';
  }
  return activity.submit_url || '';
}

function fileMeta(upload) {
  return {
    name: upload.file_name,
    size: upload.file_size,
    type: upload.file_type,
    url: upload.file_url,
    storage_key: upload.storage_key,
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

function ActivityCard({ item, onPreviewCover }) {
  const navigate = useNavigate();
  const submitActivity = () => {
    const submitUrl = getSubmitUrl(item);
    if (submitUrl) {
      window.open(submitUrl, '_blank', 'noopener,noreferrer');
      return;
    }
    navigate(`/market/${item.id}/submit`);
  };
  return (
    <div className='market-activity-row'>
      <button
        type='button'
        className='market-activity-cover'
        onClick={() => item.cover_url && onPreviewCover(item)}
      >
        {item.cover_url ? (
          <img src={item.cover_url} alt={item.title} />
        ) : (
          <span>活动封面</span>
        )}
      </button>
      <div className='market-activity-main'>
        <Space wrap>
          <Tag color='green'>{displayActivityStatus(item)}</Tag>
          <Tag>{categoryText(item.category)}</Tag>
          <Tag color={item.source_name ? 'light-blue' : 'violet'}>
            {sourceText(item.source_name)}
          </Tag>
          {item.prize_summary ? (
            <Tag color='amber' prefixIcon={<Gift size={14} />}>
              {item.prize_summary}
            </Tag>
          ) : null}
        </Space>
        <Title heading={3} className='market-activity-title'>
          {item.title}
        </Title>
        {item.subtitle ? (
          <Paragraph type='secondary' ellipsis={{ rows: 1 }} className='market-activity-subtitle'>
            {item.subtitle}
          </Paragraph>
        ) : null}
        <Space wrap className='market-activity-tags'>
          <Tag prefixIcon={<CalendarDays size={14} />}>
            {formatDate(item.start_time)} - {formatDate(item.end_time)}
          </Tag>
          <Tag>{item.submission_count || 0} 件投稿</Tag>
          {(item.policies || []).slice(0, 3).map((policy) => (
            <Tag key={policy.id || `${policy.region_name}-${policy.policy_name}`}>
              {policy.region_name} {policy.policy_name}
            </Tag>
          ))}
        </Space>
      </div>
      <div className='market-activity-actions'>
        <Button theme='solid' onClick={() => navigate(`/market/${item.id}`)}>
          查看活动
        </Button>
        {canSubmitActivity(item) ? (
          <Button type='primary' onClick={submitActivity}>
            立即投稿
          </Button>
        ) : null}
      </div>
    </div>
  );
}

export default function Market() {
  const [loading, setLoading] = useState(true);
  const [activities, setActivities] = useState([]);
  const [previewActivity, setPreviewActivity] = useState(null);

  useEffect(() => {
    let mounted = true;
    const loadActivities = async () => {
      try {
        const pageSize = 100;
        let page = 1;
        let total = 0;
        const list = [];
        do {
          const res = await API.get('/api/market/activities', {
            params: { page, page_size: pageSize },
          });
          if (!res.data.success) {
            showError(res.data.message);
            return;
          }
          const data = res.data.data || {};
          const items = data.items || [];
          total = data.total || items.length;
          list.push(...items);
          page += 1;
          if (items.length === 0) break;
        } while (list.length < total);
        if (mounted) {
          setActivities(list);
        }
      } catch (error) {
        showError(error);
      } finally {
        if (mounted) {
          setLoading(false);
        }
      }
    };
    loadActivities();
    return () => {
      mounted = false;
    };
  }, []);

  return (
    <div className='market-page' style={pageStyle}>
      <Spin spinning={loading} style={{ width: '100%' }}>
        {activities.length === 0 && !loading ? (
          <Empty description='暂无进行中的活动' />
        ) : (
          <Space vertical spacing='medium' className='market-activity-list'>
            {activities.map((item) => (
              <ActivityCard key={item.id} item={item} onPreviewCover={setPreviewActivity} />
            ))}
          </Space>
        )}
      </Spin>
      <Modal
        visible={Boolean(previewActivity)}
        title={previewActivity?.title || '活动封面'}
        footer={null}
        width='min(1120px, 92vw)'
        onCancel={() => setPreviewActivity(null)}
        className='market-cover-preview-modal'
      >
        {previewActivity?.cover_url ? (
          <img
            src={previewActivity.cover_url}
            alt={previewActivity.title}
            className='market-cover-preview-image'
          />
        ) : null}
      </Modal>
    </div>
  );
}

export function MarketDetail() {
  const { id } = useParams();
  const navigate = useNavigate();
  const [loading, setLoading] = useState(true);
  const [activity, setActivity] = useState(null);
  const [works, setWorks] = useState([]);

  useEffect(() => {
    setLoading(true);
    Promise.all([
      API.get(`/api/market/activities/${id}`),
      API.get(`/api/market/activities/${id}/works`, { params: { page_size: 12 } }),
    ])
      .then(([activityRes, worksRes]) => {
        if (activityRes.data.success) {
          setActivity(activityRes.data.data);
        } else {
          showError(activityRes.data.message);
        }
        if (worksRes.data.success) {
          setWorks(worksRes.data.data.items || []);
        }
      })
      .catch(showError)
      .finally(() => setLoading(false));
  }, [id]);

  const submitActivity = () => {
    if (!activity) return;
    const submitUrl = getSubmitUrl(activity);
    if (submitUrl) {
      window.open(submitUrl, '_blank', 'noopener,noreferrer');
      return;
    }
    navigate(`/market/${activity.id}/submit`);
  };

  return (
    <div className='market-detail-page' style={pageStyle}>
      <Spin spinning={loading}>
        {!activity ? (
          <Empty description='活动不存在或未发布' />
        ) : !canSubmitActivity(activity) ? (
          <Empty description='当前活动暂不可投稿' />
        ) : getSubmitUrl(activity) ? (
          <Empty
            description='该活动通过外部页面投稿'
          >
            <Button theme='solid' onClick={() => window.open(getSubmitUrl(activity), '_blank', 'noopener,noreferrer')}>
              前往投稿
            </Button>
          </Empty>
        ) : (
          <Space vertical align='start' spacing='loose' style={{ width: '100%' }}>
            <Button icon={<ArrowLeft size={16} />} onClick={() => navigate('/market')}>
              返回活动广场
            </Button>
            <div className='market-detail-header'>
              <div className='market-detail-heading'>
                <Title heading={1}>{activity.title}</Title>
                {activity.subtitle ? <Paragraph>{activity.subtitle}</Paragraph> : null}
                <Space wrap className='market-detail-meta'>
                  <Tag color='green'>{displayActivityStatus(activity)}</Tag>
                  <Tag>{categoryText(activity.category)}</Tag>
                  <Tag color={activity.source_name ? 'light-blue' : 'violet'}>
                    {sourceText(activity.source_name)}
                  </Tag>
                  <Tag prefixIcon={<CalendarDays size={14} />}>
                    {formatDate(activity.start_time)} - {formatDate(activity.end_time)}
                  </Tag>
                  <Tag>{activity.submission_count || 0} 件投稿</Tag>
                  {activity.prize_summary ? (
                    <Tag color='amber' prefixIcon={<Gift size={14} />}>
                      {activity.prize_summary}
                    </Tag>
                  ) : null}
                </Space>
              </div>
              <div className='market-detail-actions'>
                {canSubmitActivity(activity) ? (
                  <Button theme='solid' size='large' onClick={submitActivity}>
                    立即投稿
                  </Button>
                ) : null}
              </div>
            </div>
            <MarketSection title='活动详情' content={getActivityDetailContent(activity)} />
            {(activity.policies || []).length > 0 ? (
              <Card style={{ width: '100%', borderRadius: 8 }}>
                <Title heading={4}>政策支持</Title>
                <Space wrap>
                  {activity.policies.map((policy) => (
                    <Tag key={policy.id || policy.policy_name}>
                      {policy.region_name} {policy.policy_name}
                    </Tag>
                  ))}
                </Space>
              </Card>
            ) : null}
            <Card style={{ width: '100%', borderRadius: 8 }}>
              <Title heading={4}>作品展示</Title>
              {works.length === 0 ? (
                <Empty description='暂无通过审核的作品' />
              ) : (
                <div className='market-work-grid'>
                  {works.map((work) => (
                    <div className='market-work-card' key={work.id}>
                      {work.cover_url ? <img src={work.cover_url} alt={work.title} /> : null}
                      <Space wrap>
                        {work.is_featured ? <Tag color='amber' prefixIcon={<Star size={14} />}>精选</Tag> : null}
                        <Tag>{activityStatusText(work.status)}</Tag>
                      </Space>
                      <Text strong>{work.title}</Text>
                      {work.description ? <Text type='secondary'>{work.description}</Text> : null}
                    </div>
                  ))}
                </div>
              )}
            </Card>
          </Space>
        )}
      </Spin>
    </div>
  );
}

function MarketSection({ title, content }) {
  return (
    <section className='market-detail-content'>
      <Title heading={4} className='market-detail-content-title'>
        {title}
      </Title>
      <MarkdownRenderer content={content} className='market-rich-content' />
    </section>
  );
}

function getActivityDetailContent(activity) {
  return activity.detail_content?.trim() || '管理员暂未填写活动详情。';
}

export function MarketSubmit() {
  const { id } = useParams();
  const navigate = useNavigate();
  const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const [uploading, setUploading] = useState(false);
  const [activity, setActivity] = useState(null);
  const [form, setForm] = useState({
    title: '',
    description: '',
    work_url: '',
    cover_url: '',
  });
  const [attachments, setAttachments] = useState([]);
  const [submissionId, setSubmissionId] = useState('');

  useEffect(() => {
    API.get(`/api/market/activities/${id}`)
      .then((res) => {
        if (res.data.success) {
          setActivity(res.data.data);
        } else {
          showError(res.data.message);
        }
      })
      .catch(showError)
      .finally(() => setLoading(false));
  }, [id]);

  const updateField = (key, value) => {
    setForm((prev) => ({ ...prev, [key]: value }));
  };

  const onCoverChange = async (event) => {
    const file = event.target.files?.[0];
    if (!file) return;
    if (!file.type.startsWith('image/')) {
      showError('请选择图片文件');
      event.target.value = '';
      return;
    }
    setUploading(true);
    try {
      const upload = await uploadMarketFile(file, 'submission_cover');
      updateField('cover_url', upload.file_url);
      showSuccess('作品封面已上传');
    } catch (error) {
      showError(error);
    } finally {
      setUploading(false);
      event.target.value = '';
    }
  };

  const onAttachmentChange = async (event) => {
    const files = Array.from(event.target.files || []);
    if (files.length === 0) return;
    if (attachments.length + files.length > 5) {
      showError('最多上传 5 个附件');
      event.target.value = '';
      return;
    }
    setUploading(true);
    try {
      const uploads = await Promise.all(files.map((file) => uploadMarketFile(file, 'submission_attachment')));
      setAttachments((prev) => [...prev, ...uploads.map(fileMeta)].slice(0, 5));
      showSuccess('附件已上传');
    } catch (error) {
      showError(error);
    } finally {
      setUploading(false);
      event.target.value = '';
    }
  };

  const submit = async () => {
    if (!form.title.trim()) {
      showError('请填写作品标题');
      return;
    }
    if (!form.work_url.trim() && attachments.length === 0) {
      showError('请填写作品链接或上传作品附件');
      return;
    }
    setSubmitting(true);
    setSubmissionId('');
    try {
      const res = await API.post(`/api/market/activities/${id}/submissions`, {
        ...form,
        attachments,
      });
      if (!res.data?.success) {
        showError(res.data?.message || '提交失败');
        return;
      }
      setSubmissionId(res.data.data.id);
      showSuccess('作品已提交，等待审核');
    } catch (error) {
      showError(error);
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div style={pageStyle}>
      <Spin spinning={loading}>
        {!activity ? (
          <Empty description='活动不存在或未发布' />
        ) : (
          <Space vertical align='start' spacing='loose' style={{ width: '100%' }}>
            <Button icon={<ArrowLeft size={16} />} onClick={() => navigate(`/market/${id}`)}>
              返回活动详情
            </Button>
            <Card style={{ width: '100%', borderRadius: 8 }}>
              <Space vertical align='start' spacing='loose' style={{ width: '100%' }}>
                <div>
                  <Title heading={3}>{activity.title}</Title>
                  <Text type='secondary'>提交已经创作完成的作品，管理员审核通过后会进入作品展示。</Text>
                </div>
                <Input placeholder='作品标题' value={form.title} onChange={(v) => updateField('title', v)} />
                <Input placeholder='作品链接，可填写公开视频、图文或作品页面地址' value={form.work_url} onChange={(v) => updateField('work_url', v)} />
                <TextArea
                  autosize={{ minRows: 5, maxRows: 10 }}
                  placeholder='作品说明、创作思路、授权说明或补充信息'
                  value={form.description}
                  onChange={(v) => updateField('description', v)}
                />
                <Space wrap>
                  <Button icon={<ImagePlus size={16} />} loading={uploading} onClick={() => document.getElementById('market-submit-cover')?.click()}>
                    上传作品封面
                  </Button>
                  <Button icon={<FileUp size={16} />} loading={uploading} disabled={attachments.length >= 5} onClick={() => document.getElementById('market-submit-files')?.click()}>
                    上传作品附件
                  </Button>
                  <Text type='tertiary'>附件最多 5 个，单个不超过 100MB。</Text>
                </Space>
                <input id='market-submit-cover' type='file' accept='image/*' hidden onChange={onCoverChange} />
                <input id='market-submit-files' type='file' multiple hidden onChange={onAttachmentChange} />
                {form.cover_url ? <img className='market-submit-cover-preview' src={form.cover_url} alt='作品封面' /> : null}
                {attachments.length > 0 ? (
                  <Space wrap>
                    {attachments.map((item) => (
                      <Tag key={`${item.storage_key}-${item.name}`} closable onClose={() => setAttachments((prev) => prev.filter((file) => file !== item))}>
                        {item.name}
                      </Tag>
                    ))}
                  </Space>
                ) : null}
                <Space>
                  <Button theme='solid' loading={submitting} onClick={submit}>
                    提交作品
                  </Button>
                  <Button onClick={() => navigate('/market')}>返回活动广场</Button>
                </Space>
                {submissionId ? <Tag color='green' size='large'>投稿已提交：{submissionId}</Tag> : null}
              </Space>
            </Card>
          </Space>
        )}
      </Spin>
    </div>
  );
}
