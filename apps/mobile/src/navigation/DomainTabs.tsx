import { Pressable, StyleSheet, Text, View } from 'react-native';

import { colors } from '../components/theme';
import type { Domain } from './domainRoutes';

type DomainTabsProps = {
  activeDomain: Domain;
  hasPending: boolean;
  onSelect: (domain: Domain) => void;
};

// Appbar domain tabs (B3): 育儿 / 健康档案, active text bold with a 2px accent
// underline; health carries a pending dot when actionable report review exists.
// Pure presentation so screen tests can exercise activation without a navigator.
export function DomainTabs({ activeDomain, hasPending, onSelect }: DomainTabsProps) {
  return (
    <View testID="domain-tabs" style={styles.tabs}>
      <DomainTab
        label="育儿"
        active={activeDomain === 'care'}
        testID="domain-tab-care"
        onPress={() => onSelect('care')}
      />
      <DomainTab
        label="健康档案"
        active={activeDomain === 'health'}
        showDot={hasPending}
        testID="domain-tab-health"
        onPress={() => onSelect('health')}
      />
    </View>
  );
}

function DomainTab({
  label,
  active,
  showDot = false,
  testID,
  onPress,
}: {
  label: string;
  active: boolean;
  showDot?: boolean;
  testID: string;
  onPress: () => void;
}) {
  return (
    <Pressable
      testID={testID}
      accessibilityRole="tab"
      accessibilityState={{ selected: active }}
      onPress={onPress}
      style={styles.tab}
    >
      <View style={styles.labelRow}>
        <Text
          testID={`${testID}-label`}
          style={[styles.label, active && styles.labelActive]}
        >
          {label}
        </Text>
        {showDot ? <View testID={`${testID}-dot`} style={styles.dot} /> : null}
      </View>
      <View
        testID={`${testID}-underline`}
        style={[styles.underline, active && styles.underlineActive]}
      />
    </Pressable>
  );
}

const styles = StyleSheet.create({
  tabs: { flexDirection: 'row', alignItems: 'flex-end', gap: 18 },
  tab: { paddingTop: 12 },
  labelRow: { flexDirection: 'row', alignItems: 'center', gap: 5 },
  label: { fontSize: 16, color: colors.muted },
  labelActive: { color: colors.text, fontWeight: '600' },
  dot: { width: 6, height: 6, borderRadius: 3, backgroundColor: colors.accent },
  underline: { height: 2, marginTop: 6, backgroundColor: 'transparent' },
  underlineActive: { backgroundColor: colors.accent },
});
