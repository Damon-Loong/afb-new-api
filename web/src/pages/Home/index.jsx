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

import React, { useContext, useEffect, useState } from 'react';
import { Button, ScrollList, ScrollItem } from '@douyinfe/semi-ui';
import { API, showError, copy, showSuccess } from '../../helpers';
import { useIsMobile } from '../../hooks/common/useIsMobile';
import { API_ENDPOINTS } from '../../constants/common.constant';
import { StatusContext } from '../../context/Status';
import { useActualTheme } from '../../context/Theme';
import { marked } from 'marked';
import { useTranslation } from 'react-i18next';
import { IconPlay, IconCopy, IconFile, IconGithubLogo } from '@douyinfe/semi-icons';
import { Link } from 'react-router-dom';
import NoticeModal from '../../components/layout/NoticeModal';
import { OpenAI, Claude, Gemini, DeepSeek, Qwen } from '@lobehub/icons';

import './aperture.css';

const Home = () => {
  const { t, i18n } = useTranslation();
  const [statusState] = useContext(StatusContext);
  const actualTheme = useActualTheme();
  const [homePageContentLoaded, setHomePageContentLoaded] = useState(false);
  const [homePageContent, setHomePageContent] = useState('');
  const [noticeVisible, setNoticeVisible] = useState(false);
  const isMobile = useIsMobile();
  const rawServerAddress = statusState?.status?.server_address || '';
  const isLocalServerAddress =
    /^https?:\/\/(localhost|127\.0\.0\.1|0\.0\.0\.0)(:\d+)?/i.test(
      rawServerAddress,
    );
  const serverAddress =
    rawServerAddress && !isLocalServerAddress
      ? rawServerAddress
      : `${window.location.origin}`;
  const endpointItems = API_ENDPOINTS.map((e) => ({ value: e }));
  const [endpointIndex, setEndpointIndex] = useState(0);
  const isChinese = i18n.language.startsWith('zh');

  const displayHomePageContent = async () => {
    setHomePageContent(localStorage.getItem('home_page_content') || '');
    const res = await API.get('/api/home_page_content');
    const { success, message, data } = res.data;
    if (success) {
      let content = data;
      if (!data.startsWith('https://')) {
        content = marked.parse(data);
      }
      setHomePageContent(content);
      localStorage.setItem('home_page_content', content);

      // 如果内容是 URL，则发送主题模式
      if (data.startsWith('https://')) {
        const iframe = document.querySelector('iframe');
        if (iframe) {
          iframe.onload = () => {
            iframe.contentWindow.postMessage({ themeMode: actualTheme }, '*');
            iframe.contentWindow.postMessage({ lang: i18n.language }, '*');
          };
        }
      }
    } else {
      showError(message);
      setHomePageContent('加载首页内容失败...');
    }
    setHomePageContentLoaded(true);
  };

  const handleCopyBaseURL = async () => {
    const ok = await copy(serverAddress);
    if (ok) {
      showSuccess(t('已复制到剪切板'));
    }
  };

  useEffect(() => {
    const checkNoticeAndShow = async () => {
      const lastCloseDate = localStorage.getItem('notice_close_date');
      const today = new Date().toDateString();
      if (lastCloseDate !== today) {
        try {
          const res = await API.get('/api/notice');
          const { success, data } = res.data;
          if (success && data && data.trim() !== '') {
            setNoticeVisible(true);
          }
        } catch (error) {
          console.error('获取公告失败:', error);
        }
      }
    };

    checkNoticeAndShow();
  }, []);

  useEffect(() => {
    displayHomePageContent().then();
  }, []);

  useEffect(() => {
    const timer = setInterval(() => {
      setEndpointIndex((prev) => (prev + 1) % endpointItems.length);
    }, 3000);
    return () => clearInterval(timer);
  }, [endpointItems.length]);

  return (
    <div className='w-full overflow-x-hidden'>
      <NoticeModal
        visible={noticeVisible}
        onClose={() => setNoticeVisible(false)}
        isMobile={isMobile}
      />
      {homePageContentLoaded && homePageContent === '' ? (
        <div className='aperture-home' data-theme={actualTheme}>
          <div className='aperture-scene' aria-hidden='true' />
          <section className='aperture-hero' aria-labelledby='aperture-title'>
            <h1 id='aperture-title' className='aperture-title'>
              {isChinese ? (
                <>
                  <span className='shine-text'>
                    {i18n.language === 'zh-TW'
                      ? '企業級大模型'
                      : '企业级大模型'}
                  </span>
                  <span className='shine-text'>
                    {i18n.language === 'zh-TW' ? '介面' : '接口'}
                    <em className='shine-text'>
                      {i18n.language === 'zh-TW' ? '閘道' : '网关'}
                    </em>
                  </span>
                </>
              ) : (
                <span className='shine-text'>{t('企业级大模型接口网关')}</span>
              )}
            </h1>
            <p className='aperture-lead'>
              {t('更好的价格，更好的稳定性，只需要将模型基址替换为：')}
            </p>
            <div className='aperture-endpoint'>
              <input
                readOnly
                value={serverAddress}
                aria-label={t('统一入口')}
              />
              <div className='aperture-endpoint__path'>
                <ScrollList
                  bodyHeight={32}
                  style={{ border: 'unset', boxShadow: 'unset' }}
                >
                  <ScrollItem
                    mode='wheel'
                    cycled={true}
                    list={endpointItems}
                    selectedIndex={endpointIndex}
                    onSelect={({ index }) => setEndpointIndex(index)}
                  />
                </ScrollList>
              </div>
              <Button
                aria-label={t('复制')}
                onClick={handleCopyBaseURL}
                icon={<IconCopy />}
                theme='borderless'
              />
            </div>
            <div className='aperture-actions'>
              <Link
                className='aperture-button aperture-button--primary'
                to='/console'
              >
                <IconPlay />
                {t('获取密钥')}
              </Link>
              <Link className='aperture-button' to='/interface-docs'>
                <IconFile />
                {t('接口文档')}
              </Link>
              {statusState?.status?.demo_site_enabled && statusState?.status?.version ? (
                <a className='aperture-button' href='https://github.com/QuantumNous/new-api' target='_blank' rel='noopener noreferrer'>
                  <IconGithubLogo />
                  {statusState.status.version}
                </a>
              ) : null}
            </div>
          </section>
          <section className='aperture-summary' aria-label={t('运行概览')}>
            <div
              className='aperture-providers'
              aria-label={t('支持众多的大模型供应商')}
            >
              <div>
                <OpenAI size={30} />
                <span>OpenAI</span>
              </div>
              <div>
                <Claude.Color size={30} />
                <span>Claude</span>
              </div>
              <div>
                <Gemini.Color size={30} />
                <span>Gemini</span>
              </div>
              <div>
                <DeepSeek.Color size={34} />
                <span>DeepSeek</span>
              </div>
              <div>
                <Qwen.Color size={30} />
                <span>Qwen</span>
              </div>
              <div className='aperture-provider-count'>
                <strong>30+</strong>
                <span>{t('支持供应商')}</span>
              </div>
            </div>
            <div className='aperture-capabilities'>
              <div>
                <span>{t('接入能力')}</span>
                <strong>Chat / Image / Audio / Realtime</strong>
              </div>
              <div>
                <span>{t('适用场景')}</span>
                <strong>{t('聚合、鉴权、计费、控制台')}</strong>
              </div>
            </div>
          </section>
        </div>
      ) : (
        <div className='overflow-x-hidden w-full home-section-card'>
          {homePageContent.startsWith('https://') ? (
            <iframe
              src={homePageContent}
              className='w-full h-screen border-none'
            />
          ) : (
            <div
              className='mt-[60px]'
              dangerouslySetInnerHTML={{ __html: homePageContent }}
            />
          )}
        </div>
      )}
    </div>
  );
};

export default Home;
