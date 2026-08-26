// SPDX-License-Identifier: Apache-2.0

import React from 'react';

interface HeaderProps {
  onLogout?: () => void;
}

export const Header: React.FC<HeaderProps> = ({ onLogout }) => {
  return (
    <header style={{
      backgroundColor: '#fff',
      borderBottom: '1px solid #e0e0e0',
      position: 'sticky',
      top: 0,
      zIndex: 1000,
    }}>
      <div style={{
        maxWidth: '1400px',
        margin: '0 auto',
        padding: '16px 24px',
        display: 'flex',
        justifyContent: 'space-between',
        alignItems: 'center',
      }}>
        {/* Logo/Brand */}
        <div style={{ display: 'flex', alignItems: 'center', gap: '32px' }}>
          <h1 style={{
            margin: 0,
            fontSize: '24px',
            fontWeight: '700',
            color: '#000',
          }}>
            HyperSDK
          </h1>

          {/* Navigation Menu */}
          <nav style={{ display: 'flex', gap: '24px' }}>
            <a
              href="#dashboard"
              style={{
                color: '#222324',
                fontSize: '14px',
                fontWeight: '600',
                textDecoration: 'none',
                transition: 'color 0.25s cubic-bezier(0.215, 0.61, 0.355, 1)',
              }}
              onMouseEnter={(e) => e.currentTarget.style.color = '#f0583a'}
              onMouseLeave={(e) => e.currentTarget.style.color = '#222324'}
            >
              Dashboard
            </a>
            <a
              href="#jobs"
              style={{
                color: '#222324',
                fontSize: '14px',
                fontWeight: '600',
                textDecoration: 'none',
                transition: 'color 0.25s cubic-bezier(0.215, 0.61, 0.355, 1)',
              }}
              onMouseEnter={(e) => e.currentTarget.style.color = '#f0583a'}
              onMouseLeave={(e) => e.currentTarget.style.color = '#222324'}
            >
              Jobs
            </a>
            <a
              href="#manage"
              style={{
                color: '#222324',
                fontSize: '14px',
                fontWeight: '600',
                textDecoration: 'none',
                transition: 'color 0.25s cubic-bezier(0.215, 0.61, 0.355, 1)',
              }}
              onMouseEnter={(e) => e.currentTarget.style.color = '#f0583a'}
              onMouseLeave={(e) => e.currentTarget.style.color = '#222324'}
            >
              Manage
            </a>
            <a
              href="#providers"
              style={{
                color: '#222324',
                fontSize: '14px',
                fontWeight: '600',
                textDecoration: 'none',
                transition: 'color 0.25s cubic-bezier(0.215, 0.61, 0.355, 1)',
              }}
              onMouseEnter={(e) => e.currentTarget.style.color = '#f0583a'}
              onMouseLeave={(e) => e.currentTarget.style.color = '#222324'}
            >
              Providers
            </a>
          </nav>
        </div>

        {/* Right Side Actions */}
        <div style={{ display: 'flex', alignItems: 'center', gap: '16px' }}>
          <span style={{
            fontSize: '12px',
            color: '#6b7280',
            fontWeight: '500',
          }}>
            Support: 1-800-HyperSDK
          </span>
          {onLogout && (
            <button
              onClick={onLogout}
              style={{
                padding: '8px 16px',
                backgroundColor: 'transparent',
                color: '#222324',
                border: '1px solid #d1d5db',
                borderRadius: '4px',
                fontSize: '12px',
                fontWeight: '600',
                cursor: 'pointer',
                transition: 'all 0.25s cubic-bezier(0.215, 0.61, 0.355, 1)',
              }}
              onMouseEnter={(e) => {
                e.currentTarget.style.borderColor = '#f0583a';
                e.currentTarget.style.color = '#f0583a';
              }}
              onMouseLeave={(e) => {
                e.currentTarget.style.borderColor = '#d1d5db';
                e.currentTarget.style.color = '#222324';
              }}
            >
              Logout
            </button>
          )}
        </div>
      </div>
    </header>
  );
};
