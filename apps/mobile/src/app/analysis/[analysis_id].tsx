import { Link, router, Stack, useFocusEffect, useLocalSearchParams } from 'expo-router';
import { useCallback, useRef, useState } from 'react';
import { ActivityIndicator, Pressable, ScrollView, StyleSheet, Text, View } from 'react-native';

import {
  createAnalysis,
  getAnalysis,
  submitAnalysisFeedback,
  type AnalysisResult,
  type AnalysisSource,
  type AnalysisWindowMonths,
} from '../../api/analyses';
import { resolveApiClient } from '../../api/resolveClient';
import {
  ExplanationCard,
  FrameworkCard,
  MIN_TOUCH_TARGET,
  NoteCard,
  ReportLink,
  Section,
  SourceSheet,
} from '../../analysis/components';
import {
  AnalysisFeedbackModal,
  type FeedbackReason,
} from '../../analysis/FeedbackModal';
import {
  explanationUnavailableNextStep,
  explanationUnavailableText,
  frameworkCoreLabels,
  hasFrameworkLabels,
} from '../../analysis/framework';
import { TimeRangeModal } from '../../analysis/TimeRangeModal';
import {
  INSUFFICIENT_EVIDENCE_TEXT,
  NO_DEVIATION_TEXT,
  NO_NOTES_TEXT,
  UNCOVERED_DEVIATION_TEXT,
  analysisCreatedAtText,
  deviationCoverage,
  evidenceFlag,
  evidenceRef,
  evidenceValue,
  entryNotice,
  indicatorLabels,
  windowText,
} from '../../analysis/facts';
import { ANALYSIS_SOURCE_STATEMENT } from '../../analysis/consent';
import { sharedStyles } from '../../components/styles';
import { colors } from '../../components/theme';
import { FLAG_COLORS } from '../../reports/colors';
import { panelLabel, type PanelCode } from '../../reports/panels';

type ScreenState =
  | { status: 'loading' }
  | { status: 'unconfigured' }
  | { status: 'error'; detail: string }
  | { status: 'ready'; result: AnalysisResult };

const CLINICAL_NOTE_RERUN_TEXT = '病历分析请回到原病历详情页重新发起。';

