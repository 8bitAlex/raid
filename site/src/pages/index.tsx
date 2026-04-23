import Link from '@docusaurus/Link';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import HomepageFeatures from '@site/src/components/HomepageFeatures';
import { comparisonFeatures, type Support } from '@site/src/lib/comparison';
import Heading from '@theme/Heading';
import Layout from '@theme/Layout';
import ThemedImage from '@theme/ThemedImage';
import clsx from 'clsx';
import { Check, Copy, Minus, Star, X } from 'lucide-react';
import { useEffect, useState, type ReactNode } from 'react';

import styles from './index.module.css';

function GitHubButtons() {
  const [stars, setStars] = useState<number | null>(null);

  useEffect(() => {
    fetch('https://api.github.com/repos/8bitalex/raid')
      .then((res) => res.json())
      .then((data) => setStars(data.stargazers_count))
      .catch(() => {});
  }, []);

  return (
    <a
      href="https://github.com/8bitalex/raid"
      target="_blank"
      rel="noopener noreferrer"
      className={styles.githubGroup}>
      <span className={clsx('button button--lg', styles.buttonGhost, styles.githubLabel)}>
        View on GitHub
      </span>
      {stars !== null && (
        <span className={clsx('button button--lg', styles.buttonGhost, styles.starButton)}>
          <Star size={14} />
          {stars.toLocaleString()}
        </span>
      )}
    </a>
  );
}

function HomepageHeader() {
  return (
    <header className={clsx('hero', styles.heroBanner)}>
      <div className="container">
        <Heading as="h1" className={styles.heroTitle}>
          <ThemedImage
            alt="Raid logo"
            className={styles.heroLogo}
            sources={{
              light: '/img/logo-light.svg',
              dark: '/img/logo-dark.svg',
            }}
          />
          Raid
        </Heading>
        <p className={styles.heroSubtitle}>
          Open-source CLI for orchestrating complex development workflows.
        </p>
        <div className={styles.buttons}>
          <Link className="button button--primary button--lg" to="/docs/overview">
            Get Started
          </Link>
          <GitHubButtons />
          <a
            href="https://www.producthunt.com/products/raid?embed=true&utm_source=badge-featured&utm_medium=badge&utm_campaign=badge-raid"
            target="_blank"
            rel="noopener noreferrer"
            className={styles.productHuntBadge}>
            <img
              alt="Raid - Open-source development workflow orchestrator | Product Hunt"
              src="https://api.producthunt.com/widgets/embed-image/v1/featured.svg?post_id=1128226&theme=neutral&t=1776801713624"
            />
          </a>
        </div>
        <img
          src="/img/raid-comparison.gif"
          alt="Raid CLI demo"
          className={styles.heroGif}
        />
      </div>
    </header>
  );
}

function InstallOption({ label, cmd }: { label: string; cmd: string }) {
  const [copied, setCopied] = useState(false);

  function handleCopy() {
    navigator.clipboard.writeText(cmd).then(() => {
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    });
  }

  return (
    <div className={styles.installOption}>
      <span className={styles.installLabel}>{label}</span>
      <code className={styles.installCmd}>{cmd}</code>
      <button className={styles.copyButton} onClick={handleCopy} aria-label="Copy to clipboard">
        {copied ? <Check size={15} /> : <Copy size={15} />}
      </button>
    </div>
  );
}

function InstallSection() {
  return (
    <section className={styles.install}>
      <div className="container">
        <Heading as="h2" className={styles.installTitle}>Easy Install</Heading>
        <div className={styles.installOptions}>
          <InstallOption label="Homebrew" cmd="brew install 8bitalex/tap/raid" />
          <InstallOption label="Script" cmd="curl -fsSL https://raidcli.dev/install.sh | bash" />
        </div>
        <a
          href="https://github.com/8bitAlex/raid/releases/latest"
          target="_blank"
          rel="noopener noreferrer"
          className={styles.manualDownload}>
          Or download manually from GitHub (Windows, Linux, macOS)
        </a>
      </div>
    </section>
  );
}

function SupportIcon({ value }: { value: Support }) {
  if (value === 'yes') return <Check size={16} className={styles.iconYes} />;
  if (value === 'partial') return <Minus size={16} className={styles.iconPartial} />;
  return <X size={16} className={styles.iconNo} />;
}

function ComparisonSection() {
  return (
    <section className={styles.comparison}>
      <div className="container">
        <div className={styles.tableWrapper}>
          <Heading as="h2" className={styles.comparisonTitle}>How does Raid stack up?</Heading>
          <p className={styles.comparisonSubtitle}>
            See how Raid compares to other popular task runners and dev tools.
          </p>
          <table className={styles.comparisonTable}>
            <thead>
              <tr>
                <th className={styles.thFeature}></th>
                <th className={styles.thRaid}>Raid</th>
                <th className={styles.thOther}>make</th>
                <th className={styles.thOther}>just</th>
                <th className={styles.thOther}>mise</th>
                <th className={styles.thOther}>turbo</th>
              </tr>
            </thead>
            <tbody>
              {comparisonFeatures.map((row) => (
                <tr key={row.label} className={styles.tableRow}>
                  <td className={styles.featureLabel}>{row.label}</td>
                  <td className={clsx(styles.cell, styles.colRaid)}><SupportIcon value={row.raid} /></td>
                  <td className={styles.cell}><SupportIcon value={row.make} /></td>
                  <td className={styles.cell}><SupportIcon value={row.just} /></td>
                  <td className={styles.cell}><SupportIcon value={row.mise} /></td>
                  <td className={styles.cell}><SupportIcon value={row.turbo} /></td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </section>
  );
}

export default function Home(): ReactNode {
  const {siteConfig} = useDocusaurusContext();
  return (
    <Layout
      title={siteConfig.title}
      description="Open-source CLI for orchestrating complex development workflows.">
      <HomepageHeader />
      <main>
        <InstallSection />
        <HomepageFeatures />
        
        <ComparisonSection />
      </main>
    </Layout>
  );
}
