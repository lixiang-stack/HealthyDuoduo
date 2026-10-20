import { IMAGING_REPORT_TYPE, isImagingReport } from './category';

describe('isImagingReport', () => {
  it('wire 值固定为「影像」,仅该类别按影像报告呈现', () => {
    expect(IMAGING_REPORT_TYPE).toBe('影像');
    expect(isImagingReport({ report_type: '影像' })).toBe(true);
    expect(isImagingReport({ report_type: '临检' })).toBe(false);
    expect(isImagingReport({ report_type: '影像报告' })).toBe(false);
  });
});
