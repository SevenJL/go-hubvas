import { api } from './api';
import type { CanvasInfo, CanvasMember } from '../types';

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

  async listShared(): Promise<CanvasInfo[]> {
    const res = await api.get<CanvasInfo[]>('/canvases/shared');
    if (res.code !== 0 || !res.data) throw new Error(res.message);
    return res.data;
  },

  async listMembers(id: string): Promise<CanvasMember[]> {
    const res = await api.get<CanvasMember[]>(`/canvases/${id}/members`);
    if (res.code !== 0 || !res.data) throw new Error(res.message);
    return res.data;
  },

  async addMember(id: string, username: string, role: Exclude<CanvasMember['role'], 'owner'>): Promise<CanvasMember> {
    const res = await api.post<CanvasMember>(`/canvases/${id}/members`, { username, role });
    if (res.code !== 0 || !res.data) throw new Error(res.message);
    return res.data;
  },

  async updateMemberRole(id: string, userId: string, role: Exclude<CanvasMember['role'], 'owner'>): Promise<CanvasMember> {
    const res = await api.put<CanvasMember>(`/canvases/${id}/members/${userId}`, { role });
    if (res.code !== 0 || !res.data) throw new Error(res.message);
    return res.data;
  },

  async removeMember(id: string, userId: string): Promise<void> {
    const res = await api.delete<void>(`/canvases/${id}/members/${userId}`);
    if (res.code !== 0) throw new Error(res.message);
  },

  async publish(id: string): Promise<void> {
    const res = await api.post<void>(`/canvases/${id}/publish`);
    if (res.code !== 0) throw new Error(res.message);
  },

  async fork(id: string): Promise<CanvasInfo> {
    const res = await api.postIdempotent<CanvasInfo>(`/canvases/${id}/fork`);
    if (res.code !== 0 || !res.data) throw new Error(res.message);
    return res.data;
  },

  async delete(id: string): Promise<void> {
    const res = await api.delete<void>(`/canvases/${id}`);
    if (res.code !== 0) throw new Error(res.message);
  },
};