// 分析结果页(分析结果 UX 方案 A):只回答“这意味着什么、下一步做什么”。
// 历史趋势、全部指标与体格发育回到项目页 / 成员页;结果页只保留结论、指标分级、
// 异常摘要、医生记录与数据说明。
export default function AnalysisScreen() {
  const params = useLocalSearchParams();
  const analysisId = Number(
    typeof params.analysis_id === 'string' ? params.analysis_id : Number.NaN,
  );
  const [state, setState] = useState<ScreenState>({ status: 'loading' });
  const loadedAnalysisRef = useRef<number | undefined>(undefined);

  const load = useCallback(
    async (silent = false) => {
      if (!Number.isInteger(analysisId) || analysisId <= 0) {
        setState({ status: 'error', detail: '分析编号无效' });
        return;
      }
      if (!silent) {
        setState({ status: 'loading' });
      }
      const resolved = await resolveApiClient();
      if (!resolved) {
        setState({ status: 'unconfigured' });
        return;
      }
      try {
        setState({ status: 'ready', result: await getAnalysis(resolved.client, analysisId) });
      } catch (error) {
        setState({
          status: 'error',
          detail: error instanceof Error ? error.message : String(error),
        });
      }
    },
    [analysisId],
  );

  // 从报告详情返回时静默刷新,保留展开状态与滚动位置;首次进入才显示 loading。
  useFocusEffect(
    useCallback(() => {
      const silent = loadedAnalysisRef.current === analysisId;
      loadedAnalysisRef.current = analysisId;
      void load(silent);
    }, [analysisId, load]),
  );

  const [feedbackOpen, setFeedbackOpen] = useState(false);
  const [feedbackReason, setFeedbackReason] = useState<FeedbackReason | ''>('');
  const [feedbackEntryId, setFeedbackEntryId] = useState<string>();
  const [feedbackBusy, setFeedbackBusy] = useState(false);
  const [feedbackDone, setFeedbackDone] = useState(false);
  const [feedbackError, setFeedbackError] = useState<string>();
  const [sourceSheetOpen, setSourceSheetOpen] = useState(false);
  const [rangeOpen, setRangeOpen] = useState(false);
  const [creating, setCreating] = useState(false);
  const [createError, setCreateError] = useState<string>();

  const openFeedback = useCallback((entryId?: string) => {
    setFeedbackEntryId(entryId);
    setFeedbackReason('');
    setFeedbackError(undefined);
    setFeedbackOpen(true);
  }, []);

  const submitFeedback = useCallback(async () => {
    if (feedbackReason.trim().length === 0) {
      return;
    }
    setFeedbackBusy(true);
    setFeedbackError(undefined);
    const resolved = await resolveApiClient();
    if (!resolved) {
      setFeedbackBusy(false);
      setFeedbackError('尚未配置后端地址。');
      return;
    }
    try {
      await submitAnalysisFeedback(resolved.client, analysisId, {
        reason: feedbackReason.trim(),
        ...(feedbackEntryId ? { entry_id: feedbackEntryId } : {}),
      });
      setFeedbackDone(true);
      setFeedbackOpen(false);
      setFeedbackReason('');
      setFeedbackEntryId(undefined);
    } catch (error) {
      setFeedbackError(error instanceof Error ? error.message : String(error));
    } finally {
      setFeedbackBusy(false);
    }
  }, [analysisId, feedbackEntryId, feedbackReason]);

  const result = state.status === 'ready' ? state.result : undefined;
  const facts = result?.layers.facts;
  const framework = result?.layers.framework;
  const explanation = result?.layers.explanation;
  const hasExplanation = Boolean(explanation && explanation.sections.length > 0);
  const coverage =
    facts && hasExplanation ? deviationCoverage(explanation, facts.deviations) : undefined;
  const explanationUnavailable = result
    ? explanationUnavailableText(result.quota.explanation_reason, result.quota.explanation_available)
    : undefined;
  const explanationNextStep = result
    ? explanationUnavailableNextStep(
        result.quota.explanation_reason,
        result.quota.explanation_available,
      )
    : '';
  const remainingQuota = result
    ? Math.max(0, result.quota.limit_today - result.quota.used_today)
    : 0;
  const heading = facts ? `${panelLabel(facts.panel as PanelCode)}分析` : '分析结果';
  const nameLabels = facts
    ? indicatorLabels(facts.deviations.map((point) => point.name))
    : new Map<string, string>();
  const visibleSources = visibleAnalysisSources(result, framework, explanation);
  const oldWindowDates = facts
    ? [
        ...new Set([
          ...facts.reports
            .filter((report) => report.report_date && report.report_date < facts.window.from)
            .map((report) => report.report_date as string),
          ...facts.excluded
            .filter((item) => item.reason === 'outside_window' && item.report_date)
            .map((item) => item.report_date as string),
        ]),
      ].sort().reverse()
    : [];
  const inWindowReportCount = facts
    ? facts.reports.filter(
        (report) => !report.report_date || report.report_date >= facts.window.from,
      ).length
    : 0;

  // 重新分析沿用当前 panel / 入口 / 指标与用户选择的窗口;创建新的 analysis_id,
  // 不覆盖旧快照。病历入口缺少 report_sha 快照字段,先引导回原病历页。
  const createNewAnalysis = useCallback(
    async (months: AnalysisWindowMonths) => {
      if (!result || !facts) {
        return;
      }
      if (facts.entry === 'clinical_note') {
        setRangeOpen(false);
        setCreateError(CLINICAL_NOTE_RERUN_TEXT);
        return;
      }
      setCreating(true);
      setCreateError(undefined);
      const resolved = await resolveApiClient();
      if (!resolved) {
        setCreating(false);
        setCreateError('尚未配置后端地址，请先到设置填写。');
        return;
      }
      try {
        const next = await createAnalysis(resolved.client, {
          member_id: result.snapshot.member_id,
          panel: result.snapshot.panel,
          entry: facts.entry,
          ...(facts.indicator_name ? { indicator_name: facts.indicator_name } : {}),
          window_months: months,
        });
        setRangeOpen(false);
        router.push({
          pathname: '/analysis/[analysis_id]',
          params: { analysis_id: String(next.analysis_id) },
        });
      } catch (error) {
        setCreateError(error instanceof Error ? error.message : String(error));
      } finally {
        setCreating(false);
      }
    },
    [facts, result],
  );

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
            <View style={styles.header}>
              <Text testID="analysis-meta" style={sharedStyles.muted}>
                {result.snapshot.member_name} · {panelLabel(facts.panel as PanelCode)}
              </Text>
              <Text testID="analysis-window" style={sharedStyles.muted}>
                {windowText(facts)}
              </Text>
              {analysisCreatedAtText(result.created_at) ? (
                <Text testID="analysis-created-at" style={styles.mutedSmall}>
                  {analysisCreatedAtText(result.created_at)}
                </Text>
              ) : null}
              {entryNotice(facts) ? (
                <Text testID="analysis-entry-notice" style={styles.notice}>
                  {entryNotice(facts)}
                </Text>
              ) : null}
              {result.stale ? (
                <View testID="analysis-stale" style={styles.statusBlock}>
                  <Text style={styles.notice}>数据已更新，以下内容基于保存时的数据。</Text>
                  {facts.entry === 'clinical_note' ? (
                    <Text style={styles.mutedSmall}>{CLINICAL_NOTE_RERUN_TEXT}</Text>
                  ) : (
                    <Pressable
                      testID="analysis-rerun"
                      accessibilityRole="button"
                      disabled={creating}
                      onPress={() =>
                        void createNewAnalysis(facts.window.months as AnalysisWindowMonths)
                      }
                      style={styles.secondaryButton}
                    >
                      <Text style={styles.secondaryButtonText}>
                        {creating ? '正在创建新分析…' : '基于最新数据重新分析'}
                      </Text>
                    </Pressable>
                  )}
                </View>
              ) : null}
              {createError ? (
                <Text testID="analysis-create-error" style={styles.fail}>
                  {createError}
                </Text>
              ) : null}
              {facts.insufficient_evidence ? (
                <View testID="analysis-insufficient" style={styles.statusBlock}>
                  <Text style={styles.insufficient}>{INSUFFICIENT_EVIDENCE_TEXT}</Text>
                  <Link href="/timeline" style={styles.link}>
                    去核对报告
                  </Link>
                </View>
              ) : null}
            </View>

            {hasExplanation && explanation ? (
              <ExplanationCard
                explanation={explanation}
                coverage={coverage}
                sources={visibleSources}
                onShowSources={() => setSourceSheetOpen(true)}
                onFeedback={openFeedback}
              />
            ) : explanationUnavailable ? (
              <View testID="analysis-explanation-unavailable" style={styles.statusBlock}>
                <Text style={styles.sectionTitle}>本次解读</Text>
                <Text style={styles.notice}>{explanationUnavailable}</Text>
                {explanationNextStep ? (
                  <Text style={styles.mutedSmall}>{explanationNextStep}</Text>
                ) : null}
              </View>
            ) : null}

            {hasFrameworkLabels(framework) ? <FrameworkCard framework={framework} /> : null}

            {!facts.insufficient_evidence ? (
              <Section
                title={`异常 ${facts.deviations.length} 项`}
                subtitle="按每项最近一次结果"
                testID="analysis-deviations"
              >
                {facts.deviations.length === 0 ? (
                  <Text style={sharedStyles.muted}>{NO_DEVIATION_TEXT}</Text>
                ) : (
                  facts.deviations.map((point) => {
                    const covered = hasExplanation
                      ? coverage?.coveredIds.has(point.evidence_id)
                      : undefined;
                    return (
                      <View
                        key={point.evidence_id}
                        testID={`analysis-deviation-${point.evidence_id}`}
                        style={styles.row}
                      >
                        <View style={styles.rowHeader}>
                          <Text style={styles.rowName}>
                            {nameLabels.get(point.name) ?? point.name}
                          </Text>
                          <Text style={[styles.flag, { color: FLAG_COLORS[point.flag] }]}>
                            {evidenceFlag(point)}
                          </Text>
                        </View>
                        {covered !== undefined ? (
                          <Text
                            testID={`analysis-deviation-coverage-${point.evidence_id}`}
                            style={covered ? styles.coveredTag : styles.uncoveredTag}
                          >
                            {covered ? '已解释' : '暂无对应解读'}
                          </Text>
                        ) : null}
                        <View style={styles.rowBody}>
                          <Text style={styles.value}>{evidenceValue(point)}</Text>
                          <Text style={styles.mutedSmall}>{evidenceRef(point)}</Text>
                        </View>
                        <ReportLink
                          reportId={point.report_id}
                          date={point.report_date}
                          reportType={point.report_type}
                          testID={`analysis-deviation-report-${point.evidence_id}`}
                        />
                      </View>
                    );
                  })
                )}
                {hasExplanation && coverage && coverage.covered < coverage.total ? (
                  <Text testID="analysis-deviations-uncovered" style={styles.mutedSmall}>
                    {UNCOVERED_DEVIATION_TEXT}
                  </Text>
                ) : null}
              </Section>
            ) : null}

            <Section title={`相关病历 ${facts.notes.length} 份`} testID="analysis-notes">
              {facts.notes.length === 0 ? (
                <Text style={sharedStyles.muted}>{NO_NOTES_TEXT}</Text>
              ) : (
                facts.notes.map((note) => <NoteCard key={note.report_id} note={note} />)
              )}
            </Section>

            <View testID="analysis-data-note" style={styles.card}>
              <Text style={styles.sectionTitle}>数据说明</Text>
              <Text style={styles.mutedSmall}>
                本次判断使用 {facts.window.from} 之后的 {inWindowReportCount} 份报告
                {facts.notes.length > 0 ? `、${facts.notes.length} 份病历` : ''}。
              </Text>
              {oldWindowDates.length > 0 ? (
                <Text style={styles.mutedSmall}>
                  {oldWindowDates.join('、')} 的旧点只用于历史趋势，不参与本次异常判断。
                </Text>
              ) : null}
              <Text style={styles.mutedSmall}>
                历史趋势在「{panelLabel(facts.panel as PanelCode)}」项目页查看；体格发育（BMI 等）在成员页查看。
              </Text>
              {facts.entry === 'clinical_note' && facts.notes[0] ? (
                <Link
                  asChild
                  href={{
                    pathname: '/report/[report_id]',
                    params: { report_id: facts.notes[0].report_id },
                  }}
                >
                  <Pressable testID="analysis-panel-link" style={styles.dataLink}>
                    <Text style={styles.link}>查看病历原文 ›</Text>
                  </Pressable>
                </Link>
              ) : (
                <Link
                  asChild
                  href={{ pathname: '/panel/[panel]', params: { panel: facts.panel } }}
                >
                  <Pressable testID="analysis-panel-link" style={styles.dataLink}>
                    <Text style={styles.link}>
                      查看{panelLabel(facts.panel as PanelCode)}全部指标与趋势 ›
                    </Text>
                  </Pressable>
                </Link>
              )}
            </View>

            {result.quota.limit_today > 0 ? (
              <Text testID="analysis-quota-remaining" style={styles.mutedSmall}>
                今日剩余解读次数 {remainingQuota} 次
              </Text>
            ) : null}

            <View style={styles.actions}>
              {facts.entry !== 'clinical_note' ? (
                <Pressable
                  testID="analysis-range-open"
                  accessibilityRole="button"
                  disabled={creating}
                  onPress={() => {
                    setCreateError(undefined);
                    setRangeOpen(true);
                  }}
                  style={styles.secondaryButton}
                >
                  <Text style={styles.secondaryButtonText}>切换时间范围</Text>
                </Pressable>
              ) : null}
              <Pressable
                testID="analysis-feedback-open"
                accessibilityRole="button"
                onPress={() => openFeedback()}
                style={styles.secondaryButton}
              >
                <Text style={styles.secondaryButtonText}>反馈问题</Text>
              </Pressable>
            </View>
            {feedbackDone ? (
              <Text testID="analysis-feedback-done" style={styles.mutedSmall}>
                已记录，谢谢反馈。
              </Text>
            ) : null}

            <Text testID="analysis-source-statement" style={styles.statement}>
              {ANALYSIS_SOURCE_STATEMENT}
            </Text>

            <TimeRangeModal
              key={`range-${rangeOpen ? 'open' : 'closed'}-${facts.window.months}`}
              visible={rangeOpen}
              busy={creating}
              initialMonths={facts.window.months as AnalysisWindowMonths}
              onConfirm={(months) => void createNewAnalysis(months)}
              onCancel={() => setRangeOpen(false)}
            />
          </View>
        ) : null}
      </ScrollView>
      <SourceSheet
        visible={sourceSheetOpen}
        sources={visibleSources}
        onClose={() => setSourceSheetOpen(false)}
      />
      <AnalysisFeedbackModal
        visible={feedbackOpen}
        reason={feedbackReason}
        busy={feedbackBusy}
        error={feedbackError}
        entryId={feedbackEntryId}
        onChange={setFeedbackReason}
        onSubmit={() => void submitFeedback()}
        onClose={() => setFeedbackOpen(false)}
      />
    </>
  );
}

