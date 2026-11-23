import { Redirect } from 'expo-router';
import { useEffect, useState } from 'react';
import { ActivityIndicator, StyleSheet, View } from 'react-native';

import { listMembers } from '../api/members';
import { resolveApiClient } from '../api/resolveClient';
import { colors } from '../components/theme';
import {
  DEFAULT_DOMAIN_NAVIGATION,
  resolveEntryPath,
  type DomainPath,
} from '../navigation/domainRoutes';
import { loadDomainNavigation } from '../navigation/domainNavigationStore';

// B3 default entry: remembered domain / secondary view wins; first launch starts in
// 日常 today when the family has a child, otherwise in 健康档案 reports. The welcome
// chooser from the prototype is optional in the spec and intentionally not added here.
export default function EntryScreen() {
  const [target, setTarget] = useState<DomainPath>();

  useEffect(() => {
    let cancelled = false;

    void (async () => {
      let state = DEFAULT_DOMAIN_NAVIGATION;
      try {
        state = await loadDomainNavigation();
      } catch {
        // Keep the safe defaults if local storage is unavailable.
      }

      let hasChild = false;
      if (state.lastDomain === undefined) {
        const resolved = await resolveApiClient();
        if (resolved) {
          try {
            const members = await listMembers(resolved.client);
            hasChild = members.some((member) => member.is_child);
          } catch {
            // The archive route owns error presentation; fall back to health reports.
          }
        }
      }

      if (!cancelled) {
        setTarget(resolveEntryPath(state, hasChild));
      }
    })();

    return () => {
      cancelled = true;
    };
  }, []);

  if (target === undefined) {
    return (
      <View testID="entry-loading" style={styles.loading}>
        <ActivityIndicator />
      </View>
    );
  }

  return <Redirect href={target} />;
}

const styles = StyleSheet.create({
  loading: { flex: 1, alignItems: 'center', justifyContent: 'center', backgroundColor: colors.background },
});
