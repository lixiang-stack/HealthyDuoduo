// flag 色板(详情/核对/趋势共用):normal 绿 / high 红 / low 蓝 / abnormal 红 /
// unknown 中性灰。只表示与参考范围的比较结果,不暗示诊断。
import type { ReportItem } from '../api/reports';
import { colors } from '../components/theme';

export const FLAG_COLORS: Record<ReportItem['flag'], string> = {
  normal: colors.ok,
  high: colors.fail,
  low: colors.low,
  abnormal: colors.fail,
  unknown: colors.muted,
  not_tested: colors.muted,
};
