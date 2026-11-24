// Today's timeline in the selected A layout: a time gutter, a continuous rail,
// typed nodes, a "now" marker and tappable rows. Newest stays on top.
import { Pressable, StyleSheet, Text, View } from 'react-native';

import {
  CARE_EVENT_COLORS,
  CARE_EVENT_SOURCE_LABEL,
  careEventMeta,
  careEventTitle,
  elapsedMinutes,
  formatElapsedMinutes,
  localClockTime,
  localDateKey,
} from '../care/careEventDisplay';
import type { CareEvent } from '../care/careEvents';
import { colors } from '../components/theme';
import { CareEventIcon } from './CareEventIcon';

const TIME_COLUMN_WIDTH = 42;
const RAIL_COLUMN_WIDTH = 28;
const COLUMN_GAP = 8;
const RAIL_LINE_LEFT = TIME_COLUMN_WIDTH + COLUMN_GAP + RAIL_COLUMN_WIDTH / 2 - 1;

export function CareTimeline({
  events,
  now,
  onPressEvent,
}: {
  events: CareEvent[];
  now: Date;
  onPressEvent: (event: CareEvent) => void;
}) {
  return (
    <View testID="today-timeline-rail" style={styles.container}>
      {events.length > 0 ? <View pointerEvents="none" style={styles.railLine} /> : null}
      <NowRow now={now} />
      {events.length === 0 ? (
        <EmptyTimeline />
      ) : (
        events.map((event) => (
          <EventRow
            key={event.id}
            event={event}
            now={now}
            onPress={() => onPressEvent(event)}
          />
        ))
      )}
    </View>
  );
}

function NowRow({ now }: { now: Date }) {
  return (
    <View style={styles.nowRow}>
      <View style={styles.timeCell} />
      <View style={styles.railCell}>
        <View style={styles.nowDotRing}>
          <View style={styles.nowDot} />
        </View>
      </View>
      <Text testID="today-now" style={styles.nowText}>
        现在 {localClockTime(now)}
      </Text>
    </View>
  );
}

function EventRow({
  event,
  now,
  onPress,
}: {
  event: CareEvent;
  now: Date;
  onPress: () => void;
}) {
  const palette = CARE_EVENT_COLORS[event.type];
  const ongoing = event.type === 'sleep' && event.ended_at === null;
  const crossDay = localDateKey(event.started_at) !== localDateKey(now);
  const note = careEventMeta(event);
  const ongoingMinutes = ongoing ? elapsedMinutes(event.started_at, now) : null;
  const ongoingLabel =
    ongoingMinutes === null
      ? null
      : ongoingMinutes < 1
        ? '刚刚入睡'
        : `已睡 ${formatElapsedMinutes(ongoingMinutes)}`;

  return (
    <Pressable
      testID={`today-event-${event.id}`}
      accessibilityRole="button"
      accessibilityLabel={`${crossDay ? '昨天 ' : ''}${localClockTime(event.started_at)} ${careEventTitle(event)}`}
      accessibilityHint="点按修改"
      onPress={onPress}
      style={styles.eventRow}
    >
      <View style={styles.timeCell}>
        {crossDay ? <Text style={styles.eventDay}>昨天</Text> : null}
        <Text style={styles.eventTime}>{localClockTime(event.started_at)}</Text>
      </View>
      <View style={styles.railCell}>
        <View
          testID={`today-event-node-${event.id}`}
          style={[styles.nodeRing, { borderColor: palette.color }]}
        >
          <View style={[styles.node, { backgroundColor: palette.background }]}>
            <CareEventIcon type={event.type} color={palette.color} size={13} />
          </View>
        </View>
      </View>
      <View style={[styles.eventBody, ongoing && styles.eventBodyOngoing]}>
        <Text style={styles.eventTitle}>{careEventTitle(event)}</Text>
        <View style={styles.eventMetaRow}>
          {ongoingLabel ? <Text style={styles.eventNote}>{ongoingLabel}</Text> : null}
          {note ? <Text style={styles.eventNote}>{note}</Text> : null}
          {ongoing ? <Text style={[styles.chip, styles.chipOngoing]}>进行中</Text> : null}
          {crossDay ? <Text style={[styles.chip, styles.chipCross]}>跨夜</Text> : null}
          <Text style={styles.tag}>{CARE_EVENT_SOURCE_LABEL}</Text>
        </View>
      </View>
      <Text style={styles.chevron}>›</Text>
    </Pressable>
  );
}

