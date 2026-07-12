import { api } from './api';
import type { FeedResponse, NotificationPage, PublicProfile, RelationshipPage, ReportInfo, ReportPage } from '../types';
const unwrap = <T>(r:{code:number;message:string;data?:T}) => { if(r.code!==0||!r.data) throw new Error(r.message); return r.data };
export const socialService = {
 profile: async(username:string)=>unwrap(await api.get<PublicProfile>(`/users/${encodeURIComponent(username)}`)),
 canvases: async(username:string,page=1)=>unwrap(await api.get<FeedResponse>(`/users/${encodeURIComponent(username)}/canvases?page=${page}`)),
 followingFeed: async(page=1)=>unwrap(await api.get<FeedResponse>(`/community/following?page=${page}`)),
 follow: async(id:string)=>unwrap(await api.post<{following:boolean}>(`/users/${id}/follow`)),
 unfollow: async(id:string)=>unwrap(await api.delete<{following:boolean}>(`/users/${id}/follow`)),
 block: async(id:string)=>unwrap(await api.post<{blocked:boolean}>(`/users/${id}/block`)),
 unblock: async(id:string)=>unwrap(await api.delete<{blocked:boolean}>(`/users/${id}/block`)),
 followers: async(id:string)=>unwrap(await api.get<RelationshipPage>(`/users/${id}/followers`)),
 following: async(id:string)=>unwrap(await api.get<RelationshipPage>(`/users/${id}/following`)),
 blocks: async()=>unwrap(await api.get<RelationshipPage>('/blocks')),
 notifications: async(page=1)=>unwrap(await api.get<NotificationPage>(`/notifications?page=${page}`)),
 unread: async()=>unwrap(await api.get<{count:number}>('/notifications/unread-count')),
 read: async(id:string)=>unwrap(await api.patch<{read:boolean}>(`/notifications/${id}/read`)),
 readAll: async()=>unwrap(await api.post<{read:boolean}>('/notifications/read-all')),
 report: async(target_type:'user'|'canvas'|'comment',target_id:string,reason:string,details='')=>unwrap(await api.post<ReportInfo>('/reports',{target_type,target_id,reason,details})),
 reports: async(status='pending')=>unwrap(await api.get<ReportPage>(`/admin/reports?status=${status}`)),
 reviewReport: async(id:string,status:string,note:string)=>unwrap(await api.patch<ReportInfo>(`/admin/reports/${id}`,{status,note})),
 setUserStatus: async(id:string,status:'active'|'suspended')=>unwrap(await api.patch<{status:string}>(`/admin/users/${id}/status`,{status})),
 moderateComment: async(id:string,status:'visible'|'hidden')=>unwrap(await api.patch(`/admin/comments/${id}/moderation`,{status})),
 moderateCanvas: async(id:string,status:'visible'|'hidden')=>unwrap(await api.patch(`/admin/canvases/${id}/moderation`,{status})),
};
