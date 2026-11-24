// Reminder settings stack page. Reminders are local to this device, default off and
// always timed from the current child's latest parent-recorded feed. No remote push.
import { useState } from 'react';
import {
  ActivityIndicator,
  Pressable,
  ScrollView,
  StyleSheet,
  Text,
  View,
} from 'react-native';

import type { Member } from '../api/members';
import { localClockTime } from '../care/careEventDisplay';
import { useTodayCareEvents } from '../care/useTodayCareEvents';
import { sharedStyles } from '../components/styles';
import { colors } from '../components/theme';
import { resolveCurrentChild } from '../members/currentChild';
import { useCurrentMember } from '../members/CurrentMemberContext';
import { useMembers } from '../members/useMembers';
import {
  cancelReminderNotifications,
  ensureReminderPermission,
  syncReminderNotification,
} from './reminderNotifications';
import {
  REMINDER_INTERVAL_OPTIONS,
  type ReminderIntervalHours,
} from './reminderSettings';
import {
  reminderNotificationContent,
  reminderTimingLabel,
} from './reminderSchedule';
import { useReminderSettings } from './useReminderSettings';

export function ReminderSettingsScreen() {
  const { currentChildId } = useCurrentMember();
  const { state } = useMembers();

  if (state.status === 'loading') {
    return (
      <View style={styles.container}>
        <View style={sharedStyles.horizontal}>
          <ActivityIndicator />
          <Text style={sharedStyles.muted}>正在加载家庭成员…</Text>
        </View>
      </View>
    );
  }

  if (state.status === 'unconfigured') {
    return (
      <View style={styles.container}>
        <Text style={sharedStyles.muted}>尚未配置后端地址，请先到设置页填写。</Text>
      </View>
    );
  }

  if (state.status === 'error') {
    return (
      <View style={styles.container}>
        <Text style={styles.error}>家庭成员加载失败:{state.detail}</Text>
      </View>
    );
  }

  const childState = resolveCurrentChild(state.members, currentChildId);
  if (childState.status !== 'selected') {
    return (
      <View style={styles.container}>
        <Text style={sharedStyles.muted}>
          {childState.status === 'no-children'
            ? '还没有孩子成员。提醒只用于孩子的喂养记录。'
            : '请先回到今天页选择孩子。'}
        </Text>
      </View>
    );
  }

  return <ReminderSettingsContent child={childState.child} />;
}

function ReminderSettingsContent({ child }: { child: Member }) {
  const { state: settingsState, save } = useReminderSettings();
  const { state: careState } = useTodayCareEvents(child.member_id);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();
  const settings = settingsState.status === 'ready' ? settingsState.settings : undefined;
  const lastFeed = careState.status === 'ready' ? careState.lastFeed : undefined;
  const disabled = busy || careState.status === 'loading';
  const now = new Date();

  const setEnabled = async (enabled: boolean) => {
    if (!settings) {
      return;
    }
    setBusy(true);
    setError(undefined);
    try {
      if (!enabled) {
        await cancelReminderNotifications();
        await save({ ...settings, enabled: false });
        return;
      }
      const permission = await ensureReminderPermission();
      if (permission !== 'granted') {
        setError('通知权限未开启，提醒仍保持关闭。请在系统设置中允许通知后重试。');
        return;
      }
      const next = { ...settings, enabled: true };
      await syncReminderNotification(next, lastFeed?.started_at, now);
      await save(next);
    } catch (failure) {
      setError(failureMessage(failure));
    } finally {
      setBusy(false);
    }
  };

  const setInterval = async (intervalHours: ReminderIntervalHours) => {
    if (!settings || settings.intervalHours === intervalHours) {
      return;
    }
    setBusy(true);
    setError(undefined);
    try {
      const next = { ...settings, intervalHours };
      if (next.enabled) {
        await syncReminderNotification(next, lastFeed?.started_at, now);
      }
      await save(next);
    } catch (failure) {
      setError(failureMessage(failure));
    } finally {
      setBusy(false);
    }
  };

  if (!settings) {
    return (
      <View style={styles.container}>
        <ActivityIndicator />
      </View>
    );
  }

  const status = settings.enabled
    ? `已开启 · ${reminderTimingLabel(lastFeed?.started_at, settings.intervalHours, now)}`
    : '默认关闭 · 开启后从上次喂养开始计时';
  const preview = reminderNotificationContent(settings.intervalHours);

  return (
    <ScrollView style={styles.scroll} contentContainerStyle={styles.content}>
      <View style={styles.card}>
        <View style={styles.switchRow}>
          <View style={styles.switchCopy}>
            <Text style={styles.title}>喂奶间隔提醒</Text>
            <Text testID="reminder-status" style={styles.status}>
              {status}
            </Text>
          </View>
          <Pressable
            testID="reminder-switch"
            accessibilityRole="switch"
            accessibilityLabel="喂奶间隔提醒"
            accessibilityState={{ checked: settings.enabled, disabled }}
            disabled={disabled}
            onPress={() => void setEnabled(!settings.enabled)}
            style={[styles.track, settings.enabled && styles.trackOn, disabled && styles.disabled]}
          >
            <View style={[styles.thumb, settings.enabled && styles.thumbOn]} />
          </Pressable>
        </View>

        <Text testID="reminder-last-feed" style={styles.lastFeed}>
          {careState.status === 'ready'
            ? lastFeed
              ? `上次喂养 ${localClockTime(lastFeed.started_at)}`
              : '还没有喂养记录'
            : careState.status === 'error'
              ? '上次喂养加载失败'
              : '正在加载上次喂养…'}
        </Text>

        <View style={styles.intervalRow}>
          {REMINDER_INTERVAL_OPTIONS.map((hours) => {
            const active = settings.intervalHours === hours;
            return (
              <Pressable
                key={hours}
                testID={`reminder-interval-${hours}`}
                accessibilityRole="button"
                accessibilityLabel={`每 ${hours} 小时`}
                accessibilityState={{ selected: active, disabled }}
                disabled={disabled}
                onPress={() => void setInterval(hours)}
                style={[styles.interval, active && styles.intervalActive]}
              >
                <Text style={active ? styles.intervalTextActive : styles.intervalText}>
                  每 {hours} 小时
                </Text>
              </Pressable>
            );
          })}
        </View>
      </View>

      {error ? (
        <Text testID="reminder-permission-error" style={styles.error}>
          {error}
        </Text>
      ) : null}

      <Text style={styles.sectionTitle}>通知预览</Text>
      {settings.enabled ? (
        <View testID="reminder-preview" style={styles.preview}>
          <Text style={styles.previewApp}>健康豆豆 · 本机通知</Text>
          <Text style={styles.previewTitle}>{preview.title}</Text>
          <Text style={styles.previewBody}>{preview.body}</Text>
        </View>
      ) : (
        <Text style={styles.muted}>
          开启后，这里会显示一条本机通知预览。提醒只陈述时间，不提供喂养建议。
        </Text>
      )}

      <Text testID="reminder-limitations" style={styles.note}>
        提醒只在本机生效；没有账号、云同步或远程推送。不承诺准时：Android
        省电策略 / 精确闹钟限制与 iOS 通知权限都可能影响到达时间。
      </Text>

      <Text style={styles.sectionTitle}>提醒类型</Text>
      <ReminderTypeRow title="睡眠提醒" />
      <ReminderTypeRow title="尿布提醒" />
    </ScrollView>
  );
}

