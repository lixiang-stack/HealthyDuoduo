import { Link, router, useLocalSearchParams } from "expo-router";
import { useState } from "react";
import {
  ActivityIndicator,
  Image,
  Pressable,
  ScrollView,
  StyleSheet,
  Text,
  View,
} from "react-native";

import { AnalysisConsentModal } from "../../analysis/ConsentModal";
import { TimeRangeModal } from "../../analysis/TimeRangeModal";
import { useStartAnalysis } from "../../analysis/useStartAnalysis";
import { sharedStyles } from "../../components/styles";
import { colors } from "../../components/theme";
import { useImageViewer } from "../../components/imageViewer/ImageViewerProvider";
import { useCurrentMember } from "../../members/CurrentMemberContext";
import { isClinicalNoteReport } from "../../reports/category";
import { CLINICAL_NOTE_PANEL } from "../../reports/panels";
import {
  ReportCard,
  ReportManageCard,
  ReportSectionCard,
} from "../../reports/components";
import {
  abnormalItemCount,
  reportItemGroups,
} from "../../reports/panelOverview";
import { pendingItems } from "../../review/items";
import { ReviewItemCard } from "../../review/components";
import { examNameText, reportSectionCards } from "../../reports/sections";
import {
  useReportData,
  useReportDetailViewModel,
  type ReportDataState,
} from "../../reports/useReportData";
import type { ReportDetail } from "../../api/reports";

// 报告详情页(票 07;票 09 起为核对闭环的「详情顺手」入口;票 10 起含自愈操作):
// 头部含报告短 ID(前 12 hex,问题定位直接引用);每个指标项经 ReviewItemCard
// 可确认/修正(低置信与高置信一视同仁);有剩余待核对项时给出核对页入口;
// failed 展示图像与失败原因(无可核对项,出口 = 重新拍照);底部管理卡可改归属
// (时间线随之移位)与删除(确认步骤防误触;删除后回时间线,报告已不在)。
/** 详情页头部短 ID 长度(前 12 hex,git 风格短标识;完整值即 report_id)。 */
const SHORT_ID_LENGTH = 12;

export default function ReportDetailScreen() {
  const params = useLocalSearchParams();
  const reportId =
    typeof params.report_id === "string" ? params.report_id : undefined;
  const { state, reload, applyReview, applyReassign } = useReportData(reportId);
  const { currentMemberId } = useCurrentMember();
  const { open } = useImageViewer();
  // undefined = 按报告状态缺省(failed 展开、其余折叠);点按后由用户覆盖。
  const [imageOverride, setImageOverride] = useState<boolean>();

  const {
    report,
    narrative,
    imageNoun,
    pending,
    itemGroups,
    examName,
    sectionCards,
    imageVisible,
  } = useReportDetailViewModel(state, imageOverride);
  const analysis = useStartAnalysis(
    report?.member_id ?? currentMemberId,
    CLINICAL_NOTE_PANEL,
  );

  return (
    <>
      <ScrollView
        style={styles.container}
        contentContainerStyle={styles.content}
      >
        {state.status === "loading" ? (
          <View style={sharedStyles.horizontal}>
            <ActivityIndicator />
            <Text style={sharedStyles.muted}>正在加载报告…</Text>
          </View>
        ) : null}

        {state.status === "unconfigured" ? (
          <Text style={sharedStyles.muted}>
            尚未配置后端地址,请先到
            <Link href="/settings" style={styles.link}>
              设置
            </Link>
            填写。
          </Text>
        ) : null}

        {state.status === "error" ? (
          <View style={styles.stack}>
            <Text style={styles.fail}>加载失败:{state.detail}</Text>
            <Pressable style={styles.button} onPress={() => void reload()}>
              <Text style={styles.buttonText}>重试</Text>
            </Pressable>
          </View>
        ) : null}

        {state.status === "ready" && report ? (
          <ReportReadyView
            report={report}
            state={state}
            analysis={analysis}
            examName={examName}
            sectionCards={sectionCards}
            narrative={narrative}
            imageNoun={imageNoun}
            pending={pending}
            itemGroups={itemGroups}
            imageVisible={imageVisible}
            onToggleImage={() => setImageOverride(!imageVisible)}
            onOpenImage={() => open(state.imageUrl, imageNoun)}
            onReviewUpdated={applyReview}
            onReassigned={applyReassign}
            onDeleted={() => router.dismissTo("/timeline")}
          />
        ) : null}
      </ScrollView>
      <TimeRangeModal
        key={analysis.rangeOpen ? "range-open" : "range-closed"}
        visible={analysis.rangeOpen}
        busy={analysis.starting}
        onConfirm={(months) => void analysis.confirmRange(months)}
        onCancel={analysis.cancelRange}
      />
      <AnalysisConsentModal
        visible={analysis.pendingConsent}
        busy={analysis.starting}
        onConfirm={() => void analysis.confirmConsent()}
        onCancel={analysis.cancelConsent}
      />
    </>
  );
}

type ReadyReportState = Extract<ReportDataState, { status: "ready" }>;

