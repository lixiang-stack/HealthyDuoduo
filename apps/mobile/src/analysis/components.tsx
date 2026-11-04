// 分析结果页的非路由展示组件(llm-analysis 票 04/08):本次范围卡、通用分组、
// 框架层标签与四段式解释;解释原文由服务端从条目拼接,App 只做排版。
import { useState, type ReactNode } from 'react';
import { Pressable, StyleSheet, Text, View } from 'react-native';

import type { AnalysisExplanation, AnalysisFramework, AnalysisResult } from '../api/analyses';
import { colors } from '../components/theme';
import { reportTypeLabel } from '../reports/category';
import { noteDate, scopeText } from './facts';
import { frameworkSourceText } from './framework';

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

export function FrameworkCard({ framework }: { framework: AnalysisFramework }) {
  if (framework.labels.length === 0) {
    return null;
  }
  return (
    <Section title="判读标签" testID="analysis-framework">
      {framework.labels.map((label) => (
        <View key={label.code} style={styles.labelRow}>
          <View style={styles.labelHeader}>
            <Text style={styles.labelTitle}>{label.label}</Text>
            {label.value ? <Text style={styles.labelValue}>{label.value}</Text> : null}
          </View>
          <Text style={styles.mutedSmall}>{label.explanation}</Text>
          {frameworkSourceText(label) ? (
            <Text style={styles.sourceText}>依据 {frameworkSourceText(label)}</Text>
          ) : null}
        </View>
      ))}
    </Section>
  );
}

export function ExplanationCard({ explanation }: { explanation: AnalysisExplanation }) {
  if (explanation.sections.length === 0) {
    return null;
  }
  return (
    <Section title="解读" testID="analysis-explanation">
      {explanation.sections.map((section) => (
        <ExplanationSectionCard key={section.entry_id} section={section} />
      ))}
      {explanation.truncated ? (
        <Text style={styles.mutedSmall}>解释内容达到长度上限，已截断。</Text>
      ) : null}
    </Section>
  );
}

function ExplanationSectionCard({
  section,
}: {
  section: AnalysisExplanation['sections'][number];
}) {
  const [showSources, setShowSources] = useState(false);
  const sources = section.source_ids ?? [];
  // 旧分析快照可能没有四段式字段;缺字段时只显示 bridge_text,不让结果页崩溃。
  const explanation = section.explanation ?? '';
  const checkDirections = section.check_directions ?? [];
  const generalAdvice = section.general_advice ?? [];
  const trackItems = section.track_items ?? [];
  return (
    <View testID={`analysis-explanation-section-${section.entry_id}`} style={styles.labelRow}>
      <Text style={styles.labelTitle}>{section.bridge_text}</Text>
      {explanation ? (
        <>
          <Text style={styles.partTitle}>可能原因</Text>
          <Text style={styles.bodyText}>{explanation}</Text>
        </>
      ) : null}
      <ExplanationLines title="进一步检查" lines={checkDirections} />
      <ExplanationLines title="生活建议" lines={generalAdvice} />
      {trackItems.length > 0 ? (
        <>
          <Text style={styles.partTitle}>长期跟踪</Text>
          <Text style={styles.bodyText}>建议持续记录：{trackItems.join('、')}</Text>
        </>
      ) : null}
      {sources.length > 0 ? (
        <Pressable
          testID={`analysis-explanation-sources-${section.entry_id}`}
          accessibilityRole="button"
          accessibilityState={{ expanded: showSources }}
          onPress={() => setShowSources((open) => !open)}
        >
          <Text style={styles.sourceToggle}>{showSources ? '收起依据' : '查看依据'}</Text>
        </Pressable>
      ) : null}
      {showSources ? (
        <View testID={`analysis-explanation-sources-detail-${section.entry_id}`}>
          {sources.map((source, index) => (
            <Text key={`${source.standard}-${index}`} style={styles.sourceText}>
              {source.standard}
              {source.note ? ` · ${source.note}` : ''}
              {source.url ? ` · ${source.url}` : ''}
            </Text>
          ))}
        </View>
      ) : null}
    </View>
  );
}

function ExplanationLines({ title, lines }: { title: string; lines: string[] }) {
  if (lines.length === 0) {
    return null;
  }
  return (
    <>
      <Text style={styles.partTitle}>{title}</Text>
      {lines.map((line) => (
        <Text key={line} style={styles.bodyText}>
          · {line}
        </Text>
      ))}
    </>
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
  partTitle: { fontSize: 13, color: colors.text, fontWeight: '600', marginTop: 4 },
  bodyText: { fontSize: 13, color: colors.text, lineHeight: 19 },
  sourceToggle: { fontSize: 12, color: colors.accent, marginTop: 4 },
  sourceText: { fontSize: 11, color: colors.muted, lineHeight: 16 },
});
