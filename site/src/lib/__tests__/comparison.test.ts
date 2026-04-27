import {
  comparisonFeatures,
  supportScore,
  toolScore,
  tools,
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
    for (const {id} of tools.filter((t) => t.id !== 'raid')) {
      expect(raid).toBeGreaterThan(toolScore(comparisonFeatures, id));
    }
  });

  test('score is bounded by the number of features', () => {
    for (const {id} of tools) {
      const score = toolScore(comparisonFeatures, id);
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
    for (const feature of comparisonFeatures) {
      expect(feature.label).toBeTruthy();
      for (const {id} of tools) {
        expect(['yes', 'no', 'partial']).toContain(feature[id]);
      }
    }
  });
});
