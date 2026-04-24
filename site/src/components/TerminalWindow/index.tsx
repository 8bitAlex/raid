import type {ReactNode} from 'react';
import clsx from 'clsx';
import {Highlight, themes} from 'prism-react-renderer';
import styles from './styles.module.css';

type TerminalWindowProps = {
  title?: string;
  language?: string;
  children: string;
};

export default function TerminalWindow({title, language = 'bash', children}: TerminalWindowProps) {
  const code = children.trimEnd();
  const isShell = language === 'bash' || language === 'shell' || language === 'terminal';

  return (
    <div className={styles.window}>
      <div className={styles.bar}>
        <span className={styles.dots}>
          <span className={clsx(styles.dot, styles.dotRed)} />
          <span className={clsx(styles.dot, styles.dotYellow)} />
          <span className={clsx(styles.dot, styles.dotGreen)} />
        </span>
        {title && <span className={styles.title}>{title}</span>}
      </div>
      <div className={styles.body}>
        {isShell ? (
          <ShellOutput code={code} />
        ) : (
          <Highlight code={code} language={language} theme={themes.nightOwl}>
            {({className, tokens, getLineProps, getTokenProps}) => (
              <pre className={clsx(className, styles.pre)}>
                {tokens.map((line, i) => (
                  <div key={i} {...getLineProps({line})}>
                    {line.map((token, key) => (
                      <span key={key} {...getTokenProps({token})} />
                    ))}
                  </div>
                ))}
              </pre>
            )}
          </Highlight>
        )}
      </div>
    </div>
  );
}

function ShellOutput({code}: {code: string}) {
  const lines = code.split('\n');
  return (
    <pre className={styles.pre}>
      {lines.map((line, i) => (
        <div key={i} className={styles.line}>
          {renderShellLine(line)}
          {i < lines.length - 1 && '\n'}
        </div>
      ))}
    </pre>
  );
}

function renderShellLine(line: string): ReactNode {
  if (line.startsWith('$ ')) {
    return (
      <>
        <span className={styles.prompt}>$</span> {line.slice(2)}
      </>
    );
  }
  if (line.startsWith('→ ')) {
    return (
      <>
        <span className={styles.arrow}>→</span>{' '}
        <span className={styles.arrowText}>{line.slice(2)}</span>
      </>
    );
  }
  if (line.startsWith('✓ ')) {
    return (
      <>
        <span className={styles.check}>✓</span> {line.slice(2)}
      </>
    );
  }
  if (line.startsWith('✗ ')) {
    return (
      <>
        <span className={styles.xmark}>✗</span> {line.slice(2)}
      </>
    );
  }
  if (/^Done\b/.test(line)) {
    return <span className={styles.done}>{line}</span>;
  }
  return line || ' ';
}
