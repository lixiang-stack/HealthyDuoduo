// platform 抽象:验证环境(Web 文件选择)与真机(相机/相册)的差异只在此文件收口。
// 业务页面一律调用 pickImage,自身不出现 Platform 分支。
import * as ImagePicker from 'expo-image-picker';
import { Platform } from 'react-native';

export type ImageSource = 'camera' | 'library';

export type PickedImage = {
  uri: string;
  mimeType?: string;
  fileName?: string;
  /** Web 文件选择通道返回的 File(expo-image-picker web 提供);原生上传走 uri 形态。 */
  file?: File;
};

export type PickImageResult =
  | { kind: 'picked'; images: PickedImage[] }
  | { kind: 'cancelled' }
  | { kind: 'denied' }
  | { kind: 'unsupported'; reason: string };

/** 相册一次可选多张(selectionLimit 为本次上限);相机固定单张。 */
export async function pickImage(
  source: ImageSource,
  selectionLimit: number,
): Promise<PickImageResult> {
  // Web 验证环境没有相机能力,收口为 unsupported(改用文件选择)。
  if (Platform.OS === 'web' && source === 'camera') {
    return { kind: 'unsupported', reason: '验证环境(Web)没有相机,请改用相册/文件选择' };
  }
  const permission =
    source === 'camera'
      ? await ImagePicker.requestCameraPermissionsAsync()
      : await ImagePicker.requestMediaLibraryPermissionsAsync();
  if (!permission.granted) {
    return { kind: 'denied' };
  }
  const options: ImagePicker.ImagePickerOptions =
    source === 'camera'
      ? { mediaTypes: ['images'] }
      : { mediaTypes: ['images'], allowsMultipleSelection: true, selectionLimit };
  const result =
    source === 'camera'
      ? await ImagePicker.launchCameraAsync(options)
      : await ImagePicker.launchImageLibraryAsync(options);
  if (result.canceled || result.assets.length === 0) {
    return { kind: 'cancelled' };
  }
  return {
    kind: 'picked',
    images: result.assets.map((asset) => ({
      uri: asset.uri,
      mimeType: asset.mimeType,
      fileName: asset.fileName ?? undefined,
      file: asset.file ?? undefined,
    })),
  };
}
