// 报告展示组件:时间线卡片(票 06)与详情指标项卡 + 参考范围条带(票 07)。
// 时间线卡片含类别、日期、状态文案、异常徽标与「待核对 N 项」;failed 展示失败原因。
// 详情指标项含 flag 染色与文案、条带(值点位)、低置信高亮、原始 OCR 行文面板。
import { StyleSheet, Text, View } from 'react-native';

import type { ReportItem, ReportSummary } from '../api/reports';
import { colors } from '../components/theme';
import { parseNumericForm, parseRefRange, stripLayout, type StripLayout } from './refRange';
import { flagBadges, flagLabel, statusLabel, unreviewedBadge } from './status';

// 报告状态色:时间线卡片与详情头部共用。
export const STATUS_COLORS: Record<ReportSummary['status'], string> = {
  success: colors.ok,
  partial: colors.accent,
  failed: colors.fail,
};

// flag 色板(票 07 实施期定,spec「色板实施期定」):normal 绿 / high 红 / low 蓝 /
// unknown 中性灰。只表示与参考范围的比较结果,不暗示诊断。
const FLAG_COLORS: Record<ReportItem['flag'], string> = {
  normal: colors.ok,
  high: colors.fail,
  low: colors.low,
  unknown: colors.muted,
};

// ReportBadges 异常徽标(偏高/偏低/未知)+「待核对 N 项」;时间线与详情头部共用。
export function ReportBadges({ report }: { report: ReportSummary }) {
  const badges = flagBadges(report.flag_counts);
  const unreviewed = unreviewedBadge(report.unreviewed_low_confidence_count);
  if (badges.length === 0 && !unreviewed) {
    return null;
  }
  return (
    <View style={styles.badges}>
      {badges.map((badge) => (
        <Text key={badge} style={styles.badge}>
          {badge}
        </Text>
      ))}
      {unreviewed ? <Text style={[styles.badge, styles.reviewBadge]}>{unreviewed}</Text> : null}
    </View>
  );
}

export function ReportCard({ report }: { report: ReportSummary }) {
  const failed = report.status === 'failed';
  return (
    <View style={styles.card}>
      <View style={styles.headerRow}>
        <Text style={styles.type}>{report.report_type}</Text>
        <Text style={[styles.status, { color: STATUS_COLORS[report.status] }]}>
          {statusLabel(report.status)}
        </Text>
      </View>
      <Text style={styles.date}>{report.report_date ?? '日期未知'}</Text>
      {failed && report.fail_reason ? <Text style={styles.fail}>{report.fail_reason}</Text> : null}
      <ReportBadges report={report} />
    </View>
  );
}

// MetricItem 详情指标项卡:名称、值/单位(flag 色)+ flag 文案、参考范围原文、
// 条带(参考范围 + 值位置)、原始 OCR 行文面板;low_confidence 高亮提示对照原文。
export function MetricItem({ item }: { item: ReportItem }) {
  const layout = stripLayout(parseRefRange(item.ref_range), parseNumericForm(item.value));
  const flagColor = FLAG_COLORS[item.flag];
  return (
    <View style={[styles.itemCard, item.low_confidence && styles.itemCardLowConfidence]}>
      <View style={styles.itemHeader}>
        <Text style={styles.itemName}>{item.name}</Text>
        {item.low_confidence ? <Text style={styles.lowTag}>低置信</Text> : null}
        <Text style={[styles.itemFlag, { color: flagColor }]}>{flagLabel(item.flag)}</Text>
      </View>
      <View style={styles.itemValueRow}>
        <Text style={[styles.itemValue, { color: flagColor }]}>
          {item.value ?? '—'}
          {item.unit ? <Text style={styles.itemUnit}> {item.unit}</Text> : null}
        </Text>
        {item.ref_range ? <Text style={styles.itemRef}>参考 {item.ref_range}</Text> : null}
      </View>
      {layout ? <MetricStrip layout={layout} color={flagColor} /> : null}
      <Text style={styles.itemRaw}>原文:{item.raw_text}</Text>
    </View>
  );
}

