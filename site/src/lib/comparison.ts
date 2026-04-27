export type Support = 'yes' | 'no' | 'partial';

export type Tool = 'raid' | 'make' | 'just' | 'mise' | 'turbo';

export type ComparisonFeature = { label: string } & Record<Tool, Support>;

/** Tools in display order, with the label rendered in the comparison table header. */
export const tools: { id: Tool; label: string }[] = [
  { id: 'raid',  label: 'Raid'  },
  { id: 'make',  label: 'make'  },
  { id: 'just',  label: 'just'  },
  { id: 'mise',  label: 'mise'  },
  { id: 'turbo', label: 'turbo' },
];

export const comparisonFeatures: ComparisonFeature[] = [
  { label: 'Multi-repo orchestration', raid: 'yes', make: 'no',      just: 'no',      mise: 'partial', turbo: 'partial' },
  { label: 'Team profile sharing',     raid: 'yes', make: 'no',      just: 'no',      mise: 'no',      turbo: 'no'      },
  { label: 'One-command onboarding',   raid: 'yes', make: 'no',      just: 'no',      mise: 'partial', turbo: 'no'      },
  { label: 'Environment switching',    raid: 'yes', make: 'no',      just: 'no',      mise: 'yes',     turbo: 'no'      },
  { label: 'Custom task runner',       raid: 'yes', make: 'yes',     just: 'yes',     mise: 'yes',     turbo: 'yes'     },
  { label: 'YAML config',              raid: 'yes', make: 'no',      just: 'no',      mise: 'partial', turbo: 'no'      },
  { label: 'No DSL to learn',          raid: 'yes', make: 'no',      just: 'partial', mise: 'yes',     turbo: 'yes'     },
  { label: 'Concurrent task execution',raid: 'yes', make: 'partial', just: 'no',      mise: 'no',      turbo: 'yes'     },
  { label: 'Language-agnostic',        raid: 'yes', make: 'yes',     just: 'yes',     mise: 'yes',     turbo: 'no'      },
];

/** Numeric weight for a support value: yes=1, partial=0.5, no=0. */
export function supportScore(value: Support): number {
  switch (value) {
    case 'yes':     return 1;
    case 'partial': return 0.5;
    case 'no':      return 0;
  }
}

/** Sum of supportScore() across every feature for the given tool. */
export function toolScore(features: ComparisonFeature[], tool: Tool): number {
  return features.reduce((sum, feature) => sum + supportScore(feature[tool]), 0);
}
