import { Link, useFocusEffect } from 'expo-router';
import { useCallback, useEffect, useState } from 'react';
import { ActivityIndicator, Image, Pressable, ScrollView, StyleSheet, Text, View } from 'react-native';

import { listMembers, type Member } from '../api/members';
import { uploadReport } from '../api/reports';
import { resolveApiClient, type ResolvedApiClient } from '../api/resolveClient';
import { sharedStyles } from '../components/styles';
import { colors } from '../components/theme';
import { MemberChips } from '../members/components';
import { useCurrentMember } from '../members/CurrentMemberContext';
import { sortMembers } from '../members/order';
import { pickImage, type ImageSource, type PickedImage } from '../platform/imagePicker';
import { needsReview } from '../review/items';
import { uploadResultText, uploadWaitText } from '../reports/status';
import {
  appendPickedImages,
  MAX_UPLOAD_IMAGES,
  uploadBatchCounts,
  uploadBatchSummaryText,
  uploadImagesSerially,
  uploadOutcomeLabel,
  type UploadOutcome,
} from '../reports/uploadBatch';

// 上传页(票 06;多图批次):相册多选/拍照逐张追加 → 确认归属成员(整批同一成员)
// → 逐张串行 multipart 直传、同步等待识别。单张失败不阻断后续;整批完成后
// 逐行给出核对/查看入口,只重试传输失败的图。
type ScreenState =
  | { status: 'loading' }
  | { status: 'unconfigured' }
  | { status: 'error'; detail: string }
  | { status: 'ready'; members: Member[] };

type UploadPhase = 'idle' | 'uploading' | 'done';

function rowStatusText(
  outcome: UploadOutcome | undefined,
  phase: UploadPhase,
  active: boolean,
): string | undefined {
  if (outcome !== undefined) {
    return uploadOutcomeLabel(outcome);
  }
  if (phase !== 'uploading') {
    return undefined;
  }
  return active ? '识别中…' : '等待识别';
}

function summaryText(settled: UploadOutcome[]): string {
  if (settled.length === 1) {
    const [outcome] = settled;
    return outcome.kind === 'done'
      ? uploadResultText(outcome.result)
      : `上传失败:${outcome.message}`;
  }
  return uploadBatchSummaryText(settled);
}

function summaryFailed(settled: UploadOutcome[]): boolean {
  if (settled.length === 1) {
    const [outcome] = settled;
    return outcome.kind === 'failed' || outcome.result.report.status === 'failed';
  }
  return uploadBatchCounts(settled).failed > 0;
}

function UploadRow({
  image,
  index,
  outcome,
  phase,
  active,
  onRemove,
}: {
  image: PickedImage;
  index: number;
  outcome?: UploadOutcome;
  phase: UploadPhase;
  active: boolean;
  onRemove: (index: number) => void;
}) {
  const status = rowStatusText(outcome, phase, active);
  return (
    <View style={styles.row} testID={`upload-row-${index}`}>
      <Image source={{ uri: image.uri }} style={styles.thumb} />
      <View style={styles.rowBody}>
        <Text style={styles.fileName} numberOfLines={1}>
          {image.fileName ?? `照片 ${index + 1}`}
        </Text>
        {status !== undefined ? (
          <Text
            testID={`upload-row-status-${index}`}
            style={[styles.rowStatus, rowStatusStyle(outcome)]}
          >
            {status}
          </Text>
        ) : null}
      </View>
      {phase === 'idle' ? (
        <Pressable testID={`upload-remove-${index}`} onPress={() => onRemove(index)}>
          <Text style={styles.link}>移除</Text>
        </Pressable>
      ) : null}
      {phase === 'done' && outcome?.kind === 'done' ? (
        needsReview(outcome.result.report) ? (
          <Link
            asChild
            href={{
              pathname: '/review/[report_id]',
              params: { report_id: outcome.result.report.report_id },
            }}
          >
            <Pressable testID={`upload-row-review-${index}`}>
              <Text style={styles.link}>立即核对</Text>
            </Pressable>
          </Link>
        ) : (
          <Link
            asChild
            href={{
              pathname: '/report/[report_id]',
              params: { report_id: outcome.result.report.report_id },
            }}
          >
            <Pressable testID={`upload-row-open-${index}`}>
              <Text style={styles.link}>查看报告</Text>
            </Pressable>
          </Link>
        )
      ) : null}
    </View>
  );
}

function rowStatusStyle(outcome: UploadOutcome | undefined) {
  if (outcome === undefined) {
    return sharedStyles.muted;
  }
  if (outcome.kind === 'failed' || outcome.result.report.status === 'failed') {
    return styles.fail;
  }
  if (!outcome.result.duplicate && needsReview(outcome.result.report)) {
    return styles.review;
  }
  return styles.success;
}

