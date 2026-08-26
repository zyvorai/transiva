// SPDX-License-Identifier: Apache-2.0

import React, { useState, useEffect } from 'react';

interface LoginProps {
  onLogin: (username: string, password: string) => Promise<void>;
}

const orbs = [
  { size: 260, top: '10%', left: '8%', delay: '0s', dur: '9s' },
  { size: 170, top: '58%', left: '14%', delay: '2s', dur: '11s' },
  { size: 320, top: '72%', left: '62%', delay: '1s', dur: '12s' },
];

const features = [
  { icon: '☁️', title: 'Multi-cloud export', desc: 'VMware vSphere, OVA/OVF, and artifact manifests' },
  { icon: '⚡', title: 'hyper2kvm pipeline', desc: 'Inspect, fix, convert, validate — end to end' },
  { icon: '📦', title: 'Job orchestration', desc: 'Daemon-backed exports with live progress' },
  { icon: '🔐', title: 'Enterprise auth', desc: 'PAM, JWT sessions, audit-ready events' },
];

function IconUser() {
  return (
    <svg className="sdk-login-input-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
      <path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2" />
      <circle cx="12" cy="7" r="4" />
    </svg>
  );
}

function IconLock() {
  return (
    <svg className="sdk-login-input-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
      <rect x="3" y="11" width="18" height="11" rx="2" />
      <path d="M7 11V7a5 5 0 0 1 10 0v4" />
    </svg>
  );
}

function IconEye({ off }: { off?: boolean }) {
  if (off) {
    return (
      <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
        <path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94" />
        <path d="M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19" />
        <line x1="1" y1="1" x2="23" y2="23" />
      </svg>
    );
  }
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
      <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z" />
      <circle cx="12" cy="12" r="3" />
    </svg>
  );
}

function IconShield() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
      <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z" />
    </svg>
  );
}

