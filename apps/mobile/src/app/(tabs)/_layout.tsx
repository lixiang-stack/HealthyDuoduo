import { Link } from 'expo-router';
import { Tabs } from 'expo-router/js-tabs';
import { Pressable, StyleSheet, Text, type ColorValue } from 'react-native';
import Svg, { Line, Path } from 'react-native-svg';

import { colors } from '../../components/theme';

function FamilyTabIcon({ color, size }: { color: ColorValue; size: number }) {
  return (
    <Svg width={size} height={size} viewBox="0 0 24 24" fill="none">
      <Path
        d="M4 10.5 12 4l8 6.5V20a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1v-9.5Z"
        stroke={color}
        strokeWidth={1.7}
        strokeLinejoin="round"
      />
    </Svg>
  );
}

function ArchiveTabIcon({ color, size }: { color: ColorValue; size: number }) {
  return (
    <Svg width={size} height={size} viewBox="0 0 24 24" fill="none">
      <Line x1="4" y1="7" x2="20" y2="7" stroke={color} strokeWidth={1.7} strokeLinecap="round" />
      <Line x1="4" y1="12" x2="20" y2="12" stroke={color} strokeWidth={1.7} strokeLinecap="round" />
      <Line x1="4" y1="17" x2="20" y2="17" stroke={color} strokeWidth={1.7} strokeLinecap="round" />
    </Svg>
  );
}

function HeaderSettings({ testID }: { testID: string }) {
  return (
    <Link asChild href="/settings">
      <Pressable
        testID={testID}
        accessibilityRole="button"
        accessibilityLabel="设置"
        hitSlop={12}
        style={styles.headerAction}
      >
        <Text style={styles.headerLink}>设置</Text>
      </Pressable>
    </Link>
  );
}

// 双 Tab(票 06):家庭 / 档案;设置只进 header,内容区不再出现设置链接。
// 档案 Tab 承载 `/timeline`(按时间)与 `/panels`(按项目)两个 URL;详情 / 核对 /
// 上传 / 项目 / 异常指标仍走根 Stack。
export default function TabsLayout() {
  return (
    <Tabs
      screenOptions={{
        headerRightContainerStyle: styles.headerActionContainer,
        tabBarActiveTintColor: colors.accent,
        tabBarInactiveTintColor: colors.muted,
      }}
    >
      <Tabs.Screen
        name="index"
        options={{
          title: '家庭',
          tabBarButtonTestID: 'tab-family',
          tabBarIcon: ({ color, size }) => <FamilyTabIcon color={color} size={size} />,
          headerRight: () => <HeaderSettings testID="header-settings-family" />,
        }}
      />
      <Tabs.Screen
        name="[archive]"
        options={{
          title: '档案',
          href: '/timeline',
          tabBarButtonTestID: 'tab-archive',
          tabBarIcon: ({ color, size }) => <ArchiveTabIcon color={color} size={size} />,
          headerRight: () => <HeaderSettings testID="header-settings-archive" />,
        }}
      />
    </Tabs>
  );
}

const styles = StyleSheet.create({
  headerActionContainer: { paddingRight: 16 },
  headerAction: { paddingVertical: 4, paddingHorizontal: 4 },
  headerLink: { color: colors.accent, fontSize: 15 },
});
