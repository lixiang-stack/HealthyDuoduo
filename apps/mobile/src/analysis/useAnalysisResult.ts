// 解读结果页的数据与交互状态:加载、反馈、重新解读。
// 页面组件只负责组合，行为与原 AnalysisScreen 内联实现逐字一致。
import { router, useFocusEffect } from "expo-router";
import { useCallback, useRef, useState } from "react";

import {
  createAnalysis,
  getAnalysis,
  submitAnalysisFeedback,
  type AnalysisFacts,
  type AnalysisResult,
  type AnalysisWindowMonths,
} from "../api/analyses";
import { resolveApiClient } from "../api/resolveClient";
import { panelLabel, type PanelCode } from "../reports/panels";
import { deviationCoverage, indicatorLabels } from "./facts";
import type { FeedbackReason } from "./FeedbackModal";
import {
  explanationUnavailableNextStep,
  explanationUnavailableText,
} from "./framework";

export type AnalysisScreenState =
  | { status: "loading" }
  | { status: "unconfigured" }
  | { status: "error"; detail: string }
  | { status: "ready"; result: AnalysisResult };

export const CLINICAL_NOTE_RERUN_TEXT = "病历解读请回到原病历详情页重新发起。";

/** 加载分析结果;从报告详情返回时静默刷新,保留展开状态与滚动位置。 */
export function useAnalysisScreen(analysisId: number) {
  const [state, setState] = useState<AnalysisScreenState>({
    status: "loading",
  });
  const loadedAnalysisRef = useRef<number | undefined>(undefined);

  const load = useCallback(
    async (silent = false) => {
      if (!Number.isInteger(analysisId) || analysisId <= 0) {
        setState({ status: "error", detail: "解读编号无效" });
        return;
      }
      if (!silent) {
        setState({ status: "loading" });
      }
      const resolved = await resolveApiClient();
      if (!resolved) {
        setState({ status: "unconfigured" });
        return;
      }
      try {
        setState({
          status: "ready",
          result: await getAnalysis(resolved.client, analysisId),
        });
      } catch (error) {
        setState({
          status: "error",
          detail: error instanceof Error ? error.message : String(error),
        });
      }
    },
    [analysisId],
  );

  useFocusEffect(
    useCallback(() => {
      const silent = loadedAnalysisRef.current === analysisId;
      loadedAnalysisRef.current = analysisId;
      void load(silent);
    }, [analysisId, load]),
  );

  return { state, load };
}

/** 结果级 / 条目级反馈的弹层状态与提交。 */
export function useAnalysisFeedback(analysisId: number) {
  const [feedbackOpen, setFeedbackOpen] = useState(false);
  const [feedbackReason, setFeedbackReason] = useState<FeedbackReason | "">("");
  const [feedbackEntryId, setFeedbackEntryId] = useState<string>();
  const [feedbackBusy, setFeedbackBusy] = useState(false);
  const [feedbackDone, setFeedbackDone] = useState(false);
  const [feedbackError, setFeedbackError] = useState<string>();

  const openFeedback = useCallback((entryId?: string) => {
    setFeedbackEntryId(entryId);
    setFeedbackReason("");
    setFeedbackError(undefined);
    setFeedbackOpen(true);
  }, []);

  const submitFeedback = useCallback(async () => {
    if (feedbackReason.trim().length === 0) {
      return;
    }
    setFeedbackBusy(true);
    setFeedbackError(undefined);
    const resolved = await resolveApiClient();
    if (!resolved) {
      setFeedbackBusy(false);
      setFeedbackError("尚未配置后端地址。");
      return;
    }
    try {
      await submitAnalysisFeedback(resolved.client, analysisId, {
        reason: feedbackReason.trim(),
        ...(feedbackEntryId ? { entry_id: feedbackEntryId } : {}),
      });
      setFeedbackDone(true);
      setFeedbackOpen(false);
      setFeedbackReason("");
      setFeedbackEntryId(undefined);
    } catch (error) {
      setFeedbackError(error instanceof Error ? error.message : String(error));
    } finally {
      setFeedbackBusy(false);
    }
  }, [analysisId, feedbackEntryId, feedbackReason]);

  return {
    feedbackOpen,
    feedbackReason,
    feedbackEntryId,
    feedbackBusy,
    feedbackDone,
    feedbackError,
    openFeedback,
    submitFeedback,
    closeFeedback: () => setFeedbackOpen(false),
    setFeedbackReason,
  };
}

