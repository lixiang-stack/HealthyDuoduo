// 报告 API 封装(GET /reports 时间线,POST /reports multipart 上传,
// GET /reports/{report_id} 详情,PATCH /reports/{report_id} 改归属,
// DELETE /reports/{report_id} 删除,PATCH /reports/{report_id}/items/{item_id} 核对,
// GET /images/{sha256} 图像;契约 schemas/openapi.yaml)。
// 失败一律抛 Error:解包统一走 client 的 unwrap/assertOk(契约错误体中性说明,
// 未声明错误体回落 HTTP 状态)。
import type { ApiClient } from './client';
import { assertOk, unwrap } from './client';
import type { PickedImage } from '../platform/imagePicker';
import { appendImagePart } from '../platform/imageUpload';
import type { components } from './schema';

export type ReportSummary = components['schemas']['ReportSummary'];
export type ReportDetail = components['schemas']['ReportDetail'];
export type ReportItem = components['schemas']['ReportItem'];
export type FlagCounts = components['schemas']['FlagCounts'];
export type UploadResult = components['schemas']['UploadResult'];
export type OcrLineLink = components['schemas']['OcrLineLink'];
export type ReportSection = components['schemas']['ReportSection'];
export type ReviewResult = components['schemas']['ReviewResult'];

// 契约 ReviewRequest 的 oneOf 经 codegen 退化为 unknown(correct 分支的 anyOf
// 收敛为 unknown),此处手写同一判别联合:confirm 只带 action(不产生修正对);
// correct 至少一项纸面事实(value/unit/name/ref_range;flag 无入口——服务端按
// 修正后的值×范围重算)。请求构造见 review/items 的 buildCorrection。
export type CorrectionRequest = {
  action: 'correct';
  value?: string;
  unit?: string;
  name?: string;
  ref_range?: string;
};
export type ReviewRequest = { action: 'confirm' } | CorrectionRequest;

export async function listReports(
  client: ApiClient,
  memberId: number,
): Promise<ReportSummary[]> {
  return unwrap(
    await client.GET('/reports', { params: { query: { member_id: memberId } } }),
  ).reports;
}

export async function getReport(client: ApiClient, reportId: string): Promise<ReportDetail> {
  return unwrap(
    await client.GET('/reports/{report_id}', { params: { path: { report_id: reportId } } }),
  );
}

/** 图像直出地址(GET /images/{sha256};报告对外身份即图像内容 sha256)。 */
export function reportImageUrl(baseUrl: string, reportId: string): string {
  return `${baseUrl}/images/${reportId}`;
}

export async function uploadReport(
  client: ApiClient,
  memberId: number,
  image: PickedImage,
): Promise<UploadResult> {
  return unwrap(
    await client.POST('/reports', {
      // body 只是 openapi-fetch 的类型载体(契约把图像声明为 string,承载不了
      // RN 的 {uri,name,type});真实 multipart 由 bodySerializer 经 platform 模块装配。
      body: { image: image.uri, member_id: memberId },
      bodySerializer: () => {
        const form = new FormData();
        form.append('member_id', String(memberId));
        appendImagePart(form, image);
        return form;
      },
    }),
  );
}

/** 核对(确认/修正)单次动作;响应含动作后报告级派生状态与本次指标项(单响应刷新整屏)。 */
export async function reviewItem(
  client: ApiClient,
  reportId: string,
  itemId: number,
  request: ReviewRequest,
): Promise<ReviewResult> {
  return unwrap(
    await client.PATCH('/reports/{report_id}/items/{item_id}', {
      params: { path: { report_id: reportId, item_id: itemId } },
      body: request,
    }),
  );
}

/** 改归属:归属即刻生效,时间线随成员过滤移位;响应 = 刷新后的报告头部。 */
export async function reassignReport(
  client: ApiClient,
  reportId: string,
  memberId: number,
): Promise<ReportSummary> {
  return unwrap(
    await client.PATCH('/reports/{report_id}', {
      params: { path: { report_id: reportId } },
      body: { member_id: memberId },
    }),
  );
}

/** 删除报告:报告与指标项、修正记录清除;原始图像与识别历史留痕(204 无响应体)。 */
export async function deleteReport(client: ApiClient, reportId: string): Promise<void> {
  assertOk(
    await client.DELETE('/reports/{report_id}', { params: { path: { report_id: reportId } } }),
  );
}
