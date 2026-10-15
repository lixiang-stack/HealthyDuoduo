import { Stack } from 'expo-router';
import { StatusBar } from 'expo-status-bar';

export default function RootLayout() {
  return (
    <>
      <Stack>
        <Stack.Screen name="index" options={{ title: '后端连通状态' }} />
        <Stack.Screen name="settings" options={{ title: '设置' }} />
      </Stack>
      <StatusBar style="auto" />
    </>
  );
}