/** 切换时间范围并创建新快照;病历入口缺少 report_sha 时引导回原病历页。 */
export function useAnalysisRerun(
  result: AnalysisResult | undefined,
  facts: AnalysisFacts | undefined,
) {
  const [rangeOpen, setRangeOpen] = useState(false);
  const [creating, setCreating] = useState(false);
  const [createError, setCreateError] = useState<string>();

  const createNewAnalysis = useCallback(
    async (months: AnalysisWindowMonths) => {
      if (!result || !facts) {
        return;
      }
      if (facts.entry === "clinical_note") {
        setRangeOpen(false);
        setCreateError(CLINICAL_NOTE_RERUN_TEXT);
        return;
      }
      setCreating(true);
      setCreateError(undefined);
      const resolved = await resolveApiClient();
      if (!resolved) {
        setCreating(false);
        setCreateError("尚未配置后端地址，请先到设置填写。");
        return;
      }
      try {
        const next = await createAnalysis(resolved.client, {
          member_id: result.snapshot.member_id,
          panel: result.snapshot.panel,
          entry: facts.entry,
          ...(facts.indicator_name
            ? { indicator_name: facts.indicator_name }
            : {}),
          window_months: months,
        });
        setRangeOpen(false);
        router.push({
          pathname: "/analysis/[analysis_id]",
          params: { analysis_id: String(next.analysis_id) },
        });
      } catch (error) {
        setCreateError(error instanceof Error ? error.message : String(error));
      } finally {
        setCreating(false);
      }
    },
    [facts, result],
  );

  const openRange = useCallback(() => {
    setCreateError(undefined);
    setRangeOpen(true);
  }, []);

  return {
    rangeOpen,
    creating,
    createError,
    createNewAnalysis,
    openRange,
    closeRange: () => setRangeOpen(false),
  };
}

/** 结果页的派生视图数据:把多层 optional/条件收敛在一处,屏幕只做状态分支。 */
export function useAnalysisViewModel(result: AnalysisResult | undefined) {
  const facts = result?.layers.facts;
  const framework = result?.layers.framework;
  const explanation = result?.layers.explanation;
  const hasExplanation = Boolean(
    explanation && explanation.sections.length > 0,
  );
  const coverage =
    facts && hasExplanation
      ? deviationCoverage(explanation, facts.deviations)
      : undefined;
  const explanationUnavailable = result
    ? explanationUnavailableText(
        result.quota.explanation_reason,
        result.quota.explanation_available,
      )
    : undefined;
  const explanationNextStep = result
    ? explanationUnavailableNextStep(
        result.quota.explanation_reason,
        result.quota.explanation_available,
      )
    : "";
  const remainingQuota = result
    ? Math.max(0, result.quota.limit_today - result.quota.used_today)
    : 0;
  const heading = facts
    ? `${panelLabel(facts.panel as PanelCode)}解读`
    : "解读结果";
  const nameLabels = facts
    ? indicatorLabels(facts.deviations.map((point) => point.name))
    : new Map<string, string>();
  const oldWindowDates = facts
    ? [
        ...new Set([
          ...facts.reports
            .filter(
              (report) =>
                report.report_date && report.report_date < facts.window.from,
            )
            .map((report) => report.report_date as string),
          ...facts.excluded
            .filter(
              (item) => item.reason === "outside_window" && item.report_date,
            )
            .map((item) => item.report_date as string),
        ]),
      ]
        .sort()
        .reverse()
    : [];
  const inWindowReportCount = facts
    ? facts.reports.filter(
        (report) =>
          !report.report_date || report.report_date >= facts.window.from,
      ).length
    : 0;
  return {
    facts,
    framework,
    explanation,
    hasExplanation,
    coverage,
    explanationUnavailable,
    explanationNextStep,
    remainingQuota,
    heading,
    nameLabels,
    oldWindowDates,
    inWindowReportCount,
  };
}
