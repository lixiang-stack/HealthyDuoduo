// Loading / saving the device-local reminder setting. Both the today card and the
// settings page read it on focus, so returning from the stack page refreshes the card.
import { useFocusEffect } from 'expo-router';
import { useCallback, useState } from 'react';

import {
  DEFAULT_REMINDER_SETTINGS,
  loadReminderSettings,
  saveReminderSettings,
  type ReminderSettings,
} from './reminderSettings';

export type ReminderSettingsState =
  | { status: 'loading' }
  | { status: 'ready'; settings: ReminderSettings };

export function useReminderSettings(): {
  state: ReminderSettingsState;
  save: (settings: ReminderSettings) => Promise<void>;
} {
  const [state, setState] = useState<ReminderSettingsState>({ status: 'loading' });

  useFocusEffect(
    useCallback(() => {
      let cancelled = false;
      void loadReminderSettings()
        .catch(() => DEFAULT_REMINDER_SETTINGS)
        .then((settings) => {
          if (!cancelled) {
            setState({ status: 'ready', settings });
          }
        });
      return () => {
        cancelled = true;
      };
    }, []),
  );

  const save = useCallback(async (settings: ReminderSettings) => {
    await saveReminderSettings(settings);
    setState({ status: 'ready', settings });
  }, []);

  return { state, save };
}
