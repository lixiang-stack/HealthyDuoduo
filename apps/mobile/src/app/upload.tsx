import { Link, useFocusEffect } from 'expo-router';
import { useCallback, useEffect, useState } from 'react';
import { ActivityIndicator, Image, Pressable, ScrollView, StyleSheet, Text, View } from 'react-native';

import { listMembers, type Member } from '../api/members';
import { uploadReport, type UploadResult } from '../api/reports';
import { resolveApiClient } from '../api/resolveClient';
import { sharedStyles } from '../components/styles';
import { colors } from '../components/theme';
import { MemberChips } from '../members/components';
import { useCurrentMember } from '../members/CurrentMemberContext';
import { sortMembers } from '../members/order';
import { pickImage, type ImageSource, type PickedImage } from '../platform/imagePicker';
import { needsReview } from '../review/items';
import { uploadResultText, uploadWaitText } from '../reports/status';

// 上传页(票 06;票 09 核对分岔):选图 → 确认归属成员(默认当前成员)→ multipart 直传、
// 同步等待识别;等待可感知(计时 + 超时提示),上传返回即分岔——需核对报告(partial/
// 含低置信项)给「立即核对」直达核对页(纸还在手边),干净 success 轻提示已归档。
type ScreenState =
  | { status: 'loading' }
  | { status: 'unconfigured' }
  | { status: 'error'; detail: string }
  | { status: 'ready'; members: Member[] };

type UploadState =
  | { status: 'idle' }
  | { status: 'uploading' }
  | { status: 'done'; result: UploadResult }
  | { status: 'failed'; message: string };

