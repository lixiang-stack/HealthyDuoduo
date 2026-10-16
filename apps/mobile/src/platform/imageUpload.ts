// 上传装包(platform 抽象之一):把选中图像装成 multipart part——Web 直传 File、
// 原生用 {uri,name,type} 形态。与选图(imagePicker,依赖 expo)分离:本模块无原生依赖,
// 服务端交互层(api 封装)只消费它,不把 expo 选图通道拉进请求构造路径。
import type { PickedImage } from './imagePicker';

/** multipart 图像字段名(契约 POST /reports 的 image part)。 */
const UPLOAD_IMAGE_FIELD = 'image';

/** 无显式文件名的兜底(原生选择器可能不给 fileName)。 */
const DEFAULT_FILENAME = 'photo.jpg';

/** RN fetch 认识的原生文件对象形态(仅原生通道使用)。 */
export type NativeImagePart = { uri: string; name: string; type: string };

/** 图像 part 的取值形态:Web = File 直传;原生 = {uri,name,type}。 */
export function imagePartValue(image: PickedImage): File | NativeImagePart {
  if (image.file) {
    return image.file;
  }
  return {
    uri: image.uri,
    name: image.fileName ?? DEFAULT_FILENAME,
    type: image.mimeType ?? 'image/jpeg',
  };
}

/** 把选中图像装入 multipart 上传表单(取值形态见 imagePartValue)。 */
export function appendImagePart(form: FormData, image: PickedImage): void {
  // 原生形态不是 DOM Blob 类型,但在 RN fetch 的 FormData 里是合法的文件 part。
  form.append(UPLOAD_IMAGE_FIELD, imagePartValue(image) as unknown as Blob);
}
