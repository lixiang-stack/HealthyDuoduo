// 真实 OCR smoke:只用 1-2 张不在 golden 回放名单内的样张,断言粗粒度事实
// (上传成功、报告非空、类型/日期/段落合理),不驱动改归属/筛选/核对等 UI 行为。
// 详细识别质量由 Pipeline E2E/golden 覆盖,不在这里做。
import { test } from '@e2e-dev/web';
import { expect } from 'e2e';

import { createMember, uploadSample } from '../support';

test('真实 OCR smoke:检验单与影像单上传成功且报告结构合理', { timeout: 300_000 }, async () => {
  const member = await createMember('E2E OCR smoke', false);

  const lab = await uploadSample('cbc_01.jpeg', member.member_id);
  expect(lab.status).not.toBe('failed');
  expect(lab.report_type.length).toBeGreaterThan(0);
  expect(lab.items.length).toBeGreaterThan(0);
  expect(lab.report_date).toMatch(/^\d{4}-\d{2}-\d{2}$/);

  const imaging = await uploadSample('us_02.jpeg', member.member_id);
  expect(imaging.status).not.toBe('failed');
  expect(imaging.report_type.length).toBeGreaterThan(0);
  expect(imaging.items.length).toBe(0);
  expect((imaging.sections ?? []).length).toBeGreaterThan(0);
});
