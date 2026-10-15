// 成员 API 封装(GET/POST /members 与 PATCH /members/{member_id};契约 schemas/openapi.yaml)。
// 失败一律抛 Error:契约声明了错误体(Error 形状)的端点用其中性说明,GET /members
// 契约未声明错误体,回落到 HTTP 状态;页面层负责呈现。
import type { ApiClient } from './client';
import type { components } from './schema';

export type Member = components['schemas']['Member'];
export type MemberCreate = components['schemas']['MemberCreate'];
export type MemberUpdate = components['schemas']['MemberUpdate'];

export async function listMembers(client: ApiClient): Promise<Member[]> {
  const { data, response } = await client.GET('/members');
  if (!data) {
    throw new Error(`HTTP ${response.status}`);
  }
  return data.members;
}

export async function createMember(client: ApiClient, body: MemberCreate): Promise<Member> {
  const { data, error, response } = await client.POST('/members', { body });
  if (!data) {
    throw new Error(error?.message ?? `HTTP ${response.status}`);
  }
  return data;
}

export async function updateMember(
  client: ApiClient,
  memberId: number,
  body: MemberUpdate,
): Promise<Member> {
  const { data, error, response } = await client.PATCH('/members/{member_id}', {
    params: { path: { member_id: memberId } },
    body,
  });
  if (!data) {
    throw new Error(error?.message ?? `HTTP ${response.status}`);
  }
  return data;
}
