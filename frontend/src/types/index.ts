// ---- API Response Envelope ----
export interface ApiResponse<T> {
  code: number;
  message: string;
  data?: T;
}

// ---- Auth ----
export interface User {
  id: number;
  username: string;
  email: string;
  avatar_url: string;
  created_at: string;
}

export interface TokenResponse {
  access_token: string;
  refresh_token: string;
  expires_at: number;
  token_type: string;
}

// ---- Canvas ----
export interface CanvasInfo {
  id: number;
  owner_id: number;
  title: string;
  visibility: 'private' | 'published';
  forked_from?: number;
  member_count: number;
  online_count: number;
  created_at: string;
  updated_at: string;
}

export interface CanvasMember {
  user_id: number;
  username: string;
  role: 'owner' | 'editor' | 'viewer' | 'commenter';
}

// ---- Community ----
export interface PublishedCanvas {
  canvas_id: number;
  author_id: number;
  author_name: string;
  title: string;
  snapshot_url: string;
  tags: string[];
  like_count: number;
  comment_count: number;
  fork_count: number;
  published_at: number;
}

export interface CommentInfo {
  id: number;
  canvas_id: number;
  author_id: number;
  author_name: string;
  content: string;
  created_at: number;
}

export interface FeedResponse {
  items: PublishedCanvas[];
  total_count: number;
  page: number;
  page_size: number;
}

// ---- WebSocket ----
export type WSMessageType = 'sync' | 'awareness' | 'presence' | 'chat' | 'ack' | 'error';

export interface WSMessage {
  type: WSMessageType;
  seq?: number;
  payload?: unknown;
}

export interface PresenceMember {
  user_id: number;
  username: string;
  avatar_url: string;
  role: string;
}

export interface PresencePayload {
  online: PresenceMember[];
}

export interface CursorData {
  x: number;
  y: number;
}

export interface AwarenessPayload {
  cursor?: CursorData;
  selection?: { x: number; y: number; width: number; height: number };
  editing_obj?: string;
}

// ---- Presence ----
export interface RoomPresence {
  user_id: number;
  username: string;
  avatar_url: string;
  role: string;
  cursor_x?: number;
  cursor_y?: number;
  editing_obj?: string;
}
