import reportSchema from '../../../../schemas/report.schema.json';

import { PANEL_LABELS, PANEL_ORDER, panelLabel } from './panels';

describe('PANEL_ORDER', () => {
  it('覆盖全部 16 个 code,顺序固定(spec §5.3)', () => {
    expect(PANEL_ORDER).toEqual([
      'liver',
      'kidney',
      'lipid',
      'glucose',
      'uric_acid',
      'electrolyte',
      'vitanemia',
      'thyroid',
      'sex_hormone',
      'infection',
      'autoimmune',
      'coag',
      'cbc',
      'urinalysis',
      'imaging',
      'other',
    ]);
    expect(Object.keys(PANEL_LABELS).sort()).toEqual([...PANEL_ORDER].sort());
  });

  it('与识别端 schema 的枚举 / §5.3 顺序同源', () => {
    const panelSchema = reportSchema.$defs.panel;
    expect(panelSchema['x-panel-order']).toEqual(PANEL_ORDER);
    expect(Object.keys(PANEL_LABELS).sort()).toEqual([...panelSchema.enum].sort());
  });

  it('label 映射穷尽且可直接取用', () => {
    for (const code of PANEL_ORDER) {
      expect(panelLabel(code)).toBe(PANEL_LABELS[code]);
      expect(panelLabel(code)).not.toBe('');
    }
  });
});