function ReportReadyView({
  report,
  state,
  analysis,
  examName,
  sectionCards,
  narrative,
  imageNoun,
  pending,
  itemGroups,
  imageVisible,
  onToggleImage,
  onOpenImage,
  onReviewUpdated,
  onReassigned,
  onDeleted,
}: {
  report: ReportDetail;
  state: ReadyReportState;
  analysis: ReturnType<typeof useStartAnalysis>;
  examName: ReturnType<typeof examNameText>;
  sectionCards: ReturnType<typeof reportSectionCards>;
  narrative: boolean;
  imageNoun: string;
  pending: ReturnType<typeof pendingItems>;
  itemGroups: ReturnType<typeof reportItemGroups>;
  imageVisible: boolean;
  onToggleImage: () => void;
  onOpenImage: () => void;
  onReviewUpdated: ReturnType<typeof useReportData>["applyReview"];
  onReassigned: ReturnType<typeof useReportData>["applyReassign"];
  onDeleted: () => void;
}) {
  return (
    <View style={styles.stack}>
      <ReportCard report={report} examName={examName} />
      <Text testID="report-short-id" style={styles.shortId}>
        报告 {report.report_id.slice(0, SHORT_ID_LENGTH)}
      </Text>

      {/* 影像单叙述段落(票 02):检查所见 / 超声提示在指标项上方;failed 影像单
              与失败原因、原图并存,不被下方空态遮挡。 */}
      {sectionCards.map(({ index, label, section }) => (
        <ReportSectionCard
          key={index}
          index={index}
          label={label}
          section={section}
          imageUrl={state.imageUrl}
        />
      ))}

      {report && isClinicalNoteReport(report) && report.member_id !== null ? (
        <View style={styles.stack}>
          <Pressable
            testID="report-analyze-note"
            accessibilityRole="button"
            style={styles.button}
            onPress={() =>
              void analysis.start("clinical_note", undefined, report.report_id)
            }
            disabled={analysis.starting}
          >
            <Text style={styles.buttonText}>
              {analysis.starting ? "正在解读…" : "解读这份病历"}
            </Text>
          </Pressable>
          {analysis.error ? (
            <Text style={styles.fail}>{analysis.error}</Text>
          ) : null}
        </View>
      ) : null}

      {report.status === "failed" ? (
        <View style={styles.card}>
          {!narrative ? (
            <Text style={sharedStyles.muted}>
              这张检查单没有识别出可核对的指标项。
            </Text>
          ) : null}
          <Link href="/upload" style={styles.link}>
            重新拍照上传(生成新报告,原报告与识别记录保留)
          </Link>
        </View>
      ) : null}

      {pending.length > 0 ? (
        <Link
          asChild
          href={{
            pathname: "/review/[report_id]",
            params: { report_id: report.report_id },
          }}
        >
          <Pressable testID="report-pending-entry">
            <Text style={styles.link}>逐项核对 {pending.length} 项</Text>
          </Pressable>
        </Link>
      ) : null}

      {!narrative && report.items.length > 0 ? (
        <Text testID="report-items-title" style={styles.sectionTitle}>
          指标项({report.items.length})
        </Text>
      ) : null}
      {itemGroups.map((group) => {
        const abnormal = abnormalItemCount(group.items);
        const groupKey = group.panel ?? "fallback";
        return (
          <View key={groupKey} style={styles.itemGroup}>
            <View
              testID={`report-panel-group-${groupKey}`}
              style={styles.groupHeader}
            >
              <Text style={styles.groupName}>{group.label}</Text>
              <Text style={sharedStyles.muted}>
                {group.items.length} 项 · {abnormal} 项异常
              </Text>
            </View>
            {group.items.map((item) => (
              <ReviewItemCard
                key={item.item_id}
                item={item}
                reportId={report.report_id}
                client={state.client}
                imageUrl={state.imageUrl}
                onUpdated={onReviewUpdated}
              />
            ))}
          </View>
        );
      })}

      <Pressable testID="report-image-toggle" onPress={onToggleImage}>
        <Text style={styles.link}>
          {imageVisible ? `收起${imageNoun}` : `查看${imageNoun}`}
        </Text>
      </Pressable>
      {imageVisible ? (
        <>
          <Pressable
            testID="report-image-open"
            accessibilityRole="button"
            accessibilityLabel={`放大查看${imageNoun}`}
            onPress={onOpenImage}
          >
            <Image
              testID="report-image"
              source={{ uri: state.imageUrl }}
              style={styles.image}
              resizeMode="contain"
            />
          </Pressable>
          <Text style={styles.hint}>点按原图放大查看</Text>
        </>
      ) : null}

      {/* 自愈操作(票 10):改归属(时间线移位)与删除(确认后回时间线,报告已不在)。 */}
      <ReportManageCard
        report={report}
        client={state.client}
        onReassigned={onReassigned}
        onDeleted={onDeleted}
      />
    </View>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: colors.background },
  content: { padding: 24, gap: 12 },
  stack: { gap: 12 },
  card: {
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 8,
    padding: 12,
    gap: 4,
  },
  sectionTitle: { fontSize: 13, color: colors.muted },
  itemGroup: { gap: 8 },
  groupHeader: {
    flexDirection: "row",
    justifyContent: "space-between",
    alignItems: "baseline",
    gap: 8,
    marginTop: 4,
  },
  groupName: { fontSize: 15, color: colors.text, fontWeight: "600" },
  shortId: { fontSize: 12, color: colors.muted },
  fail: { fontSize: 13, color: colors.fail, lineHeight: 20 },
  link: { color: colors.accent, fontSize: 15 },
  image: {
    width: "100%",
    height: 320,
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 8,
  },
  hint: { fontSize: 12, color: colors.muted },
  button: {
    alignSelf: "flex-start",
    paddingVertical: 10,
    paddingHorizontal: 16,
    backgroundColor: colors.accent,
    borderRadius: 8,
  },
  buttonText: { color: colors.onAccent, fontSize: 15 },
});
