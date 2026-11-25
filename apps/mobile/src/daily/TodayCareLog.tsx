// Today's care-log page content: hero, four quick actions, today summary, the
// reminder entry and the current child's timeline. Charts belong to the stats view.
import { useState } from 'react';
import {
  ActivityIndicator,
  Pressable,
  RefreshControl,
  ScrollView,
  StyleSheet,
  Text,
  View,
} from 'react-native';

import type { Member } from '../api/members';
import { useCareEventRepository } from '../care/CareEventProvider';
import { CARE_EVENT_TYPES, type CareEvent, type CareEventType } from '../care/careEvents';
import {
  CARE_EVENT_LABELS,
  CARE_EVENT_SOURCE_LABEL,
  elapsedMinutes,
  feedSummary,
  formatElapsedMinutes,
  localClockTime,
  summarizeToday,
} from '../care/careEventDisplay';
import { useTodayCareEvents } from '../care/useTodayCareEvents';
import { colors } from '../components/theme';
import { ReminderCard } from '../reminders/ReminderCard';
import { syncReminderToLatestFeed } from '../reminders/syncReminderToLatestFeed';
import { CareTimeline } from './CareTimeline';
import { RecordSheet } from './RecordSheet';

type SheetState = { type: CareEventType; editingEvent?: CareEvent };

export function TodayCareLog({ child, members }: { child: Member; members: Member[] }) {
  const repository = useCareEventRepository();
  const { state, reload } = useTodayCareEvents(child.member_id);
  const [sheet, setSheet] = useState<SheetState>();
  const [refreshing, setRefreshing] = useState(false);
  const now = new Date();
  const events = state.status === 'ready' ? state.events : [];
  const lastFeed = state.status === 'ready' ? state.lastFeed : undefined;
  const lastFeedDetail = lastFeed?.type === 'feed' ? feedSummary(lastFeed) : null;
  const summary = summarizeToday(events, now);
  const loading = state.status === 'loading';
  const loadFailed = state.status === 'error';
  const errorDetail =
    state.status === 'error' ? state.detail : state.status === 'ready' ? state.error : undefined;

  const refresh = async () => {
    setRefreshing(true);
    try {
      await reload();
    } finally {
      setRefreshing(false);
    }
  };

  const closeAndReload = () => {
    setSheet(undefined);
    void reload();
    // A new / edited / deleted feed changes the latest feed, so the local reminder
    // window must move with it. Failures stay out of the record path.
    void syncReminderToLatestFeed({
      repository,
      memberId: child.member_id,
    }).catch(() => undefined);
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

      <ReminderCard lastFeed={lastFeed} now={now} />

      <View style={styles.timelineSection}>
        <View style={styles.sectionRow}>
          <Text style={styles.sectionTitle}>今日时间线</Text>
          <Text style={styles.muted}>{events.length} 条</Text>
        </View>

        <ScrollView
          testID="today-timeline"
          style={styles.timeline}
          contentContainerStyle={styles.timelineContent}
          keyboardShouldPersistTaps="handled"
          refreshControl={
            <RefreshControl
              refreshing={refreshing}
              onRefresh={() => void refresh()}
              tintColor={colors.accent}
              colors={[colors.accent]}
            />
          }
        >
          {loading ? <ActivityIndicator testID="today-loading" /> : null}
          {errorDetail ? (
            <View style={styles.stack}>
              <Text testID="today-error" style={styles.error}>
                护理记录加载失败:{errorDetail}
              </Text>
              <Pressable style={styles.retry} onPress={() => void reload()}>
                <Text style={styles.retryText}>重试</Text>
              </Pressable>
            </View>
          ) : null}
          {state.status === 'ready' ? (
            <CareTimeline
              events={events}
              now={now}
              onPressEvent={(event) => setSheet({ type: event.type, editingEvent: event })}
            />
          ) : null}
        </ScrollView>
      </View>

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

const styles = StyleSheet.create({
  container: { flex: 1, gap: 14 },
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
  timelineSection: { flex: 1, gap: 8 },
  timeline: { flex: 1 },
  timelineContent: { paddingBottom: 8 },
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
