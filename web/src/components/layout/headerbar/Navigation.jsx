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

import React, { useEffect, useRef, useState } from 'react';
import { Link, useLocation } from 'react-router-dom';
import { Button } from '@douyinfe/semi-ui';
import { Menu, X, ExternalLink } from 'lucide-react';
import SkeletonWrapper from '../components/SkeletonWrapper';

const Navigation = ({
  mainNavLinks,
  isMobile,
  isLoading,
  userState,
  pricingRequireAuth,
  t,
}) => {
  const [menuOpen, setMenuOpen] = useState(false);
  const { pathname } = useLocation();
  const navRef = useRef(null);
  useEffect(() => setMenuOpen(false), [pathname, isMobile]);
  useEffect(() => {
    if (!menuOpen) return;
    const closeOutside = (event) => {
      if (!navRef.current?.contains(event.target)) setMenuOpen(false);
    };
    const closeOnEscape = (event) => {
      if (event.key === 'Escape') setMenuOpen(false);
    };
    document.addEventListener('pointerdown', closeOutside);
    document.addEventListener('keydown', closeOnEscape);
    return () => {
      document.removeEventListener('pointerdown', closeOutside);
      document.removeEventListener('keydown', closeOnEscape);
    };
  }, [menuOpen]);
  const targetFor = (link) => {
    if (link.itemKey === 'console' && !userState.user) return '/login';
    if (link.itemKey === 'pricing' && pricingRequireAuth && !userState.user)
      return '/login';
    return link.to;
  };
  const currentLink = mainNavLinks.find(
    (link) =>
      !link.isExternal &&
      (link.to === '/'
        ? pathname === '/'
        : pathname === link.to || pathname.startsWith(link.to + '/')),
  );

  if (isMobile) {
    return (
      <nav
        ref={navRef}
        className='compact-header-nav relative flex-1 min-w-0 mx-2 h-10'
      >
        <div className='absolute inset-0 flex items-center min-w-0'>
          <Button
            theme='borderless'
            type='tertiary'
            aria-label={menuOpen ? t('收起') : t('展开')}
            aria-expanded={menuOpen}
            aria-controls='compact-header-links'
            onClick={() => setMenuOpen((open) => !open)}
            className='!text-current !px-1.5 !bg-transparent flex-shrink-0'
          >
            {menuOpen ? <X size={18} /> : <Menu size={18} />}
            {!menuOpen && (
              <span className='truncate ml-1 max-w-[calc(100vw-250px)]'>
                {currentLink?.text || t('展开')}
              </span>
            )}
          </Button>
          {menuOpen && (
            <div
              id='compact-header-links'
              className='compact-header-links flex items-center min-w-0 flex-1 overflow-x-auto whitespace-nowrap scrollbar-hide'
            >
              {mainNavLinks.map((link) => {
                const className = `flex-shrink-0 flex items-center gap-1 px-2 py-2 text-sm font-semibold ${currentLink?.itemKey === link.itemKey ? 'text-semi-color-primary' : 'text-current'}`;
                return link.isExternal ? (
                  <a
                    key={link.itemKey}
                    href={link.externalLink}
                    target='_blank'
                    rel='noopener noreferrer'
                    className={className}
                    onClick={() => setMenuOpen(false)}
                  >
                    {link.text}
                    <ExternalLink size={12} />
                  </a>
                ) : (
                  <Link
                    key={link.itemKey}
                    to={targetFor(link)}
                    aria-current={
                      currentLink?.itemKey === link.itemKey ? 'page' : undefined
                    }
                    className={className}
                    onClick={() => setMenuOpen(false)}
                  >
                    {link.text}
                  </Link>
                );
              })}
            </div>
          )}
        </div>
      </nav>
    );
  }

  const renderNavLinks = () => {
    const baseClasses =
      'flex-shrink-0 flex items-center gap-1 font-semibold rounded-md transition-all duration-200 ease-in-out';
    const hoverClasses = 'hover:text-semi-color-primary';
    const spacingClasses = isMobile ? 'p-1' : 'p-2';

    const commonLinkClasses = `${baseClasses} ${spacingClasses} ${hoverClasses}`;

    return mainNavLinks.map((link) => {
      const linkContent = <span>{link.text}</span>;

      if (link.isExternal) {
        return (
          <a
            key={link.itemKey}
            href={link.externalLink}
            target='_blank'
            rel='noopener noreferrer'
            className={commonLinkClasses}
          >
            {linkContent}
          </a>
        );
      }

      let targetPath = link.to;
      if (link.itemKey === 'console' && !userState.user) {
        targetPath = '/login';
      }
      if (link.itemKey === 'pricing' && pricingRequireAuth && !userState.user) {
        targetPath = '/login';
      }

      return (
        <Link key={link.itemKey} to={targetPath} className={commonLinkClasses}>
          {linkContent}
        </Link>
      );
    });
  };

  return (
    <nav className='flex flex-1 items-center gap-1 lg:gap-2 mx-2 md:mx-4 overflow-x-auto whitespace-nowrap scrollbar-hide'>
      <SkeletonWrapper
        loading={isLoading}
        type='navigation'
        count={4}
        width={60}
        height={16}
        isMobile={isMobile}
      >
        {renderNavLinks()}
      </SkeletonWrapper>
    </nav>
  );
};

export default Navigation;
