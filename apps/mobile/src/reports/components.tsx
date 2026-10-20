// 报告展示组件:时间线卡片(票 06)与报告级徽标(票 07 起时间线与详情头部共用)。
// 时间线卡片含类别、日期、状态文案、异常徽标与「待核对 N 项」;failed 展示失败原因。
// 指标项卡(条带/原文/核对动作)在 review/components(票 09 起,详情与核对页共用)。
// 自愈操作卡 ReportManageCard(票 10):改归属(选中目标成员即刻生效,时间线移位)
// 与删除(确认步骤防误触);成败经回调上抛页面(状态合并与导航在页面)。
import { useState } from 'react';
import { ActivityIndicator, Pressable, StyleSheet, Text, View } from 'react-native';

import type { ApiClient } from '../api/client';
import { listMembers, type Member } from '../api/members';
import {
  deleteReport,
  reassignReport,
  type ReportSection,
  type ReportSummary,
} from '../api/reports';
import { LineZoom } from '../components/LineZoom';
import { sharedStyles } from '../components/styles';
import { colors } from '../components/theme';
import { MemberChips } from '../members/components';
import { sortMembers } from '../members/order';
import { needsReassign, reassignConfirmText } from './manage';
import { EXAM_NAME_KIND, SECTION_LABELS } from './sections';
import { reportBadgeLabels, reportDateLabel, reportStatusLabel } from './status';

// 报告状态色:时间线卡片与详情头部共用。
export const STATUS_COLORS: Record<ReportSummary['status'], string> = {
  success: colors.ok,
  partial: colors.accent,
  failed: colors.fail,
};

// ReportBadges 异常徽标(偏高/偏低/未知)+「待核对 N 项」;时间线与详情头部共用。
// 徽标口径由报告类别派生(影像单无指标项、无核对入口:未知与待核对都不显示)。
export function ReportBadges({ report }: { report: ReportSummary }) {
  const { flags, unreviewed } = reportBadgeLabels(report);
  if (flags.length === 0 && !unreviewed) {
    return null;
  }
  return (
    <View style={styles.badges}>
      {flags.map((badge) => (
        <Text key={badge} style={styles.badge}>
          {badge}
        </Text>
      ))}
      {unreviewed ? <Text style={[styles.badge, styles.reviewBadge]}>{unreviewed}</Text> : null}
    </View>
  );
}

export function ReportCard({
  report,
  examName,
}: {
  report: ReportSummary;
  /** 影像单检查项目(票 02):仅详情页传入,时间线/核对页头部保持原样。 */
  examName?: string;
}) {
  const failed = report.status === 'failed';
  const status = reportStatusLabel(report);
  return (
    <View testID={`report-card-${report.report_id}`} style={styles.card}>
      <View style={styles.headerRow}>
        <Text style={styles.type}>{report.report_type}</Text>
        {status ? (
          <Text style={[styles.status, { color: STATUS_COLORS[report.status] }]}>{status}</Text>
        ) : null}
      </View>
      {examName ? (
        <Text testID="report-exam-name" style={styles.examName}>
          {SECTION_LABELS[EXAM_NAME_KIND]}:{examName}
        </Text>
      ) : null}
      <Text style={styles.date}>{reportDateLabel(report)}</Text>
      {failed && report.fail_reason ? <Text style={styles.fail}>{report.fail_reason}</Text> : null}
      <ReportBadges report={report} />
    </View>
  );
}

// ReportSectionCard 报告段落卡(票 02;影像单检查所见 / 超声提示):整段原文按
// sections 数组序展示;点卡片在同位置展开 LineZoom,按该段 ocr_lines 缩放到原图
// 对应区域(与核对项三方对照共用实现)。
export function ReportSectionCard({
  index,
  label,
  section,
  imageUrl,
}: {
  index: number;
  label: string;
  section: ReportSection;
  imageUrl: string;
}) {
  const [expanded, setExpanded] = useState(false);
  return (
    <Pressable
      testID={`report-section-${index}`}
      style={styles.sectionCard}
      onPress={() => setExpanded((prev) => !prev)}
    >
      <Text style={styles.sectionLabel}>{label}</Text>
      <Text style={styles.sectionText}>{section.text}</Text>
      {expanded ? (
        <LineZoom
          testID={`report-section-zoom-${index}`}
          imageUrl={imageUrl}
          lines={section.ocr_lines}
        />
      ) : null}
      <Text style={styles.sectionHint}>{expanded ? '收起原图对照' : '点击对照原图'}</Text>
    </Pressable>
  );
}

