import { deriveDefaultBaseUrl, normalizeBaseUrl } from './baseUrl';

describe('deriveDefaultBaseUrl', () => {
  it('真机取开发机 LAN IP,不产出 localhost', () => {
    expect(deriveDefaultBaseUrl('192.168.1.5:8081')).toBe('http://192.168.1.5:8080/api/v1');
  });

  it('无开发服务器信息:不产出默认值', () => {
    expect(deriveDefaultBaseUrl(undefined)).toBeUndefined();
    expect(deriveDefaultBaseUrl('')).toBeUndefined();
  });
});

describe('normalizeBaseUrl', () => {
  it('去首尾空白与末尾斜杠', () => {
    expect(normalizeBaseUrl('  http://192.168.1.5:8080/api/v1/  ')).toBe(
      'http://192.168.1.5:8080/api/v1',
    );
  });

  it('未带 API 前缀时补上', () => {
    expect(normalizeBaseUrl('http://192.168.1.5:8080')).toBe('http://192.168.1.5:8080/api/v1');
  });

  it('拒绝缺少协议、非 http(s) 与空串', () => {
    expect(normalizeBaseUrl('192.168.1.5:8080')).toBeUndefined();
    expect(normalizeBaseUrl('ftp://192.168.1.5')).toBeUndefined();
    expect(normalizeBaseUrl('   ')).toBeUndefined();
  });
});
