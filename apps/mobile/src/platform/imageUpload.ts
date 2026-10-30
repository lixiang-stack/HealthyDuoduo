// 上传装包(platform 抽象之一):把选中图像装成 multipart part——用 expo-file-system
// 的 File(实现 Blob 接口与 bytes())。
// 注意:Expo SDK 57 原生全局 fetch 为 WinterCG 实现,FormData part 只接受
// string | Blob | 带 bytes() 的对象;旧式 RN 文件对象 {uri,name,type} 会被拒
// ("Unsupported FormDataPart implementation"),故不能再走 uri 描述对象。
// 与选图(imagePicker,依赖 expo)分离:本模块无选图通道依赖,服务端交互层
// (api 封装)只消费它,不把 expo 选图通道拉进请求构造路径。
import { File } from 'expo-file-system';

import type { PickedImage } from './imagePicker';

/** multipart 图像字段名(契约 POST /reports 的 image part)。 */
const UPLOAD_IMAGE_FIELD = 'image';

/** 图像 part 取值:expo-file-system File(带 bytes(),供 expo/fetch 读取)。 */
export function imagePartValue(image: PickedImage): Blob {
  return new File(image.uri);
}

/** 把选中图像装入 multipart 上传表单(取值形态见 imagePartValue)。 */
export function appendImagePart(form: FormData, image: PickedImage): void {
  form.append(UPLOAD_IMAGE_FIELD, imagePartValue(image));
}
