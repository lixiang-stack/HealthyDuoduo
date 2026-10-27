import type { ReportDetail, ReportSummary } from '../api/reports';

import { applyReassignResult, needsReassign, reassignConfirmText } from './manage';

// 真实形状(与 items.test 同源样例报告形状)。
function summary(overrides: Partial<ReportSummary> = {}): ReportSummary {
  return {
    report_id: 'a'.repeat(64),
    member_id: 2,
    report_type: '血常规',
    report_date: '2026-10-01',
    status: 'partial',
    fail_reason: null,
    flag_counts: { high: 1, low: 0, abnormal: 0, unknown: 0 },
    panels: ['cbc'],
    not_tested_count: 0,
    unreviewed_low_confidence_count: 1,
    ...overrides,
  };
}

function detail(overrides: Partial<ReportDetail> = {}): ReportDetail {
  return { ...summary({ member_id: 1 }), items: [], ...overrides };
}

describe('applyReassignResult', () => {
  it('只并入归属:状态/计数/指标项保持本地现值,不改动入参', () => {
    const before = detail({ member_id: 1 });
    const after = applyReassignResult(before, summary({ member_id: 2 }));
    expect(after).not.toBe(before);
    expect(after.member_id).toBe(2);
    expect(after.items).toBe(before.items);
    expect(after.status).toBe(before.status);
    expect(after.unreviewed_low_confidence_count).toBe(before.unreviewed_low_confidence_count);
    expect(before.member_id).toBe(1);
  });

  it('无主报告(null)改归属后归属生效', () => {
    const after = applyReassignResult(detail({ member_id: null }), summary({ member_id: 3 }));
    expect(after.member_id).toBe(3);
  });
});

describe('needsReassign', () => {
  it('目标成员与当前归属相同 → 保持原样(不产生请求)', () => {
    expect(needsReassign(2, 2)).toBe(false);
  });

  it('目标成员不同 → 需要改派', () => {
    expect(needsReassign(1, 2)).toBe(true);
  });

  it('无主报告(null)点选任何成员都要改派', () => {
    expect(needsReassign(null, 1)).toBe(true);
    expect(needsReassign(null, 2)).toBe(true);
  });
});

describe('reassignConfirmText', () => {
  it('显式给出 from→to', () => {
    expect(reassignConfirmText('小虎', '妈妈')).toBe('把报告从「小虎」改派给「妈妈」?');
  });

  it('无主报告只给目标侧', () => {
    expect(reassignConfirmText(null, '妈妈')).toBe('把该报告改派给「妈妈」?');
  });
});
