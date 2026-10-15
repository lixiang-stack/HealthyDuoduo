import Constants from 'expo-constants';
import { Link, useFocusEffect } from 'expo-router';
import { useCallback, useState } from 'react';
import { Pressable, StyleSheet, Text, View } from 'react-native';

import { createApiClient } from '../api/client';
import type { HealthState } from '../api/connectivity';
import { checkHealth } from '../api/health';
import { ConnectivityStatus } from '../components/ConnectivityStatus';
import { colors } from '../components/theme';
import { deriveDefaultBaseUrl } from '../settings/baseUrl';
import { loadBaseUrl } from '../settings/baseUrlStore';

// 冷启动即探一次后端:显示连通状态与版本;未配置/不可达都呈现可理解的失败态,不崩溃。
export default function HomeScreen() {
  const [baseUrl, setBaseUrl] = useState<string>();
  const [state, setState] = useState<HealthState>({ status: 'checking' });

  const runCheck = useCallback(async () => {
    setState({ status: 'checking' });
    const effective = (await loadBaseUrl()) ?? deriveDefaultBaseUrl(Constants.expoConfig?.hostUri);
    setBaseUrl(effective);
    if (!effective) {
      setState({ status: 'unconfigured' });
      return;
    }
    setState(await checkHealth(createApiClient(effective)));
  }, []);

  useFocusEffect(
    useCallback(() => {
      void runCheck();
    }, [runCheck]),
  );

  return (
    <View style={styles.container}>
      <Text style={styles.label}>后端地址</Text>
      <Text style={styles.value}>{baseUrl ?? '未配置'}</Text>
      <ConnectivityStatus state={state} />
      <Pressable style={styles.button} onPress={() => void runCheck()}>
        <Text style={styles.buttonText}>重新检测</Text>
      </Pressable>
      <Link href="/settings" style={styles.link}>
        去设置后端地址
      </Link>
    </View>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, padding: 24, gap: 12, backgroundColor: colors.background },
  label: { fontSize: 13, color: colors.muted },
  value: { fontSize: 15, color: colors.text },
  button: { alignSelf: 'flex-start', paddingVertical: 10, paddingHorizontal: 16, backgroundColor: colors.accent, borderRadius: 8 },
  buttonText: { color: colors.onAccent, fontSize: 15 },
  link: { marginTop: 8, color: colors.accent, fontSize: 15 },
});
