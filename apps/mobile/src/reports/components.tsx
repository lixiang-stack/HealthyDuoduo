// 时间线报告卡片(票 06):报告类别、日期、状态文案、按 flag 聚合的异常徽标与
// 「待核对 N 项」;failed 展示流水线产出的失败原因。详情入口属票 07。
import { StyleSheet, Text, View } from 'react-native';

import type { ReportSummary } from '../api/reports';
import { colors } from '../components/theme';
import { flagBadges, statusLabel, unreviewedBadge } from './status';

const STATUS_COLORS: Record<ReportSummary['status'], string> = {
  success: colors.ok,
  partial: colors.accent,
  failed: colors.fail,
};

export function ReportCard({ report }: { report: ReportSummary }) {
  const badges = flagBadges(report.flag_counts);
  const unreviewed = unreviewedBadge(report.unreviewed_low_confidence_count);
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
      {badges.length > 0 || unreviewed ? (
        <View style={styles.badges}>
          {badges.map((badge) => (
            <Text key={badge} style={styles.badge}>
              {badge}
            </Text>
          ))}
          {unreviewed ? <Text style={[styles.badge, styles.reviewBadge]}>{unreviewed}</Text> : null}
        </View>
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
});
