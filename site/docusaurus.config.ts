import type * as Preset from '@docusaurus/preset-classic';
import type { Config } from '@docusaurus/types';
import { themes as prismThemes } from 'prism-react-renderer';

// This runs in Node.js - Don't use client-side code here (browser APIs, JSX...)

const config: Config = {
  title: 'Raid',
  tagline: 'Distributed development environment orchestration tool',
  favicon: 'img/favicon.svg',

  future: {
    v4: true,
  },

  url: 'https://raidcli.dev',
  baseUrl: '/',

  organizationName: '8bitalex',
  projectName: 'raid',
  trailingSlash: false,

  onBrokenLinks: 'throw',

  i18n: {
    defaultLocale: 'en',
    locales: ['en'],
  },

  plugins: [
    [
      '@easyops-cn/docusaurus-search-local',
      {
        hashed: true,
        docsRouteBasePath: '/docs',
      },
    ],
    ...(process.env.POSTHOG_API_KEY
      ? [[
          'posthog-docusaurus',
          {
            apiKey: process.env.POSTHOG_API_KEY,
            appUrl: process.env.POSTHOG_HOST,
            enableInDevelopment: false,
          },
        ]]
      : []),
  ],

  presets: [
    [
      'classic',
      {
        docs: {
          sidebarPath: './sidebars.ts',
          editUrl: 'https://github.com/8bitalex/raid/tree/docsite-source/',
        },
        blog: false,
        theme: {
          customCss: './src/css/custom.css',
        },
      } satisfies Preset.Options,
    ],
  ],

  themeConfig: {
    image: 'img/social-preview.png',
    colorMode: {
      defaultMode: 'dark',
      respectPrefersColorScheme: false,
    },
    navbar: {
      title: 'Raid',
      logo: {
        alt: 'Raid logo',
        src: 'img/logo-light.svg',
        srcDark: 'img/logo-dark.svg',
      },
      items: [
        {
          type: 'docSidebar',
          sidebarId: 'tutorialSidebar',
          position: 'left',
          label: 'Docs',
        },
        {
          type: 'doc',
          docId: 'whats-new',
          position: 'left',
          label: 'What\'s New',
        },
        {
          type: 'search',
          position: 'right',
        },
        {
          type: 'custom-github-metrics',
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
            {
              label: 'Getting Started',
              to: '/docs/overview',
            },
            {
              label: 'What\'s New',
              to: '/docs/whats-new',
            },
            {
              label: 'Usage',
              to: '/docs/category/usage',
            },
            {
              label: 'Features',
              to: '/docs/category/features',
            },
            {
              label: 'Examples',
              to: '/docs/category/examples',
            },
            {
              label: 'References',
              to: '/docs/category/references',
            },
          ],
        },
        {
          title: 'More',
          items: [
            {
              label: 'GitHub',
              href: 'https://github.com/8bitalex/raid',
            },
            {
              label: 'Product Hunt',
              href: 'https://www.producthunt.com/products/raid',
            },
            {
              label: 'Launch Llama',
              href: 'https://tools.launchllama.co/products/raid',
            },
          ],
        },
        {
          title: 'Featured On',
          items: [
            {
              html: `<a href="https://launchllama.co?utm_source=badge&utm_medium=referral" target="_blank" rel="noopener noreferrer"><img src="https://speaktechenglish.com/wp-content/uploads/2026/04/Screenshot_2026-04-09_at_17.40.44-removebg-preview.png" alt="Featured on Launch Llama" width="200" height="50" onerror="this.src='/img/launch-llama-badge.png'" /></a>`,
            },
            {
              html: `<a href="https://www.producthunt.com/products/raid?embed=true&utm_source=badge-featured&utm_medium=badge&utm_campaign=badge-raid" target="_blank" rel="noopener noreferrer"><img src="https://api.producthunt.com/widgets/embed-image/v1/featured.svg?post_id=1128226&theme=neutral&t=1776801713624" alt="Featured on Product Hunt" width="200" height="43" /></a>`,
            },
          ],
        },
      ],
      copyright: `Copyright © ${new Date().getFullYear()} Raid.`,
    },
    prism: {
      theme: prismThemes.github,
      darkTheme: prismThemes.dracula,
    },
  } satisfies Preset.ThemeConfig,
};

export default config;
