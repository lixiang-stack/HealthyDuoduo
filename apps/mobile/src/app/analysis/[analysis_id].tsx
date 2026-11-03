import { Link, Stack, useFocusEffect, useLocalSearchParams } from 'expo-router';
import { useCallback, useState } from 'react';
import { ActivityIndicator, Pressable, ScrollView, StyleSheet, Text, View } from 'react-native';

import { getAnalysis, type AnalysisResult } from '../../api/analyses';
import { resolveApiClient } from '../../api/resolveClient';
import {
  INSUFFICIENT_EVIDENCE_TEXT,
  NO_DEVIATION_TEXT,
  NO_HISTORY_TEXT,
  NO_NOTES_TEXT,
  entryNotice,
  evidenceFlag,
  evidenceRef,
  evidenceValue,
  exclusionGroups,
  exclusionItemsText,
  historySummary,
  noteDate,
  noteSectionLabel,
  quoteNoteText,
  scopeText,
  windowText,
} from '../../analysis/facts';
import { ScopeCard, Section } from '../../analysis/components';
import { ANALYSIS_SOURCE_STATEMENT } from '../../analysis/consent';
import { sharedStyles } from '../../components/styles';
import { colors } from '../../components/theme';
import { reportTypeLabel } from '../../reports/category';
import { panelLabel, type PanelCode } from '../../reports/panels';

type ScreenState =
  | { status: 'loading' }
  | { status: 'unconfigured' }
  | { status: 'error'; detail: string }
  | { status: 'ready'; result: AnalysisResult };

