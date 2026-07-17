export interface User {
  id: string;
  username: string;
  avatar_color?: string;
  avatar_url?: string;
  email?: string;
  status?: string;
  last_seen?: string;
  role?: string;
}

export interface Reaction {
  emoji: string;
  count: number;
  user_ids?: string[];
  me?: boolean;
}

export interface Attachment {
  id: string;
  filename: string;
  mime_type: string;
  size: number;
  url: string;
}

export interface PinnedContent {
  id: string;
  content: string;
  pinned_at: string;
}

export interface Message {
  id: string;
  chat_id: string;
  content: string;
  user_id: string;
  author?: User;
  created_at: string;
  edited_at?: string | null;
  deleted?: boolean;
  attachments?: Attachment[];
  reactions?: Reaction[];
  reaction_count?: number;
  streaming?: boolean;
  source?: () => void;
}

export interface Chat {
  id: string;
  type: string;
  name?: string;
  icon_color?: string;
  owner_id?: string;
  visibility?: string;
  pinned?: boolean;
  created_at: string;
  last_message_at?: string;
  last_message?: Message;
  member_count?: number;
  members?: User[];
  pinned_message?: PinnedContent | null;
  pinned_updated_at?: string | null;
  pinned_last_read_at?: string | null;
  unread_count?: number;
}

export interface StreamSource {
  type?: 'mock' | 'sse';
  fn?: () => void;
  url?: string;
}

export interface AuthResponse {
  user: User;
  access_token: string;
  expires_in: number;
}

export interface ApiListResponse<T> {
  status?: number;
  error?: string;
  message?: string;
  [key: string]: unknown;
}
