// Daily > stats: four metrics x three chart types over the last seven local days.
// It reads only the current child's parent-recorded care events; reminders live on
// today's page and are deliberately absent here.
import { router } from 'expo-router';
import { useState } from 'react';
import { ActivityIndicator, Pressable, StyleSheet, Text, View } from 'react-native';

import type { Member } from '../api/members';
import {
  CARE_METRICS,
  CARE_METRIC_CONFIG,
  CHART_TYPES,
  buildCareStats,
  type CareMetric,
  type ChartType,
} from '../care/careStats';
import { useStatsCareEvents } from '../care/useStatsCareEvents';
import { colors } from '../components/theme';
import { CHART_COPY, StatsChart } from './CareStatsChart';

export function StatsCareLog({ child }: { child: Member }) {
  const { state, reload } = useStatsCareEvents(child.member_id);
  const [metric, setMetric] = useState<CareMetric>('feeds');
  const [chart, setChart] = useState<ChartType>('bar');

  if (state.status === 'loading') {
    return (
      <View testID="stats-loading" style={styles.center}>
        <ActivityIndicator />
        <Text style={styles.muted}>正在加载统计…</Text>
      </View>
    );
  }

  if (state.status === 'error') {
    return (
      <View testID="stats-error" style={styles.center}>
        <Text style={styles.error}>护理记录加载失败:{state.detail}</Text>
        <Pressable
          testID="stats-retry"
          accessibilityRole="button"
          onPress={() => void reload()}
          style={styles.primary}
        >
          <Text style={styles.primaryText}>重试</Text>
        </Pressable>
      </View>
    );
  }

  const stats = buildCareStats(state.events, new Date());
  const current = stats[metric];

  return (
    <View testID="stats-care-log" style={styles.container}>
      <View style={styles.segmented}>
        {CARE_METRICS.map((value) => {
          const active = metric === value;
          return (
            <Pressable
              key={value}
              testID={`stats-metric-${value}`}
              accessibilityRole="button"
              accessibilityLabel={CARE_METRIC_CONFIG[value].label}
              accessibilityState={{ selected: active }}
              onPress={() => setMetric(value)}
              style={[styles.segment, active && styles.segmentActive]}
            >
              <Text style={active ? styles.segmentTextActive : styles.segmentText}>
                {CARE_METRIC_CONFIG[value].label}
              </Text>
            </Pressable>
          );
        })}
      </View>

      <View style={styles.segmented}>
        {CHART_TYPES.map((value) => {
          const active = chart === value;
          return (
            <Pressable
              key={value}
              testID={`stats-chart-tab-${value}`}
              accessibilityRole="button"
              accessibilityState={{ selected: active }}
              onPress={() => setChart(value)}
              style={[styles.segment, active && styles.segmentActive]}
            >
              <Text style={active ? styles.segmentTextActive : styles.segmentText}>
                {value === 'bar' ? '柱状' : value === 'stacked' ? '堆叠' : '折线'}
              </Text>
            </Pressable>
          );
        })}
      </View>

      <Text testID="stats-question" style={styles.question}>
        当前问题：{CHART_COPY[chart].question}
      </Text>

      {current.hasData ? (
        <>
          <View style={styles.kpis}>
            {current.kpis.map((kpi, index) => (
              <View key={kpi.label} testID={`stats-kpi-${index}`} style={styles.kpi}>
                <Text style={styles.kpiLabel}>{kpi.label}</Text>
                <Text style={styles.kpiValue}>
                  {kpi.value}
                  {kpi.unit ? <Text style={styles.kpiUnit}> {kpi.unit}</Text> : null}
                </Text>
              </View>
            ))}
          </View>
          <StatsChart metric={metric} chart={chart} stats={current} />
        </>
      ) : (
        <View testID="stats-empty" style={styles.empty}>
          <Text style={styles.emptyTitle}>还没有记录</Text>
          <Text style={styles.muted}>
            过去 7 天还没有{CARE_METRIC_CONFIG[metric].label}记录，图表不会用假数据补齐。
          </Text>
          <Pressable
            testID="stats-go-today"
            accessibilityRole="button"
            onPress={() => router.replace('/daily/today')}
            style={styles.primary}
          >
            <Text style={styles.primaryText}>去今天页</Text>
          </Pressable>
        </View>
      )}
    </View>
  );
}

const styles = StyleSheet.create({
  container: { gap: 8 },
  center: { gap: 8, paddingVertical: 20 },
  segmented: {
    flexDirection: 'row',
    gap: 4,
    padding: 3,
    borderRadius: 10,
    backgroundColor: colors.chip,
  },
  segment: { flex: 1, alignItems: 'center', paddingVertical: 7, borderRadius: 8 },
  segmentActive: {
    backgroundColor: colors.background,
    shadowColor: colors.text,
    shadowOpacity: 0.08,
    shadowRadius: 3,
    shadowOffset: { width: 0, height: 1 },
    elevation: 1,
  },
  segmentText: { fontSize: 11, color: colors.muted },
  segmentTextActive: { fontSize: 11, color: colors.text, fontWeight: '600' },
  question: { fontSize: 11, color: colors.muted, marginHorizontal: 2, marginBottom: 2 },
  kpis: { flexDirection: 'row', gap: 7 },
  kpi: {
    flex: 1,
    gap: 3,
    padding: 9,
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 10,
  },
  kpiLabel: { fontSize: 10, color: colors.muted },
  kpiValue: { fontSize: 18, color: colors.text, fontWeight: '600' },
  kpiUnit: { fontSize: 9, color: colors.muted, fontWeight: '400' },
  empty: { gap: 8, paddingVertical: 16 },
  emptyTitle: { fontSize: 16, color: colors.text, fontWeight: '600' },
  muted: { fontSize: 12, lineHeight: 18, color: colors.muted },
  error: { fontSize: 13, color: colors.fail },
  primary: {
    alignSelf: 'flex-start',
    paddingVertical: 9,
    paddingHorizontal: 14,
    borderRadius: 8,
    backgroundColor: colors.accent,
  },
  primaryText: { fontSize: 14, color: colors.onAccent },
});
