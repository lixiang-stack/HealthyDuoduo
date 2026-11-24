// Today's care-log page content: hero, four quick actions, today summary and the
// current child's timeline. Reminders (ticket 06) and charts (ticket 07) are not here.
import { useState } from 'react';
import { ActivityIndicator, Pressable, StyleSheet, Text, View } from 'react-native';

import type { Member } from '../api/members';
import { CARE_EVENT_TYPES, type CareEvent, type CareEventType } from '../care/careEvents';
import {
  CARE_EVENT_LABELS,
  CARE_EVENT_SOURCE_LABEL,
  careEventMeta,
  careEventTitle,
  elapsedMinutes,
  feedSummary,
  formatElapsedMinutes,
  localClockTime,
  summarizeToday,
} from '../care/careEventDisplay';
import { useTodayCareEvents } from '../care/useTodayCareEvents';
import { colors } from '../components/theme';
import { RecordSheet } from './RecordSheet';

type SheetState = { type: CareEventType; editingEvent?: CareEvent };

export function TodayCareLog({ child, members }: { child: Member; members: Member[] }) {
  const { state, reload } = useTodayCareEvents(child.member_id);
  const [sheet, setSheet] = useState<SheetState>();
  const now = new Date();
  const events = state.status === 'ready' ? state.events : [];
  const lastFeed = state.status === 'ready' ? state.lastFeed : undefined;
  const lastFeedDetail = lastFeed?.type === 'feed' ? feedSummary(lastFeed) : null;
  const summary = summarizeToday(events, now);
  const loading = state.status === 'loading';
  const loadFailed = state.status === 'error';

  const closeAndReload = () => {
    setSheet(undefined);
    void reload();
  };

  return (
    <View testID="today-care-log" style={styles.container}>
      <View testID="today-hero" style={styles.hero}>
        <Text testID="today-hero-eyebrow" style={styles.heroEyebrow}>
          {child.name} · 距上次喂养
        </Text>
        <Text testID="today-last-feed" style={styles.heroValue}>
          {loading
            ? '…'
            : loadFailed
              ? '—'
              : lastFeed
                ? formatElapsedMinutes(elapsedMinutes(lastFeed.started_at, now))
                : '—'}
        </Text>
        <Text testID="today-last-feed-detail" style={styles.heroSub}>
          {loading
            ? '正在加载今天记录…'
            : loadFailed
              ? '护理记录加载失败'
              : lastFeed && lastFeedDetail
                ? `上次 ${localClockTime(lastFeed.started_at)} · ${lastFeedDetail}`
                : '今天还没有喂养记录'}
        </Text>
        <View style={styles.heroFoot}>
          <Text style={styles.heroFootText}>以家长记录为准</Text>
          <Text style={styles.tag}>{CARE_EVENT_SOURCE_LABEL}</Text>
        </View>
      </View>

      <View style={styles.quickGrid}>
        {CARE_EVENT_TYPES.map((type) => (
          <Pressable
            key={type}
            testID={`today-quick-${type}`}
            accessibilityRole="button"
            accessibilityLabel={`记录${CARE_EVENT_LABELS[type]}`}
            onPress={() => setSheet({ type })}
            style={styles.quick}
          >
            <Text style={styles.quickText}>{CARE_EVENT_LABELS[type]}</Text>
          </Pressable>
        ))}
      </View>

      {!loadFailed ? (
        <View testID="today-summary" style={styles.summaryStrip}>
          <SummaryCell
            testID="today-summary-feeds"
            value={loading ? '…' : summary.feeds}
            label="喂养"
          />
          <SummaryCell
            testID="today-summary-amount"
            value={loading ? '…' : summary.amountMl}
            label="奶量 ml"
          />
          <SummaryCell
            testID="today-summary-sleep"
            value={loading ? '…' : summary.sleepMinutes}
            label="睡眠 min"
          />
          <SummaryCell
            testID="today-summary-diapers"
            value={loading ? '…' : summary.diapers}
            label="尿布"
          />
        </View>
      ) : null}

      <View style={styles.sectionRow}>
        <Text style={styles.sectionTitle}>今日时间线</Text>
        <Text style={styles.muted}>{events.length} 条</Text>
      </View>

      {loading ? <ActivityIndicator testID="today-loading" /> : null}
      {state.status === 'error' ? (
        <View style={styles.stack}>
          <Text testID="today-error" style={styles.error}>
            护理记录加载失败:{state.detail}
          </Text>
          <Pressable style={styles.retry} onPress={() => void reload()}>
            <Text style={styles.retryText}>重试</Text>
          </Pressable>
        </View>
      ) : null}
      {state.status === 'ready' ? (
        events.length === 0 ? (
          <Text testID="today-empty" style={styles.muted}>
            今天还没有记录，从上面的快捷动作开始。
          </Text>
        ) : (
          events.map((event) => (
            <EventRow
              key={event.id}
              event={event}
              onPress={() => setSheet({ type: event.type, editingEvent: event })}
            />
          ))
        )
      ) : null}

      {sheet ? (
        <RecordSheet
          type={sheet.type}
          members={members}
          initialTargetMemberId={child.member_id}
          editingEvent={sheet.editingEvent}
          onClose={() => setSheet(undefined)}
          onSaved={closeAndReload}
        />
      ) : null}
    </View>
  );
}

