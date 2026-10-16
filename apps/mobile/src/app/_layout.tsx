import { Stack } from 'expo-router';
import { StatusBar } from 'expo-status-bar';

import { CurrentMemberProvider } from '../members/CurrentMemberContext';

export default function RootLayout() {
  return (
    <CurrentMemberProvider>
      <Stack>
        <Stack.Screen name="index" options={{ title: '后端连通状态' }} />
        <Stack.Screen name="settings" options={{ title: '设置' }} />
        <Stack.Screen name="members" options={{ title: '家庭成员' }} />
        <Stack.Screen name="timeline" options={{ title: '报告时间线' }} />
        <Stack.Screen name="upload" options={{ title: '上传检查单' }} />
      </Stack>
      <StatusBar style="auto" />
    </CurrentMemberProvider>
  );
}
