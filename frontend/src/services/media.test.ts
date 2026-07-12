import { beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('./api', () => ({
  api: {
    post: vi.fn(),
    postIdempotent: vi.fn(),
    delete: vi.fn(),
  },
  putFile: vi.fn(),
  uploadForm: vi.fn(),
}));

import { api, putFile, uploadForm } from './api';
import { mediaService } from './media';

const mockedPost = vi.mocked(api.post);
const mockedPostIdempotent = vi.mocked(api.postIdempotent);
const mockedPutFile = vi.mocked(putFile);
const mockedUploadForm = vi.mocked(uploadForm);
const crop = { x: 0, y: 0, width: 100, height: 100 };
const file = new File(['avatar'], 'avatar.png', { type: 'image/png' });
const result = { avatar_url: 'https://cdn.example/avatars/u/1.webp', avatar_version: 1 };

describe('mediaService.uploadAvatar', () => {
  beforeEach(() => vi.clearAllMocks());

  it('uses presigned PUT and completes the upload', async () => {
    mockedPost.mockResolvedValueOnce({
      code: 0,
      message: 'ok',
      data: { upload_id: 'upload-1', upload_url: 'https://storage.example/put', headers: { 'Content-Type': 'image/png' }, expires_at: '2026-07-12T00:00:00Z' },
    });
    mockedPostIdempotent.mockResolvedValueOnce({ code: 0, message: 'ok', data: result });
    mockedPutFile.mockResolvedValueOnce();

    await expect(mediaService.uploadAvatar(file, crop, vi.fn())).resolves.toEqual(result);

    expect(mockedPutFile).toHaveBeenCalledWith('https://storage.example/put', file, { 'Content-Type': 'image/png' }, expect.any(Function));
    expect(mockedPostIdempotent).toHaveBeenCalledWith('/media/avatars/complete', { upload_id: 'upload-1', crop });
    expect(mockedUploadForm).not.toHaveBeenCalled();
  });

  it('falls back to multipart when the storage PUT transport fails', async () => {
    mockedPost.mockResolvedValueOnce({
      code: 0,
      message: 'ok',
      data: { upload_id: 'upload-2', upload_url: 'https://storage.example/put', headers: {}, expires_at: '2026-07-12T00:00:00Z' },
    });
    mockedPutFile.mockRejectedValueOnce(new Error('network'));
    mockedUploadForm.mockResolvedValueOnce({ code: 0, message: 'ok', data: result });

    await expect(mediaService.uploadAvatar(file, crop, vi.fn())).resolves.toEqual(result);

    expect(mockedUploadForm).toHaveBeenCalledOnce();
    const submitted = mockedUploadForm.mock.calls[0][1];
    expect(submitted.get('file')).toBe(file);
    expect(submitted.get('crop_width')).toBe('100');
  });

  it('does not retry multipart when complete rejects crop or image content', async () => {
    mockedPost.mockResolvedValueOnce({
      code: 0,
      message: 'ok',
      data: { upload_id: 'upload-3', upload_url: 'https://storage.example/put', headers: {}, expires_at: '2026-07-12T00:00:00Z' },
    });
    mockedPostIdempotent.mockResolvedValueOnce({ code: 400, message: 'invalid crop' });
    mockedPutFile.mockResolvedValueOnce();

    await expect(mediaService.uploadAvatar(file, crop, vi.fn())).rejects.toThrow('invalid crop');
    expect(mockedUploadForm).not.toHaveBeenCalled();
  });
});
