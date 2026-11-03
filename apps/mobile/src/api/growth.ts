// 生长记录 API 封装(POST/GET /members/{member_id}/growth;契约 schemas/openapi.yaml)。
// BMI 由服务端用测量时点值实时计算;source=manual 时只有数值,没有超重 / 肥胖判定。
import type { ApiClient } from './client';
import { unwrap } from './client';
import type { components } from './schema';

export type GrowthRecord = components['schemas']['GrowthRecord'];
export type GrowthRecordCreate = components['schemas']['GrowthRecordCreate'];
export type GrowthSource = GrowthRecord['source'];

export async function createGrowth(
  client: ApiClient,
  memberId: number,
  body: GrowthRecordCreate,
): Promise<GrowthRecord> {
  return unwrap(
    await client.POST('/members/{member_id}/growth', {
      params: { path: { member_id: memberId } },
      body,
    }),
  );
}
