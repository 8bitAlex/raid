import {useEffect, useState} from 'react';
import clsx from 'clsx';
import styles from './styles.module.css';

export type Section = {
  id: string;
  label: string;
};

type Props = {
  sections: Section[];
};

export default function SectionsNav({sections}: Props) {
  const [active, setActive] = useState<string>(sections[0]?.id ?? '');

  useEffect(() => {
    const elements = sections
      .map(({id}) => document.getElementById(id))
      .filter((el): el is HTMLElement => el !== null);

    if (elements.length === 0) return;

    const observer = new IntersectionObserver(
      (entries) => {
        const visible = entries
          .filter((e) => e.isIntersecting)
          .sort((a, b) => a.boundingClientRect.top - b.boundingClientRect.top);

        if (visible.length > 0) {
          setActive(visible[0].target.id);
        }
      },
      {
        rootMargin: '-25% 0px -65% 0px',
        threshold: 0,
      },
    );

    elements.forEach((el) => observer.observe(el));
    return () => observer.disconnect();
  }, [sections]);

  function handleClick(e: React.MouseEvent<HTMLAnchorElement>, id: string) {
    e.preventDefault();
    const el = document.getElementById(id);
    if (!el) return;
    el.scrollIntoView({behavior: 'smooth', block: 'start'});
    history.replaceState(null, '', `#${id}`);
  }

  return (
    <nav className={styles.nav} aria-label="Page sections">
      <ul className={styles.list}>
        {sections.map(({id, label}) => {
          const isActive = active === id;
          return (
            <li key={id}>
              <a
                href={`#${id}`}
                className={clsx(styles.item, isActive && styles.active)}
                aria-current={isActive ? 'location' : undefined}
                onClick={(e) => handleClick(e, id)}>
                <span className={styles.label}>{label}</span>
                <span className={styles.indicator} aria-hidden />
              </a>
            </li>
          );
        })}
      </ul>
    </nav>
  );
}
