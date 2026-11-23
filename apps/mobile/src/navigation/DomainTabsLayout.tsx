import { Tabs } from 'expo-router/js-tabs';

import { colors } from '../components/theme';
import { DOMAIN_TABS } from './domainTabConfig';
import { DomainHeader } from './DomainHeader';
import type { Domain } from './domainRoutes';

// Shared B3 JS-tabs layout: the domain only selects which contextual tabs and appbar
// state to render; every domain uses the same header and tab styling.
export function DomainTabsLayout({ domain }: { domain: Domain }) {
  return (
    <Tabs
      screenOptions={{
        header: () => <DomainHeader activeDomain={domain} />,
        tabBarActiveTintColor: colors.accent,
        tabBarInactiveTintColor: colors.muted,
      }}
    >
      {DOMAIN_TABS[domain].map(({ name, title, Icon }) => (
        <Tabs.Screen
          key={name}
          name={name}
          options={{
            title,
            tabBarButtonTestID: `${domain}-tab-${name}`,
            tabBarIcon: ({ color, size }) => <Icon color={color} size={size} />,
          }}
        />
      ))}
    </Tabs>
  );
}
