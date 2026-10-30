// platform 抽象:相机/相册通道与业务页面分离,页面一律调用 pickImage。
import * as ImagePicker from 'expo-image-picker';

export type ImageSource = 'camera' | 'library';

export type PickedImage = {
  uri: string;
  mimeType?: string;
  fileName?: string;
};

export type PickImageResult =
  | { kind: 'picked'; images: PickedImage[] }
  | { kind: 'cancelled' }
  | { kind: 'denied' };

/** 相册一次可选多张(selectionLimit 为本次上限);相机固定单张。 */
export async function pickImage(
  source: ImageSource,
  selectionLimit: number,
): Promise<PickImageResult> {
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
    })),
  };
}
