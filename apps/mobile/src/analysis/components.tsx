// 分析结果页的非路由展示组件(分析结果 UX 方案 A):指标分级、四段式解读、
// 来源说明弹层与病历归属。解释原文由服务端从审核条目拼接,App 只做排版;
// 来源只用人类可读名称 + 一个「打开指南原文」,不展示原始 URL。
import * as Linking from 'expo-linking';
import { Link } from 'expo-router';
import { useEffect, useRef, useState, type ReactNode } from 'react';
import {
  Modal,
  Pressable,
  ScrollView,
  StyleSheet,
  Text,
  View,
  type ScrollViewInstance,
} from 'react-native';

import type {
  AnalysisExplanation,
  AnalysisFramework,
  AnalysisFrameworkLabel,
  AnalysisNote,
  AnalysisSource,
} from '../api/analyses';
import { colors } from '../components/theme';
import { reportTypeLabel } from '../reports/category';
import {
  NOTE_ATTRIBUTION_WARNING,
  NOT_DIAGNOSIS_TEXT,
  humanizeClinicalText,
  noteDate,
  noteSectionAttributionLabel,
  quoteNoteText,
  sourceDisplay,
  sourceReferenceText,
  trackItemLabel,
} from './facts';
import { frameworkCoreLabels } from './framework';

export const MIN_TOUCH_TARGET = 44;

export function ReportLink({
  reportId,
  date,
  reportType,
  testID,
}: {
  reportId: string;
  date: string | null;
  reportType?: string;
  testID?: string;
}) {
  return (
    <Link
      asChild
      href={{ pathname: '/report/[report_id]', params: { report_id: reportId } }}
    >
      <Pressable
        testID={testID}
        accessibilityRole="link"
        accessibilityLabel={`查看 ${date ?? '日期未知'} 的报告`}
        style={styles.reportLinkTarget}
      >
        <Text style={styles.reportLink}>
          {date ?? '日期未知'}
          {reportType ? ` · ${reportTypeLabel(reportType)}` : ''} ›
        </Text>
      </Pressable>
    </Link>
  );
}

export function Section({
  title,
  testID,
  subtitle,
  children,
}: {
  title: string;
  testID: string;
  subtitle?: string;
  children: ReactNode;
}) {
  return (
    <View testID={testID} style={styles.section}>
      <Text style={styles.sectionTitle}>{title}</Text>
      {subtitle ? <Text style={styles.mutedSmall}>{subtitle}</Text> : null}
      {children}
    </View>
  );
}

export function FrameworkCard({ framework }: { framework: AnalysisFramework }) {
  const labels = frameworkCoreLabels(framework);
  if (labels.length === 0) {
    return null;
  }
  return (
    <Section title="指标分级" subtitle="按公开标准，不是诊断" testID="analysis-framework">
      {labels.map((label) => (
        <FrameworkLabelRow key={label.code} label={label} testID={`analysis-framework-${label.code}`} />
      ))}
    </Section>
  );
}

function FrameworkLabelRow({ label, testID }: { label: AnalysisFrameworkLabel; testID: string }) {
  return (
    <View testID={testID} style={styles.labelRow}>
      <View style={styles.labelHeader}>
        <Text style={styles.labelTitle}>{humanizeClinicalText(label.label)}</Text>
        {label.value ? (
          <Text style={styles.labelValue}>{humanizeClinicalText(label.value)}</Text>
        ) : null}
      </View>
      <Text style={styles.bodyText}>{humanizeClinicalText(label.explanation)}</Text>
      {label.sources.map((source, index) => (
        <Text key={`${source.standard}-${index}`} style={styles.sourceText}>
          参考：{sourceReferenceText(source)}
        </Text>
      ))}
    </View>
  );
}