// ReportManageCard 报告自愈操作卡(详情页;票 10)。改归属:展开成员选择(孩子优先,
// 与时间线/上传同一排序)——点选只改选择态(与全 app 成员 chips 的「选择」语义一致),
// 「确认改派」才提交(确认行显式给出 from→to,提交后时间线上移到该成员;维护者
// 2026-10-18 增补);删除:确认步骤防误触,确认后报告与指标项、修正记录一并清除,
// 原始图像与识别历史保留(同一张图可以重新上传)。当前成员点按视为保持原样,不产生请求。
export function ReportManageCard({
  report,
  client,
  onReassigned,
  onDeleted,
}: {
  report: ReportSummary;
  client: ApiClient;
  onReassigned: (summary: ReportSummary) => void;
  onDeleted: () => void;
}) {
  const [mode, setMode] = useState<'reassign' | 'delete'>();
  const [pendingMemberId, setPendingMemberId] = useState<number>();
  const [members, setMembers] = useState<Member[]>();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();
  const [notice, setNotice] = useState<string>();

  const message = (e: unknown) => (e instanceof Error ? e.message : String(e));

  const loadMembers = () => {
    void (async () => {
      setBusy(true);
      setError(undefined);
      try {
        setMembers(sortMembers(await listMembers(client)));
      } catch (loadError) {
        setError(message(loadError));
      } finally {
        setBusy(false);
      }
    })();
  };

  const closePanel = () => {
    setMode(undefined);
    setPendingMemberId(undefined);
  };

  const openReassign = () => {
    setMode('reassign');
    setPendingMemberId(undefined);
    setNotice(undefined);
    setError(undefined);
    if (members === undefined) {
      loadMembers();
    }
  };

  // 点选 = 选择态:目标与当前归属相同(点当前成员)即清除选择,不产生请求。
  const selectTarget = (memberId: number) => {
    setPendingMemberId(needsReassign(report.member_id, memberId) ? memberId : undefined);
    setError(undefined);
  };

  const confirmReassign = () => {
    if (pendingMemberId === undefined) {
      return;
    }
    const targetId = pendingMemberId;
    void (async () => {
      setBusy(true);
      setError(undefined);
      try {
        onReassigned(await reassignReport(client, report.report_id, targetId));
        const target = members?.find((m) => m.member_id === targetId);
        setNotice(`已改派给「${target?.name ?? targetId}」,时间线上已移到该成员名下。`);
        closePanel();
      } catch (submitError) {
        setError(message(submitError));
      } finally {
        setBusy(false);
      }
    })();
  };

  const confirmDelete = () => {
    void (async () => {
      setBusy(true);
      setError(undefined);
      try {
        await deleteReport(client, report.report_id);
        onDeleted();
      } catch (submitError) {
        setError(message(submitError));
      } finally {
        setBusy(false);
      }
    })();
  };

  const currentMember = members?.find((m) => m.member_id === report.member_id);
  const pendingTarget =
    pendingMemberId === undefined ? undefined : members?.find((m) => m.member_id === pendingMemberId);

  return (
    <View style={[styles.card, styles.manageCard]}>
      {mode === undefined ? (
        <View style={styles.manageActions}>
          <Pressable testID="report-manage-reassign" onPress={openReassign} disabled={busy}>
            <Text style={styles.manageLink}>改归属</Text>
          </Pressable>
          <Pressable
            testID="report-manage-delete"
            onPress={() => {
              setMode('delete');
              setNotice(undefined);
              setError(undefined);
            }}
            disabled={busy}
          >
            <Text style={styles.manageDanger}>删除</Text>
          </Pressable>
        </View>
      ) : null}

      {mode === 'reassign' ? (
        <View style={styles.managePanel}>
          <Text style={sharedStyles.label}>改派给谁</Text>
          {members === undefined ? (
            busy ? (
              <View style={sharedStyles.horizontal}>
                <ActivityIndicator />
                <Text style={sharedStyles.muted}>正在加载成员…</Text>
              </View>
            ) : (
              <Pressable onPress={loadMembers}>
                <Text style={styles.manageLink}>重试加载成员</Text>
              </Pressable>
            )
          ) : (
            <MemberChips
              members={members}
              selectedId={pendingMemberId ?? report.member_id ?? undefined}
              disabled={busy}
              onSelect={selectTarget}
            />
          )}
          {pendingTarget ? (
            <View style={styles.managePanel}>
              <Text style={sharedStyles.muted}>
                {reassignConfirmText(currentMember?.name ?? null, pendingTarget.name)}
              </Text>
              <View style={styles.manageActions}>
                <Pressable
                  testID="report-manage-confirm-reassign"
                  style={[
                    styles.manageButton,
                    styles.managePrimaryFill,
                    busy && styles.manageButtonDisabled,
                  ]}
                  disabled={busy}
                  onPress={confirmReassign}
                >
                  <Text style={styles.manageButtonText}>确认改派</Text>
                </Pressable>
                <Pressable onPress={closePanel} disabled={busy}>
                  <Text style={styles.manageLink}>取消</Text>
                </Pressable>
              </View>
            </View>
          ) : (
            <Pressable onPress={closePanel} disabled={busy}>
              <Text style={styles.manageLink}>取消</Text>
            </Pressable>
          )}
        </View>
      ) : null}

      {mode === 'delete' ? (
        <View style={styles.managePanel}>
          <Text style={sharedStyles.muted}>
            删除后:报告与指标项、修正记录一并清除;原始图像与识别历史保留,同一张图可以重新上传。
          </Text>
          <View style={styles.manageActions}>
            <Pressable
              testID="report-manage-confirm-delete"
              style={[styles.manageButton, styles.manageDangerFill, busy && styles.manageButtonDisabled]}
              disabled={busy}
              onPress={confirmDelete}
            >
              <Text style={styles.manageButtonText}>确认删除</Text>
            </Pressable>
            <Pressable onPress={closePanel} disabled={busy}>
              <Text style={styles.manageLink}>取消</Text>
            </Pressable>
          </View>
        </View>
      ) : null}

      {notice ? <Text testID="report-manage-notice" style={styles.success}>{notice}</Text> : null}
      {error ? <Text testID="report-manage-error" style={styles.fail}>操作失败:{error}</Text> : null}
    </View>
  );
}