function EmptyTimeline() {
  return (
    <View style={styles.emptyContainer}>
      <View style={styles.emptyRail} />
      <View style={styles.emptyGhost} />
      <Text testID="today-empty" style={styles.emptyMessage}>
        今天还没有记录{'\n'}从上面的快捷动作开始。
      </Text>
    </View>
  );
}

const styles = StyleSheet.create({
  container: { position: 'relative', paddingBottom: 4 },
  railLine: {
    position: 'absolute',
    left: RAIL_LINE_LEFT,
    top: 14,
    bottom: 14,
    width: 2,
    backgroundColor: colors.border,
  },
  nowRow: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: COLUMN_GAP,
    minHeight: 28,
  },
  timeCell: {
    width: TIME_COLUMN_WIDTH,
    alignItems: 'flex-end',
    paddingTop: 3,
  },
  railCell: {
    width: RAIL_COLUMN_WIDTH,
    alignItems: 'center',
    paddingTop: 3,
  },
  nowDotRing: {
    width: 16,
    height: 16,
    borderRadius: 8,
    alignItems: 'center',
    justifyContent: 'center',
    backgroundColor: colors.accentSoft,
  },
  nowDot: { width: 8, height: 8, borderRadius: 4, backgroundColor: colors.accent },
  nowText: { fontSize: 11, color: colors.accent, fontWeight: '600' },
  eventRow: {
    flexDirection: 'row',
    alignItems: 'flex-start',
    gap: COLUMN_GAP,
    paddingVertical: 8,
  },
  eventDay: { fontSize: 9, color: colors.muted, lineHeight: 12 },
  eventTime: {
    fontSize: 11,
    color: colors.muted,
    lineHeight: 14,
    fontVariant: ['tabular-nums'],
  },
  nodeRing: {
    width: 26,
    height: 26,
    borderRadius: 13,
    borderWidth: 1.5,
    alignItems: 'center',
    justifyContent: 'center',
    backgroundColor: colors.background,
  },
  node: {
    width: 21,
    height: 21,
    borderRadius: 10.5,
    alignItems: 'center',
    justifyContent: 'center',
  },
  eventBody: { flex: 1, minWidth: 0, paddingTop: 1, paddingBottom: 2 },
  eventBodyOngoing: {
    backgroundColor: CARE_EVENT_COLORS.sleep.background,
    borderRadius: 8,
    paddingHorizontal: 6,
    paddingVertical: 4,
    marginVertical: -4,
  },
  eventTitle: { fontSize: 14, lineHeight: 19, color: colors.text },
  eventMetaRow: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 6,
    flexWrap: 'wrap',
    marginTop: 3,
  },
  eventNote: { fontSize: 11, color: colors.muted },
  chip: {
    fontSize: 10,
    lineHeight: 16,
    paddingHorizontal: 7,
    borderRadius: 999,
    overflow: 'hidden',
  },
  chipOngoing: {
    backgroundColor: CARE_EVENT_COLORS.sleep.background,
    color: CARE_EVENT_COLORS.sleep.color,
  },
  chipCross: {
    borderWidth: 1,
    borderColor: colors.border,
    color: colors.muted,
  },
  tag: {
    fontSize: 10,
    lineHeight: 16,
    paddingHorizontal: 7,
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 999,
    color: colors.muted,
  },
  chevron: { alignSelf: 'center', fontSize: 18, color: colors.muted },
  emptyContainer: { position: 'relative', minHeight: 180, paddingTop: 6 },
  emptyRail: {
    position: 'absolute',
    left: RAIL_LINE_LEFT,
    top: 0,
    height: 140,
    borderLeftWidth: 2,
    borderStyle: 'dashed',
    borderColor: colors.border,
  },
  emptyGhost: {
    position: 'absolute',
    left: RAIL_LINE_LEFT - 7,
    top: 0,
    width: 16,
    height: 16,
    borderRadius: 8,
    borderWidth: 2,
    borderStyle: 'dashed',
    borderColor: colors.border,
    backgroundColor: colors.background,
  },
  emptyMessage: {
    marginLeft: RAIL_LINE_LEFT + 16,
    paddingTop: 2,
    fontSize: 12,
    lineHeight: 18,
    color: colors.muted,
  },
});