// MetricStrip 参考范围条带:条 = 正常区(中性灰;标量参考退化为细基准线),
// 点 = 当前值(flag 色;空心 = 带不等号的非精确值,置于条端外),
// 跨度条 = 区间值(spec:区间无点)。几何全部来自 stripLayout(纯逻辑)。
function MetricStrip({ layout, color }: { layout: StripLayout; color: string }) {
  const { band, marker } = layout;
  const scalarBand = band.start === band.end;
  return (
    <View style={styles.stripTrack}>
      <View
        style={[
          styles.stripBand,
          scalarBand && styles.stripBandLine,
          {
            left: `${band.start * 100}%`,
            width: scalarBand ? 2 : `${(band.end - band.start) * 100}%`,
          },
        ]}
      />
      {marker?.kind === 'span' ? (
        <View
          style={[
            styles.stripSpan,
            {
              left: `${marker.start * 100}%`,
              width: `${(marker.end - marker.start) * 100}%`,
              backgroundColor: color,
            },
          ]}
        />
      ) : null}
      {marker?.kind === 'dot' ? (
        <View
          style={[
            styles.stripDot,
            { left: `${marker.at * 100}%`, borderColor: color },
            marker.open ? styles.stripDotOpen : { backgroundColor: color },
          ]}
        />
      ) : null}
    </View>
  );
}

const styles = StyleSheet.create({
  card: {
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 8,
    padding: 12,
    gap: 4,
  },
  headerRow: { flexDirection: 'row', justifyContent: 'space-between', alignItems: 'center', gap: 8 },
  type: { fontSize: 16, color: colors.text },
  status: { fontSize: 13 },
  date: { fontSize: 13, color: colors.muted },
  fail: { fontSize: 13, color: colors.fail },
  badges: { flexDirection: 'row', flexWrap: 'wrap', gap: 6, marginTop: 4 },
  badge: {
    fontSize: 12,
    color: colors.text,
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 10,
    paddingHorizontal: 8,
    paddingVertical: 2,
  },
  reviewBadge: { borderColor: colors.accent, color: colors.accent },
  itemCard: {
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 8,
    padding: 12,
    gap: 6,
  },
  itemCardLowConfidence: { borderColor: colors.accent },
  itemHeader: { flexDirection: 'row', alignItems: 'center', gap: 8 },
  itemName: { flex: 1, fontSize: 15, color: colors.text },
  lowTag: {
    fontSize: 11,
    color: colors.accent,
    borderWidth: 1,
    borderColor: colors.accent,
    borderRadius: 8,
    paddingHorizontal: 6,
    paddingVertical: 1,
  },
  itemFlag: { fontSize: 13 },
  itemValueRow: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'baseline',
    gap: 8,
  },
  itemValue: { fontSize: 20, fontVariant: ['tabular-nums'] },
  itemUnit: { fontSize: 13 },
  itemRef: { fontSize: 13, color: colors.muted },
  stripTrack: {
    height: 12,
    borderRadius: 6,
    backgroundColor: colors.border,
    marginTop: 4,
  },
  stripBand: { position: 'absolute', top: 0, bottom: 0, backgroundColor: colors.muted, borderRadius: 6 },
  stripBandLine: { marginLeft: -1, borderRadius: 1 },
  stripSpan: { position: 'absolute', top: 4, height: 4, borderRadius: 2 },
  stripDot: {
    position: 'absolute',
    top: 1,
    width: 10,
    height: 10,
    borderRadius: 5,
    borderWidth: 2,
    marginLeft: -5,
  },
  stripDotOpen: { backgroundColor: colors.background },
  itemRaw: { fontSize: 12, color: colors.muted, lineHeight: 18 },
});
