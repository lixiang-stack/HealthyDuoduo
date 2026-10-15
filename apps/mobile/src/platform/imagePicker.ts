// platform 抽象:验证环境(Web 文件选择)与真机(相机/相册)的差异只在此文件收口。
// 业务页面一律调用 pickImage,自身不出现 Platform 分支。
import * as ImagePicker from 'expo-image-picker';
import { Platform } from 'react-native';

export type ImageSource = 'camera' | 'library';

export type PickedImage = {
  uri: string;
  mimeType?: string;
  fileName?: string;
};

export type PickImageResult =
  | { kind: 'picked'; image: PickedImage }
  | { kind: 'cancelled' }
  | { kind: 'denied' }
  | { kind: 'unsupported'; reason: string };

export async function pickImage(source: ImageSource): Promise<PickImageResult> {
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
  const options: ImagePicker.ImagePickerOptions = { mediaTypes: ['images'] };
  const result =
    source === 'camera'
      ? await ImagePicker.launchCameraAsync(options)
      : await ImagePicker.launchImageLibraryAsync(options);
  if (result.canceled) {
    return { kind: 'cancelled' };
  }
  const asset = result.assets[0];
  if (!asset) {
    return { kind: 'cancelled' };
  }
  return {
    kind: 'picked',
    image: { uri: asset.uri, mimeType: asset.mimeType, fileName: asset.fileName ?? undefined },
  };
}
