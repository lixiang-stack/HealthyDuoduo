import { Stack } from 'expo-router';
import { StatusBar } from 'expo-status-bar';

import { CurrentMemberProvider } from '../members/CurrentMemberContext';

export default function RootLayout() {
  return (
    <CurrentMemberProvider>
      <Stack>
        <Stack.Screen name="index" options={{ title: '成员概览' }} />
        <Stack.Screen name="settings" options={{ title: '设置' }} />
        <Stack.Screen name="members" options={{ title: '家庭成员' }} />
        <Stack.Screen name="timeline" options={{ title: '全部档案' }} />
        <Stack.Screen name="trends" options={{ title: '异常指标趋势' }} />
        <Stack.Screen name="report/[report_id]" options={{ title: '报告详情' }} />
        <Stack.Screen name="review/[report_id]" options={{ title: '核对报告' }} />
        <Stack.Screen name="upload" options={{ title: '上传检查单' }} />
      </Stack>
      <StatusBar style="auto" />
    </CurrentMemberProvider>
  );
}
