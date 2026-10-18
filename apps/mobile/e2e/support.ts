// App E2E 支撑:契约类型直接复用 App 的 OpenAPI 生成物;API、样张、storage 键、
// testID 约定单点定义(用例只消费,不散落裸字符串)。
import { readFile } from 'node:fs/promises';
import { basename, resolve } from 'node:path';

import type { Browser } from '@e2e-dev/web';
import type { App } from 'e2e';

import type { components } from '../src/api/schema';

export type Member = components['schemas']['Member'];
export type ReportDetail = components['schemas']['ReportDetail'];
export type UploadResult = components['schemas']['UploadResult'];

/** 专用栈 API 基址;run.sh 以 E2E_API_BASE_URL 导出(与 HDD_SERVE_PORT 同源)。 */
export const API_BASE_URL = process.env.E2E_API_BASE_URL ?? 'http://127.0.0.1:58080/api/v1';

const REPO_ROOT = resolve(import.meta.dirname, '../../..');
const SAMPLES = resolve(REPO_ROOT, 'samples');

export const samplePath = (name: string) => resolve(SAMPLES, name);

// 与 App 侧 settings/baseUrlStore.ts、members/currentMemberStore.ts 的键逐字一致
// (Web 下 AsyncStorage = localStorage;预置后各屏免走设置页)。
const BASE_URL_STORAGE_KEY = 'healthyduoduo.baseUrl';
const CURRENT_MEMBER_STORAGE_KEY = 'healthyduoduo.currentMemberId';

/** 预置后端地址/当前成员后打开页面;每次调用追加一段页面初始化脚本。 */
export async function openConfigured(
  app: App,
  browser: Browser,
  path: string,
  options: { memberId?: number; baseUrl?: string } = {},
): Promise<void> {
  const lines = [
    `localStorage.setItem(${JSON.stringify(BASE_URL_STORAGE_KEY)}, ${JSON.stringify(options.baseUrl ?? API_BASE_URL)});`,
  ];
  if (options.memberId !== undefined) {
    lines.push(`localStorage.setItem(${JSON.stringify(CURRENT_MEMBER_STORAGE_KEY)}, '${options.memberId}');`);
  }
  await browser.addInitScript(lines.join('\n'));
  await app.open(path);
}

/** Expo Web 相册通道经 `input.dispatchEvent(new MouseEvent('click'))` 打开原生文件选择器;
 *  headless 下 chooser 被自动取消,expo 随即移除 input,setInputFiles 就找不到节点。
 *  屏蔽这次 click、保留 input,再由测试用 setInputFiles 注入文件(change 事件照常触发)。 */
export async function stubFileChooser(browser: Browser): Promise<void> {
  await browser.addInitScript(`
    const originalDispatchEvent = HTMLInputElement.prototype.dispatchEvent;
    HTMLInputElement.prototype.dispatchEvent = function (event) {
      if (this.type === 'file' && event && event.type === 'click') {
        return true;
      }
      return originalDispatchEvent.call(this, event);
    };
  `);
}

/** 确定性 testID 生成;与 App 侧各组件模板一一对应。 */export const testID = {
  memberChip: (memberId: number) => `member-chip-${memberId}`,
  memberRow: (memberId: number) => `member-row-${memberId}`,
  memberSelect: (memberId: number) => `member-select-${memberId}`,
  reportCard: (reportId: string) => `report-card-${reportId}`,
  reviewItem: (itemId: number) => `review-item-${itemId}`,
  reviewConfirm: (itemId: number) => `review-confirm-${itemId}`,
  reviewCorrect: (itemId: number) => `review-correct-${itemId}`,
  reviewValue: (itemId: number) => `review-field-value-${itemId}`,
  reviewSave: (itemId: number) => `review-save-${itemId}`,
} as const;

async function api<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(`${API_BASE_URL}${path}`, init);
  if (!response.ok) {
    const detail = await response.text().catch(() => '');
    throw new Error(`API ${init?.method ?? 'GET'} ${path}: HTTP ${response.status} ${detail}`);
  }
  return (await response.json()) as T;
}

export async function listMembers(): Promise<Member[]> {
  const payload = await api<{ members: Member[] }>('/members');
  return payload.members;
}

export async function createMember(name: string, isChild: boolean): Promise<Member> {
  return api<Member>('/members', {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ name, is_child: isChild }),
  });
}

const MIME_BY_SUFFIX: Record<string, string> = {
  '.png': 'image/png',
  '.jpg': 'image/jpeg',
  '.jpeg': 'image/jpeg',
  '.webp': 'image/webp',
};

/** 经真实上传端点入档样张(同步识别);duplicate=true 说明栈未重置,直接失败。 */
export async function uploadSample(name: string, memberId: number): Promise<ReportDetail> {
  const path = samplePath(name);
  const bytes = await readFile(path);
  const suffix = path.slice(path.lastIndexOf('.')).toLowerCase();
  const form = new FormData();
  form.append('member_id', String(memberId));
  form.append(
    'image',
    new Blob([new Uint8Array(bytes)], { type: MIME_BY_SUFFIX[suffix] ?? 'application/octet-stream' }),
    basename(path),
  );
  const result = await api<UploadResult>('/reports', {
    method: 'POST',
    body: form,
    signal: AbortSignal.timeout(120_000),
  });
  if (result.duplicate) {
    throw new Error(`样张 ${name} 已在库:专用栈未重置,请用 e2e/run.sh 重跑`);
  }
  return result.report;
}

export type Seed = {
  xiaoming: Member;
  mama: Member;
  reviewReport: ReportDetail;
  healReport: ReportDetail;
};

/** 种子:两成员 + 一份含低置信项的报告(核对) + 一份改归属用报告(自愈)。 */
export async function seedData(): Promise<Seed> {
  const xiaoming = await createMember('E2E 小明', true);
  const mama = await createMember('E2E 妈妈', false);
  const reviewReport = await uploadSample('cbc_03.png', xiaoming.member_id);
  if (!reviewReport.items.some((item) => item.low_confidence)) {
    throw new Error('cbc_03 未识别出低置信项,核对闭环无法用例化');
  }
  const healReport = await uploadSample('cbc_04.png', mama.member_id);
  return { xiaoming, mama, reviewReport, healReport };
}
