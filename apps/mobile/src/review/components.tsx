// 核对交互组件(票 09):指标项核对卡(确认/修正/编辑器;核对页与详情页共用)与
// OCR 行区域缩放对照 LineZoom。flag 只展示服务端返回值(修正后由服务端按值×范围
// 重算),无手改入口;修正字段限 name/value/unit/ref_range 四项纸面事实。
import { useEffect, useState } from 'react';
import { Pressable, StyleSheet, Text, TextInput, View } from 'react-native';

import type { ApiClient } from '../api/client';
import { reviewItem, type ReportItem, type ReviewResult } from '../api/reports';
import { LineZoom } from '../components/LineZoom';
import { colors } from '../components/theme';
import { parseNumericForm, parseRefRange, stripLayout, type StripLayout } from '../reports/refRange';
import { flagLabel } from '../reports/status';
import { buildCorrection, draftFromItem, reviewStatusLabel, type CorrectionDraft } from './items';

// flag 色板(票 07 实施期定,详情/核对共用):normal 绿 / high 红 / low 蓝 /
// unknown 中性灰。只表示与参考范围的比较结果,不暗示诊断。
const FLAG_COLORS: Record<ReportItem['flag'], string> = {
  normal: colors.ok,
  high: colors.fail,
  low: colors.low,
  unknown: colors.muted,
};

// ReviewItemCard 指标项核对卡:与详情指标项同一展示(名称/值/单位/参考范围/条带/
// 原文/低置信高亮)+ 核对状态标签、确认/修正动作与四项纸面事实编辑器。
// 低置信项(或编辑中)带 LineZoom 三方对照的「图像缩放至 OCR 行区域」一侧。
export function ReviewItemCard({
  item,
  reportId,
  client,
  imageUrl,
  onUpdated,
}: {
  item: ReportItem;
  reportId: string;
  client: ApiClient;
  imageUrl: string;
  onUpdated: (result: ReviewResult) => void;
}) {
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState<CorrectionDraft>(() => draftFromItem(item));
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();

  // 非编辑态跟随最新生效值(单响应刷新后草稿复位;编辑中保留用户输入)。
  useEffect(() => {
    if (!editing) {
      setDraft(draftFromItem(item));
    }
  }, [item, editing]);

  const correction = buildCorrection(item, draft);
  const layout = stripLayout(parseRefRange(item.ref_range), parseNumericForm(item.value));
  const flagColor = FLAG_COLORS[item.flag];
  const statusLabel = reviewStatusLabel(item.review_status);
  const showZoom = (item.low_confidence || editing) && item.ocr_lines.length > 0;

  const submit = async (request: Parameters<typeof reviewItem>[3]) => {
    setBusy(true);
    setError(undefined);
    try {
      onUpdated(await reviewItem(client, reportId, item.item_id, request));
      setEditing(false);
    } catch (submitError) {
      setError(submitError instanceof Error ? submitError.message : String(submitError));
    } finally {
      setBusy(false);
    }
  };

  const field = (key: keyof CorrectionDraft, label: string) => (
    <View style={styles.editorField} key={key}>
      <Text style={styles.editorLabel}>{label}</Text>
      <TextInput
        testID={`review-field-${key}-${item.item_id}`}
        style={styles.editorInput}
        value={draft[key]}
        onChangeText={(text) => setDraft((prev) => ({ ...prev, [key]: text }))}
        placeholder={label}
        placeholderTextColor={colors.muted}
      />
    </View>
  );

  return (
    <View
      testID={`review-item-${item.item_id}`}
      style={[styles.itemCard, item.low_confidence && styles.itemCardLowConfidence]}
    >
      <View style={styles.itemHeader}>
        <Text style={styles.itemName}>{item.name}</Text>
        {item.low_confidence ? <Text style={styles.lowTag}>低置信</Text> : null}
        {statusLabel ? <Text style={styles.reviewTag}>{statusLabel}</Text> : null}
        <Text style={[styles.itemFlag, { color: flagColor }]}>{flagLabel(item.flag)}</Text>
      </View>
      <View style={styles.itemValueRow}>
        <Text style={[styles.itemValue, { color: flagColor }]}>
          {item.value ?? '—'}
          {item.unit ? <Text style={styles.itemUnit}> {item.unit}</Text> : null}
        </Text>
        {item.ref_range ? <Text style={styles.itemRef}>参考 {item.ref_range}</Text> : null}
      </View>
      {layout ? <MetricStrip layout={layout} color={flagColor} /> : null}
      {showZoom ? <LineZoom imageUrl={imageUrl} lines={item.ocr_lines} /> : null}
      <Text style={styles.itemRaw}>原文:{item.raw_text}</Text>

      {editing ? (
        <View style={styles.editor}>
          {field('name', '名称')}
          {field('value', '数值')}
          {field('unit', '单位')}
          {field('ref_range', '参考范围')}
          <View style={styles.actions}>
            <Pressable
              testID={`review-save-${item.item_id}`}
              style={[styles.actionPrimary, (!correction || busy) && styles.actionDisabled]}
              disabled={!correction || busy}
              onPress={() => correction && void submit(correction)}
            >
              <Text style={styles.actionPrimaryText}>保存修正</Text>
            </Pressable>
            <Pressable
              testID={`review-cancel-${item.item_id}`}
              style={styles.actionGhost}
              disabled={busy}
              onPress={() => {
                setEditing(false);
                setDraft(draftFromItem(item));
              }}
            >
              <Text style={styles.actionGhostText}>取消</Text>
            </Pressable>
          </View>
        </View>
      ) : (
        <View style={styles.actions}>
          {/* 确认是对「未确信」项的动作:已确认后再点是无变化空操作,禁用;
              已修正项仍可再确认/再修正(决策可重复,ADR-0005)。 */}
          <Pressable
            testID={`review-confirm-${item.item_id}`}
            style={[
              styles.actionPrimary,
              (busy || item.review_status === 'confirmed') && styles.actionDisabled,
            ]}
            disabled={busy || item.review_status === 'confirmed'}
            onPress={() => void submit({ action: 'confirm' })}
          >
            <Text style={styles.actionPrimaryText}>确认</Text>
          </Pressable>
          <Pressable
            testID={`review-correct-${item.item_id}`}
            style={styles.actionGhost}
            disabled={busy}
            onPress={() => setEditing(true)}
          >
            <Text style={styles.actionGhostText}>修正</Text>
          </Pressable>
        </View>
      )}
      {error ? <Text style={styles.error}>核对失败:{error}</Text> : null}
    </View>
  );
}

