import { imagePartValue } from './imageUpload';

describe('imagePartValue', () => {
  it('Web 文件通道:直传 File 本体', () => {
    const file = new File(['jpeg-bytes'], 'sheet.jpg', { type: 'image/jpeg' });
    expect(imagePartValue({ uri: 'blob:web', file })).toBe(file);
  });

  it('原生通道:装入 {uri,name,type} 形态(RN fetch 识别的文件对象)', () => {
    expect(
      imagePartValue({ uri: 'file:///tmp/sheet.jpg', fileName: 'sheet.jpg', mimeType: 'image/jpeg' }),
    ).toEqual({ uri: 'file:///tmp/sheet.jpg', name: 'sheet.jpg', type: 'image/jpeg' });
  });

  it('缺失文件名/mime:回落 photo.jpg 与 image/jpeg', () => {
    expect(imagePartValue({ uri: 'file:///tmp/x' })).toEqual({
      uri: 'file:///tmp/x',
      name: 'photo.jpg',
      type: 'image/jpeg',
    });
  });
});
