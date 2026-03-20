import type {ReactNode} from 'react';
import clsx from 'clsx';
import Link from '@docusaurus/Link';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import Layout from '@theme/Layout';
import HomepageFeatures from '@site/src/components/HomepageFeatures';
import Heading from '@theme/Heading';

import styles from './index.module.css';

const terminal = `$ raid install

  Cloning repositories...
  ✓ api-service       → ~/dev/api-service
  ✓ frontend          → ~/dev/frontend
  ✓ shared-libs       → ~/dev/shared-libs

  Running install tasks...
  ✓ npm install       (api-service)
  ✓ npm install       (frontend)
  ✓ brew install node (shared-libs)

  Done in 12s`;

function HomepageHeader() {
  return (
    <header className={clsx('hero', styles.heroBanner)}>
      <div className="container">
        <Heading as="h1" className={styles.heroTitle}>
          Stop explaining setup.<br />Start shipping.
        </Heading>
        <p className={styles.heroSubtitle}>
          Codify your team's dev environment into a single YAML profile.<br />
          New teammates go from zero to running with one command.
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
        <div className={styles.terminal}>
          <div className={styles.terminalBar}>
            <span className={styles.dot} />
            <span className={styles.dot} />
            <span className={styles.dot} />
          </div>
          <pre className={styles.terminalBody}>{terminal}</pre>
        </div>
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
