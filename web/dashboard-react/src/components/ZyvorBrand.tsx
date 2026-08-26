// SPDX-License-Identifier: Apache-2.0

/**
 * Zyvor suite branding — text only.
 * Footer: zyvor.dev · © 2026 (orange) · optional host OS
 * Header: use ZyvorLogoMark (link only) — no copyright, no product name.
 */
import React from 'react';

export const ZYVOR_URL = 'https://zyvor.dev';
export const ZYVOR_BRAND = 'Zyvor';
export const ZYVOR_COPY = '© 2026';
export const ZYVOR_LINE = `zyvor.dev · ${ZYVOR_COPY}`;

const ORANGE = '#f97316';
const MUTED = 'rgba(148, 163, 184, 0.75)';

const linkStyle: React.CSSProperties = {
  color: ORANGE,
  textDecoration: 'none',
  fontWeight: 600,
};

const linkHover = (e: React.MouseEvent<HTMLAnchorElement>) => {
  e.currentTarget.style.color = '#fb923c';
};

const linkLeave = (e: React.MouseEvent<HTMLAnchorElement>) => {
  e.currentTarget.style.color = ORANGE;
};

const sep = <span aria-hidden style={{ color: MUTED }}> · </span>;

function ZyvorDevLink({ className = '' }: { className?: string }) {
  return (
    <a
      href={ZYVOR_URL}
      target="_blank"
      rel="noopener noreferrer"
      className={className}
      style={linkStyle}
      onMouseEnter={linkHover}
      onMouseLeave={linkLeave}
    >
      zyvor.dev
    </a>
  );
}

type BrandProps = {
  product?: string;
  className?: string;
  style?: React.CSSProperties;
  includeCopyright?: boolean;
};

/** Compact line: zyvor.dev · optional product · © 2026 */
export function ZyvorInline({
  className = '',
  style,
  product,
  includeCopyright = false,
}: BrandProps) {
  const label = product ?? ZYVOR_BRAND;
  return (
    <span
      className={`zyvor-inline whitespace-normal ${className}`.trim()}
      style={{
        fontSize: '12px',
        lineHeight: 1.5,
        color: MUTED,
        ...style,
      }}
    >
      <ZyvorDevLink />
      {label ? (
        <>
          {sep}
          <span>{label}</span>
        </>
      ) : null}
      {includeCopyright ? (
        <>
          {sep}
          <span style={{ color: ORANGE }}>{ZYVOR_COPY}</span>
        </>
      ) : null}
    </span>
  );
}

type FooterProps = {
  className?: string;
  /** Host OS pretty name (e.g. Rocky Linux 9.4) — shown when provided. */
  hostOs?: string;
  product?: string;
};

/** Page footer — zyvor.dev and © 2026 in orange; optional host OS line. */
export function ZyvorFooter({ className = '', hostOs, product }: FooterProps) {
  return (
    <footer
      className={`zyvor-footer shrink-0 py-3 text-center ${className}`.trim()}
      style={{
        marginTop: 'auto',
        background: 'transparent',
        border: 'none',
      }}
      role="contentinfo"
    >
      <div
        style={{
          fontSize: '12px',
          lineHeight: 1.5,
          color: MUTED,
        }}
      >
        <ZyvorDevLink />
        {product ? (
          <>
            {sep}
            <span>{product}</span>
          </>
        ) : null}
        {sep}
        <span style={{ color: ORANGE, fontWeight: 500 }}>{ZYVOR_COPY}</span>
      </div>
      {hostOs ? (
        <div
          className="mt-1 text-[11px] text-slate-500"
          title="Daemon host operating system"
        >
          {hostOs}
        </div>
      ) : null}
    </footer>
  );
}

/** @deprecated Use ZyvorFooter or ZyvorInline. */
export function ZyvorHelpStrip(_props: BrandProps) {
  return null;
}

/** Header: zyvor.dev link only. */
export function ZyvorLogoMark({ className = '' }: { className?: string }) {
  return (
    <a
      href={ZYVOR_URL}
      target="_blank"
      rel="noopener noreferrer"
      title="zyvor.dev"
      className={className}
      style={{
        fontWeight: 600,
        fontSize: '13px',
        color: ORANGE,
        textDecoration: 'none',
      }}
      onMouseEnter={linkHover}
      onMouseLeave={linkLeave}
    >
      zyvor.dev
    </a>
  );
}

export default ZyvorFooter;
