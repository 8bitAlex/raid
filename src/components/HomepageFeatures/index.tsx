import type {ReactNode} from 'react';
import clsx from 'clsx';
import Heading from '@theme/Heading';
import styles from './styles.module.css';

type FeatureItem = {
  icon: string;
  title: string;
  description: ReactNode;
};

const FeatureList: FeatureItem[] = [
  {
    icon: '⚡',
    title: 'One-command onboarding',
    description: (
      <>
        <code>raid install</code> clones every repo in your profile and runs
        their install tasks concurrently. A new teammate is fully set up before
        they finish their coffee.
      </>
    ),
  },
  {
    icon: '📋',
    title: 'Tribal knowledge, codified',
    description: (
      <>
        Every setup step, script, and gotcha lives in <code>raid.yaml</code>{' '}
        alongside the code. No wiki to update, no Slack thread to dig through —
        the repo <em>is</em> the runbook.
      </>
    ),
  },
  {
    icon: '🛠️',
    title: 'Shared team commands',
    description: (
      <>
        Define custom commands once in your profile — <code>raid deploy</code>,{' '}
        <code>raid migrate</code>, whatever your team needs. Everyone gets the
        same commands without any extra setup.
      </>
    ),
  },
  {
    icon: '🌍',
    title: 'Environment switching',
    description: (
      <>
        <code>raid env staging</code> writes the right <code>.env</code> files
        into every repo and runs environment tasks across all of them at once.
        Switch contexts in seconds, not minutes.
      </>
    ),
  },
];

function Feature({icon, title, description}: FeatureItem) {
  return (
    <div className={clsx('col col--3', styles.feature)}>
      <div className={styles.featureIcon}>{icon}</div>
      <Heading as="h3" className={styles.featureTitle}>{title}</Heading>
      <p className={styles.featureDesc}>{description}</p>
    </div>
  );
}

export default function HomepageFeatures(): ReactNode {
  return (
    <section className={styles.features}>
      <div className="container">
        <div className="row">
          {FeatureList.map((props, idx) => (
            <Feature key={idx} {...props} />
          ))}
        </div>
      </div>
    </section>
  );
}