/** 只展示当前结果页可见分级 / 解读引用的来源;排除仅 BMI 使用的来源。 */
function visibleAnalysisSources(
  result: AnalysisResult | undefined,
  framework: AnalysisResult['layers']['framework'],
  explanation: AnalysisResult['layers']['explanation'],
): AnalysisSource[] {
  if (!result) {
    return [];
  }
  const standards = new Set<string>();
  if (framework) {
    for (const label of frameworkCoreLabels(framework)) {
      for (const source of label.sources) {
        standards.add(source.standard);
      }
    }
  }
  if (explanation) {
    for (const section of explanation.sections) {
      for (const source of section.source_ids ?? []) {
        standards.add(source.standard);
      }
    }
  }
  const candidates = standards.size > 0 ? result.sources.filter((source) => standards.has(source.standard)) : result.sources;
  const seen = new Set<string>();
  return candidates.filter((source) => {
    const key = `${source.standard}|${source.url ?? ''}`;
    if (seen.has(key)) {
      return false;
    }
    seen.add(key);
    return true;
  });
}

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: colors.background },
  content: { padding: 24, gap: 12, paddingBottom: 32 },
  stack: { gap: 12 },
  header: { gap: 6 },
  sectionTitle: { fontSize: 15, color: colors.text, fontWeight: '600', marginTop: 4 },
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
  flag: { fontSize: 13 },
  mutedSmall: { fontSize: 12, color: colors.muted, lineHeight: 18 },
  coveredTag: { fontSize: 12, color: colors.ok, lineHeight: 18 },
  uncoveredTag: { fontSize: 12, color: colors.muted, lineHeight: 18 },
  notice: { fontSize: 13, color: colors.accent, lineHeight: 20 },
  statusBlock: { gap: 4 },
  card: {
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 8,
    padding: 12,
    gap: 4,
  },
  dataLink: { minHeight: MIN_TOUCH_TARGET, justifyContent: 'center' },
  actions: { flexDirection: 'row', flexWrap: 'wrap', gap: 8 },
  secondaryButton: {
    minHeight: MIN_TOUCH_TARGET,
    justifyContent: 'center',
    alignSelf: 'flex-start',
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 8,
    paddingVertical: 8,
    paddingHorizontal: 14,
  },
  secondaryButtonText: { fontSize: 14, color: colors.text },
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
