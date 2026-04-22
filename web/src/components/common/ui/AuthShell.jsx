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

import React from 'react';
import Title from '@douyinfe/semi-ui/lib/es/typography/title';
import Text from '@douyinfe/semi-ui/lib/es/typography/text';

const AuthShell = ({
  logo,
  systemName,
  eyebrow,
  title,
  description,
  children,
  footer,
  sidebar,
  maxWidth = 'max-w-6xl',
}) => {
  return (
    <div className='auth-shell'>
      <div className='auth-shell__grid'>
        <div className={`auth-shell__panel ${maxWidth}`}>
          <div className='auth-shell__hero'>
            <div className='auth-shell__brand'>
              <div className='auth-shell__brand-mark'>
                <img src={logo} alt={systemName} className='auth-shell__logo' />
              </div>
              <div>
                <Text className='auth-shell__eyebrow'>
                  {eyebrow || systemName}
                </Text>
                <Title heading={2} className='auth-shell__title'>
                  {title}
                </Title>
                {description ? (
                  <Text className='auth-shell__description'>{description}</Text>
                ) : null}
              </div>
            </div>
            <div className='auth-shell__hero-metrics'>
              <div className='auth-shell__metric-card'>
                <span className='auth-shell__metric-label'>{systemName}</span>
                <strong className='auth-shell__metric-value'>Gateway</strong>
              </div>
              <div className='auth-shell__metric-card'>
                <span className='auth-shell__metric-label'>Security</span>
                <strong className='auth-shell__metric-value'>Passkey</strong>
              </div>
              <div className='auth-shell__metric-card'>
                <span className='auth-shell__metric-label'>Runtime</span>
                <strong className='auth-shell__metric-value'>Realtime</strong>
              </div>
            </div>
          </div>

          <div className='auth-shell__content'>{children}</div>

          {footer ? <div className='auth-shell__footer'>{footer}</div> : null}
        </div>

        <aside className='auth-shell__sidebar'>
          {sidebar || (
            <>
              <div className='auth-shell__sidebar-chip'>AI Gateway Control</div>
              <Title heading={4} className='auth-shell__sidebar-title'>
                更成熟的 AI 网关工作台
              </Title>
              <Text className='auth-shell__sidebar-text'>
                统一管理供应商、密钥、路由、订阅与使用情况。保持原有逻辑与能力，只升级界面的秩序、质感和操作节奏。
              </Text>
              <div className='auth-shell__sidebar-grid'>
                <div className='auth-shell__sidebar-card'>
                  <span>多模型</span>
                  <strong>30+</strong>
                </div>
                <div className='auth-shell__sidebar-card'>
                  <span>统一鉴权</span>
                  <strong>OAuth / 2FA</strong>
                </div>
                <div className='auth-shell__sidebar-card'>
                  <span>可观测性</span>
                  <strong>Charts & Logs</strong>
                </div>
                <div className='auth-shell__sidebar-card'>
                  <span>部署模式</span>
                  <strong>Self / External</strong>
                </div>
              </div>
            </>
          )}
        </aside>
      </div>
    </div>
  );
};

export default AuthShell;
