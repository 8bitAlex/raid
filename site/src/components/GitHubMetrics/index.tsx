import {useEffect, useState} from 'react';
import {Star, Tag} from 'lucide-react';
import styles from './styles.module.css';

const REPO = '8bitalex/raid';
const CACHE_KEY = 'raid-gh-metrics';
const CACHE_TTL_MS = 10 * 60 * 1000;

type Metrics = {
  stars: number | null;
  version: string | null;
};

type CacheEntry = Metrics & {savedAt: number};

function readCache(): Metrics | null {
  try {
    const raw = sessionStorage.getItem(CACHE_KEY);
    if (!raw) return null;
    const parsed = JSON.parse(raw) as CacheEntry;
    if (Date.now() - parsed.savedAt > CACHE_TTL_MS) return null;
    return {stars: parsed.stars, version: parsed.version};
  } catch {
    return null;
  }
}

function writeCache(metrics: Metrics) {
  try {
    sessionStorage.setItem(
      CACHE_KEY,
      JSON.stringify({...metrics, savedAt: Date.now()} satisfies CacheEntry),
    );
  } catch {}
}

function formatStars(n: number): string {
  if (n >= 10000) return `${(n / 1000).toFixed(0)}k`;
  if (n >= 1000) return `${(n / 1000).toFixed(1)}k`;
  return n.toLocaleString();
}

function GitHubIcon() {
  return (
    <svg
      className={styles.logo}
      viewBox="0 0 24 24"
      aria-hidden
      focusable="false">
      <path
        fill="currentColor"
        d="M12 .297c-6.63 0-12 5.373-12 12 0 5.303 3.438 9.8 8.205 11.385.6.113.82-.258.82-.577 0-.285-.01-1.04-.015-2.04-3.338.724-4.042-1.61-4.042-1.61C4.422 18.07 3.633 17.7 3.633 17.7c-1.087-.744.084-.729.084-.729 1.205.084 1.838 1.236 1.838 1.236 1.07 1.835 2.809 1.305 3.495.998.108-.776.417-1.305.76-1.605-2.665-.3-5.466-1.332-5.466-5.93 0-1.31.465-2.38 1.235-3.22-.135-.303-.54-1.523.105-3.176 0 0 1.005-.322 3.3 1.23.96-.267 1.98-.399 3-.405 1.02.006 2.04.138 3 .405 2.28-1.552 3.285-1.23 3.285-1.23.645 1.653.24 2.873.12 3.176.765.84 1.23 1.91 1.23 3.22 0 4.61-2.805 5.625-5.475 5.92.42.36.81 1.096.81 2.22 0 1.606-.015 2.896-.015 3.286 0 .315.21.69.825.57C20.565 22.092 24 17.592 24 12.297c0-6.627-5.373-12-12-12"
      />
    </svg>
  );
}

export default function GitHubMetrics() {
  const [metrics, setMetrics] = useState<Metrics>({stars: null, version: null});

  useEffect(() => {
    const cached = readCache();
    if (cached) setMetrics(cached);

    Promise.all([
      fetch(`https://api.github.com/repos/${REPO}`).then((r) => (r.ok ? r.json() : null)),
      fetch(`https://api.github.com/repos/${REPO}/releases/latest`).then((r) => (r.ok ? r.json() : null)),
    ])
      .then(([repo, release]) => {
        const next: Metrics = {
          stars: repo?.stargazers_count ?? null,
          version: release?.tag_name ?? null,
        };
        setMetrics(next);
        writeCache(next);
      })
      .catch(() => {});
  }, []);

  return (
    <a
      href={`https://github.com/${REPO}`}
      target="_blank"
      rel="noopener noreferrer"
      className={styles.group}
      aria-label={`${REPO} on GitHub`}>
      <GitHubIcon />
      <div className={styles.content}>
        <span className={styles.repo}>{REPO}</span>
        <span className={styles.metrics}>
          {metrics.version && (
            <span className={styles.metric}>
              <Tag size={11} aria-hidden />
              {metrics.version}
            </span>
          )}
          {metrics.stars !== null && (
            <span className={styles.metric}>
              <Star size={11} aria-hidden />
              {formatStars(metrics.stars)}
            </span>
          )}
        </span>
      </div>
    </a>
  );
}
