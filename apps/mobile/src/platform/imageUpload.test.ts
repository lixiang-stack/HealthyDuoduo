import { appendImagePart, imagePartValue } from './imageUpload';

// 回归:Expo SDK 57 原生 fetch(WinterCG)的 FormData 只认 string | Blob | 带
// bytes() 的对象;旧式 RN {uri,name,type} 会抛 "Unsupported FormDataPart implementation"。
// 原生 part 必须提供 bytes()(expo-file-system File 即实现 Blob 接口)。

describe('imagePartValue', () => {
  it('expo-file-system File(uri 保留,带 bytes(),供 expo/fetch 读取)', () => {
    const part = imagePartValue({ uri: 'file:///tmp/sheet.jpg' }) as unknown as {
      uri: string;
      bytes: unknown;
    };
    expect(part.uri).toBe('file:///tmp/sheet.jpg');
    expect(typeof part.bytes).toBe('function');
  });
});

describe('appendImagePart', () => {
  it('装入 image 字段,取值为 imagePartValue(原生值为带 bytes() 的 File)', () => {
    const appended: [string, unknown][] = [];
    const form = {
      append: (name: string, value: unknown) => appended.push([name, value]),
    } as unknown as FormData;
    appendImagePart(form, { uri: 'file:///tmp/sheet.jpg' });
    expect(appended.map(([name]) => name)).toEqual(['image']);
    expect(typeof (appended[0][1] as { bytes?: unknown }).bytes).toBe('function');
  });
});
