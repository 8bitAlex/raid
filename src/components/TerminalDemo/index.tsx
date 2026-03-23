import type { ReactNode } from 'react';
import styles from './styles.module.css';

// Catppuccin Mocha palette
const C = {
  text:    '#cdd6f4',
  subtext: '#a6adc8',
  muted:   '#6c7086',
  green:   '#a6e3a1',
  blue:    '#89b4fa',
};

type TermLine =
  | { k: 'cmd';     text: string }
  | { k: 'success'; label: string; detail: string }
  | { k: 'header';  text: string }
  | { k: 'done';    text: string }
  | { k: 'output';  text: string }
  | { k: 'blank' };

const lines: TermLine[] = [
  { k: 'cmd',     text: 'raid install' },
  { k: 'blank' },
  { k: 'header',  text: 'Cloning repositories...' },
  { k: 'success', label: 'api-service', detail: '~/dev/api-service' },
  { k: 'success', label: 'frontend',    detail: '~/dev/frontend' },
  { k: 'blank' },
  { k: 'header',  text: 'Running install tasks...' },
  { k: 'done',    text: 'Done in 12s' },
  { k: 'blank' },
  { k: 'cmd',     text: 'raid test' },
  { k: 'blank' },
  { k: 'header',  text: 'Running tests...' },
  { k: 'success', label: 'api-service', detail: 'All tests passed' },
  { k: 'success', label: 'frontend',    detail: 'All tests passed' },
  { k: 'done',    text: 'Done in 8s' },
];

function TermLine({ line }: { line: TermLine }): ReactNode {
  switch (line.k) {
    case 'blank':
      return <br />;
    case 'cmd':
      return (
        <div>
          <span style={{ color: C.muted }}>$ </span>
          <span style={{ color: C.text }}>{line.text}</span>
        </div>
      );
    case 'header':
      return (
        <div style={{ color: C.subtext, paddingLeft: '1.5ch' }}>
          {line.text}
        </div>
      );
    case 'success':
      return (
        <div style={{ paddingLeft: '1.5ch' }}>
          <span style={{ color: C.green }}>✓ </span>
          <span style={{ color: C.text, display: 'inline-block', minWidth: '14ch' }}>{line.label}</span>
          <span style={{ color: C.muted }}> → </span>
          <span style={{ color: C.blue }}>{line.detail}</span>
        </div>
      );
    case 'done':
      return (
        <div style={{ color: C.green, paddingLeft: '1.5ch' }}>
          {line.text}
        </div>
      );
    case 'output':
      return (
        <div style={{ color: C.subtext, paddingLeft: '1.5ch' }}>
          {line.text}
        </div>
      );
  }
}

export default function TerminalDemo(): ReactNode {
  return (
    <div className={styles.terminal}>
      <div className={styles.terminalBar}>
        <span className={styles.dot} />
        <span className={styles.dot} />
        <span className={styles.dot} />
        <span className={styles.terminalTitle}>raid</span>
      </div>
      <div className={styles.terminalBody}>
        {lines.map((line, i) => <TermLine key={i} line={line} />)}
      </div>
    </div>
  );
}