const styles = StyleSheet.create({
  card: {
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 8,
    padding: 12,
    gap: 4,
  },
  headerRow: { flexDirection: 'row', justifyContent: 'space-between', alignItems: 'center', gap: 8 },
  type: { fontSize: 16, color: colors.text },
  status: { fontSize: 13 },
  date: { fontSize: 13, color: colors.muted },
  examName: { fontSize: 14, color: colors.text, lineHeight: 20 },
  sectionCard: {
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 8,
    padding: 12,
    gap: 6,
  },
  sectionLabel: { fontSize: 13, color: colors.muted },
  sectionText: { fontSize: 14, color: colors.text, lineHeight: 21 },
  sectionHint: { fontSize: 12, color: colors.accent },
  fail: { fontSize: 13, color: colors.fail },
  badges: { flexDirection: 'row', flexWrap: 'wrap', gap: 6, marginTop: 4 },
  badge: {
    fontSize: 12,
    color: colors.text,
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 10,
    paddingHorizontal: 8,
    paddingVertical: 2,
  },
  reviewBadge: { borderColor: colors.accent, color: colors.accent },
  manageCard: { gap: 8 },
  manageActions: { flexDirection: 'row', alignItems: 'center', gap: 16 },
  managePanel: { gap: 8 },
  manageLink: { color: colors.accent, fontSize: 15 },
  manageDanger: { color: colors.fail, fontSize: 15 },
  manageButton: {
    alignSelf: 'flex-start',
    paddingVertical: 10,
    paddingHorizontal: 16,
    borderRadius: 8,
  },
  managePrimaryFill: { backgroundColor: colors.accent },
  manageDangerFill: { backgroundColor: colors.fail },
  manageButtonText: { color: colors.onAccent, fontSize: 15 },
  manageButtonDisabled: { backgroundColor: colors.border },
  success: { fontSize: 13, color: colors.ok, lineHeight: 20 },
});