export default function UploadScreen() {
  const { currentMemberId, selectMember } = useCurrentMember();
  const [state, setState] = useState<ScreenState>({ status: 'loading' });
  const [image, setImage] = useState<PickedImage>();
  const [pickMessage, setPickMessage] = useState<string>();
  const [upload, setUpload] = useState<UploadState>({ status: 'idle' });
  const [elapsedMs, setElapsedMs] = useState(0);

  const reload = useCallback(async () => {
    const resolved = await resolveApiClient();
    if (!resolved) {
      setState({ status: 'unconfigured' });
      return;
    }
    try {
      setState({ status: 'ready', members: await listMembers(resolved.client) });
    } catch (error) {
      setState({ status: 'error', detail: error instanceof Error ? error.message : String(error) });
    }
  }, []);

  useFocusEffect(
    useCallback(() => {
      void reload();
    }, [reload]),
  );

  // 上传等待计时:超时阈值后上传页展示确认后端可达的提示(请求不中止,幂等重试安全)。
  useEffect(() => {
    if (upload.status !== 'uploading') {
      return;
    }
    const startedAt = Date.now();
    setElapsedMs(0);
    const timer = setInterval(() => setElapsedMs(Date.now() - startedAt), 1000);
    return () => clearInterval(timer);
  }, [upload.status]);

  const onPick = useCallback(async (source: ImageSource) => {
    setPickMessage(undefined);
    setUpload({ status: 'idle' });
    const result = await pickImage(source);
    switch (result.kind) {
      case 'picked':
        setImage(result.image);
        break;
      case 'cancelled':
        break;
      case 'denied':
        setPickMessage('需要相机/相册权限才能选择检查单照片');
        break;
      case 'unsupported':
        setPickMessage(result.reason);
        break;
    }
  }, []);

  const onUpload = useCallback(() => {
    if (!image || currentMemberId === undefined) {
      return;
    }
    void (async () => {
      setUpload({ status: 'uploading' });
      try {
        const resolved = await resolveApiClient();
        if (!resolved) {
          setState({ status: 'unconfigured' });
          setUpload({ status: 'idle' });
          return;
        }
        setUpload({ status: 'done', result: await uploadReport(resolved.client, currentMemberId, image) });
      } catch (error) {
        setUpload({ status: 'failed', message: error instanceof Error ? error.message : String(error) });
      }
    })();
  }, [image, currentMemberId]);

  const members = state.status === 'ready' ? sortMembers(state.members) : [];
  const canUpload =
    state.status === 'ready' &&
    members.length > 0 &&
    image !== undefined &&
    currentMemberId !== undefined &&
    upload.status !== 'uploading';

  return (
    <ScrollView style={styles.container} contentContainerStyle={styles.content}>
      {state.status === 'loading' ? (
        <View style={sharedStyles.horizontal}>
          <ActivityIndicator />
          <Text style={sharedStyles.muted}>正在加载…</Text>
        </View>
      ) : null}

      {state.status === 'unconfigured' ? (
        <Text style={sharedStyles.muted}>
          尚未配置后端地址,请先到<Link href="/settings" style={styles.link}>设置</Link>填写。
        </Text>
      ) : null}

      {state.status === 'error' ? (
        <View style={styles.stack}>
          <Text style={styles.fail}>加载失败:{state.detail}</Text>
          <Pressable style={styles.button} onPress={() => void reload()}>
            <Text style={styles.buttonText}>重试</Text>
          </Pressable>
        </View>
      ) : null}

      {state.status === 'ready' ? (
        <View style={styles.stack}>
          {members.length === 0 ? (
            <Text style={sharedStyles.muted}>
              还没有家庭成员。先到<Link href="/members" style={styles.link}>家庭成员</Link>建一个,再上传检查单。
            </Text>
          ) : (
            <>
              <Text style={sharedStyles.label}>归属成员</Text>
              <MemberChips
                members={members}
                selectedId={currentMemberId}
                disabled={upload.status === 'uploading'}
                onSelect={selectMember}
              />
              {currentMemberId === undefined ? (
                <Text style={sharedStyles.muted}>先选择这张检查单归属哪位成员。</Text>
              ) : null}

              <Text style={sharedStyles.label}>检查单照片</Text>
              {image ? (
                <View style={styles.previewRow}>
                  <Image source={{ uri: image.uri }} style={styles.preview} />
                  <Text style={sharedStyles.muted}>{image.fileName ?? '已选择照片'}</Text>
                </View>
              ) : null}
              <View style={sharedStyles.horizontal}>
                <Pressable
                  style={styles.buttonGhost}
                  onPress={() => void onPick('camera')}
                  disabled={upload.status === 'uploading'}
                >
                  <Text style={styles.link}>拍照</Text>
                </Pressable>
                <Pressable
                  testID="upload-pick-library"
                  style={styles.buttonGhost}
                  onPress={() => void onPick('library')}
                  disabled={upload.status === 'uploading'}
                >
                  <Text style={styles.link}>从相册选择</Text>
                </Pressable>
              </View>
              {pickMessage ? <Text style={sharedStyles.muted}>{pickMessage}</Text> : null}

              <Pressable
                testID="upload-submit"
                style={[styles.button, !canUpload && styles.buttonDisabled]}
                onPress={onUpload}
                disabled={!canUpload}
              >
                <Text style={styles.buttonText}>上传并识别</Text>
              </Pressable>

              {upload.status === 'uploading' ? (
                <View style={sharedStyles.horizontal}>
                  <ActivityIndicator />
                  <Text style={sharedStyles.muted}>{uploadWaitText(elapsedMs)}</Text>
                </View>
              ) : null}

              {upload.status === 'done' ? (
                <View style={styles.card}>
                  <Text
                    testID="upload-result"
                    style={
                      upload.result.report.status === 'failed' ? styles.fail : styles.success
                    }
                  >
                    {uploadResultText(upload.result)}
                  </Text>
                  {needsReview(upload.result.report) ? (
                    <Link
                      asChild
                      href={{
                        pathname: '/review/[report_id]',
                        params: { report_id: upload.result.report.report_id },
                      }}
                    >
                      <Pressable testID="upload-review" style={styles.button}>
                        <Text style={styles.buttonText}>立即核对</Text>
                      </Pressable>
                    </Link>
                  ) : null}
                  <View style={sharedStyles.horizontal}>
                    <Link href="/timeline" style={styles.link}>
                      去时间线查看
                    </Link>
                    <Pressable style={styles.buttonGhost} onPress={() => setUpload({ status: 'idle' })}>
                      <Text style={styles.link}>再传一张</Text>
                    </Pressable>
                  </View>
                </View>
              ) : null}

              {upload.status === 'failed' ? (
                <View style={styles.card}>
                  <Text style={styles.fail}>上传失败:{upload.message}</Text>
                  <Pressable style={styles.buttonGhost} onPress={onUpload}>
                    <Text style={styles.link}>重试</Text>
                  </Pressable>
                </View>
              ) : null}
            </>
          )}
        </View>
      ) : null}
    </ScrollView>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: colors.background },
  content: { padding: 24, gap: 12 },
  stack: { gap: 12 },
  fail: { fontSize: 13, color: colors.fail, lineHeight: 20 },
  success: { fontSize: 13, color: colors.ok, lineHeight: 20 },
  link: { color: colors.accent, fontSize: 15 },
  card: { borderWidth: 1, borderColor: colors.border, borderRadius: 8, padding: 12, gap: 8 },
  previewRow: { flexDirection: 'row', alignItems: 'center', gap: 12 },
  preview: { width: 96, height: 96, borderRadius: 8, borderWidth: 1, borderColor: colors.border },
  button: {
    alignSelf: 'flex-start',
    paddingVertical: 10,
    paddingHorizontal: 16,
    backgroundColor: colors.accent,
    borderRadius: 8,
  },
  buttonDisabled: { backgroundColor: colors.border },
  buttonGhost: { paddingVertical: 10, paddingHorizontal: 12 },
  buttonText: { color: colors.onAccent, fontSize: 15 },
});