export function ExplanationCard({
  explanation,
  coverage,
  sources,
  onShowSources,
  onFeedback,
}: {
  explanation: AnalysisExplanation;
  coverage?: { covered: number; total: number; uncoveredNames: string[] };
  sources: AnalysisSource[];
  onShowSources?: () => void;
  onFeedback?: () => void;
}) {
  if (explanation.sections.length === 0) {
    return null;
  }
  const causes = explanation.sections
    .map((section) => section.explanation)
    .filter((text): text is string => Boolean(text));
  const legacyBridges = explanation.sections
    .filter((section) => !section.explanation)
    .map((section) => section.bridge_text)
    .filter((text): text is string => Boolean(text));
  const checks = uniqueStrings(
    explanation.sections.flatMap((section) => section.check_directions ?? []),
  );
  const advice = uniqueStrings(
    explanation.sections.flatMap((section) => section.general_advice ?? []),
  );
  const trackItems = uniqueStrings(
    explanation.sections.flatMap((section) => section.track_items ?? []),
  );
  return (
    <Section title="本次解读" testID="analysis-explanation">
      <Text style={styles.disclaimer}>{NOT_DIAGNOSIS_TEXT}</Text>
      {coverage && coverage.total > 0 ? (
        <View testID="analysis-explanation-coverage" style={styles.coverage}>
          <Text style={styles.coverageTitle}>
            {coverage.total} 项异常里，本次解读解释了 {coverage.covered} 项
          </Text>
          {coverage.uncoveredNames.length > 0 ? (
            <Text style={styles.coverageSub}>
              另外 {coverage.uncoveredNames.length} 项（
              {coverage.uncoveredNames.join('、')}
              ）在日期范围内，但暂无对应的审核解读条目；请结合医生意见。
            </Text>
          ) : null}
        </View>
      ) : null}
      <View testID="analysis-explanation-summary" style={styles.labelRow}>
        {causes.length > 0 ? (
          <>
            <Text style={styles.partTitle}>可能原因</Text>
            {causes.map((text, index) => (
              <Text key={`cause-${index}`} style={styles.bodyText}>
                · {humanizeClinicalText(text)}
              </Text>
            ))}
          </>
        ) : null}
        {legacyBridges.map((text, index) => (
          <Text key={`legacy-${index}`} style={styles.legacyBridge}>
            {humanizeClinicalText(text)}
          </Text>
        ))}
        {checks.length > 0 ? (
          <>
            <Text style={styles.partTitle}>进一步检查</Text>
            {checks.map((line, index) => (
              <Text key={`check-${index}`} style={styles.bodyText}>
                · {humanizeClinicalText(line)}
              </Text>
            ))}
          </>
        ) : null}
        {advice.length > 0 ? (
          <>
            <Text style={styles.partTitle}>生活建议</Text>
            {advice.map((line, index) => (
              <Text key={`advice-${index}`} style={styles.bodyText}>
                · {humanizeClinicalText(line)}
              </Text>
            ))}
          </>
        ) : null}
        {trackItems.length > 0 ? (
          <>
            <Text style={styles.partTitle}>长期跟踪</Text>
            <Text style={styles.bodyText}>
              · 建议持续记录：{trackItems.map(trackItemLabel).join('、')}
            </Text>
          </>
        ) : null}
        {onFeedback ? (
          <Pressable
            testID="analysis-explanation-feedback"
            accessibilityRole="button"
            onPress={() => onFeedback()}
            style={styles.inlineToggle}
          >
            <Text style={styles.sourceToggle}>反馈这段内容</Text>
          </Pressable>
        ) : null}
      </View>
      {explanation.truncated ? (
        <Text testID="analysis-explanation-truncated" style={styles.mutedSmall}>
          解释内容达到长度上限，已截断；只影响解释层，事实与分级仍完整。
        </Text>
      ) : null}
      {sources.length > 0 ? (
        <View style={styles.sourceLine}>
          <Text style={styles.mutedSmall}>
            参考：{sourceReferenceText(sources[0])}
            {sources.length > 1 ? `等 ${sources.length} 份` : ''}
          </Text>
          {onShowSources ? (
            <Pressable
              testID="analysis-sources-open"
              accessibilityRole="button"
              onPress={onShowSources}
              style={styles.inlineToggle}
            >
              <Text style={styles.sourceToggle}>查看来源说明 ›</Text>
            </Pressable>
          ) : null}
        </View>
      ) : null}
    </Section>
  );
}

function uniqueStrings(values: string[]): string[] {
  return [...new Set(values.map((value) => value.trim()).filter(Boolean))];
}

export function SourceSheet({
  visible,
  sources,
  onClose,
}: {
  visible: boolean;
  sources: AnalysisSource[];
  onClose: () => void;
}) {
  const [error, setError] = useState<string>();
  const scrollRef = useRef<ScrollViewInstance>(null);
  useEffect(() => {
    if (visible) {
      scrollRef.current?.scrollTo({ y: 0, animated: false });
    }
  }, [visible]);
  const open = async (url: string | null) => {
    if (!url) {
      return;
    }
    setError(undefined);
    try {
      await Linking.openURL(url);
    } catch {
      setError('打开失败，请稍后重试。');
    }
  };
  return (
    <Modal visible={visible} transparent animationType="fade" onRequestClose={onClose}>
      <View style={styles.backdrop}>
        <Pressable style={StyleSheet.absoluteFill} onPress={onClose} />
        <View testID="analysis-source-sheet" style={styles.sheet}>
          <Text style={styles.sheetTitle}>这些参考来源是什么？</Text>
          <Text style={styles.mutedSmall}>
            把专业指南翻译成家长能看懂的话；指南不能替代医生。
          </Text>
          <ScrollView
            ref={scrollRef}
            testID="analysis-source-list"
            style={styles.sheetScroll}
            contentContainerStyle={styles.sheetScrollContent}
            showsVerticalScrollIndicator
          >
            {sources.map((source, index) => {
              const display = sourceDisplay(source);
              return (
                <View key={`${source.standard}-${index}`} style={styles.sourceItem}>
                  <Text style={styles.sourceTitle}>{display.title}</Text>
                  <Text style={styles.sourceOrg}>{display.org}</Text>
                  <Text style={styles.sourceDescription}>{display.description}</Text>
                  {source.url ? (
                    <Pressable
                      testID={`analysis-source-open-${index}`}
                      accessibilityRole="button"
                      onPress={() => void open(source.url)}
                      style={styles.sourceOpen}
                    >
                      <Text style={styles.sourceOpenText}>打开指南原文</Text>
                    </Pressable>
                  ) : null}
                </View>
              );
            })}
            {error ? <Text style={styles.sourceError}>{error}</Text> : null}
          </ScrollView>
          <Pressable
            testID="analysis-source-close"
            accessibilityRole="button"
            onPress={onClose}
            style={styles.sheetClose}
          >
            <Text style={styles.sheetCloseText}>关闭</Text>
          </Pressable>
        </View>
      </View>
    </Modal>
  );
}

