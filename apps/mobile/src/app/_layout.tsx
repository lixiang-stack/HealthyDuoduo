import { Stack } from 'expo-router';
import { StatusBar } from 'expo-status-bar';
import { Platform, StyleSheet } from 'react-native';
import { GestureHandlerRootView } from 'react-native-gesture-handler';

import { HeaderBackButton } from '../components/HeaderBackButton';
import { ImageViewerProvider } from '../components/imageViewer/ImageViewerProvider';
import { CareEventProvider } from '../care/CareEventProvider';
import { CurrentMemberProvider } from '../members/CurrentMemberContext';
import { DomainNavigationProvider } from '../navigation/DomainNavigationContext';

export default function RootLayout() {
  return (
    <GestureHandlerRootView style={styles.root}>
      <CurrentMemberProvider>
        <DomainNavigationProvider>
          <CareEventProvider>
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
                <Stack.Screen name="index" options={{ headerShown: false }} />
                <Stack.Screen name="care" options={{ headerShown: false }} />
                <Stack.Screen name="health" options={{ headerShown: false }} />
                <Stack.Screen name="settings" options={{ title: '设置' }} />
                <Stack.Screen name="members" options={{ title: '家庭成员' }} />
                <Stack.Screen name="today-records" options={{ title: '今日记录' }} />
                <Stack.Screen name="reminder" options={{ title: '提醒' }} />
                <Stack.Screen name="report/[report_id]" options={{ title: '报告详情' }} />
                <Stack.Screen name="image/[report_id]" options={{ title: '原始图像' }} />
                <Stack.Screen name="panel/[panel]" options={{ title: '项目详情' }} />
                <Stack.Screen name="analysis/[analysis_id]" options={{ title: '解读结果' }} />
                <Stack.Screen name="review/[report_id]" options={{ title: '核对报告' }} />
                <Stack.Screen name="upload" options={{ title: '上传检查单' }} />
              </Stack>
              <StatusBar style="auto" />
            </ImageViewerProvider>
          </CareEventProvider>
        </DomainNavigationProvider>
      </CurrentMemberProvider>
    </GestureHandlerRootView>
  );
}

const styles = StyleSheet.create({
  root: { flex: 1 },
});
