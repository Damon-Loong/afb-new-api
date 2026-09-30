import { useTranslation } from 'react-i18next';
import React, { useEffect, useMemo, useState } from 'react';
import {
  Button,
  Card,
  Form,
  Modal,
  Popconfirm,
  Space,
  Switch,
  Table,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import { ExternalLink, Plus, RefreshCw } from 'lucide-react';
import { API, showError, showSuccess } from '../../helpers';

const { Title, Text } = Typography;

const emptyProject = {
  key: '',
  name: '',
  official_url: '',
  description: '',
  icon_url: '',
  billing_secret: '',
  enabled: true,
  sso_enabled: true,
  sort: 0,
};

function getOrigin(value) {
  try {
    return new URL(value).origin;
  } catch {
    return '-';
  }
}

export default function ProjectAdmin() {
  const { t } = useTranslation();
  const [projects, setProjects] = useState([]);
  const [loading, setLoading] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const [editing, setEditing] = useState(null);
  const [form, setForm] = useState(emptyProject);
  const [saving, setSaving] = useState(false);

  const loadProjects = async () => {
    setLoading(true);
    try {
      const res = await API.get('/api/admin/projects');
      if (!res.data?.success) {
        throw new Error(res.data?.message || t('加载项目失败'));
      }
      setProjects(res.data.data || []);
    } catch (err) {
      showError(err.message || t('加载项目失败'));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadProjects();
  }, []);

  const openCreate = () => {
    setEditing(null);
    setForm(emptyProject);
    setModalOpen(true);
  };

  const openEdit = (project) => {
    setEditing(project);
    setForm({
      key: project.key || '',
      name: project.name || '',
      official_url: project.official_url || '',
      description: project.description || '',
      icon_url: project.icon_url || '',
      billing_secret: project.billing_secret || '',
      enabled: project.enabled !== false,
      sso_enabled: project.sso_enabled !== false,
      sort: project.sort || 0,
    });
    setModalOpen(true);
  };

  const saveProject = async () => {
    setSaving(true);
    try {
      const payload = {
        ...form,
        sort: Number(form.sort) || 0,
      };
      const res = editing
        ? await API.put(`/api/admin/projects/${editing.id}`, payload)
        : await API.post('/api/admin/projects', payload);
      if (!res.data?.success) {
        throw new Error(res.data?.message || t('保存项目失败'));
      }
      showSuccess(t('项目已保存'));
      setModalOpen(false);
      loadProjects();
    } catch (err) {
      showError(err.message || t('保存项目失败'));
    } finally {
      setSaving(false);
    }
  };

  const deleteProject = async (project) => {
    try {
      const res = await API.delete(`/api/admin/projects/${project.id}`);
      if (!res.data?.success) {
        throw new Error(res.data?.message || t('删除项目失败'));
      }
      showSuccess(t('项目已删除'));
      loadProjects();
    } catch (err) {
      showError(err.message || t('删除项目失败'));
    }
  };

  const columns = useMemo(
    () => [
      {
        title: t('项目'),
        dataIndex: 'name',
        render: (_, record) => (
          <Space vertical spacing={2} align='start'>
            <Text strong>{record.name}</Text>
            <Text type='tertiary' size='small'>
              {record.key}
            </Text>
          </Space>
        ),
      },
      {
        title: t('官网 URL'),
        dataIndex: 'official_url',
        render: (value) =>
          value ? (
            <Space vertical spacing={2} align='start'>
              <a href={value} target='_blank' rel='noreferrer'>
                <Space spacing={4}>
                  {value}
                  <ExternalLink size={14} />
                </Space>
              </a>
              <Text type='tertiary' size='small'>
                {t('白名单 origin：')}
                {getOrigin(value)}
              </Text>
            </Space>
          ) : (
            <Tag color='orange'>{t('未限制回跳地址')}</Tag>
          ),
      },
      {
        title: t('状态'),
        dataIndex: 'enabled',
        width: 160,
        render: (_, record) => (
          <Space>
            <Tag color={record.enabled ? 'green' : 'grey'}>
              {record.enabled ? t('已启用') : t('已禁用')}
            </Tag>
            <Tag color={record.sso_enabled ? 'blue' : 'grey'}>
              {record.sso_enabled ? 'SSO' : t('无 SSO')}
            </Tag>
          </Space>
        ),
      },
      {
        title: t('排序'),
        dataIndex: 'sort',
        width: 90,
      },
      {
        title: t('操作'),
        width: 180,
        render: (_, record) => (
          <Space>
            <Button size='small' onClick={() => openEdit(record)}>
              {t('编辑')}
            </Button>
            <Popconfirm
              title={t('确定删除这个项目？')}
              content={t('删除后该项目不能再通过 SSO 跳转。')}
              onConfirm={() => deleteProject(record)}
            >
              <Button size='small' type='danger' theme='borderless'>
                {t('删除')}
              </Button>
            </Popconfirm>
          </Space>
        ),
      },
    ],
    [t],
  );

  return (
    <div className='p-4'>
      <Card>
        <div className='flex items-center justify-between mb-4'>
          <div>
            <Title heading={4} style={{ margin: 0 }}>
              {t('项目管理')}
            </Title>
            <Text type='tertiary'>
              {t(
                '建议填写官网 URL 作为 SSO 回跳和跨域白名单；留空将接受任意 HTTP(S) 地址，仅建议临时测试使用。',
              )}
            </Text>
          </div>
          <Space>
            <Button icon={<RefreshCw size={16} />} onClick={loadProjects}>
              {t('刷新')}
            </Button>
            <Button
              theme='solid'
              icon={<Plus size={16} />}
              onClick={openCreate}
            >
              {t('新增项目')}
            </Button>
          </Space>
        </div>
        <Table
          rowKey='id'
          columns={columns}
          dataSource={projects}
          loading={loading}
          pagination={false}
        />
      </Card>

      <Modal
        title={editing ? t('编辑项目') : t('新增项目')}
        visible={modalOpen}
        onCancel={() => setModalOpen(false)}
        onOk={saveProject}
        confirmLoading={saving}
        width={640}
      >
        <Form labelPosition='top'>
          <Form.Input
            field='key'
            label={t('项目标识')}
            placeholder='ota'
            initValue={form.key}
            onChange={(value) => setForm((prev) => ({ ...prev, key: value }))}
          />
          <Form.Input
            field='name'
            label={t('项目名称')}
            placeholder={t('OTA 酒店价格工具')}
            initValue={form.name}
            onChange={(value) => setForm((prev) => ({ ...prev, name: value }))}
          />
          <Form.Input
            field='official_url'
            label={t('官网 URL')}
            placeholder={t('https://ota.xxx.com（留空则不限制地址）')}
            initValue={form.official_url}
            onChange={(value) =>
              setForm((prev) => ({ ...prev, official_url: value }))
            }
          />
          <Form.Input
            field='icon_url'
            label={t('图标 URL')}
            placeholder='https://.../icon.png'
            initValue={form.icon_url}
            onChange={(value) =>
              setForm((prev) => ({ ...prev, icon_url: value }))
            }
          />
          <Form.Input
            field='billing_secret'
            label={t('扣费密钥')}
            placeholder={t('项目后端调用扣费接口用的共享密钥')}
            initValue={form.billing_secret}
            onChange={(value) =>
              setForm((prev) => ({ ...prev, billing_secret: value }))
            }
          />
          <Form.TextArea
            field='description'
            label={t('简介')}
            placeholder={t('展示在项目入口的简短说明')}
            initValue={form.description}
            onChange={(value) =>
              setForm((prev) => ({ ...prev, description: value }))
            }
          />
          <Form.InputNumber
            field='sort'
            label={t('排序')}
            initValue={form.sort}
            onChange={(value) => setForm((prev) => ({ ...prev, sort: value }))}
          />
          <Space>
            <span>{t('启用项目')}</span>
            <Switch
              checked={form.enabled}
              onChange={(checked) =>
                setForm((prev) => ({ ...prev, enabled: checked }))
              }
            />
            <span>{t('允许 SSO')}</span>
            <Switch
              checked={form.sso_enabled}
              onChange={(checked) =>
                setForm((prev) => ({ ...prev, sso_enabled: checked }))
              }
            />
          </Space>
        </Form>
      </Modal>
    </div>
  );
}
