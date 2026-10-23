// 独立错误态用例:不依赖 seed/栈数据,单独成文件避免主链路失败时被跳过。
import { test } from '@e2e-dev/web';
import { expect } from 'e2e';

import { openConfigured } from '../support';

test('后端不可达时首页显示失败态', async ({ app, browser, screen }) => {
  await openConfigured(app, browser, '/', { baseUrl: 'http://127.0.0.1:1/api/v1' });
  await expect(screen.getByText(/加载失败/)).toBeVisible();
});
