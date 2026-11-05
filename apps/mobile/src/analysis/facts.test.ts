import {
  analysisCreatedAtText,
  deviationCoverage,
  evidenceValue,
  humanizeClinicalText,
  indicatorDisplayName,
  indicatorLabels,
  noteSectionAttributionLabel,
  quoteNoteText,
  sourceDisplay,
  sourceReferenceText,
  trackItemLabel,
  windowText,
} from './facts';
import {
  makeAnalysisEvidencePoint,
  makeAnalysisFacts,
} from '../test/fixtures';

describe('analysis facts display helpers', () => {
  it('shows the chosen window and explains per-indicator sparse expansion', () => {
    expect(windowText(makeAnalysisFacts())).toBe('2025-11-03 至 2026-11-03（12 个月）');
    expect(
      windowText(
        makeAnalysisFacts({
          window: { months: 24, from: '2024-11-03', to: '2026-11-03', sparse: true },
        }),
      ),
    ).toBe(
      '2024-11-03 至 2026-11-03（24 个月）；部分指标历史数据较少，已按指标向前扩展参考，具体见各指标。',
    );
  });

  it('does not claim the whole window expanded when only some series are sparse', () => {
    expect(windowText(makeAnalysisFacts())).not.toMatch(/扩展参考/);
  });

  it('formats the analysis creation time and tolerates invalid values', () => {
    expect(analysisCreatedAtText('2026-11-05T06:20:00Z')).toMatch(
      /^生成于 \d{4}-\d{2}-\d{2} \d{2}:\d{2}$/,
    );
    expect(analysisCreatedAtText(undefined)).toBeUndefined();
    expect(analysisCreatedAtText('not-a-date')).toBeUndefined();
  });

  it('computes deviation coverage and the uncovered names', () => {
    const deviations = [
      makeAnalysisEvidencePoint({ evidence_id: 'ev-1', name: '丙氨酸氨基转移酶测定' }),
      makeAnalysisEvidencePoint({ evidence_id: 'ev-2', name: '天门冬氨酸氨基转移酶测定' }),
      makeAnalysisEvidencePoint({ evidence_id: 'ev-3', name: '总蛋白测定' }),
      makeAnalysisEvidencePoint({ evidence_id: 'ev-4', name: '白蛋白测定' }),
    ];
    const coverage = deviationCoverage(
      {
        sections: [
          {
            entry_id: 'entry',
            evidence_ids: ['ev-1', 'ev-2'],
            bridge_text: 'bridge',
            explanation: 'explanation',
            check_directions: [],
            general_advice: [],
            track_items: [],
          },
        ],
        truncated: false,
      },
      deviations,
    );
    expect(coverage).toMatchObject({ covered: 2, total: 4 });
    expect(coverage.uncoveredNames).toEqual(['总蛋白', '白蛋白']);
    expect(coverage.coveredIds.has('ev-1')).toBe(true);
    expect(coverage.coveredIds.has('ev-3')).toBe(false);
  });

  it('shows a full indicator name with its abbreviation and maps track items', () => {
    expect(indicatorDisplayName('丙氨酸氨基转移酶测定')).toBe('丙氨酸氨基转移酶（ALT）');
    expect(indicatorDisplayName('天门冬氨酸氨基转移酶测定')).toBe(
      '天门冬氨酸氨基转移酶（AST）',
    );
    expect(indicatorDisplayName('碱性磷酸酶测定')).toBe('碱性磷酸酶（ALP）');
    expect(indicatorDisplayName('总胆红素测定')).toBe('总胆红素（TBIL）');
    expect(indicatorDisplayName('白蛋白/球蛋白')).toBe('白蛋白/球蛋白');
    expect(trackItemLabel('ALT')).toBe('丙氨酸氨基转移酶（ALT）');
    expect(trackItemLabel('AST')).toBe('天门冬氨酸氨基转移酶（AST）');

    const labels = indicatorLabels([
      '丙氨酸氨基转移酶测定',
      '丙氨酸转氨酶测定',
      '天门冬氨酸氨基转移酶测定',
    ]);
    expect(labels.get('丙氨酸氨基转移酶测定')).toBe('丙氨酸氨基转移酶（ALT）');
    expect(labels.get('丙氨酸转氨酶测定')).toBe('ALT');
    expect(labels.get('天门冬氨酸氨基转移酶测定')).toBe(
      '天门冬氨酸氨基转移酶（AST）',
    );
  });

  it('does not confuse prealbumin with albumin', () => {
    expect(indicatorDisplayName('前白蛋白测定')).toBe('前白蛋白（PA）');
    expect(indicatorDisplayName('白蛋白测定')).toBe('白蛋白（ALB）');
  });

  it('drops dimensionless units and uses the display unit', () => {
    const ratio = makeAnalysisEvidencePoint({ value: '1.41', unit: '1', unit_display: null });
    const bilirubin = makeAnalysisEvidencePoint({
      value: '15',
      unit: 'umol/L',
      unit_display: 'μmol/L',
    });
    expect(evidenceValue(ratio)).toBe('1.41');
    expect(evidenceValue(bilirubin)).toBe('15 μmol/L');
  });

  it('turns internal time markers and ULN into user language', () => {
    expect(humanizeClinicalText('T-0 时 ALT 2.09×ULN，T-3m 时 2–5×ULN')).toBe(
      '最近一次 ALT 2.09 倍正常上限，约 3 个月前 2–5 倍正常上限',
    );
    expect(humanizeClinicalText('T-12m 时无明显变化')).toBe('约 12 个月前无明显变化');
    expect(humanizeClinicalText('总胆红素 15 umol/L')).toBe('总胆红素 15 μmol/L');
    expect(humanizeClinicalText('ALT 83.5 U/L')).toBe('ALT 83.5 U/L');
    expect(humanizeClinicalText('T-0 时 T-3m 时')).toBe('最近一次 约 3 个月前');
  });

  it('explains ACG / AASLD sources in plain language', () => {
    const acg = sourceDisplay({
      standard: 'ACG 2016',
      note: '转氨酶升高分级',
      url: 'https://example.com/acg',
    });
    expect(acg.title).toBe('转氨酶升高分级');
    expect(acg.org).toContain('美国胃肠病学会');
    expect(acg.description).toContain('2–5 倍为轻度');
    expect(sourceReferenceText({ standard: 'ACG 2016', note: '', url: null })).toBe(
      '转氨酶升高分级（ACG 2016）',
    );

    const aasld = sourceDisplay({ standard: 'AASLD', note: '', url: null });
    expect(aasld.title).toBe('AST/ALT 比值');
    expect(aasld.org).toContain('美国肝病学会');

    const duvall = sourceDisplay({ standard: 'Duvall 2023', note: '', url: null });
    expect(duvall.title).toBe('总蛋白与白蛋白解读');
    expect(duvall.description).toContain('总蛋白由白蛋白和球蛋白组成');

    const aspen = sourceDisplay({ standard: 'ASPEN 2021', note: '', url: null });
    expect(aspen.title).toBe('白蛋白不是营养标志');

    const kdigo = sourceDisplay({ standard: 'KDIGO 2021', note: '', url: null });
    expect(kdigo.title).toBe('蛋白尿与肾脏丢失');
  });

  it('attributes note sections to the doctor and the original visit', () => {
    expect(noteSectionAttributionLabel('diagnosis')).toBe('医生当时诊断（原文）');
    expect(noteSectionAttributionLabel('treatment')).toBe(
      '医生当时处理（原文，不是建议）',
    );
    expect(noteSectionAttributionLabel('note')).toBe('医生记录（原文）');
    expect(quoteNoteText('考虑缺铁性贫血')).toBe('「考虑缺铁性贫血」');
    expect(quoteNoteText('缺铁性贫血')).toBe('缺铁性贫血');
  });
});
