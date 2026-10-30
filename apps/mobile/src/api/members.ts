// 成员 API 封装(GET/POST /members、PATCH/DELETE /members/{member_id};契约 schemas/openapi.yaml)。
// 失败一律抛 Error:解包统一走 client 的 unwrap / assertOk(契约说明或 HTTP 状态);页面层负责呈现。
import type { ApiClient } from './client';
import { assertOk, unwrap } from './client';
import type { components } from './schema';

export type Member = components['schemas']['Member'];
export type MemberCreate = components['schemas']['MemberCreate'];
export type MemberUpdate = components['schemas']['MemberUpdate'];

export async function listMembers(client: ApiClient): Promise<Member[]> {
  return unwrap(await client.GET('/members')).members;
}

export async function createMember(client: ApiClient, body: MemberCreate): Promise<Member> {
  return unwrap(await client.POST('/members', { body }));
}

export async function updateMember(
  client: ApiClient,
  memberId: number,
  body: MemberUpdate,
): Promise<Member> {
  return unwrap(
    await client.PATCH('/members/{member_id}', {
      params: { path: { member_id: memberId } },
      body,
    }),
  );
}

/** 删除成员及其全部报告;成功 204,未知成员抛 404 文案。 */
export async function deleteMember(client: ApiClient, memberId: number): Promise<void> {
  assertOk(
    await client.DELETE('/members/{member_id}', {
      params: { path: { member_id: memberId } },
    }),
  );
}
