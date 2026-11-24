// Today-page entry to the reminder settings. The reminder is not a tab and must not
// appear on the stats page; this card is its only entry.
import { router } from 'expo-router';
import { Pressable, StyleSheet, Text, View } from 'react-native';

import type { CareEvent } from '../care/careEvents';
import { colors } from '../components/theme';
import { reminderTimingLabel } from './reminderSchedule';
import { useReminderSettings } from './useReminderSettings';

export function ReminderCard({ lastFeed, now }: { lastFeed?: CareEvent; now: Date }) {
  const { state } = useReminderSettings();
  const settings = state.status === 'ready' ? state.settings : undefined;
  const meta =
    settings === undefined
      ? '…'
      : settings.enabled
        ? `每 ${settings.intervalHours} 小时 · ${reminderTimingLabel(
            lastFeed?.started_at,
            settings.intervalHours,
            now,
          )}`
        : '未开启';

  return (
    <Pressable
      testID="today-reminder"
      accessibilityRole="button"
      accessibilityLabel="喂奶间隔提醒设置"
      onPress={() => router.push('/reminder')}
      style={styles.card}
    >
      <View style={styles.copy}>
        <Text style={styles.title}>喂奶间隔提醒</Text>
        <Text testID="today-reminder-meta" style={styles.meta}>
          {meta}
        </Text>
      </View>
      <Text style={styles.chevron}>›</Text>
    </Pressable>
  );
}

const styles = StyleSheet.create({
  card: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 10,
    paddingVertical: 10,
    paddingHorizontal: 12,
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 10,
  },
  copy: { flex: 1, minWidth: 0, gap: 2 },
  title: { fontSize: 13, color: colors.text, fontWeight: '600' },
  meta: { fontSize: 11, color: colors.muted },
  chevron: { fontSize: 18, color: colors.muted },
});
