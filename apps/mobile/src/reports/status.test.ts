import type { ReportDetail, UploadResult } from '../api/reports';

import { IMAGING_REPORT_TYPE } from './category';
import {
  flagBadges,
  flagLabel,
  reportBadgeLabels,
  reportDateLabel,
  reportStatusLabel,
  unreviewedBadge,
  uploadResultText,
  uploadWaitText,
} from './status';

const report: ReportDetail = {
  report_id: 'a'.repeat(64),
  member_id: 1,
  report_type: '血常规',
  report_date: '2026-10-01',
  status: 'success',
  fail_reason: null,
  flag_counts: { high: 0, low: 0, unknown: 0 },
  unreviewed_low_confidence_count: 0,
  items: [],
};

const imaging = { ...report, report_type: IMAGING_REPORT_TYPE };

describe('reportStatusLabel', () => {
  it('检验单状态文案固定:success→识别完成、partial→待核对、failed→识别失败', () => {
    expect(reportStatusLabel({ ...report, status: 'success' })).toBe('识别完成');
    expect(reportStatusLabel({ ...report, status: 'partial' })).toBe('待核对');
    expect(reportStatusLabel({ ...report, status: 'failed' })).toBe('识别失败');
  });

  it('影像单不显示没有对应动作的「待核对」;success/failed 仍给出状态', () => {
    expect(reportStatusLabel({ ...imaging, status: 'partial' })).toBeUndefined();
    expect(reportStatusLabel({ ...imaging, status: 'success' })).toBe('识别完成');
    expect(reportStatusLabel({ ...imaging, status: 'failed' })).toBe('识别失败');
  });
});

describe('reportDateLabel', () => {
  it('检验单缺日期显示「日期未知」,有日期照常显示', () => {
    expect(reportDateLabel({ ...report, report_date: null })).toBe('日期未知');
    expect(reportDateLabel(report)).toBe('2026-10-01');
  });

  it('影像单缺日期显示「检查单未印日期」,有日期照常显示', () => {
    expect(reportDateLabel({ ...imaging, report_date: null })).toBe('检查单未印日期');
    expect(reportDateLabel({ ...imaging, report_date: '2024-09-15' })).toBe('2024-09-15');
  });
});

describe('flagLabel', () => {
  it('指标项 flag 文案:正常/偏高/偏低/未知(中性事实侧)', () => {
    expect(flagLabel('normal')).toBe('正常');
    expect(flagLabel('high')).toBe('偏高');
    expect(flagLabel('low')).toBe('偏低');
    expect(flagLabel('unknown')).toBe('未知');
  });
});

describe('flagBadges', () => {
  it('按偏高/偏低/未知聚合;normal 不计、为 0 的档不出现', () => {
    expect(flagBadges({ ...report, flag_counts: { high: 5, low: 2, unknown: 1 } })).toEqual([
      '偏高 5',
      '偏低 2',
      '未知 1',
    ]);
    expect(flagBadges({ ...report, flag_counts: { high: 0, low: 0, unknown: 0 } })).toEqual([]);
    expect(flagBadges({ ...report, flag_counts: { high: 3, low: 0, unknown: 2 } })).toEqual([
      '偏高 3',
      '未知 2',
    ]);
  });

  it('影像单按类别关闭未知档(指标项不呈现),偏高/偏低仍保留', () => {
    expect(
      flagBadges({ ...imaging, flag_counts: { high: 3, low: 1, unknown: 2 } }),
    ).toEqual(['偏高 3', '偏低 1']);
    expect(flagBadges({ ...imaging, flag_counts: { high: 0, low: 0, unknown: 2 } })).toEqual([]);
  });
});

describe('reportBadgeLabels', () => {
  it('影像单不出未知 / 待核对徽标;检验单保持原口径', () => {
    expect(
      reportBadgeLabels({
        ...imaging,
        flag_counts: { high: 1, low: 0, unknown: 6 },
        unreviewed_low_confidence_count: 1,
      }),
    ).toEqual({ flags: ['偏高 1'], unreviewed: undefined });
    expect(
      reportBadgeLabels({
        ...report,
        flag_counts: { high: 3, low: 0, unknown: 2 },
        unreviewed_low_confidence_count: 1,
      }),
    ).toEqual({ flags: ['偏高 3', '未知 2'], unreviewed: '待核对 1 项' });
  });
});

describe('unreviewedBadge', () => {
  it('「待核对 N 项」;0 不出徽标(failed 恒 0、success 免核对)', () => {
    expect(unreviewedBadge(3)).toBe('待核对 3 项');
    expect(unreviewedBadge(0)).toBeUndefined();
  });
});

describe('uploadWaitText', () => {
  it('秒级等待中性提示;超时阈值后提示确认后端与识别服务', () => {
    expect(uploadWaitText(0)).toBe('正在上传并识别(通常几秒)…');
    expect(uploadWaitText(9_999)).toBe('正在上传并识别(通常几秒)…');
    expect(uploadWaitText(10_000)).toContain('识别用时较长');
  });
});

describe('uploadResultText', () => {
  it('重复上传:明确已归档、未重复上传', () => {
    const result: UploadResult = { duplicate: true, report };
    expect(uploadResultText(result)).toBe('这张图已经归档过,未重复上传');
  });

  it('成功:轻提示已归档', () => {
    expect(uploadResultText({ duplicate: false, report })).toBe('识别完成,已归档');
  });

  it('待核对:带未核对低置信项数', () => {
    const partial = {
      ...report,
      status: 'partial' as const,
      unreviewed_low_confidence_count: 3,
    };
    expect(uploadResultText({ duplicate: false, report: partial })).toBe('已归档,有 3 项待核对');
  });

  it('识别失败:展示后端失败原因且说明已留痕', () => {
    const failed = { ...report, status: 'failed' as const, fail_reason: 'no metric items recognized' };
    expect(uploadResultText({ duplicate: false, report: failed })).toContain('no metric items recognized');
    expect(uploadResultText({ duplicate: false, report: failed })).toContain('已留痕');
  });
});
