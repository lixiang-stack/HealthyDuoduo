import type { ApiClient } from './client';
import { UPLOAD_TIMEOUT_MS, uploadReport } from './reports';

const image = { uri: 'file:///tmp/sheet.jpg' };

describe('uploadReport', () => {
  afterEach(() => {
    jest.useRealTimers();
  });

  it('超过单张上限时中止请求,并给出可重试文案', async () => {
    jest.useFakeTimers();
    const post = jest.fn(
      (_path: string, options: { signal: AbortSignal }) =>
        new Promise((_resolve, reject) => {
          options.signal.addEventListener('abort', () => reject(new Error('aborted')));
        }),
    );
    const client = { POST: post } as unknown as ApiClient;

    const promise = uploadReport(client, 1, image);
    const assertion = expect(promise).rejects.toThrow('上传超时');
    await jest.advanceTimersByTimeAsync(UPLOAD_TIMEOUT_MS);
    await assertion;
    expect(post).toHaveBeenCalledTimes(1);
  });

  it('非超时错误原样抛出', async () => {
    const client = {
      POST: () => Promise.reject(new Error('HTTP 500')),
    } as unknown as ApiClient;

    await expect(uploadReport(client, 1, image)).rejects.toThrow('HTTP 500');
  });
});
