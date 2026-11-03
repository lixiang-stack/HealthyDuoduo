// 分析结果页的非路由展示组件(llm-analysis 票 04):本次范围卡与通用分组。
import type { ReactNode } from 'react';
import { StyleSheet, Text, View } from 'react-native';

import type { AnalysisResult } from '../api/analyses';
import { colors } from '../components/theme';
import { reportTypeLabel } from '../reports/category';
import { noteDate, scopeText } from './facts';

export function ScopeCard({ result }: { result: AnalysisResult }) {
  const facts = result.layers.facts;
  return (
    <View testID="analysis-scope" style={styles.card}>
      <Text style={styles.sectionTitle}>本次范围</Text>
      <Text style={styles.mutedSmall}>{scopeText(facts)}</Text>
      {facts.reports.map((report) => (
        <Text key={report.report_id} style={styles.mutedSmall}>
          {report.report_date ?? '日期未知'} · {reportTypeLabel(report.report_type)}
        </Text>
      ))}
      {facts.notes.map((note) => (
        <Text key={note.report_id} style={styles.mutedSmall}>
          病历 {noteDate(note.report_date)}
          {note.outside_window ? '（早于分析窗口）' : ''}
        </Text>
      ))}
    </View>
  );
}

export function Section({
  title,
  testID,
  children,
}: {
  title: string;
  testID: string;
  children: ReactNode;
}) {
  return (
    <View testID={testID} style={styles.section}>
      <Text style={styles.sectionTitle}>{title}</Text>
      {children}
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
  section: { gap: 8 },
  sectionTitle: { fontSize: 15, color: colors.text, fontWeight: '600', marginTop: 4 },
  mutedSmall: { fontSize: 12, color: colors.muted, lineHeight: 18 },
});
