import * as ImagePicker from 'expo-image-picker';

import { pickImage } from './imagePicker';

jest.mock('expo-image-picker', () => ({
  requestCameraPermissionsAsync: jest.fn(),
  requestMediaLibraryPermissionsAsync: jest.fn(),
  launchCameraAsync: jest.fn(),
  launchImageLibraryAsync: jest.fn(),
}));

const asset = (uri: string, fileName: string): ImagePicker.ImagePickerAsset =>
  ({
    uri,
    fileName,
    mimeType: 'image/jpeg',
    width: 100,
    height: 100,
    type: 'image',
  }) as ImagePicker.ImagePickerAsset;

const granted = { granted: true } as never;
const denied = { granted: false } as never;

beforeEach(() => {
  jest.clearAllMocks();
  jest.mocked(ImagePicker.requestCameraPermissionsAsync).mockResolvedValue(granted);
  jest.mocked(ImagePicker.requestMediaLibraryPermissionsAsync).mockResolvedValue(granted);
});

describe('pickImage', () => {
  it('相册多选:返回全部 assets,并把上限传给 selectionLimit', async () => {
    jest.mocked(ImagePicker.launchImageLibraryAsync).mockResolvedValue({
      canceled: false,
      assets: [asset('file:///a.jpg', 'a.jpg'), asset('file:///b.jpg', 'b.jpg')],
    } as never);

    await expect(pickImage('library', 3)).resolves.toEqual({
      kind: 'picked',
      images: [
        { uri: 'file:///a.jpg', mimeType: 'image/jpeg', fileName: 'a.jpg', file: undefined },
        { uri: 'file:///b.jpg', mimeType: 'image/jpeg', fileName: 'b.jpg', file: undefined },
      ],
    });
    expect(ImagePicker.launchImageLibraryAsync).toHaveBeenCalledWith({
      mediaTypes: ['images'],
      allowsMultipleSelection: true,
      selectionLimit: 3,
    });
  });

  it('拍照:保持单张,不开启多选', async () => {
    jest.mocked(ImagePicker.launchCameraAsync).mockResolvedValue({
      canceled: false,
      assets: [asset('file:///camera.jpg', 'camera.jpg')],
    } as never);

    const result = await pickImage('camera', 9);
    expect(result.kind).toBe('picked');
    expect(result.kind === 'picked' ? result.images : []).toHaveLength(1);
    expect(ImagePicker.launchCameraAsync).toHaveBeenCalledWith({ mediaTypes: ['images'] });
  });

  it('权限拒绝:不打开选择器', async () => {
    jest.mocked(ImagePicker.requestMediaLibraryPermissionsAsync).mockResolvedValue(denied);
    await expect(pickImage('library', 9)).resolves.toEqual({ kind: 'denied' });
    expect(ImagePicker.launchImageLibraryAsync).not.toHaveBeenCalled();
  });

  it('取消或空选择:返回 cancelled', async () => {
    jest.mocked(ImagePicker.launchImageLibraryAsync).mockResolvedValue({
      canceled: true,
      assets: null,
    } as never);
    await expect(pickImage('library', 9)).resolves.toEqual({ kind: 'cancelled' });

    jest.mocked(ImagePicker.launchImageLibraryAsync).mockResolvedValue({
      canceled: false,
      assets: [],
    } as never);
    await expect(pickImage('library', 9)).resolves.toEqual({ kind: 'cancelled' });
  });
});
