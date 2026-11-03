// 分析 API 封装(POST /analyses、GET /analyses/{analysis_id};契约 schemas/openapi.yaml)。
// 事实层永远返回;框架层 / 解释层由后续工单追加。创建时永久保存快照。
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
export type AnalysisWindowMonths = AnalysisRequest['window_months'];

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
