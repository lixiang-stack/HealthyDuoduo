// 报告 API 封装(GET /reports 时间线,POST /reports multipart 上传,
// GET /reports/{report_id} 详情,PATCH /reports/{report_id} 改归属,
// DELETE /reports/{report_id} 删除,PATCH /reports/{report_id}/items/{item_id} 核对,
// GET /members/{member_id}/indicators/{name}/trend 趋势,GET /images/{sha256} 图像;
// 契约 schemas/openapi.yaml)。
// 失败一律抛 Error:解包统一走 client 的 unwrap/assertOk(契约错误体中性说明,
// 未声明错误体回落 HTTP 状态)。
import type { ApiClient } from './client';
import { assertOk, unwrap } from './client';
import type { PickedImage } from '../platform/imagePicker';
import { appendImagePart } from '../platform/imageUpload';
import type { PanelCode } from '../reports/panels';
import type { components } from './schema';

export type ReportSummary = components['schemas']['ReportSummary'];
export type ReportDetail = components['schemas']['ReportDetail'];
export type ReportItem = components['schemas']['ReportItem'];
export type FlagCounts = components['schemas']['FlagCounts'];
export type UploadResult = components['schemas']['UploadResult'];
export type OcrLineLink = components['schemas']['OcrLineLink'];
export type ReportSection = components['schemas']['ReportSection'];
export type ReviewResult = components['schemas']['ReviewResult'];
export type TrendPoint = components['schemas']['TrendPoint'];
export type TrendResult = components['schemas']['TrendResult'];
export type PanelSummary = components['schemas']['PanelSummary'];
export type PanelDetail = components['schemas']['PanelDetail'];
export type PanelItem = components['schemas']['PanelItem'];

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
export type ReviewRequest =
  | { action: 'confirm' }
  | { action: 'no_reference' }
  | CorrectionRequest;

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

/** 成员维度趋势时序(单位可选;缺省返回全部单位,供 selectSeries 过滤/计数)。 */
export async function getTrend(
  client: ApiClient,
  memberId: number,
  name: string,
  unit?: string,
): Promise<TrendResult> {
  return unwrap(
    await client.GET('/members/{member_id}/indicators/{name}/trend', {
      params: {
        path: { member_id: memberId, name },
        query: unit ? { unit } : undefined,
      },
    }),
  );
}

/** 当前成员可浏览的项目列表(只包含实际出现过的项目;排序由服务端给)。 */
export async function listMemberPanels(
  client: ApiClient,
  memberId: number,
): Promise<PanelSummary[]> {
  return unwrap(
    await client.GET('/members/{member_id}/panels', {
      params: { path: { member_id: memberId } },
    }),
  ).panels;
}

/** 项目详情:该项目全部指标的最新值与跨报告点(按 name+unit 分行)。 */
export async function getPanelDetail(
  client: ApiClient,
  memberId: number,
  panel: PanelCode,
): Promise<PanelDetail> {
  return unwrap(
    await client.GET('/members/{member_id}/panels/{panel}', {
      params: { path: { member_id: memberId, panel } },
    }),
  );
}

/** 图像直出地址(GET /images/{sha256};报告对外身份即图像内容 sha256)。 */
export function reportImageUrl(baseUrl: string, reportId: string): string {
  return `${baseUrl}/images/${reportId}`;
}

/** 单张上传等待上限:后端识别 60s 超时 + 1 次重试,再留上传/归档余量。 */
export const UPLOAD_TIMEOUT_MS = 180_000;

export async function uploadReport(
  client: ApiClient,
  memberId: number,
  image: PickedImage,
): Promise<UploadResult> {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), UPLOAD_TIMEOUT_MS);
  try {
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
        signal: controller.signal,
      }),
    );
  } catch (error) {
    if (controller.signal.aborted) {
      throw new Error(`上传超时(${UPLOAD_TIMEOUT_MS / 1000} 秒),请检查后端与识别服务后重试`);
    }
    throw error;
  } finally {
    clearTimeout(timer);
  }
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
