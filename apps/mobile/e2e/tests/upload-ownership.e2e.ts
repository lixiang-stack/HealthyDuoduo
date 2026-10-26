// 上传归属成员校验:清库/换后端后本地持久化的 currentMemberId 可能已不存在,
// 此时页面不得把该旧 id 当作用户选择继续上传(服务端会 404 member not found)。
import { describe, test } from '@e2e-dev/web';
import { expect } from 'e2e';

import { createMember, openConfigured, samplePath, stubFileChooser, testID } from '../support';

describe('上传归属成员', { serial: true, timeout: 300_000 }, () => {
  test('持久化的当前成员已不存在时要求重新选择', async ({ app, browser, screen }) => {
    const member = await createMember('归属校验', false);
    await stubFileChooser(browser);
    await openConfigured(app, browser, '/upload', { memberId: 999999999 });

    await screen.getByTestId('upload-pick-library').tap();
    await browser.locator('[data-testid="file-input"]').setInputFiles(samplePath('glu_02.jpeg'));
    await expect(screen.getByText('glu_02.jpeg')).toBeVisible();

    expect(await screen.getByTestId('upload-submit').isDisabled()).toBe(true);
    await expect(screen.getByText('先选择检查单归属哪位成员。')).toBeVisible();

    await screen.getByTestId(testID.memberChip(member.member_id)).tap();
    await expect(screen.getByTestId('upload-submit')).toBeEnabled();
  });
});
