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
        throw new Error(res.data?.message || '加载项目失败');
      }
      setProjects(res.data.data || []);
    } catch (err) {
      showError(err.message || '加载项目失败');
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
        throw new Error(res.data?.message || '保存项目失败');
      }
      showSuccess('项目已保存');
      setModalOpen(false);
      loadProjects();
    } catch (err) {
      showError(err.message || '保存项目失败');
    } finally {
      setSaving(false);
    }
  };

  const deleteProject = async (project) => {
    try {
      const res = await API.delete(`/api/admin/projects/${project.id}`);
      if (!res.data?.success) {
        throw new Error(res.data?.message || '删除项目失败');
      }
      showSuccess('项目已删除');
      loadProjects();
    } catch (err) {
      showError(err.message || '删除项目失败');
    }
  };

  const columns = useMemo(
    () => [
      {
        title: '项目',
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
        title: '官网 URL',
        dataIndex: 'official_url',
        render: (value) => (
          <Space vertical spacing={2} align='start'>
            <a href={value} target='_blank' rel='noreferrer'>
              <Space spacing={4}>
                {value}
                <ExternalLink size={14} />
              </Space>
            </a>
            <Text type='tertiary' size='small'>
              白名单 origin：{getOrigin(value)}
            </Text>
          </Space>
        ),
      },
      {
        title: '状态',
        dataIndex: 'enabled',
        width: 160,
        render: (_, record) => (
          <Space>
            <Tag color={record.enabled ? 'green' : 'grey'}>
              {record.enabled ? '已启用' : '已禁用'}
            </Tag>
            <Tag color={record.sso_enabled ? 'blue' : 'grey'}>
              {record.sso_enabled ? 'SSO' : '无 SSO'}
            </Tag>
          </Space>
        ),
      },
      {
        title: '排序',
        dataIndex: 'sort',
        width: 90,
      },
      {
        title: '操作',
        width: 180,
        render: (_, record) => (
          <Space>
            <Button size='small' onClick={() => openEdit(record)}>
              编辑
            </Button>
            <Popconfirm
              title='确定删除这个项目？'
              content='删除后该项目不能再通过 SSO 跳转。'
              onConfirm={() => deleteProject(record)}
            >
              <Button size='small' type='danger' theme='borderless'>
                删除
              </Button>
            </Popconfirm>
          </Space>
        ),
      },
    ],
    [],
  );

  return (
    <div className='p-4'>
      <Card>
        <div className='flex items-center justify-between mb-4'>
          <div>
            <Title heading={4} style={{ margin: 0 }}>
              项目管理
            </Title>
            <Text type='tertiary'>
              配置项目名称和官网 URL，官网 origin 会自动作为 SSO 白名单。
            </Text>
          </div>
          <Space>
            <Button icon={<RefreshCw size={16} />} onClick={loadProjects}>
              刷新
            </Button>
            <Button theme='solid' icon={<Plus size={16} />} onClick={openCreate}>
              新增项目
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
        title={editing ? '编辑项目' : '新增项目'}
        visible={modalOpen}
        onCancel={() => setModalOpen(false)}
        onOk={saveProject}
        confirmLoading={saving}
        width={640}
      >
        <Form labelPosition='top'>
          <Form.Input
            field='key'
            label='项目标识'
            placeholder='ota'
            initValue={form.key}
            onChange={(value) => setForm((prev) => ({ ...prev, key: value }))}
          />
          <Form.Input
            field='name'
            label='项目名称'
            placeholder='OTA 酒店价格工具'
            initValue={form.name}
            onChange={(value) => setForm((prev) => ({ ...prev, name: value }))}
          />
          <Form.Input
            field='official_url'
            label='官网 URL'
            placeholder='https://ota.xxx.com'
            initValue={form.official_url}
            onChange={(value) => setForm((prev) => ({ ...prev, official_url: value }))}
          />
          <Form.Input
            field='icon_url'
            label='图标 URL'
            placeholder='https://.../icon.png'
            initValue={form.icon_url}
            onChange={(value) => setForm((prev) => ({ ...prev, icon_url: value }))}
          />
          <Form.Input
            field='billing_secret'
            label='扣费密钥'
            placeholder='项目后端调用扣费接口用的共享密钥'
            initValue={form.billing_secret}
            onChange={(value) => setForm((prev) => ({ ...prev, billing_secret: value }))}
          />
          <Form.TextArea
            field='description'
            label='简介'
            placeholder='展示在项目入口的简短说明'
            initValue={form.description}
            onChange={(value) => setForm((prev) => ({ ...prev, description: value }))}
          />
          <Form.InputNumber
            field='sort'
            label='排序'
            initValue={form.sort}
            onChange={(value) => setForm((prev) => ({ ...prev, sort: value }))}
          />
          <Space>
            <span>启用项目</span>
            <Switch
              checked={form.enabled}
              onChange={(checked) => setForm((prev) => ({ ...prev, enabled: checked }))}
            />
            <span>允许 SSO</span>
            <Switch
              checked={form.sso_enabled}
              onChange={(checked) => setForm((prev) => ({ ...prev, sso_enabled: checked }))}
            />
          </Space>
        </Form>
      </Modal>
    </div>
  );
}
