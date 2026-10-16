import type { ReportDetail, UploadResult } from '../api/reports';

import { flagBadges, statusLabel, unreviewedBadge, uploadResultText, uploadWaitText } from './status';

describe('statusLabel', () => {
  it('状态文案映射固定:success→识别完成、partial→待核对、failed→识别失败', () => {
    expect(statusLabel('success')).toBe('识别完成');
    expect(statusLabel('partial')).toBe('待核对');
    expect(statusLabel('failed')).toBe('识别失败');
  });
});

describe('flagBadges', () => {
  it('按偏高/偏低/未知聚合;normal 不计、为 0 的档不出现', () => {
    expect(flagBadges({ high: 5, low: 2, unknown: 1 })).toEqual(['偏高 5', '偏低 2', '未知 1']);
    expect(flagBadges({ high: 0, low: 0, unknown: 0 })).toEqual([]);
    expect(flagBadges({ high: 3, low: 0, unknown: 2 })).toEqual(['偏高 3', '未知 2']);
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
