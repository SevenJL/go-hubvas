import { api } from './api';
import type { FeedResponse, CommentInfo, CommentListResponse } from '../types';

export const communityService = {
  async browse(params: {
    q?: string;
    tags?: string[];
    sort_by?: string;
    page?: number;
    page_size?: number;
  }): Promise<FeedResponse> {
    const query = new URLSearchParams();
    if (params.q) query.set('q', params.q);
    if (params.tags?.length) query.set('tags', params.tags.join(','));
    if (params.sort_by) query.set('sort_by', params.sort_by);
    if (params.page) query.set('page', String(params.page));
    if (params.page_size) query.set('page_size', String(params.page_size));

    const res = await api.get<FeedResponse>(`/community?${query.toString()}`);
    if (res.code !== 0 || !res.data) throw new Error(res.message || 'Failed to load community');
    return res.data;
  },

  async like(canvasId: string): Promise<void> {
    const res = await api.post<void>(`/canvases/${canvasId}/like`);
    if (res.code !== 0) throw new Error(res.message || 'Failed to like');
  },

  async unlike(canvasId: string): Promise<void> {
    const res = await api.delete<void>(`/canvases/${canvasId}/like`);
    if (res.code !== 0) throw new Error(res.message || 'Failed to unlike');
  },

  async postComment(canvasId: string, content: string): Promise<CommentInfo> {
    const res = await api.post<CommentInfo>(`/canvases/${canvasId}/comments`, { content });
    if (res.code !== 0 || !res.data) throw new Error(res.message || 'Failed to post comment');
    return res.data;
  },

  async getComments(canvasId: string, page = 1, pageSize = 20): Promise<CommentListResponse> {
    const res = await api.get<CommentListResponse>(
      `/canvases/${canvasId}/comments?page=${page}&page_size=${pageSize}`,
    );
    if (res.code !== 0 || !res.data) throw new Error(res.message || 'Failed to load comments');
    return res.data;
  },
};
