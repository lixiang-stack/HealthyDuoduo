// 解读首次使用确认与固定来源声明(llm-analysis 票 04 / spec §4.14)。
// 确认只弹一次,之后记在本地;两句都不使用「免责」措辞。
import AsyncStorage from '@react-native-async-storage/async-storage';

export const ANALYSIS_CONSENT_STORAGE_KEY = 'analysis.consent.v1';

export const ANALYSIS_CONSENT_TEXT =
  '解读会把本次涉及的检验数值、年龄段和病历诊断发送到云端服务处理，不包含姓名、生日、机构名和原图。结果不是诊断，不能代替医生。';

export const ANALYSIS_SOURCE_STATEMENT =
  '内容来自你上传的检查报告、病历和公开检验标准，不是诊断。';

export async function hasAnalysisConsent(): Promise<boolean> {
  return (await AsyncStorage.getItem(ANALYSIS_CONSENT_STORAGE_KEY)) === 'granted';
}

export async function grantAnalysisConsent(): Promise<void> {
  await AsyncStorage.setItem(ANALYSIS_CONSENT_STORAGE_KEY, 'granted');
}
