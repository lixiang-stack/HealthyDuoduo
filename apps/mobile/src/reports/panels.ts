// 项目组展示口径(panel-browse 票 02):code 与中文 label 的固定映射。
// code 由识别端 / OpenAPI 契约给出;此处只做展示,不做分类推导。
import type { components } from '../api/schema';

export type PanelCode = components['schemas']['PanelCode'];

/** 固定优先级(spec §5.3):只在项目列表同日期并列时裁决,不是医学优先级;other 兜底。
 *  唯一来源是 schemas/report.schema.json $defs.panel.x-panel-order;测试锁定三端同源。 */
export const PANEL_ORDER: PanelCode[] = [
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
];

export const PANEL_LABELS: Record<PanelCode, string> = {
  cbc: '血常规',
  urinalysis: '尿常规',
  liver: '肝功',
  kidney: '肾功',
  lipid: '血脂',
  glucose: '血糖',
  thyroid: '甲状腺',
  sex_hormone: '性激素',
  uric_acid: '尿酸',
  electrolyte: '电解质',
  vitanemia: '维生素/贫血',
  infection: '感染筛查',
  autoimmune: '自身抗体',
  coag: '凝血',
  imaging: '影像',
  other: '其他',
};

export function panelLabel(code: PanelCode): string {
  return PANEL_LABELS[code];
}
