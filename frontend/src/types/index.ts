export interface ApiResponse<T> { code: number; message: string; data?: T }

export interface User {
  id: string; username: string; email: string; display_name: string; bio: string; website: string;
  avatar_url: string; account_role: 'user' | 'admin'; status: 'active' | 'suspended'; created_at: string; updated_at: string;
}
export interface TokenResponse { access_token: string; expires_at: number; token_type: string }
export interface RegisterResponse { user: User; tokens: TokenResponse }

export interface CanvasInfo { id: string; owner_id: string; title: string; visibility: 'private' | 'published'; forked_from?: string; member_count: number; online_count: number; current_role?: 'owner' | 'editor' | 'viewer' | 'commenter'; created_at: string; updated_at: string }
export interface CanvasMember { user_id: string; username: string; display_name?: string; avatar_url?: string; role: 'owner' | 'editor' | 'viewer' | 'commenter' }

export interface LikeStatus { liked: boolean; like_count: number }
export interface PublishedCanvas {
  canvas_id: string; author_id: string; author_name: string; author_username: string; author_avatar_url: string;
  title: string; snapshot_url: string; tags: string[]; like_count: number; is_liked: boolean;
  comment_count: number; fork_count: number; published_at: number;
}
export interface CommentInfo {
  id: string; canvas_id: string; author_id: string; author_name: string; author_username: string; author_avatar_url: string;
  parent_comment_id?: string; content: string; deleted: boolean; moderation_status: 'visible' | 'hidden'; created_at: number;
}
export interface FeedResponse { items: PublishedCanvas[]; total_count: number; page: number; page_size: number }
export interface CommentListResponse { items: CommentInfo[]; total: number }

export interface Actor { id: string; username: string; display_name: string; avatar_url: string }
export interface PublicProfile extends Actor { bio: string; website: string; published_count: number; followers_count: number; following_count: number; is_following: boolean; is_blocked: boolean; is_blocked_by: boolean; joined_at: string }
export interface RelationshipPage { items: Actor[]; total: number; page: number; page_size: number }
export interface NotificationInfo { id: string; event_type: 'follow'|'like'|'comment'|'reply'|'fork'; actor?: Actor; target_type: string; target_id: string; data: Record<string, unknown>; read_at?: string; created_at: string }
export interface NotificationPage { items: NotificationInfo[]; total: number; page: number; page_size: number }
export interface ReportInfo { id: string; reporter_id: string; target_type: 'user'|'canvas'|'comment'; target_id: string; reason: string; details: string; status: 'pending'|'reviewing'|'resolved'|'dismissed'; reviewer_id?: string; review_note: string; created_at: string; reviewed_at?: string }
export interface ReportPage { items: ReportInfo[]; total: number; page: number; page_size: number }
export interface AvatarPresign { upload_id: string; upload_url: string; expires_at: string; headers: Record<string,string> }
export interface AvatarResult { avatar_url: string; avatar_version: number }

export type WSMessageType = 'sync' | 'awareness' | 'presence' | 'chat' | 'lock' | 'unlock' | 'lock_state' | 'ack' | 'error';
export interface WSMessage { type: WSMessageType; seq?: number; payload?: unknown }
export interface PresenceMember { user_id: string; username: string; avatar_url: string; role: string }
export interface PresencePayload { online: PresenceMember[] }
export interface CursorData { x: number; y: number }
export interface AwarenessPayload { cursor?: CursorData; selection?: { x: number; y: number; width: number; height: number }; editing_obj?: string }
export interface RoomPresence { user_id: string; username: string; avatar_url: string; role: string; cursor_x?: number; cursor_y?: number; editing_obj?: string }
