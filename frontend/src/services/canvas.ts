import { api } from './api';
import type { CanvasInfo } from '../types';

export const canvasService = {
  async create(title: string): Promise<CanvasInfo> {
    const res = await api.post<CanvasInfo>('/canvases', { title });
    if (res.code !== 0 || !res.data) throw new Error(res.message);
    return res.data;
  },

  async get(id: string): Promise<CanvasInfo> {
    const res = await api.get<CanvasInfo>(`/canvases/${id}`);
    if (res.code !== 0 || !res.data) throw new Error(res.message);
    return res.data;
  },

  async listMine(): Promise<CanvasInfo[]> {
    const res = await api.get<CanvasInfo[]>('/canvases');
    if (res.code !== 0 || !res.data) throw new Error(res.message);
    return res.data;
  },

  async publish(id: string): Promise<void> {
    const res = await api.post<void>(`/canvases/${id}/publish`);
    if (res.code !== 0) throw new Error(res.message);
  },

  async fork(id: string): Promise<CanvasInfo> {
    const res = await api.post<CanvasInfo>(`/canvases/${id}/fork`);
    if (res.code !== 0 || !res.data) throw new Error(res.message);
    return res.data;
  },

  async delete(id: string): Promise<void> {
    const res = await api.delete<void>(`/canvases/${id}`);
    if (res.code !== 0) throw new Error(res.message);
  },
};