export default function UploadScreen() {
  const { currentMemberId, selectMember } = useCurrentMember();
  const [state, setState] = useState<ScreenState>({ status: 'loading' });
  const [images, setImages] = useState<PickedImage[]>([]);
  const [outcomes, setOutcomes] = useState<(UploadOutcome | undefined)[]>([]);
  const [phase, setPhase] = useState<UploadPhase>('idle');
  const [activeIndex, setActiveIndex] = useState<number>();
  const [picking, setPicking] = useState(false);
  const [pickMessage, setPickMessage] = useState<string>();
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

  // 上传等待计时:超时阈值后展示确认后端可达的提示(此处不中止请求;
  // 单张请求由 uploadReport 的 180s 超时兜底)。
  useEffect(() => {
    if (phase !== 'uploading') {
      return;
    }
    const startedAt = Date.now();
    const timer = setInterval(() => setElapsedMs(Date.now() - startedAt), 1000);
    return () => clearInterval(timer);
  }, [phase]);

  const onPick = useCallback(
    async (source: ImageSource) => {
      if (phase !== 'idle' || picking) {
        return;
      }
      const remaining = MAX_UPLOAD_IMAGES - images.length;
      if (remaining <= 0) {
        setPickMessage(`最多上传 ${MAX_UPLOAD_IMAGES} 张`);
        return;
      }
      setPicking(true);
      setPickMessage(undefined);
      try {
        const result = await pickImage(source, remaining);
        switch (result.kind) {
          case 'picked': {
            const { images: next, discarded } = appendPickedImages(images, result.images);
            const accepted = next.length - images.length;
            setImages(next);
            setOutcomes((prev) => [...prev, ...Array.from({ length: accepted }, () => undefined)]);
            if (discarded > 0) {
              setPickMessage(`最多 ${MAX_UPLOAD_IMAGES} 张,已忽略多选的 ${discarded} 张`);
            }
            break;
          }
          case 'cancelled':
            break;
          case 'denied':
            setPickMessage('需要相机/相册权限才能选择检查单照片');
            break;
        }
      } catch (error) {
        setPickMessage(error instanceof Error ? error.message : String(error));
      } finally {
        setPicking(false);
      }
    },
    [images, phase, picking],
  );

  const onRemove = useCallback(
    (index: number) => {
      if (phase !== 'idle') {
        return;
      }
      setImages((prev) => prev.filter((_, itemIndex) => itemIndex !== index));
      setOutcomes((prev) => prev.filter((_, itemIndex) => itemIndex !== index));
    },
    [phase],
  );

  const members = state.status === 'ready' ? sortMembers(state.members) : [];
  // 仅当持久化的当前成员仍在服务端列表里才算已选择(清库/换后端后旧 id 可能已失效)。
  const selectedMemberId = members.find((m) => m.member_id === currentMemberId)?.member_id;

  const runUpload = useCallback(
    async (indexes: number[]) => {
      if (selectedMemberId === undefined || indexes.length === 0 || phase === 'uploading') {
        return;
      }
      const memberId = selectedMemberId;
      setPickMessage(undefined);
      setPhase('uploading');
      setElapsedMs(0);
      setActiveIndex(undefined);

      let resolved: ResolvedApiClient | undefined;
      try {
        resolved = await resolveApiClient();
      } catch (error) {
        setState({ status: 'error', detail: error instanceof Error ? error.message : String(error) });
        setPhase('idle');
        return;
      }
      if (!resolved) {
        setState({ status: 'unconfigured' });
        setPhase('idle');
        return;
      }

      setOutcomes((prev) =>
        prev.map((outcome, index) => (indexes.includes(index) ? undefined : outcome)),
      );
      const client = resolved.client;
      try {
        await uploadImagesSerially(
          images,
          indexes,
          (image) => uploadReport(client, memberId, image),
          {
            onStart: setActiveIndex,
            onSettled: (index, outcome) =>
              setOutcomes((prev) => {
                const next = [...prev];
                next[index] = outcome;
                return next;
              }),
          },
        );
      } finally {
        setActiveIndex(undefined);
        setPhase('done');
      }
    },
    [selectedMemberId, images, phase],
  );

  const settled = outcomes.filter((outcome): outcome is UploadOutcome => outcome !== undefined);
  const retryIndexes = outcomes.flatMap((outcome, index) =>
    outcome?.kind === 'failed' ? [index] : [],
  );
  const canUpload =
    state.status === 'ready' &&
    members.length > 0 &&
    selectedMemberId !== undefined &&
    images.length > 0 &&
    phase === 'idle' &&
    !picking;
  const pickDisabled = picking || images.length >= MAX_UPLOAD_IMAGES;

  const onUpload = useCallback(() => {
    if (!canUpload) {
      return;
    }
    void runUpload(images.map((_, index) => index));
  }, [canUpload, images, runUpload]);

  const onRetry = useCallback(() => {
    void runUpload(retryIndexes);
  }, [retryIndexes, runUpload]);

  const onContinue = useCallback(() => {
    setImages([]);
    setOutcomes([]);
    setPhase('idle');
    setActiveIndex(undefined);
    setPickMessage(undefined);
    setElapsedMs(0);
  }, []);

  const summary = summaryText(settled);
  const uploadFailed = summaryFailed(settled);

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
                selectedId={selectedMemberId}
                disabled={phase !== 'idle'}
                onSelect={selectMember}
              />
              {selectedMemberId === undefined ? (
                <Text style={sharedStyles.muted}>先选择检查单归属哪位成员。</Text>
              ) : null}

              <Text style={sharedStyles.label}>检查单照片</Text>
              {images.length > 0 ? (
                <Text testID="upload-selected-count" style={sharedStyles.muted}>
                  已选 {images.length}/{MAX_UPLOAD_IMAGES} 张
                </Text>
              ) : null}
              {images.map((image, index) => (
                <UploadRow
                  key={`${image.uri}-${index}`}
                  image={image}
                  index={index}
                  outcome={outcomes[index]}
                  phase={phase}
                  active={activeIndex === index}
                  onRemove={onRemove}
                />
              ))}
              {phase === 'idle' ? (
                <View style={sharedStyles.horizontal}>
                  <Pressable
                    style={styles.buttonGhost}
                    onPress={() => void onPick('camera')}
                    disabled={pickDisabled}
                  >
                    <Text style={pickDisabled ? styles.disabledText : styles.link}>拍照</Text>
                  </Pressable>
                  <Pressable
                    testID="upload-pick-library"
                    style={styles.buttonGhost}
                    onPress={() => void onPick('library')}
                    disabled={pickDisabled}
                  >
                    <Text style={pickDisabled ? styles.disabledText : styles.link}>
                      从相册选择
                    </Text>
                  </Pressable>
                </View>
              ) : null}
              {pickMessage ? <Text style={sharedStyles.muted}>{pickMessage}</Text> : null}

              {phase !== 'done' ? (
                <Pressable
                  testID="upload-submit"
                  style={[styles.button, !canUpload && styles.buttonDisabled]}
                  onPress={onUpload}
                  disabled={!canUpload}
                >
                  <Text style={styles.buttonText}>
                    {images.length > 1 ? `上传并识别(${images.length} 张)` : '上传并识别'}
                  </Text>
                </Pressable>
              ) : null}

              {phase === 'uploading' ? (
                <View style={sharedStyles.horizontal}>
                  <ActivityIndicator />
                  <View style={styles.progress}>
                    <Text testID="upload-progress">
                      {activeIndex === undefined
                        ? '正在准备上传…'
                        : `正在识别 ${activeIndex + 1}/${images.length}`}
                    </Text>
                    <Text style={sharedStyles.muted}>{uploadWaitText(elapsedMs)}</Text>
                  </View>
                </View>
              ) : null}

              {phase === 'done' && settled.length > 0 ? (
                <View style={styles.card}>
                  <Text testID="upload-result" style={uploadFailed ? styles.fail : styles.success}>
                    {summary}
                  </Text>
                  <View style={styles.actions}>
                    {retryIndexes.length > 0 ? (
                      <Pressable testID="upload-retry" style={styles.button} onPress={onRetry}>
                        <Text style={styles.buttonText}>重试失败的 {retryIndexes.length} 张</Text>
                      </Pressable>
                    ) : null}
                    <Link asChild href="/timeline">
                      <Pressable testID="upload-open-timeline">
                        <Text style={styles.link}>去档案查看</Text>
                      </Pressable>
                    </Link>
                    <Pressable
                      testID="upload-continue"
                      style={styles.buttonGhost}
                      onPress={onContinue}
                    >
                      <Text style={styles.link}>继续上传</Text>
                    </Pressable>
                  </View>
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
  review: { fontSize: 13, color: colors.accent, lineHeight: 20 },
  link: { color: colors.accent, fontSize: 15 },
  disabledText: { color: colors.muted, fontSize: 15 },
  card: { borderWidth: 1, borderColor: colors.border, borderRadius: 8, padding: 12, gap: 8 },
  actions: { flexDirection: 'row', alignItems: 'center', flexWrap: 'wrap', gap: 8 },
  progress: { flex: 1, gap: 2 },
  row: { flexDirection: 'row', alignItems: 'center', gap: 12 },
  rowBody: { flex: 1, gap: 2 },
  fileName: { fontSize: 13, color: colors.text },
  rowStatus: { fontSize: 13, lineHeight: 20 },
  thumb: { width: 64, height: 64, borderRadius: 8, borderWidth: 1, borderColor: colors.border },
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
