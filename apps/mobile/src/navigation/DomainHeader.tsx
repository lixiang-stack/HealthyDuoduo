import { Pressable, StyleSheet, Text, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { colors } from '../components/theme';
import { useCurrentMember } from '../members/CurrentMemberContext';
import { pendingSummary } from '../reports/overview';
import { useMemberReports } from '../reports/useMemberReports';
import { useDomainNavigation } from './DomainNavigationContext';
import type { Domain } from './domainRoutes';
import { DomainTabs } from './DomainTabs';

// B3 appbar: domain tabs on the left, current-child slot on the right. The slot is a
// placeholder until ticket 03 wires currentChildId / the child picker.
export function DomainHeader({ activeDomain }: { activeDomain: Domain }) {
  const { navigateFromDomain } = useDomainNavigation();
  const hasPending = usePendingReviewDot();

  return (
    <SafeAreaView edges={['top']} style={styles.safe}>
      <View style={styles.row}>
        <DomainTabs
          activeDomain={activeDomain}
          hasPending={hasPending}
          onSelect={(pressedDomain) => navigateFromDomain(activeDomain, pressedDomain)}
        />
        <Pressable
          testID="current-child-slot"
          accessibilityRole="button"
          accessibilityState={{ disabled: true }}
          disabled
          style={styles.childSlot}
        >
          <Text style={styles.childText}>孩子 ▾</Text>
        </Pressable>
      </View>
    </SafeAreaView>
  );
}

/** Health-tab dot reuses the existing report pending count (no new endpoint).
 *  The current member is the archive viewing context; adults simply contribute no dot. */
function usePendingReviewDot(): boolean {
  const { currentMemberId } = useCurrentMember();
  const { state } = useMemberReports(currentMemberId);

  return state.status === 'ready' && pendingSummary(state.reports).partialReports > 0;
}

const styles = StyleSheet.create({
  safe: {
    backgroundColor: colors.background,
    borderBottomWidth: 1,
    borderBottomColor: colors.border,
  },
  row: {
    flexDirection: 'row',
    alignItems: 'flex-end',
    justifyContent: 'space-between',
    paddingHorizontal: 16,
    minHeight: 48,
  },
  childSlot: { paddingBottom: 8, paddingLeft: 12 },
  childText: { fontSize: 13, color: colors.accent },
});
