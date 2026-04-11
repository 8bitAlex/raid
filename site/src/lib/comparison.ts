export type Support = 'yes' | 'no' | 'partial';

export type Tool = 'raid' | 'make' | 'just' | 'mise';

export type ComparisonFeature = { label: string } & Record<Tool, Support>;

export const comparisonFeatures: ComparisonFeature[] = [
  { label: 'Multi-repo orchestration', raid: 'yes', make: 'no',      just: 'no',      mise: 'partial' },
  { label: 'Team profile sharing',     raid: 'yes', make: 'no',      just: 'no',      mise: 'no'      },
  { label: 'One-command onboarding',   raid: 'yes', make: 'no',      just: 'no',      mise: 'partial' },
  { label: 'Environment switching',    raid: 'yes', make: 'no',      just: 'no',      mise: 'yes'     },
  { label: 'Custom task runner',       raid: 'yes', make: 'yes',     just: 'yes',     mise: 'yes'     },
  { label: 'YAML config',              raid: 'yes', make: 'no',      just: 'no',      mise: 'partial' },
  { label: 'No DSL to learn',          raid: 'yes', make: 'no',      just: 'partial', mise: 'yes'     },
  { label: 'Concurrent task execution',raid: 'yes', make: 'partial', just: 'no',      mise: 'no'      },
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
