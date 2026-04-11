import {
  comparisonFeatures,
  supportScore,
  toolScore,
  type Tool,
} from '../comparison';

describe('supportScore', () => {
  test('yes scores 1', () => {
    expect(supportScore('yes')).toBe(1);
  });

  test('partial scores 0.5', () => {
    expect(supportScore('partial')).toBe(0.5);
  });

  test('no scores 0', () => {
    expect(supportScore('no')).toBe(0);
  });
});

describe('toolScore', () => {
  test('raid gets full marks on every comparison feature', () => {
    expect(toolScore(comparisonFeatures, 'raid')).toBe(comparisonFeatures.length);
  });

  test('raid scores strictly higher than every other tool', () => {
    const raid = toolScore(comparisonFeatures, 'raid');
    for (const tool of ['make', 'just', 'mise'] as const) {
      expect(raid).toBeGreaterThan(toolScore(comparisonFeatures, tool));
    }
  });

  test('score is bounded by the number of features', () => {
    const tools: Tool[] = ['raid', 'make', 'just', 'mise'];
    for (const tool of tools) {
      const score = toolScore(comparisonFeatures, tool);
      expect(score).toBeGreaterThanOrEqual(0);
      expect(score).toBeLessThanOrEqual(comparisonFeatures.length);
    }
  });

  test('empty feature list scores zero', () => {
    expect(toolScore([], 'raid')).toBe(0);
  });
});

describe('comparisonFeatures', () => {
  test('is non-empty', () => {
    expect(comparisonFeatures.length).toBeGreaterThan(0);
  });

  test('every feature has a non-empty label and a support value for every tool', () => {
    const tools: Tool[] = ['raid', 'make', 'just', 'mise'];
    for (const feature of comparisonFeatures) {
      expect(feature.label).toBeTruthy();
      for (const tool of tools) {
        expect(['yes', 'no', 'partial']).toContain(feature[tool]);
      }
    }
  });
});