function SummaryCell({
  testID,
  value,
  label,
}: {
  testID: string;
  value: number | string;
  label: string;
}) {
  return (
    <View testID={testID} style={styles.summaryCell}>
      <Text style={styles.summaryValue}>{value}</Text>
      <Text style={styles.summaryLabel}>{label}</Text>
    </View>
  );
}

function EventRow({ event, onPress }: { event: CareEvent; onPress: () => void }) {
  const meta = careEventMeta(event);
  return (
    <Pressable
      testID={`today-event-${event.id}`}
      accessibilityRole="button"
      accessibilityLabel={`${localClockTime(event.started_at)} ${careEventTitle(event)}`}
      onPress={onPress}
      style={styles.eventRow}
    >
      <Text style={styles.eventTime}>{localClockTime(event.started_at)}</Text>
      <View style={styles.eventBody}>
        <Text style={styles.eventTitle}>{careEventTitle(event)}</Text>
        <View style={styles.eventMetaRow}>
          {meta ? <Text style={styles.eventMeta}>{meta}</Text> : null}
          <Text style={styles.tag}>{CARE_EVENT_SOURCE_LABEL}</Text>
        </View>
      </View>
    </Pressable>
  );
}

const styles = StyleSheet.create({
  container: { gap: 14 },
  hero: {
    gap: 4,
    padding: 14,
    borderWidth: 1,
    borderColor: colors.accent,
    borderRadius: 12,
  },
  heroEyebrow: { fontSize: 13, color: colors.muted },
  heroValue: { fontSize: 26, color: colors.text, fontWeight: '600' },
  heroSub: { fontSize: 13, color: colors.muted },
  heroFoot: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    marginTop: 4,
  },
  heroFootText: { fontSize: 12, color: colors.muted },
  quickGrid: { flexDirection: 'row', gap: 8 },
  quick: {
    flex: 1,
    alignItems: 'center',
    paddingVertical: 14,
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 10,
  },
  quickText: { fontSize: 15, color: colors.text },
  summaryStrip: {
    flexDirection: 'row',
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 10,
  },
  summaryCell: {
    flex: 1,
    alignItems: 'center',
    paddingVertical: 10,
    borderRightWidth: 1,
    borderRightColor: colors.border,
  },
  summaryValue: { fontSize: 18, color: colors.text, fontWeight: '600' },
  summaryLabel: { fontSize: 11, color: colors.muted },
  sectionRow: {
    flexDirection: 'row',
    alignItems: 'baseline',
    justifyContent: 'space-between',
  },
  sectionTitle: { fontSize: 15, color: colors.text, fontWeight: '600' },
  muted: { fontSize: 13, color: colors.muted, lineHeight: 20 },
  error: { fontSize: 13, color: colors.fail },
  stack: { gap: 8 },
  retry: {
    alignSelf: 'flex-start',
    paddingVertical: 8,
    paddingHorizontal: 14,
    borderRadius: 8,
    backgroundColor: colors.accent,
  },
  retryText: { fontSize: 14, color: colors.onAccent },
  eventRow: {
    flexDirection: 'row',
    gap: 12,
    paddingVertical: 12,
    borderBottomWidth: 1,
    borderBottomColor: colors.border,
  },
  eventTime: { width: 48, fontSize: 13, color: colors.muted },
  eventBody: { flex: 1, gap: 4 },
  eventTitle: { fontSize: 15, color: colors.text },
  eventMetaRow: { flexDirection: 'row', alignItems: 'center', gap: 8, flexWrap: 'wrap' },
  eventMeta: { fontSize: 12, color: colors.muted },
  tag: {
    fontSize: 11,
    color: colors.muted,
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 10,
    paddingHorizontal: 7,
    paddingVertical: 1,
  },
});
