import {themes as prismThemes} from 'prism-react-renderer';
import type {Config} from '@docusaurus/types';
import type * as Preset from '@docusaurus/preset-classic';

const config: Config = {
  title: 'Transiva',
  tagline: 'Enterprise Workload Mobility & Migration Control Plane — Community Edition.',
  favicon: 'img/favicon.svg',

  future: {
    v4: true,
  },

  url: 'https://zyvorai.github.io',
  baseUrl: '/transiva/',

  organizationName: 'zyvorai',
  projectName: 'transiva',

  onBrokenLinks: 'warn',

  markdown: {
    format: 'md',
    hooks: {
      onBrokenMarkdownLinks: 'warn',
    },
  },

  i18n: {
    defaultLocale: 'en',
    locales: ['en'],
  },

  presets: [
    [
      'classic',
      {
        docs: {
          path: '../docs',
          routeBasePath: 'docs',
          sidebarPath: './sidebars.ts',
          editUrl: 'https://github.com/zyvorai/transiva/tree/main/docs/',
        },
        blog: false,
        theme: {
          customCss: './src/css/custom.css',
        },
      } satisfies Preset.Options,
    ],
  ],

  themeConfig: {
    colorMode: {
      respectPrefersColorScheme: true,
    },
    navbar: {
      hideOnScroll: false,
      title: 'Transiva',
      logo: {
        alt: 'Transiva',
        src: 'img/favicon.svg',
      },
      items: [
        {
          type: 'docSidebar',
          sidebarId: 'docsSidebar',
          position: 'right',
          label: 'Docs',
        },
        {
          href: 'https://github.com/zyvorai/transiva',
          label: 'GitHub',
          position: 'right',
        },
      ],
    },
    footer: {
      style: 'dark',
      links: [
        {
          title: 'Docs',
          items: [
            {label: 'Nutanix walkthrough', to: '/docs/nutanix'},
            {label: 'CE vs Platform', to: '/docs/ce-vs-enterprise'},
            {label: 'Enterprise', to: '/docs/enterprise'},
          ],
        },
        {
          title: 'Project',
          items: [
            {label: 'GitHub', href: 'https://github.com/zyvorai/transiva'},
            {
              label: 'License (Apache-2.0)',
              href: 'https://github.com/zyvorai/transiva/blob/main/LICENSE',
            },
          ],
        },
        {
          title: 'HyperSDK Platform',
          items: [
            {label: '30-day PoC', href: 'https://zyvor.dev/poc?utm_source=github-pages&utm_medium=transiva'},
            {label: 'Book a demo', href: 'https://zyvor.dev/contact?intent=demo&utm_source=github-pages&utm_medium=transiva'},
            {label: 'Pricing', href: 'https://zyvor.dev/pricing?utm_source=github-pages&utm_medium=transiva'},
          ],
        },
      ],
      copyright: `Copyright © ${new Date().getFullYear()} Zyvor AI Labs. Transiva Community Edition is Apache-2.0 licensed.`,
    },
    prism: {
      theme: prismThemes.github,
      darkTheme: prismThemes.dracula,
    },
  } satisfies Preset.ThemeConfig,
};

export default config;