export function NoteCard({ note }: { note: AnalysisNote }) {
  return (
    <View testID={`analysis-note-${note.report_id}`} style={styles.labelRow}>
      <View style={styles.rowHeader}>
        <ReportLink
          reportId={note.report_id}
          date={noteDate(note.report_date)}
          testID={`analysis-note-link-${note.report_id}`}
        />
        {note.outside_window ? <Text style={styles.outsideTag}>早于分析窗口</Text> : null}
      </View>
      <Text style={styles.noteWarning}>{NOTE_ATTRIBUTION_WARNING}</Text>
      {note.sections.map((section, index) => (
        <View key={`${note.report_id}-${index}`} style={styles.noteSection}>
          <Text style={styles.noteLabel}>{noteSectionAttributionLabel(section.kind)}</Text>
          <Text style={styles.noteText}>{quoteNoteText(section.text)}</Text>
        </View>
      ))}
    </View>
  );
}

const styles = StyleSheet.create({
  section: { gap: 8 },
  sectionTitle: { fontSize: 15, color: colors.text, fontWeight: '600', marginTop: 4 },
  mutedSmall: { fontSize: 12, color: colors.muted, lineHeight: 18 },
  reportLinkTarget: { minHeight: MIN_TOUCH_TARGET, justifyContent: 'center' },
  reportLink: { fontSize: 13, color: colors.accent, lineHeight: 20 },
  labelRow: {
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 8,
    padding: 12,
    gap: 4,
  },
  labelHeader: { gap: 2 },
  labelTitle: { fontSize: 15, color: colors.text, fontWeight: '600' },
  labelValue: { fontSize: 13, color: colors.text, fontVariant: ['tabular-nums'] },
  partTitle: { fontSize: 15, color: colors.text, fontWeight: '600', marginTop: 4 },
  bodyText: { fontSize: 14, color: colors.text, lineHeight: 21 },
  legacyBridge: { fontSize: 13, color: colors.muted, lineHeight: 20 },
  disclaimer: { fontSize: 12, color: colors.muted, lineHeight: 18 },
  coverage: {
    gap: 4,
    borderWidth: 1,
    borderColor: '#bfdbfe',
    backgroundColor: '#f8fbff',
    borderRadius: 10,
    padding: 12,
  },
  coverageTitle: { fontSize: 14, color: colors.text, fontWeight: '600', lineHeight: 20 },
  coverageSub: { fontSize: 12, color: colors.muted, lineHeight: 18 },
  sourceLine: { gap: 2, marginTop: 4 },
  sourceText: { fontSize: 12, color: colors.muted, lineHeight: 18, marginTop: 4 },
  sourceToggle: { fontSize: 13, color: colors.accent, marginTop: 4 },
  inlineToggle: { minHeight: MIN_TOUCH_TARGET, justifyContent: 'center' },
  noteWarning: { fontSize: 12, color: colors.fail, lineHeight: 18 },
  noteSection: { gap: 2, marginTop: 4 },
  noteLabel: { fontSize: 12, color: colors.muted },
  noteText: { fontSize: 14, color: colors.text, lineHeight: 21 },
  rowHeader: { flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', gap: 8 },
  outsideTag: { fontSize: 11, color: colors.muted },
  backdrop: {
    flex: 1,
    backgroundColor: colors.backdrop,
    justifyContent: 'center',
    paddingHorizontal: 20,
  },
  sheet: {
    backgroundColor: colors.background,
    borderRadius: 16,
    padding: 20,
    gap: 10,
    maxHeight: '85%',
  },
  sheetScroll: { flexShrink: 1 },
  sheetScrollContent: { gap: 10, paddingBottom: 4 },
  sheetTitle: { fontSize: 18, fontWeight: '600', color: colors.text },
  sourceItem: {
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 10,
    padding: 12,
    gap: 2,
  },
  sourceTitle: { fontSize: 14, color: colors.text, fontWeight: '600' },
  sourceOrg: { fontSize: 12, color: colors.muted },
  sourceDescription: { fontSize: 12, color: colors.text, lineHeight: 18, marginTop: 4 },
  sourceOpen: {
    alignSelf: 'flex-start',
    minHeight: 40,
    justifyContent: 'center',
    marginTop: 4,
  },
  sourceOpenText: { fontSize: 13, color: colors.accent },
  sourceError: { fontSize: 12, color: colors.fail },
  sheetClose: {
    minHeight: MIN_TOUCH_TARGET,
    alignItems: 'center',
    justifyContent: 'center',
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 10,
    marginTop: 4,
  },
  sheetCloseText: { fontSize: 15, color: colors.text },
});
