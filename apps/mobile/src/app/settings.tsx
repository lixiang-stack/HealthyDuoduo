import { useFocusEffect } from 'expo-router';
import { useCallback, useState } from 'react';
import { Pressable, StyleSheet, Text, TextInput, View } from 'react-native';

import { createApiClient } from '../api/client';
import type { HealthState } from '../api/connectivity';
import { checkHealth } from '../api/health';
import { resolveApiClient } from '../api/resolveClient';
import { ConnectivityStatus } from '../components/ConnectivityStatus';
import { sharedStyles } from '../components/styles';
import { colors } from '../components/theme';
import { normalizeBaseUrl } from '../settings/baseUrl';
import { saveBaseUrl } from '../settings/baseUrlStore';

// 手输 Base URL 兜底(二维码配对见后续票):保存即持久化,连通校验统一走 health。
export default function SettingsScreen() {
  const [input, setInput] = useState('');
  const [error, setError] = useState<string>();
  const [state, setState] = useState<HealthState>({ status: 'unconfigured' });

  useFocusEffect(
    useCallback(() => {
      void (async () => {
        const resolved = await resolveApiClient();
        setInput(resolved?.baseUrl ?? '');
        if (resolved) {
          setState({ status: 'checking' });
          setState(await checkHealth(resolved.client));
        }
      })();
    }, []),
  );

  const onSave = useCallback(async () => {
    const normalized = normalizeBaseUrl(input);
    if (!normalized) {
      setError('地址需以 http:// 或 https:// 开头');
      return;
    }
    setError(undefined);
    await saveBaseUrl(normalized);
    setInput(normalized);
    setState({ status: 'checking' });
    setState(await checkHealth(createApiClient(normalized)));
  }, [input]);

  return (
    <View style={styles.container}>
      <Text style={sharedStyles.label}>后端 Base URL</Text>
      <TextInput
        testID="settings-base-url"
        value={input}
        onChangeText={setInput}
        autoCapitalize="none"
        autoCorrect={false}
        keyboardType="url"
        placeholder="http://192.168.1.5:8080/api/v1"
        style={styles.input}
      />
      {error ? <Text style={styles.fail}>{error}</Text> : null}
      <Pressable testID="settings-save" style={styles.button} onPress={() => void onSave()}>
        <Text style={styles.buttonText}>保存并校验</Text>
      </Pressable>
      <ConnectivityStatus state={state} />
    </View>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, padding: 24, gap: 12, backgroundColor: colors.background },
  input: { borderWidth: 1, borderColor: colors.border, borderRadius: 8, paddingHorizontal: 12, paddingVertical: 10, fontSize: 15 },
  fail: { fontSize: 13, color: colors.fail },
  button: { alignSelf: 'flex-start', paddingVertical: 10, paddingHorizontal: 16, backgroundColor: colors.accent, borderRadius: 8 },
  buttonText: { color: colors.onAccent, fontSize: 15 },
});