function failureMessage(failure: unknown): string {
  return `提醒设置失败:${failure instanceof Error ? failure.message : String(failure)}`;
}

function ReminderTypeRow({ title }: { title: string }) {
  return (
    <View style={styles.typeRow}>
      <Text style={styles.typeTitle}>{title}</Text>
      <Text style={styles.typeMeta}>以后</Text>
    </View>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, padding: 24, gap: 12, backgroundColor: colors.background },
  scroll: { flex: 1, backgroundColor: colors.background },
  content: { padding: 24, gap: 12 },
  card: {
    gap: 10,
    padding: 14,
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 12,
  },
  switchRow: { flexDirection: 'row', alignItems: 'center', gap: 12 },
  switchCopy: { flex: 1, gap: 3 },
  title: { fontSize: 15, color: colors.text, fontWeight: '600' },
  status: { fontSize: 12, color: colors.muted, lineHeight: 18 },
  lastFeed: { fontSize: 12, color: colors.muted },
  track: {
    width: 46,
    height: 28,
    borderRadius: 14,
    padding: 3,
    backgroundColor: colors.border,
    justifyContent: 'center',
  },
  trackOn: { backgroundColor: colors.accent },
  disabled: { opacity: 0.6 },
  thumb: {
    width: 22,
    height: 22,
    borderRadius: 11,
    backgroundColor: colors.background,
  },
  thumbOn: { alignSelf: 'flex-end' },
  intervalRow: { flexDirection: 'row', gap: 6 },
  interval: {
    flex: 1,
    alignItems: 'center',
    paddingVertical: 8,
    borderRadius: 8,
    borderWidth: 1,
    borderColor: colors.border,
  },
  intervalActive: { borderColor: colors.accent, backgroundColor: colors.accentSoft },
  intervalText: { fontSize: 12, color: colors.muted },
  intervalTextActive: { fontSize: 12, color: colors.accent, fontWeight: '600' },
  sectionTitle: { marginTop: 4, fontSize: 13, color: colors.muted, fontWeight: '600' },
  preview: {
    gap: 3,
    padding: 12,
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 12,
    backgroundColor: colors.surface,
  },
  previewApp: { fontSize: 10, color: colors.muted },
  previewTitle: { fontSize: 13, color: colors.text, fontWeight: '600' },
  previewBody: { fontSize: 11, lineHeight: 17, color: colors.muted },
  note: {
    padding: 12,
    borderRadius: 10,
    backgroundColor: colors.surface,
    fontSize: 11,
    lineHeight: 17,
    color: colors.muted,
  },
  typeRow: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    paddingVertical: 10,
    paddingHorizontal: 12,
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 8,
  },
  typeTitle: { fontSize: 14, color: colors.text },
  typeMeta: { fontSize: 12, color: colors.muted },
  error: { fontSize: 12, lineHeight: 18, color: colors.fail },
  muted: { fontSize: 12, lineHeight: 18, color: colors.muted },
});
