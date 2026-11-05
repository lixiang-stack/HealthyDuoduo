// 分析 API 封装(POST /analyses、GET /analyses/{analysis_id}、反馈;契约 schemas/openapi.yaml)。
// 事实层永远返回;框架层按规则出现;解释层受条目与配额约束。创建时永久保存快照。
import type { ApiClient } from './client';
import { unwrap } from './client';
import type { components } from './schema';

export type AnalysisRequest = components['schemas']['AnalysisRequest'];
export type AnalysisResult = components['schemas']['AnalysisResult'];
export type AnalysisFacts = components['schemas']['AnalysisFacts'];
export type AnalysisEntry = components['schemas']['AnalysisEntry'];
export type AnalysisEvidencePoint = components['schemas']['AnalysisEvidencePoint'];
export type AnalysisHistory = components['schemas']['AnalysisHistory'];
export type AnalysisExclusion = components['schemas']['AnalysisExclusion'];
export type AnalysisNote = components['schemas']['AnalysisNote'];
export type AnalysisSource = components['schemas']['AnalysisSource'];
export type AnalysisWindowMonths = AnalysisRequest['window_months'];
export type AnalysisFramework = components['schemas']['AnalysisFramework'];
export type AnalysisFrameworkLabel = components['schemas']['AnalysisFrameworkLabel'];
export type AnalysisExplanation = components['schemas']['AnalysisExplanation'];
export type AnalysisFeedbackRequest = components['schemas']['AnalysisFeedbackRequest'];
export type AnalysisFeedbackResult = components['schemas']['AnalysisFeedbackResult'];

/** 默认回溯窗口(月);用户可在入口切换 6 / 12 / 24。 */
export const ANALYSIS_DEFAULT_WINDOW_MONTHS: AnalysisWindowMonths = 12;

export async function createAnalysis(
  client: ApiClient,
  body: AnalysisRequest,
): Promise<AnalysisResult> {
  return unwrap(await client.POST('/analyses', { body }));
}

export async function getAnalysis(
  client: ApiClient,
  analysisId: number,
): Promise<AnalysisResult> {
  return unwrap(
    await client.GET('/analyses/{analysis_id}', {
      params: { path: { analysis_id: analysisId } },
    }),
  );
}

export async function submitAnalysisFeedback(
  client: ApiClient,
  analysisId: number,
  body: AnalysisFeedbackRequest,
): Promise<AnalysisFeedbackResult> {
  return unwrap(
    await client.POST('/analyses/{analysis_id}/feedback', {
      params: { path: { analysis_id: analysisId } },
      body,
    }),
  );
}
