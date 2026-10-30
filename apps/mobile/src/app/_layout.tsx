import { router, Stack } from 'expo-router';
import { StatusBar } from 'expo-status-bar';
import { Platform, Pressable, StyleSheet, Text, type ColorValue } from 'react-native';
import { GestureHandlerRootView } from 'react-native-gesture-handler';

import { ImageViewerProvider } from '../components/imageViewer/ImageViewerProvider';
import { colors } from '../components/theme';
import { CurrentMemberProvider } from '../members/CurrentMemberContext';

// iOS 26 原生返回键在部分 Expo Go / react-native-screens 组合下会失去点击响应;
// 用 JS 返回键替换,保留右滑返回手势。
function HeaderBackButton({ tintColor }: { tintColor?: ColorValue }) {
  return (
    <Pressable
      accessibilityRole="button"
      accessibilityLabel="返回"
      hitSlop={8}
      onPress={() => router.back()}
      style={styles.headerBack}
    >
      <Text style={[styles.headerBackText, { color: tintColor ?? colors.accent }]}>‹</Text>
    </Pressable>
  );
}

export default function RootLayout() {
  return (
    <GestureHandlerRootView style={styles.root}>
      <CurrentMemberProvider>
        <ImageViewerProvider>
          <Stack
            screenOptions={
              Platform.OS === 'ios'
                ? {
                    headerBackVisible: false,
                    headerLeft: ({ canGoBack, tintColor }) =>
                      canGoBack ? <HeaderBackButton tintColor={tintColor} /> : null,
                  }
                : undefined
            }
          >
            <Stack.Screen name="(tabs)" options={{ headerShown: false }} />
            <Stack.Screen name="settings" options={{ title: '设置' }} />
            <Stack.Screen name="members" options={{ title: '家庭成员' }} />
            <Stack.Screen name="trends" options={{ title: '异常指标' }} />
            <Stack.Screen name="report/[report_id]" options={{ title: '报告详情' }} />
            <Stack.Screen name="image/[report_id]" options={{ title: '原始图像' }} />
            <Stack.Screen name="panel/[panel]" options={{ title: '项目详情' }} />
            <Stack.Screen name="review/[report_id]" options={{ title: '核对报告' }} />
            <Stack.Screen name="upload" options={{ title: '上传检查单' }} />
          </Stack>
          <StatusBar style="auto" />
        </ImageViewerProvider>
      </CurrentMemberProvider>
    </GestureHandlerRootView>
  );
}

const styles = StyleSheet.create({
  root: { flex: 1 },
  headerBack: {
    width: 44,
    height: 44,
    marginLeft: -8,
    alignItems: 'center',
    justifyContent: 'center',
  },
  headerBackText: { fontSize: 32, lineHeight: 34, marginTop: -2 },
});
