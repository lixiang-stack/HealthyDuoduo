import { Link } from 'expo-router';
import { Tabs } from 'expo-router/js-tabs';
import { Pressable, StyleSheet, Text } from 'react-native';

import { colors } from '../../components/theme';

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
    <Tabs screenOptions={{ headerRightContainerStyle: styles.headerActionContainer }}>
      <Tabs.Screen
        name="index"
        options={{
          title: '家庭',
          tabBarButtonTestID: 'tab-family',
          headerRight: () => <HeaderSettings testID="header-settings-family" />,
        }}
      />
      <Tabs.Screen
        name="[archive]"
        options={{
          title: '档案',
          href: '/timeline',
          tabBarButtonTestID: 'tab-archive',
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
