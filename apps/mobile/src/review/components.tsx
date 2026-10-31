// 核对交互组件(票 09):指标项核对卡(确认/修正/编辑器;核对页与详情页共用)与
// OCR 行区域缩放对照 LineZoom。flag 只展示服务端返回值(修正后由服务端按值×范围
// 重算),无手改入口;修正字段限 name/value/unit/ref_range 四项纸面事实。
import { useState } from 'react';
import { Pressable, StyleSheet, Text, TextInput, View } from 'react-native';

import type { ApiClient } from '../api/client';
import { reviewItem, type ReportItem, type ReviewResult } from '../api/reports';
import { LineZoom } from '../components/LineZoom';
import { colors } from '../components/theme';
import { FLAG_COLORS } from '../reports/colors';
import { parseNumericForm, parseRefRange, stripLayout, type StripLayout } from '../reports/refRange';
import { itemFlagLabel, MISSING_REF_RANGE_LABEL } from '../reports/status';
import {
  buildCorrection,
  canMarkNoReference,
  displayItemName,
  displayItemUnit,
  draftFromItem,
  flagReasonText,
  reviewActions,
  reviewStatusLabel,
  shouldShowZoom,
  unitStatusLabel,
  type CorrectionDraft,
} from './items';

// ReviewItemCard 指标项核对卡:与详情指标项同一展示(名称/值/单位/参考范围/条带/
// 原文/低置信高亮)+ 核对状态标签、确认/修正动作与四项纸面事实编辑器。
// 低置信项默认展开 LineZoom;高置信项也可点「查看原图定位」展开(无需先点修正)。
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
  const [draft, setDraft] = useState<CorrectionDraft>();
  const [zoomOpen, setZoomOpen] = useState<boolean>();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();

  // 非编辑态直接用最新生效值;编辑态保留用户输入(单响应刷新不覆盖正在编辑的草稿)。
  const currentDraft = editing ? (draft ?? draftFromItem(item)) : draftFromItem(item);
  const correction = buildCorrection(item, currentDraft);
  const layout = stripLayout(parseRefRange(item.ref_range), parseNumericForm(item.value));
  const flagColor = FLAG_COLORS[item.flag];
  const statusLabel = reviewStatusLabel(item.review_status);
  const showZoom = shouldShowZoom(item, { open: zoomOpen, editing });
  const actions = reviewActions(item);
  const displayName = displayItemName(item);
  const displayUnit = displayItemUnit(item);
  const rawUnit = item.raw_unit ? `原文单位 ${item.raw_unit}` : null;
  const unitMeta = [rawUnit, unitStatusLabel(item.unit_status)].filter(Boolean).join(' · ');

  const submit = async (request: Parameters<typeof reviewItem>[3]) => {
    setBusy(true);
    setError(undefined);
    try {
      onUpdated(await reviewItem(client, reportId, item.item_id, request));
      setEditing(false);
      setDraft(undefined);
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
        value={currentDraft[key]}
        onChangeText={(text) =>
          setDraft((prev) => ({ ...(prev ?? draftFromItem(item)), [key]: text }))
        }
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
        <Text style={styles.itemName}>{displayName}</Text>
        {item.low_confidence ? <Text style={styles.lowTag}>低置信</Text> : null}
        {statusLabel ? <Text style={styles.reviewTag}>{statusLabel}</Text> : null}
        <Text style={[styles.itemFlag, { color: flagColor }]}>{itemFlagLabel(item)}</Text>
      </View>
      <View style={styles.itemValueRow}>
        <Text style={[styles.itemValue, { color: flagColor }]}>
          {item.value ?? '—'}
          {displayUnit ? <Text style={styles.itemUnit}> {displayUnit}</Text> : null}
        </Text>
        {item.ref_range ? (
          <Text style={styles.itemRef}>参考 {item.ref_range}</Text>
        ) : (
          <Text style={styles.itemRef}>{MISSING_REF_RANGE_LABEL}</Text>
        )}
      </View>
      {layout ? <MetricStrip layout={layout} color={flagColor} /> : null}
      {item.flag === 'unknown' && flagReasonText(item.flag_reason) ? (
        <Text testID={`review-reason-${item.item_id}`} style={styles.itemReason}>
          {flagReasonText(item.flag_reason)}
        </Text>
      ) : null}
      {showZoom ? (
        <LineZoom
          testID={`review-zoom-${item.item_id}`}
          imageUrl={imageUrl}
          lines={item.ocr_lines}
        />
      ) : null}
      <Text style={styles.itemRaw}>原文:{item.raw_text}</Text>
      {unitMeta ? (
        <Text testID={`review-unit-meta-${item.item_id}`} style={styles.itemRaw}>
          {unitMeta}
        </Text>
      ) : null}
      {item.ocr_lines.length > 0 && !editing ? (
        <Pressable
          testID={`review-zoom-toggle-${item.item_id}`}
          onPress={() => setZoomOpen(!showZoom)}
        >
          <Text style={styles.zoomLink}>
            {showZoom ? '收起原图定位' : '查看原图定位'}
          </Text>
        </Pressable>
      ) : null}

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
                setDraft(undefined);
              }}
            >
              <Text style={styles.actionGhostText}>取消</Text>
            </Pressable>
          </View>
        </View>
      ) : (
        <View style={styles.actions}>
          {/* 可操作 unknown 只给补录主操作;低置信未核对给确认;其余无 filled。
              每卡最多一个 filled primary(票 02 口径)。 */}
          {actions.primary === 'confirm' ? (
            <Pressable
              testID={`review-confirm-${item.item_id}`}
              style={[styles.actionPrimary, busy && styles.actionDisabled]}
              disabled={busy}
              onPress={() => void submit({ action: 'confirm' })}
            >
              <Text style={styles.actionPrimaryText}>确认</Text>
            </Pressable>
          ) : null}
          {actions.primary === 'backfill' ? (
            <Pressable
              testID={`review-backfill-${item.item_id}`}
              style={[styles.actionPrimary, busy && styles.actionDisabled]}
              disabled={busy}
              onPress={() => {
                setDraft(undefined);
                setEditing(true);
              }}
            >
              <Text style={styles.actionPrimaryText}>补录缺失信息</Text>
            </Pressable>
          ) : null}
          {actions.primary !== 'backfill' ? (
            <Pressable
              testID={`review-correct-${item.item_id}`}
              style={styles.actionGhost}
              disabled={busy}
              onPress={() => {
                setDraft(undefined);
                setEditing(true);
              }}
            >
              <Text style={styles.actionGhostText}>{actions.correctLabel}</Text>
            </Pressable>
          ) : null}
          {canMarkNoReference(item) ? (
            <Pressable
              testID={`review-no-reference-${item.item_id}`}
              style={styles.actionGhost}
              disabled={busy}
              onPress={() => void submit({ action: 'no_reference' })}
            >
              <Text style={styles.actionGhostText}>标记无参考范围</Text>
            </Pressable>
          ) : null}
        </View>
      )}
      {actions.helper ? <Text style={styles.itemHelper}>{actions.helper}</Text> : null}
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
  itemReason: { fontSize: 12, color: colors.muted, lineHeight: 18 },
  itemHelper: { fontSize: 12, color: colors.muted, lineHeight: 18 },
  itemRaw: { fontSize: 12, color: colors.muted, lineHeight: 18 },
  zoomLink: { fontSize: 12, color: colors.accent },
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
