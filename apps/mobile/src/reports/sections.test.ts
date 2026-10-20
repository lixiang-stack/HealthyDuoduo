import type { ReportSection } from '../api/reports';

import { examNameText, reportSectionCards } from './sections';

// 文本与行框取自 samples/expected 的 us_01 真实 OCR 预期(节选逐字一致)。
const findings: ReportSection = {
  kind: 'findings',
  text: '经腹扫查：\n子宫颈管长约36mm。',
  ocr_lines: [
    {
      line: 18,
      box: [
        [6.0, 304.0],
        [73.0, 302.0],
        [74.0, 323.0],
        [7.0, 325.0],
      ],
    },
    {
      line: 19,
      box: [
        [8.0, 319.0],
        [133.0, 316.0],
        [133.0, 336.0],
        [9.0, 339.0],
      ],
    },
  ],
};

const impression: ReportSection = {
  kind: 'impression',
  text: '宫内单活胎，胎儿超声测值评估约孕12W+3d大小。',
  ocr_lines: [
    {
      line: 27,
      box: [
        [12.0, 480.0],
        [313.0, 475.0],
        [313.0, 495.0],
        [13.0, 499.0],
      ],
    },
  ],
};

const examName: ReportSection = {
  kind: 'exam_name',
  text: '筛查(自费)、11-14周NT筛查(生育)',
  ocr_lines: [
    {
      line: 16,
      box: [
        [404.0, 91.0],
        [609.0, 96.0],
        [609.0, 118.0],
        [403.0, 112.0],
      ],
    },
  ],
};

describe('examNameText', () => {
  it('同类多次出现按数组序换行并列;缺省/null/不存在返回 undefined', () => {
    const second = { ...examName, text: '第二个检查项目' };
    expect(examNameText([examName, findings, second])).toBe(
      `${examName.text}\n${second.text}`,
    );
    expect(examNameText([findings, impression])).toBeUndefined();
    expect(examNameText(undefined)).toBeUndefined();
    expect(examNameText(null)).toBeUndefined();
  });
});

describe('reportSectionCards', () => {
  it('检查项目归入头部,其余已知 kind 按数组序成卡并给出标签', () => {
    const cards = reportSectionCards([examName, findings, impression]);
    expect(cards.map((card) => [card.index, card.label, card.section])).toEqual([
      [1, '检查所见', findings],
      [2, '超声提示', impression],
    ]);
  });

  it('未知 kind 不渲染;同类重复按数组序保留(下标作唯一键)', () => {
    const unknown: ReportSection = { kind: 'other', text: '不渲染', ocr_lines: [] };
    const cards = reportSectionCards([findings, unknown, findings]);
    expect(cards.map((card) => [card.index, card.section])).toEqual([
      [0, findings],
      [2, findings],
    ]);
  });

  it('无段落返回空数组', () => {
    expect(reportSectionCards(undefined)).toEqual([]);
    expect(reportSectionCards(null)).toEqual([]);
    expect(reportSectionCards([])).toEqual([]);
  });
});