// MetricStrip 参考范围条带(票 07 画法):条 = 正常区(中性灰;标量参考退化为细
// 基准线),点 = 当前值(flag 色;空心 = 带不等号的非精确值,置于条端外),
// 跨度条 = 区间值(spec:区间无点)。几何全部来自 stripLayout(纯逻辑)。
function MetricStrip({ layout, color }: { layout: StripLayout; color: string }) {
  const { band, marker } = layout;
  const scalarBand = band.start === band.end;
  return (
    <View style={styles.stripTrack}>
      <View
        style={[
          styles.stripBand,
          scalarBand && styles.stripBandLine,
          {
            left: `${band.start * 100}%`,
            width: scalarBand ? 2 : `${(band.end - band.start) * 100}%`,
          },
        ]}
      />
      {marker?.kind === 'span' ? (
        <View
          style={[
            styles.stripSpan,
            {
              left: `${marker.start * 100}%`,
              width: `${(marker.end - marker.start) * 100}%`,
              backgroundColor: color,
            },
          ]}
        />
      ) : null}
      {marker?.kind === 'dot' ? (
        <View
          style={[
            styles.stripDot,
            { left: `${marker.at * 100}%`, borderColor: color },
            marker.open ? styles.stripDotOpen : { backgroundColor: color },
          ]}
        />
      ) : null}
    </View>
  );
}

const styles = StyleSheet.create({
  itemCard: {
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 8,
    padding: 12,
    gap: 6,
  },
  itemCardLowConfidence: { borderColor: colors.accent },
  itemHeader: { flexDirection: 'row', alignItems: 'center', gap: 8 },
  itemName: { flex: 1, fontSize: 15, color: colors.text },
  lowTag: {
    fontSize: 11,
    color: colors.accent,
    borderWidth: 1,
    borderColor: colors.accent,
    borderRadius: 8,
    paddingHorizontal: 6,
    paddingVertical: 1,
  },
  reviewTag: {
    fontSize: 11,
    color: colors.ok,
    borderWidth: 1,
    borderColor: colors.ok,
    borderRadius: 8,
    paddingHorizontal: 6,
    paddingVertical: 1,
  },
  itemFlag: { fontSize: 13 },
  itemValueRow: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'baseline',
    gap: 8,
  },
  itemValue: { fontSize: 20, fontVariant: ['tabular-nums'] },
  itemUnit: { fontSize: 13 },
  itemRef: { fontSize: 13, color: colors.muted },
  itemRaw: { fontSize: 12, color: colors.muted, lineHeight: 18 },
  stripTrack: {
    height: 12,
    borderRadius: 6,
    backgroundColor: colors.border,
    marginTop: 4,
  },
  stripBand: {
    position: 'absolute',
    top: 0,
    bottom: 0,
    backgroundColor: colors.muted,
    borderRadius: 6,
  },
  stripBandLine: { marginLeft: -1, borderRadius: 1 },
  stripSpan: { position: 'absolute', top: 4, height: 4, borderRadius: 2 },
  stripDot: {
    position: 'absolute',
    top: 1,
    width: 10,
    height: 10,
    borderRadius: 5,
    borderWidth: 2,
    marginLeft: -5,
  },
  stripDotOpen: { backgroundColor: colors.background },
  editor: { gap: 8, marginTop: 4 },
  editorField: { gap: 4 },
  editorLabel: { fontSize: 12, color: colors.muted },
  editorInput: {
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 6,
    paddingHorizontal: 10,
    paddingVertical: 6,
    fontSize: 15,
    color: colors.text,
  },
  actions: { flexDirection: 'row', alignItems: 'center', gap: 12, marginTop: 4 },
  actionPrimary: {
    paddingVertical: 8,
    paddingHorizontal: 16,
    backgroundColor: colors.accent,
    borderRadius: 8,
  },
  actionPrimaryText: { color: colors.onAccent, fontSize: 14 },
  actionDisabled: { backgroundColor: colors.border },
  actionGhost: { paddingVertical: 8, paddingHorizontal: 12 },
  actionGhostText: { color: colors.accent, fontSize: 14 },
  error: { fontSize: 12, color: colors.fail, lineHeight: 18 },
});
