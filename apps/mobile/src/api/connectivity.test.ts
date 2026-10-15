import { resolveConnectivity } from './connectivity';

describe('resolveConnectivity', () => {
  it('成功探测:输出 ok 并带版本', () => {
    expect(resolveConnectivity({ kind: 'ok', version: '0.1.0' })).toEqual({
      status: 'ok',
      version: '0.1.0',
    });
  });

  it('HTTP 非 2xx:归为后端响应异常', () => {
    expect(resolveConnectivity({ kind: 'http', status: 503 })).toEqual({
      status: 'error',
      detail: 'HTTP 503',
    });
  });

  it('网络失败:归为无法连接', () => {
    expect(resolveConnectivity({ kind: 'network', detail: 'Network request failed' })).toEqual({
      status: 'unreachable',
      detail: 'Network request failed',
    });
  });
});
