import {
  CLINICAL_NOTE_REPORT_TYPE,
  IMAGING_REPORT_TYPE,
  isNarrativeReport,
  reportTypeLabel,
} from './category';

describe('isNarrativeReport', () => {
  it('wire 值固定为「影像」与 clinical_note,仅这两类按叙述报告呈现', () => {
    expect(IMAGING_REPORT_TYPE).toBe('影像');
    expect(CLINICAL_NOTE_REPORT_TYPE).toBe('clinical_note');
    expect(isNarrativeReport({ report_type: '影像' })).toBe(true);
    expect(isNarrativeReport({ report_type: 'clinical_note' })).toBe(true);
    expect(isNarrativeReport({ report_type: '临检' })).toBe(false);
    expect(isNarrativeReport({ report_type: '影像报告' })).toBe(false);
  });
});

describe('reportTypeLabel', () => {
  it('中文类别原样,clinical_note wire 值显示为「病历」', () => {
    expect(reportTypeLabel('血常规')).toBe('血常规');
    expect(reportTypeLabel('影像')).toBe('影像');
    expect(reportTypeLabel('clinical_note')).toBe('病历');
  });
});
