// 档案按时间线展示:竖线 + 每天一个点 + 横线连到当天卡片;
// 同一天多份报告共用一个点,卡片纵向排列;日期未知的报告沉底,不画点。
import { Link } from 'expo-router';
import { Pressable, StyleSheet, Text, View } from 'react-native';

import type { ReportSummary } from '../api/reports';
import { colors } from '../components/theme';
import { ReportCard } from './components';
import {
  dayOfMonthLabel,
  groupReportsByDay,
  weekdayLabel,
  type TimelineDay,
} from './timeline';

const DATE_COLUMN_WIDTH = 52;
const RAIL_COLUMN_WIDTH = 16;
const RAIL_CENTER = DATE_COLUMN_WIDTH + RAIL_COLUMN_WIDTH / 2;
const DOT_TOP = 14;

export function ReportTimeline({ reports }: { reports: ReportSummary[] }) {
  const { months, undated } = groupReportsByDay(reports);
  return (
    <View testID="report-timeline" style={styles.timeline}>
      {months.length > 0 ? (
        <View style={styles.dated}>
          <View pointerEvents="none" style={styles.rail} />
          {months.map((month) => (
            <View key={month.key} testID={`timeline-month-${month.key}`}>
              <View style={styles.monthRow}>
                <View style={styles.dateColumn}>
                  <Text style={styles.year}>{month.year}</Text>
                  <Text style={styles.month}>{month.month} 月</Text>
                </View>
                <View style={styles.railColumn} />
                <View style={styles.cardsColumn} />
              </View>
              {month.days.map((day) => (
                <DayRow key={day.date} day={day} />
              ))}
            </View>
          ))}
        </View>
      ) : null}

      {undated.length > 0 ? (
        <View testID="timeline-undated" style={styles.undated}>
          <Text style={styles.undatedTitle}>日期未知</Text>
          {undated.map((report) => (
            <ReportLink key={report.report_id} report={report} />
          ))}
        </View>
      ) : null}
    </View>
  );
}

function DayRow({ day }: { day: TimelineDay }) {
  return (
    <View testID={`timeline-day-${day.date}`} style={styles.dayRow}>
      <View style={styles.dateColumn}>
        <Text style={styles.day}>{dayOfMonthLabel(day.date)}</Text>
        <Text style={styles.weekday}>{weekdayLabel(day.date)}</Text>
      </View>
      <View style={styles.railColumn}>
        <View style={styles.dot} />
        <View style={styles.branch} />
      </View>
      <View style={styles.cardsColumn}>
        {day.reports.map((report) => (
          <ReportLink key={report.report_id} report={report} />
        ))}
      </View>
    </View>
  );
}

function ReportLink({ report }: { report: ReportSummary }) {
  return (
    <Link
      asChild
      href={{
        pathname: '/report/[report_id]',
        params: { report_id: report.report_id },
      }}
    >
      <Pressable>
        <ReportCard report={report} />
      </Pressable>
    </Link>
  );
}

const styles = StyleSheet.create({
  timeline: { gap: 0 },
  dated: { position: 'relative' },
  rail: {
    position: 'absolute',
    left: RAIL_CENTER,
    top: 0,
    bottom: 0,
    width: 1,
    backgroundColor: colors.border,
  },
  monthRow: {
    flexDirection: 'row',
    alignItems: 'flex-start',
    paddingTop: 4,
    paddingBottom: 8,
  },
  dateColumn: { width: DATE_COLUMN_WIDTH },
  railColumn: { width: RAIL_COLUMN_WIDTH, position: 'relative', alignItems: 'center' },
  cardsColumn: { flex: 1, gap: 8 },
  year: { fontSize: 11, color: colors.muted, fontWeight: '600' },
  month: { fontSize: 13, color: colors.text, fontWeight: '600' },
  dayRow: {
    flexDirection: 'row',
    alignItems: 'flex-start',
    paddingBottom: 14,
  },
  day: {
    fontSize: 18,
    color: colors.text,
    fontWeight: '600',
    fontVariant: ['tabular-nums'],
    paddingTop: 8,
  },
  weekday: { fontSize: 11, color: colors.muted, paddingTop: 1 },
  dot: {
    width: 9,
    height: 9,
    borderRadius: 4.5,
    marginTop: DOT_TOP,
    backgroundColor: colors.accent,
  },
  branch: {
    position: 'absolute',
    left: RAIL_COLUMN_WIDTH / 2,
    top: DOT_TOP + 4,
    width: RAIL_COLUMN_WIDTH / 2,
    height: 1,
    backgroundColor: colors.border,
  },
  undated: { marginTop: 18, gap: 8 },
  undatedTitle: { fontSize: 13, color: colors.muted, fontWeight: '600' },
});