// 分析结果页(llm-analysis 票 04):只渲染事实层——纳入范围、偏离项、与本人历史比较、
// 病历诊断与处理原文、未纳入原因;底部固定来源声明。框架层 / 解释层由后续工单追加,
// 当前结果里没有它们时页面不报错。
export default function AnalysisScreen() {
  const params = useLocalSearchParams();
  const analysisId = Number(
    typeof params.analysis_id === 'string' ? params.analysis_id : Number.NaN,
  );
  const [state, setState] = useState<ScreenState>({ status: 'loading' });

  const load = useCallback(async () => {
    if (!Number.isInteger(analysisId) || analysisId <= 0) {
      setState({ status: 'error', detail: '分析编号无效' });
      return;
    }
    setState({ status: 'loading' });
    const resolved = await resolveApiClient();
    if (!resolved) {
      setState({ status: 'unconfigured' });
      return;
    }
    try {
      setState({ status: 'ready', result: await getAnalysis(resolved.client, analysisId) });
    } catch (error) {
      setState({ status: 'error', detail: error instanceof Error ? error.message : String(error) });
    }
  }, [analysisId]);

  useFocusEffect(
    useCallback(() => {
      void load();
    }, [load]),
  );

  const result = state.status === 'ready' ? state.result : undefined;
  const facts = result?.layers.facts;
  const heading = facts ? `${panelLabel(facts.panel as PanelCode)}分析` : '分析结果';

  return (
    <>
      <Stack.Screen options={{ title: heading }} />
      <ScrollView
        testID="analysis-screen"
        style={styles.container}
        contentContainerStyle={styles.content}
      >
        {state.status === 'loading' ? (
          <View style={sharedStyles.horizontal}>
            <ActivityIndicator />
            <Text style={sharedStyles.muted}>正在加载分析…</Text>
          </View>
        ) : null}

        {state.status === 'unconfigured' ? (
          <Text style={sharedStyles.muted}>
            尚未配置后端地址，请先到
            <Link href="/settings" style={styles.link}>
              设置
            </Link>
            填写。
          </Text>
        ) : null}

        {state.status === 'error' ? (
          <View style={styles.stack}>
            <Text style={styles.fail}>加载失败:{state.detail}</Text>
            <Pressable style={styles.button} onPress={() => void load()}>
              <Text style={styles.buttonText}>重试</Text>
            </Pressable>
          </View>
        ) : null}

        {result && facts ? (
          <View style={styles.stack}>
            <Text testID="analysis-heading" style={styles.heading}>
              {heading}
            </Text>
            <Text style={sharedStyles.muted}>
              {result.snapshot.member_name} · {scopeText(facts)}
            </Text>
            <Text testID="analysis-window" style={sharedStyles.muted}>
              {windowText(facts)}
            </Text>
            {entryNotice(facts) ? (
              <Text testID="analysis-entry-notice" style={styles.notice}>
                {entryNotice(facts)}
              </Text>
            ) : null}
            {result.stale ? (
              <Text testID="analysis-stale" style={styles.notice}>
                数据已更新，以下内容基于保存时的数据。
              </Text>
            ) : null}

            {facts.insufficient_evidence ? (
              <Text testID="analysis-insufficient" style={styles.insufficient}>
                {INSUFFICIENT_EVIDENCE_TEXT}
              </Text>
            ) : null}

            <ScopeCard result={result} />

            {!facts.insufficient_evidence ? (
              <Section
                title={`偏离参考范围 ${facts.deviations.length} 项`}
                testID="analysis-deviations"
              >
                {facts.deviations.length === 0 ? (
                  <Text style={sharedStyles.muted}>{NO_DEVIATION_TEXT}</Text>
                ) : (
                  facts.deviations.map((point) => (
                    <View key={point.evidence_id} style={styles.row}>
                      <View style={styles.rowHeader}>
                        <Text style={styles.rowName}>{point.name}</Text>
                        <Text style={styles.flag}>{evidenceFlag(point)}</Text>
                      </View>
                      <View style={styles.rowBody}>
                        <Text style={styles.value}>{evidenceValue(point)}</Text>
                        <Text style={styles.mutedSmall}>{evidenceRef(point)}</Text>
                      </View>
                      <Text style={styles.mutedSmall}>
                        {point.report_date ?? '日期未知'} · {reportTypeLabel(point.report_type)}
                      </Text>
                    </View>
                  ))
                )}
              </Section>
            ) : null}

            <Section title={`与本人历史比较 ${facts.history.length} 项`} testID="analysis-history">
              {facts.history.length === 0 ? (
                <Text style={sharedStyles.muted}>{NO_HISTORY_TEXT}</Text>
              ) : (
                facts.history.map((history) => (
                  <View key={`${history.trend_key}-${history.unit ?? ''}`} style={styles.row}>
                    <View style={styles.rowHeader}>
                      <Text style={styles.rowName}>{history.name}</Text>
                      <Text style={styles.flag}>{history.point_count} 个有效点</Text>
                    </View>
                    <View style={styles.rowBody}>
                      <Text style={styles.value}>{evidenceValue(history.latest)}</Text>
                      <Text style={styles.mutedSmall}>{evidenceRef(history.latest)}</Text>
                    </View>
                    <Text style={styles.mutedSmall}>
                      {history.previous
                        ? `上次 ${history.previous.report_date ?? '日期未知'}：${evidenceValue(history.previous)}`
                        : '本次窗口内只有一个有效点'}
                    </Text>
                    <Text style={styles.mutedSmall}>{historySummary(history)}</Text>
                  </View>
                ))
              )}
            </Section>

            <Section title={`相关病历 ${facts.notes.length} 份`} testID="analysis-notes">
              {facts.notes.length === 0 ? (
                <Text style={sharedStyles.muted}>{NO_NOTES_TEXT}</Text>
              ) : (
                facts.notes.map((note) => (
                  <View key={note.report_id} style={styles.row}>
                    <View style={styles.rowHeader}>
                      <Text style={styles.rowName}>病历 {noteDate(note.report_date)}</Text>
                      {note.outside_window ? (
                        <Text style={styles.outsideTag}>早于分析窗口</Text>
                      ) : null}
                    </View>
                    {note.sections.map((section, index) => (
                      <View key={`${note.report_id}-${index}`} style={styles.noteSection}>
                        <Text style={styles.noteLabel}>{noteSectionLabel(section.kind)}</Text>
                        <Text style={styles.noteText}>{quoteNoteText(section.text)}</Text>
                      </View>
                    ))}
                  </View>
                ))
              )}
            </Section>

            {facts.uncomparable.length > 0 ? (
              <Section
                title={`无法与参考范围比较 ${facts.uncomparable.length} 项`}
                testID="analysis-uncomparable"
              >
                {facts.uncomparable.map((item) => (
                  <View key={`${item.trend_key}-${item.unit ?? ''}`} style={styles.row}>
                    <Text style={styles.rowName}>{item.name}</Text>
                    <Text style={styles.mutedSmall}>
                      {evidenceValue(item.latest)} · {item.latest.report_date ?? '日期未知'}
                    </Text>
                  </View>
                ))}
              </Section>
            ) : null}

            <Section title="未纳入项及原因" testID="analysis-excluded">
              {facts.excluded.length === 0 ? (
                <Text style={sharedStyles.muted}>本次没有排除项。</Text>
              ) : (
                <>
                  {exclusionGroups(facts.excluded).map((group) => (
                    <View key={group.reason} style={styles.exclusionRow}>
                      <Text style={styles.rowName}>{group.label}</Text>
                      <Text style={styles.mutedSmall}>{group.count} 项</Text>
                      {exclusionItemsText(group) ? (
                        <Text style={styles.mutedSmall}>{exclusionItemsText(group)}</Text>
                      ) : null}
                    </View>
                  ))}
                  <Text style={styles.mutedSmall}>
                    未纳入项不作为本次结论依据。
                  </Text>
                </>
              )}
            </Section>

            <Text testID="analysis-source-statement" style={styles.statement}>
              {ANALYSIS_SOURCE_STATEMENT}
            </Text>
          </View>
        ) : null}
      </ScrollView>
    </>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: colors.background },
  content: { padding: 24, gap: 12, paddingBottom: 32 },
  stack: { gap: 12 },
  heading: { fontSize: 22, color: colors.text, fontWeight: '600' },
  row: {
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 8,
    padding: 12,
    gap: 4,
  },
  rowHeader: { flexDirection: 'row', alignItems: 'baseline', justifyContent: 'space-between', gap: 8 },
  rowBody: { flexDirection: 'row', alignItems: 'baseline', justifyContent: 'space-between', gap: 8 },
  rowName: { flex: 1, fontSize: 15, color: colors.text },
  value: { fontSize: 20, color: colors.text, fontVariant: ['tabular-nums'] },
  flag: { fontSize: 13, color: colors.fail },
  mutedSmall: { fontSize: 12, color: colors.muted, lineHeight: 18 },
  noteSection: { gap: 2, marginTop: 4 },
  noteLabel: { fontSize: 12, color: colors.muted },
  noteText: { fontSize: 14, color: colors.text, lineHeight: 21 },
  outsideTag: { fontSize: 11, color: colors.muted },
  exclusionRow: { gap: 2 },
  notice: { fontSize: 13, color: colors.accent, lineHeight: 20 },
  insufficient: { fontSize: 15, color: colors.fail, fontWeight: '600' },
  statement: { fontSize: 12, color: colors.muted, lineHeight: 18, marginTop: 12 },
  fail: { fontSize: 13, color: colors.fail, lineHeight: 20 },
  link: { color: colors.accent, fontSize: 15 },
  button: {
    alignSelf: 'flex-start',
    paddingVertical: 10,
    paddingHorizontal: 16,
    backgroundColor: colors.accent,
    borderRadius: 8,
  },
  buttonText: { color: colors.onAccent, fontSize: 15 },
});
