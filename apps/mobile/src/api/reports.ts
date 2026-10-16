// 报告 API 封装(GET /reports 时间线,POST /reports multipart 上传;契约 schemas/openapi.yaml)。
// 失败一律抛 Error:契约声明错误体(Error 形状)的端点用其中性说明,GET /reports
// 未声明错误体回落到 HTTP 状态(与 members 封装同惯例)。
import type { ApiClient } from './client';
import type { PickedImage } from '../platform/imagePicker';
import { appendImagePart } from '../platform/imageUpload';
import type { components } from './schema';

export type ReportSummary = components['schemas']['ReportSummary'];
export type ReportDetail = components['schemas']['ReportDetail'];
export type FlagCounts = components['schemas']['FlagCounts'];
export type UploadResult = components['schemas']['UploadResult'];

export async function listReports(
  client: ApiClient,
  memberId: number,
): Promise<ReportSummary[]> {
  const { data, response } = await client.GET('/reports', {
    params: { query: { member_id: memberId } },
  });
  if (!data) {
    throw new Error(`HTTP ${response.status}`);
  }
  return data.reports;
}

export async function uploadReport(
  client: ApiClient,
  memberId: number,
  image: PickedImage,
): Promise<UploadResult> {
  const { data, error, response } = await client.POST('/reports', {
    // body 只是 openapi-fetch 的类型载体(契约把图像声明为 string,承载不了
    // RN 的 {uri,name,type});真实 multipart 由 bodySerializer 经 platform 模块装配。
    body: { image: image.uri, member_id: memberId },
    bodySerializer: () => {
      const form = new FormData();
      form.append('member_id', String(memberId));
      appendImagePart(form, image);
      return form;
    },
  });
  if (!data) {
    throw new Error(error?.message ?? `HTTP ${response.status}`);
  }
  return data;
}
