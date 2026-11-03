import {
  GROWTH_SOURCE_OPTIONS,
  growthCreateBody,
  growthFormErrors,
  growthSummary,
  todayISO,
  validMeasuredOn,
} from './growth';
import { makeGrowthRecord } from '../test/fixtures';

const NOW = new Date(2026, 10, 3);

describe('growth form rules', () => {
  it('only offers measured and report sources', () => {
    expect(GROWTH_SOURCE_OPTIONS.map((option) => option.value)).toEqual(['measured', 'report']);
  });

  it('accepts a valid date and rejects invalid or future dates', () => {
    expect(validMeasuredOn('2026-10-01', NOW)).toBe(true);
    expect(validMeasuredOn('2026-02-30', NOW)).toBe(false);
    expect(validMeasuredOn('2026-11-04', NOW)).toBe(false);
    expect(validMeasuredOn('not-a-date', NOW)).toBe(false);
  });

  it('requires at least one positive measurement and validates the filled one', () => {
    expect(
      growthFormErrors(
        { measuredOn: '2026-10-01', height: '', weight: '', source: 'measured' },
        NOW,
      ),
    ).toMatchObject({ measurement: '身高和体重至少填一项' });

    expect(
      growthFormErrors(
        { measuredOn: '2026-10-01', height: '0', weight: '', source: 'measured' },
        NOW,
      ),
    ).toMatchObject({ height: '身高需为正数' });

    expect(
      growthFormErrors(
        { measuredOn: '2026-10-01', height: '118.5', weight: '', source: 'measured' },
        NOW,
      ),
    ).toEqual({});
  });

  it('omits empty measurements from the request body', () => {
    expect(
      growthCreateBody({ measuredOn: '2026-10-01', height: '118.5', weight: '', source: 'report' }),
    ).toEqual({ measured_on: '2026-10-01', height_cm: 118.5, source: 'report' });
  });

  it('formats the latest growth summary with BMI and date', () => {
    expect(growthSummary(makeGrowthRecord())).toBe(
      '实测 · 身高 118.5 cm · 体重 22.4 kg · BMI 16.0 · 2026-10-01',
    );
    expect(
      growthSummary(makeGrowthRecord({ height_cm: 119, weight_kg: null, bmi: null, source: 'report' })),
    ).toBe('体检单 · 身高 119 cm · 2026-10-01');
  });

  it('defaults the measurement date to today', () => {
    expect(todayISO(NOW)).toBe('2026-11-03');
  });
});
