import type {ReactNode} from 'react';
import clsx from 'clsx';
import Heading from '@theme/Heading';
import TerminalWindow from '@site/src/components/TerminalWindow';
import { Rocket, BookOpen, Users, Layers } from 'lucide-react';
import styles from './styles.module.css';

type CodeSample = {
  language: string;
  title?: string;
  content: string;
};

type FeatureItem = {
  icon: ReactNode;
  title: string;
  description: ReactNode;
  code: CodeSample;
};

const FeatureList: FeatureItem[] = [
  {
    icon: <Rocket size={28} />,
    title: 'One-command onboarding',
    description: (
      <>
        <code>raid install</code> clones every repo in your profile and runs
        their install tasks concurrently. A new teammate is fully set up before
        they finish their coffee.
      </>
    ),
    code: {
      language: 'bash',
      title: 'terminal',
      content: `$ raid install
→ Cloning 3 repositories
✓ api         ~/dev/my-project/api
✓ frontend    ~/dev/my-project/frontend
✓ worker      ~/dev/my-project/worker
→ Running install tasks (3 workers)
✓ api         npm install
✓ frontend    npm install
✓ worker      go mod download
Done in 1m 18s`,
    },
  },
  {
    icon: <BookOpen size={28} />,
    title: 'Tribal knowledge, codified',
    description: (
      <>
        Every setup step, script, and gotcha lives in <code>raid.yaml</code>{' '}
        alongside the code. No wiki to update, no Slack thread to dig through —
        the repo <em>is</em> the runbook.
      </>
    ),
    code: {
      language: 'yaml',
      title: 'raid.yaml',
      content: `install:
  tasks:
    - type: Shell
      cmd: brew bundle
      condition:
        platform: darwin
    - type: Shell
      cmd: npm install
    - type: Shell
      cmd: cp .env.example .env
      condition:
        exists: .env.example`,
    },
  },
  {
    icon: <Users size={28} />,
    title: 'Shared team commands',
    description: (
      <>
        Define custom commands once in your profile — <code>raid deploy</code>,{' '}
        <code>raid reset-db</code>, whatever your team needs. Everyone gets the
        same commands without any extra setup.
      </>
    ),
    code: {
      language: 'yaml',
      title: 'profile.raid.yml',
      content: `commands:
  - name: deploy
    usage: Deploy to production
    tasks:
      - type: Confirm
        message: "Deploy to production?"
      - type: Shell
        cmd: ./scripts/deploy.sh

  - name: reset-db
    usage: Reset local database
    tasks:
      - type: Shell
        cmd: docker compose down -v db
      - type: Group
        ref: db-migrate`,
    },
  },
  {
    icon: <Layers size={28} />,
    title: 'Environment switching',
    description: (
      <>
        <code>raid env staging</code> writes the right <code>.env</code> files
        into every repo and runs environment tasks across all of them at once.
        Switch contexts in seconds, not minutes.
      </>
    ),
    code: {
      language: 'yaml',
      title: 'profile.raid.yml',
      content: `environments:
  - name: dev
    variables:
      - name: NODE_ENV
        value: development
      - name: DATABASE_URL
        value: postgresql://localhost/dev
    tasks:
      - type: Shell
        cmd: docker compose up -d
      - type: Wait
        url: http://localhost:5432
        timeout: 30s`,
    },
  },
];

function Feature({icon, title, description, code, reversed}: FeatureItem & {reversed: boolean}) {
  return (
    <div className={clsx(styles.featureRow, reversed && styles.featureRowReversed)}>
      <div className={styles.featureText}>
        <div className={styles.featureIcon}>{icon}</div>
        <Heading as="h3" className={styles.featureTitle}>{title}</Heading>
        <p className={styles.featureDesc}>{description}</p>
      </div>
      <div className={styles.featureVisual}>
        <TerminalWindow language={code.language} title={code.title}>
          {code.content}
        </TerminalWindow>
      </div>
    </div>
  );
}

export default function HomepageFeatures(): ReactNode {
  return (
    <section id="features" className={styles.features}>
      <div className="container">
        {FeatureList.map((props, idx) => (
          <Feature key={idx} {...props} reversed={idx % 2 === 1} />
        ))}
      </div>
    </section>
  );
}
