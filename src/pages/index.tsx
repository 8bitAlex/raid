import Link from '@docusaurus/Link';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import HomepageFeatures from '@site/src/components/HomepageFeatures';
import TerminalDemo from '@site/src/components/TerminalDemo';
import Heading from '@theme/Heading';
import Layout from '@theme/Layout';
import clsx from 'clsx';
import type { ReactNode } from 'react';

import styles from './index.module.css';

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
          <Link
            className={clsx('button button--lg', styles.buttonGhost)}
            href="https://github.com/8bitalex/raid">
            View on GitHub
          </Link>
        </div>
        <TerminalDemo />
      </div>
    </header>
  );
}

function InstallSection() {
  return (
    <section className={styles.install}>
      <div className="container">
        <Heading as="h2" className={styles.installTitle}>Install</Heading>
        <div className={styles.installOptions}>
          <div className={styles.installOption}>
            <span className={styles.installLabel}>Homebrew</span>
            <code className={styles.installCmd}>brew install 8bitalex/tap/raid</code>
          </div>
          <div className={styles.installOption}>
            <span className={styles.installLabel}>Script</span>
            <code className={styles.installCmd}>{'curl -fsSL https://raw.githubusercontent.com/8bitalex/raid/main/install.sh | bash'}</code>
          </div>
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
        <HomepageFeatures />
        <InstallSection />
      </main>
    </Layout>
  );
}
