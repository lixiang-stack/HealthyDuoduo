// 框架层 / 解释层展示口径(llm-analysis 票 05/06/07):固定文案在服务端生成,
// App 只做排版;配额提示不静默降级,也不禁用分析入口。
import type { AnalysisFramework, AnalysisFrameworkLabel } from '../api/analyses';

export const QUOTA_EXHAUSTED_TEXT = '今日额度已用完，事实与判读仍可用。';
export const QUOTA_EXHAUSTED_HISTORY_TEXT = '本次分析生成时额度已用完，事实与判读仍可用。';
export const EXPLANATION_UNAVAILABLE_TEXT = '本次解释暂不可用，事实与判读仍可用。';
export const QUOTA_UNAVAILABLE_TEXT = '额度状态暂不可用，事实与判读仍可用。';
export const NO_ENTRIES_GUIDANCE_TEXT = '本次没有可引用的解读条目，可以带着上面的数值问医生。';

export function hasFrameworkLabels(framework: AnalysisFramework | undefined): framework is AnalysisFramework {
  return Boolean(framework && framework.labels.length > 0);
}

export function frameworkSourceText(label: AnalysisFrameworkLabel): string {
  return label.sources.map((source) => source.standard).join(' / ');
}

/** 解释层不可用提示:配额用尽优先;无条目时给固定引导,不显示空状态。 */
export function explanationUnavailableText(
  reason: string | undefined,
  available: boolean,
): string | undefined {
  if (reason === 'quota_exhausted') {
    return available ? QUOTA_EXHAUSTED_HISTORY_TEXT : QUOTA_EXHAUSTED_TEXT;
  }
  switch (reason) {
    case 'llm_timeout':
    case 'llm_error':
    case 'llm_invalid_json':
    case 'bridge_text_rejected':
    case 'input_too_large':
    case 'not_configured':
      return EXPLANATION_UNAVAILABLE_TEXT;
    case 'quota_unavailable':
      return QUOTA_UNAVAILABLE_TEXT;
    case 'no_entries':
      return NO_ENTRIES_GUIDANCE_TEXT;
    default:
      break;
  }
  return available ? undefined : QUOTA_EXHAUSTED_TEXT;
}
