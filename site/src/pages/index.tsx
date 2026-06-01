import Link from '@docusaurus/Link';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import HomepageFeatures from '@site/src/components/HomepageFeatures';
import SectionsNav from '@site/src/components/SectionsNav';
import { comparisonFeatures, tools, type Support } from '@site/src/lib/comparison';
import Heading from '@theme/Heading';
import Layout from '@theme/Layout';
import ThemedImage from '@theme/ThemedImage';
import clsx from 'clsx';
import { Check, Copy, Minus, X } from 'lucide-react';
import { useEffect, useState, type ReactNode } from 'react';

import styles from './index.module.css';

function CostSavings() {
  const HOURS_INCREMENT = 8;
  const RATE = 150;
  const RAMP_DUR = 800;
  const PAUSE_DUR = 4000;

  const [hours, setHours] = useState(0);
  const [rate, setRate] = useState(0);
  const [savings, setSavings] = useState(0);

  useEffect(() => {
    const ease = (t: number) => 1 - Math.pow(1 - t, 3);
    const clamp01 = (t: number) => Math.max(0, Math.min(t, 1));

    let raf = 0;
    let timeout = 0;
    let cancelled = false;

    const rateDelay = 300;
    const rateDur = 800;
    const savingsDelay = 700;
    const savingsDur = 900;
    const initialTotal = savingsDelay + savingsDur;
    const initialStart = performance.now();

    const initialTick = (now: number) => {
      if (cancelled) return;
      const elapsed = now - initialStart;
      setHours(Math.round(HOURS_INCREMENT * ease(clamp01(elapsed / RAMP_DUR))));
      setRate(Math.round(RATE * ease(clamp01((elapsed - rateDelay) / rateDur))));
      setSavings(Math.round(HOURS_INCREMENT * RATE * ease(clamp01((elapsed - savingsDelay) / savingsDur))));
      if (elapsed < initialTotal) {
        raf = requestAnimationFrame(initialTick);
      } else {
        timeout = window.setTimeout(() => rampFrom(HOURS_INCREMENT), PAUSE_DUR);
      }
    };

    const rampFrom = (from: number) => {
      if (cancelled) return;
      const to = from + HOURS_INCREMENT;
      const rampStart = performance.now();
      const rampTick = (now: number) => {
        if (cancelled) return;
        const t = clamp01((now - rampStart) / RAMP_DUR);
        const h = Math.round(from + (to - from) * ease(t));
        setHours(h);
        setSavings(h * RATE);
        if (t < 1) {
          raf = requestAnimationFrame(rampTick);
        } else {
          timeout = window.setTimeout(() => rampFrom(to), PAUSE_DUR);
        }
      };
      raf = requestAnimationFrame(rampTick);
    };

    raf = requestAnimationFrame(initialTick);

    return () => {
      cancelled = true;
      cancelAnimationFrame(raf);
      window.clearTimeout(timeout);
    };
  }, []);

  return (
    <div className={styles.savings} aria-label={`Time saved ${hours} hours times $${RATE} per hour equals $${savings} saved`}>
      <div className={styles.savingsItem}>
        <span className={styles.savingsLabel}>Time Saved</span>
        <span className={styles.savingsValue}>{hours}<span className={styles.savingsUnit}>hrs</span></span>
      </div>
      <span className={styles.savingsOp} aria-hidden>×</span>
      <div className={styles.savingsItem}>
        <span className={styles.savingsLabel}>Dev Cost</span>
        <span className={styles.savingsValue}>${rate}<span className={styles.savingsUnit}>/hr</span></span>
      </div>
      <span className={styles.savingsOp} aria-hidden>=</span>
      <div className={clsx(styles.savingsItem, styles.savingsResult)}>
        <span className={styles.savingsLabel}>Cost Savings</span>
        <span className={styles.savingsValue}>${savings.toLocaleString()}</span>
      </div>
    </div>
  );
}

function HomepageHeader() {
  return (
    <header id="top" className={clsx('hero', styles.heroBanner)}>
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
        <CostSavings />
        <div className={styles.buttons}>
          <Link className={clsx('button button--primary button--lg', styles.ctaPrimary)} to="/docs/overview">
            Get Started
          </Link>
          <a
            href="https://github.com/8bitAlex/raid"
            target="_blank"
            rel="noopener noreferrer"
            className={clsx('button button--secondary button--lg', styles.ctaSecondary)}>
            <svg
              aria-hidden="true"
              focusable="false"
              viewBox="0 0 16 16"
              width="20"
              height="20"
              className={styles.ctaIcon}>
              <path fill="currentColor" d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z" />
            </svg>
            Star on GitHub
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
    <section id="install" className={styles.install}>
      <div className="container">
        <Heading as="h2" className={styles.installTitle}>Easy Install</Heading>
        <div className={styles.installOptions}>
          <InstallOption label="Homebrew" cmd="brew install 8bitalex/tap/raid" />
          <InstallOption label="Script" cmd="curl -fsSL https://raidcli.dev/install.sh | bash" />
        </div>
        <a
          href="https://github.com/8bitalex/raid/releases/latest"
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
    <section id="compare" className={styles.comparison}>
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
                {tools.map(({id, label}) => (
                  <th key={id} className={id === 'raid' ? styles.thRaid : styles.thOther}>{label}</th>
                ))}
              </tr>
            </thead>
            <tbody>
              {comparisonFeatures.map((row) => (
                <tr key={row.label} className={styles.tableRow}>
                  <td className={styles.featureLabel}>{row.label}</td>
                  {tools.map(({id}) => (
                    <td key={id} className={clsx(styles.cell, id === 'raid' && styles.colRaid)}>
                      <SupportIcon value={row[id]} />
                    </td>
                  ))}
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
      <SectionsNav
        sections={[
          {id: 'top', label: 'Top'},
          {id: 'install', label: 'Install'},
          {id: 'features', label: 'Features'},
          {id: 'compare', label: 'Compare'},
        ]}
      />
      <HomepageHeader />
      <main>
        <InstallSection />
        <HomepageFeatures />
        <ComparisonSection />
      </main>
    </Layout>
  );
}