export const Login: React.FC<LoginProps> = ({ onLogin }) => {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [showPassword, setShowPassword] = useState(false);
  const [rememberMe, setRememberMe] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const savedUsername = localStorage.getItem('hypersdk_username');
    const savedPassword = localStorage.getItem('hypersdk_password');
    if (savedUsername && savedPassword) {
      setUsername(savedUsername);
      setPassword(savedPassword);
      setRememberMe(true);
    }
  }, []);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsLoading(true);
    setError(null);

    try {
      await onLogin(username, password);

      if (rememberMe) {
        localStorage.setItem('hypersdk_username', username);
        localStorage.setItem('hypersdk_password', password);
        localStorage.setItem('hypersdk_remember', 'true');
      } else {
        localStorage.removeItem('hypersdk_username');
        localStorage.removeItem('hypersdk_password');
        localStorage.removeItem('hypersdk_remember');
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Login failed');
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <div className="sdk-login-root">
      <aside className="sdk-login-hero">
        {orbs.map((orb, i) => (
          <div
            key={i}
            className="sdk-login-orb"
            style={{
              width: orb.size,
              height: orb.size,
              top: orb.top,
              left: orb.left,
              '--sdk-delay': orb.delay,
              '--sdk-dur': orb.dur,
            } as React.CSSProperties}
          />
        ))}

        <div style={{ position: 'relative', zIndex: 1 }}>
          <div className="sdk-login-fade" style={{ display: 'flex', alignItems: 'center', gap: '0.75rem', marginBottom: '2rem' }}>
            <div className="sdk-login-logo" style={{ margin: 0 }}>🚀</div>
            <span style={{ fontSize: '1.75rem', fontWeight: 700, color: '#fafafa' }}>HyperSDK</span>
          </div>
          <h2 className="sdk-login-hero-title sdk-login-fade sdk-login-fade-d1">
            Multi-cloud
            <br />
            <span className="sdk-login-hero-accent">VM export</span>
          </h2>
          <p className="sdk-login-hero-lead sdk-login-fade sdk-login-fade-d2">
            Export from vSphere, ship artifacts, and run the hyper2kvm pipeline — from one control plane.
          </p>
          <div className="sdk-login-fade sdk-login-fade-d3" style={{ display: 'flex', flexWrap: 'wrap', gap: '0.5rem', marginTop: '1.25rem' }}>
            <span className="sdk-login-pill">PAM · JWT</span>
            <span className="sdk-login-pill">Artifact manifest v1</span>
            <span className="sdk-login-pill">Windows RDP opt-in</span>
          </div>
        </div>

        <div style={{ position: 'relative', zIndex: 1, display: 'flex', flexDirection: 'column', gap: '1rem' }}>
          {features.map((feat, i) => (
            <div
              key={feat.title}
              className="sdk-login-feature sdk-login-fade"
              style={{ animationDelay: `${0.35 + i * 0.07}s`, opacity: 0 }}
            >
              <div className="sdk-login-feature-icon">{feat.icon}</div>
              <div>
                <h3>{feat.title}</h3>
                <p>{feat.desc}</p>
              </div>
            </div>
          ))}
        </div>

        <p className="sdk-login-hero-foot" style={{ position: 'relative', zIndex: 1 }}>
          transiva.cloud · hyper2kvm pipeline
        </p>
      </aside>

      <main className="sdk-login-panel">
        <div className="sdk-login-panel-dots" aria-hidden />

        <div>
          <div className="sdk-login-brand-mobile">
            <div className="sdk-login-logo">🚀</div>
            <h1 className="sdk-login-title">HyperSDK</h1>
            <p className="sdk-login-subtitle">Multi-Cloud VM Export Platform</p>
          </div>

          <div className="sdk-login-brand-desktop">
            <h2 className="sdk-login-title" style={{ fontSize: '1.5rem' }}>Welcome back</h2>
            <p className="sdk-login-subtitle">Sign in to your export dashboard</p>
          </div>

          <form onSubmit={handleSubmit} className="sdk-login-card">
            {error && (
              <div className="sdk-login-error" role="alert">
                <span>⚠</span>
                <span>{error}</span>
              </div>
            )}

            <div className="sdk-login-field">
              <label htmlFor="username" className="sdk-login-label">
                Username
              </label>
              <div className="sdk-login-input-wrap">
                <IconUser />
                <input
                  id="username"
                  type="text"
                  className="sdk-login-input"
                  value={username}
                  onChange={(e) => setUsername(e.target.value)}
                  placeholder="System username"
                  autoComplete="username"
                  autoFocus
                  required
                  disabled={isLoading}
                />
              </div>
            </div>

            <div className="sdk-login-field">
              <label htmlFor="password" className="sdk-login-label">
                Password
              </label>
              <div className="sdk-login-input-wrap">
                <IconLock />
                <input
                  id="password"
                  type={showPassword ? 'text' : 'password'}
                  className="sdk-login-input"
                  style={{ paddingRight: '2.75rem' }}
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  placeholder="Password"
                  autoComplete="current-password"
                  required
                  disabled={isLoading}
                />
                <button
                  type="button"
                  className="sdk-login-toggle-pw"
                  onClick={() => setShowPassword((v) => !v)}
                  aria-label={showPassword ? 'Hide password' : 'Show password'}
                  tabIndex={-1}
                >
                  <IconEye off={showPassword} />
                </button>
              </div>
            </div>

            <label className="sdk-login-remember">
              <input
                type="checkbox"
                checked={rememberMe}
                onChange={(e) => setRememberMe(e.target.checked)}
                disabled={isLoading}
              />
              <span>Remember me on this device</span>
            </label>

            <button type="submit" className="sdk-login-submit" disabled={isLoading || !username || !password}>
              <span className="sdk-login-submit-inner">
                {isLoading ? (
                  <>
                    <span className="spinner" style={{ width: '1rem', height: '1rem', borderWidth: 2 }} />
                    Signing in…
                  </>
                ) : (
                  <>
                    Sign in
                    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                      <path d="M5 12h14M12 5l7 7-7 7" />
                    </svg>
                  </>
                )}
              </span>
            </button>

            <div className="sdk-login-footer">
              <IconShield />
              <span>Secured with system PAM authentication</span>
            </div>
          </form>
        </div>
      </main>
    </div>
  );
};

/** Full-screen loading state matching login chrome */
export function LoginLoading() {
  return (
    <div className="sdk-login-loading">
      <div className="spinner" />
      <span>Loading HyperSDK…</span>
    </div>
  );
}
