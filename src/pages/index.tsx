import Link from '@docusaurus/Link';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import HomepageFeatures from '@site/src/components/HomepageFeatures';
import TerminalDemo from '@site/src/components/TerminalDemo';
import Heading from '@theme/Heading';
import Layout from '@theme/Layout';
import clsx from 'clsx';
import { Check, Copy, Star } from 'lucide-react';
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
          Your workflow<br />as a config.
        </Heading>
        <p className={styles.heroSubtitle}>
          Stop fighting your tools. Start writing code.<br />
          Define setup, tasks, and environments in YAML — right in your repo.
        </p>
        <div className={styles.buttons}>
          <Link className="button button--primary button--lg" to="/docs/intro">
            Get Started
          </Link>
          <GitHubButtons />
        </div>
        <TerminalDemo />
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
        <Heading as="h2" className={styles.installTitle}>Install</Heading>
        <div className={styles.installOptions}>
          <InstallOption label="Homebrew" cmd="brew install 8bitalex/tap/raid" />
          <InstallOption label="Script" cmd="curl -fsSL https://raw.githubusercontent.com/8bitalex/raid/main/install.sh | bash" />
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
      description="Distributed dev environment orchestration. Codify your team's setup into a YAML profile — onboard any repo in one command.">
      <HomepageHeader />
      <main>
        <InstallSection />
        <HomepageFeatures />
      </main>
    </Layout>
  );
}
