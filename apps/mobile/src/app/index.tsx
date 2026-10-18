import { Link, useFocusEffect } from 'expo-router';
import { useCallback, useState } from 'react';
import { Pressable, StyleSheet, Text, View } from 'react-native';

import type { HealthState } from '../api/connectivity';
import { checkHealth } from '../api/health';
import { resolveApiClient } from '../api/resolveClient';
import { ConnectivityStatus } from '../components/ConnectivityStatus';
import { sharedStyles } from '../components/styles';
import { colors } from '../components/theme';

// 冷启动即探一次后端:显示连通状态与版本;未配置/不可达都呈现可理解的失败态,不崩溃。
export default function HomeScreen() {
  const [baseUrl, setBaseUrl] = useState<string>();
  const [state, setState] = useState<HealthState>({ status: 'checking' });

  const runCheck = useCallback(async () => {
    setState({ status: 'checking' });
    const resolved = await resolveApiClient();
    setBaseUrl(resolved?.baseUrl);
    if (!resolved) {
      setState({ status: 'unconfigured' });
      return;
    }
    setState(await checkHealth(resolved.client));
  }, []);

  useFocusEffect(
    useCallback(() => {
      void runCheck();
    }, [runCheck]),
  );

  return (
    <View style={styles.container}>
      <Text style={sharedStyles.label}>后端地址</Text>
      <Text style={styles.value}>{baseUrl ?? '未配置'}</Text>
      <ConnectivityStatus state={state} />
      <Pressable style={styles.button} onPress={() => void runCheck()}>
        <Text style={styles.buttonText}>重新检测</Text>
      </Pressable>
      <Link href="/settings" style={styles.link}>
        去设置后端地址
      </Link>
      <Link href="/members" style={styles.link}>
        家庭成员
      </Link>
      <Link href="/timeline" style={styles.link}>
        报告时间线
      </Link>
      <Link href="/upload" style={styles.link}>
        上传检查单
      </Link>
    </View>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, padding: 24, gap: 12, backgroundColor: colors.background },
  value: { fontSize: 15, color: colors.text },
  button: { alignSelf: 'flex-start', paddingVertical: 10, paddingHorizontal: 16, backgroundColor: colors.accent, borderRadius: 8 },
  buttonText: { color: colors.onAccent, fontSize: 15 },
  link: { marginTop: 8, color: colors.accent, fontSize: 15 },
});
