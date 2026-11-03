import { exclusionGroups, exclusionItemsText, quoteNoteText, windowText } from './facts';
import { makeAnalysisFacts } from '../test/fixtures';

describe('analysis facts display helpers', () => {
  it('quotes doctor wording that could read as the system assertion', () => {
    expect(quoteNoteText('考虑缺铁性贫血')).toBe('「考虑缺铁性贫血」');
    expect(quoteNoteText('缺铁性贫血')).toBe('缺铁性贫血');
  });

  it('does not claim the whole window expanded when only some series are sparse', () => {
    expect(windowText(makeAnalysisFacts())).toBe('2025-11-03 至 2026-11-03');
    expect(
      windowText(
        makeAnalysisFacts({
          window: { months: 12, from: '2025-11-03', to: '2026-11-03', sparse: true },
        }),
      ),
    ).toBe('2025-11-03 至 2026-11-03（部分指标历史数据较少）');
  });

  it('groups exclusions by reason and lists up to three names', () => {
    const groups = exclusionGroups([
      { report_id: 'a', report_date: '2026-10-01', trend_key: 'ALT', name: '丙氨酸转氨酶', reason: 'unverified_unit' },
      { report_id: 'b', report_date: '2026-10-02', trend_key: 'ALT', name: '天门冬氨酸转氨酶', reason: 'unverified_unit' },
      { report_id: 'c', report_date: '2026-10-03', trend_key: null, name: null, reason: 'outside_window' },
    ]);
    expect(groups[0]).toMatchObject({ reason: 'unverified_unit', count: 2 });
    expect(exclusionItemsText(groups[0])).toBe('丙氨酸转氨酶、天门冬氨酸转氨酶');
    expect(groups[1]).toMatchObject({ reason: 'outside_window', count: 1 });
  });
});
