import React, { useState } from 'react';
import { Badge, Button, Dropdown } from '@douyinfe/semi-ui';
import {
  Bell,
  MoreHorizontal,
  Sun,
  Moon,
  Monitor,
  Languages,
} from 'lucide-react';

const languages = [
  ['zh-CN', '简体中文'],
  ['zh-TW', '繁體中文'],
  ['en', 'English'],
  ['fr', 'Français'],
  ['ja', '日本語'],
  ['ru', 'Русский'],
  ['vi', 'Tiếng Việt'],
];

const MobileTools = ({
  unreadCount,
  onNoticeOpen,
  theme,
  onThemeToggle,
  currentLang,
  onLanguageChange,
  t,
}) => {
  const [open, setOpen] = useState(false);
  const select = (action) => {
    setOpen(false);
    action();
  };
  const themes = [
    ['light', t('浅色模式'), <Sun size={17} />],
    ['dark', t('深色模式'), <Moon size={17} />],
    ['auto', t('自动模式'), <Monitor size={17} />],
  ];
  return (
    <Dropdown
      trigger='click'
      position='bottomRight'
      visible={open}
      onVisibleChange={setOpen}
      render={
        <Dropdown.Menu
          style={{
            minWidth: 200,
            maxHeight: 'calc(100dvh - 110px)',
            overflowY: 'auto',
          }}
        >
          <Dropdown.Item
            icon={<Bell size={17} />}
            onClick={() => select(onNoticeOpen)}
          >
            <span className='flex items-center justify-between gap-4 w-full'>
              {t('系统公告')}
              {unreadCount > 0 && (
                <Badge count={unreadCount} overflowCount={99} />
              )}
            </span>
          </Dropdown.Item>
          <Dropdown.Divider />
          <Dropdown.Title>{t('切换主题')}</Dropdown.Title>
          {themes.map(([value, label, icon]) => (
            <Dropdown.Item
              key={value}
              icon={icon}
              selected={theme === value}
              showTick
              onClick={() => select(() => onThemeToggle(value))}
            >
              {label}
            </Dropdown.Item>
          ))}
          <Dropdown.Divider />
          <Dropdown.Title>{t('common.changeLanguage')}</Dropdown.Title>
          {languages.map(([value, label]) => (
            <Dropdown.Item
              key={value}
              icon={<Languages size={17} />}
              selected={currentLang === value}
              showTick
              onClick={() => select(() => onLanguageChange(value))}
            >
              {label}
            </Dropdown.Item>
          ))}
        </Dropdown.Menu>
      }
    >
      <span className='inline-flex'>
        <Badge dot count={unreadCount > 0 ? 1 : 0} type='danger'>
          <Button
            icon={<MoreHorizontal size={20} />}
            aria-label={t('更多')}
            aria-haspopup='menu'
            aria-expanded={open}
            theme='borderless'
            type='tertiary'
            className='!text-current !rounded-full !bg-semi-color-fill-0 !w-9 !h-9'
          />
        </Badge>
      </span>
    </Dropdown>
  );
};
export default MobileTools;
