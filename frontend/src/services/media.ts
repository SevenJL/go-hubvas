import { api, putFile, uploadForm } from './api';
import type { AvatarPresign, AvatarResult } from '../types';

const unwrap = <T>(response: { code: number; message: string; data?: T }) => {
  if (response.code !== 0 || !response.data) throw new Error(response.message);
  return response.data;
};

export type Crop = { x: number; y: number; width: number; height: number };

async function multipartUpload(file: File, crop: Crop, onProgress: (value: number) => void) {
  const form = new FormData();
  form.append('file', file);
  form.append('crop_x', String(crop.x));
  form.append('crop_y', String(crop.y));
  form.append('crop_width', String(crop.width));
  form.append('crop_height', String(crop.height));
  onProgress(20);
  const result = unwrap(await uploadForm<AvatarResult>('/media/avatars', form));
  onProgress(100);
  return result;
}

export const mediaService = {
  async uploadAvatar(file: File, crop: Crop, onProgress: (value: number) => void) {
    const presignResponse = await api.post<AvatarPresign>('/media/avatars/presign', {
      content_type: file.type,
      size: file.size,
    });

    // Validation/auth failures will also fail through the multipart endpoint, so do
    // not hide them behind a second request. Server/storage failures can use the
    // API relay as a genuine fallback.
    if (presignResponse.code !== 0 || !presignResponse.data) {
      if (presignResponse.code >= 500) return multipartUpload(file, crop, onProgress);
      throw new Error(presignResponse.message);
    }

    const presign = presignResponse.data;
    try {
      await putFile(presign.upload_url, file, presign.headers, onProgress);
    } catch {
      return multipartUpload(file, crop, onProgress);
    }

    // Processing and crop validation errors must be shown to the user instead of
    // silently repeating the upload through another transport.
    return unwrap(await api.post<AvatarResult>('/media/avatars/complete', {
      upload_id: presign.upload_id,
      crop,
    }));
  },
  remove: async () => unwrap(await api.delete<{ avatar_url: string }>('/auth/avatar')),
};
