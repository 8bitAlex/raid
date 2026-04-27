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

type PrefixRule = {
  prefix: string;
  symbolClass: string;
  bodyClass?: string;
};

const PREFIX_RULES: PrefixRule[] = [
  { prefix: '$ ', symbolClass: styles.prompt },
  { prefix: '→ ', symbolClass: styles.arrow, bodyClass: styles.arrowText },
  { prefix: '✓ ', symbolClass: styles.check },
  { prefix: '✗ ', symbolClass: styles.xmark },
];

function renderShellLine(line: string): ReactNode {
  for (const {prefix, symbolClass, bodyClass} of PREFIX_RULES) {
    if (line.startsWith(prefix)) {
      const body = line.slice(prefix.length);
      return (
        <>
          <span className={symbolClass}>{prefix.trimEnd()}</span>{' '}
          {bodyClass ? <span className={bodyClass}>{body}</span> : body}
        </>
      );
    }
  }
  if (/^Done\b/.test(line)) {
    return <span className={styles.done}>{line}</span>;
  }
  return line || ' ';
}
