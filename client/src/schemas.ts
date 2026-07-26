import { z } from 'zod';

export const UserSchema = z.object({
  id: z.string(),
  username: z.string(),
  avatar_color: z.string().optional(),
  avatar_url: z.string().optional(),
  email: z.string().optional(),
  status: z.string().optional(),
  last_seen: z.string().optional(),
  role: z.string().optional(),
  notify_blocked: z.array(z.string()).optional(),
});

export const ReactionSchema = z.object({
  emoji: z.string(),
  count: z.number(),
  user_ids: z.array(z.string()).optional(),
  me: z.boolean().optional(),
});

export const AttachmentSchema = z.object({
  id: z.string(),
  filename: z.string(),
  mime_type: z.string(),
  size: z.number(),
  url: z.string(),
});

export const PinnedContentSchema = z.object({
  id: z.string(),
  content: z.string(),
  pinned_at: z.string(),
});

export const MessageSchema = z.object({
  id: z.string(),
  chat_id: z.string(),
  content: z.string(),
  user_id: z.string(),
  type: z.string().optional(),
  author: UserSchema.optional(),
  created_at: z.string(),
  edited_at: z.string().nullable().optional(),
  deleted_at: z.string().nullable().optional(),
  deleted: z.boolean().optional(),
  attachment_count: z.number().optional(),
  mention_count: z.number().optional(),
  reaction_count: z.number().optional(),
  attachments: z.array(AttachmentSchema).optional(),
  reactions: z.array(ReactionSchema).optional(),
  mentions: z.array(z.string()).optional(),
  stream_url: z.string().optional(),
  streaming: z.boolean().optional(),
  source: z.function().optional(),
});

export const ChatSchema = z.object({
  id: z.string(),
  type: z.string(),
  name: z.string().optional(),
  icon_color: z.string().optional(),
  avatar_url: z.string().optional(),
  banner_url: z.string().optional(),
  banner_opacity: z.number().optional(),
  background_url: z.string().optional(),
  owner_id: z.string().optional(),
  visibility: z.string().optional(),
  pinned: z.boolean().optional(),
  created_at: z.string(),
  last_message_at: z.string().optional(),
  last_message: MessageSchema.optional(),
  member_count: z.number().optional(),
  members: z.array(UserSchema).optional(),
  pinned_message: PinnedContentSchema.nullable().optional(),
  pinned_updated_at: z.string().nullable().optional(),
  pinned_last_read_at: z.string().nullable().optional(),
  notify_enabled: z.boolean().optional(),
  last_active_at: z.string().nullable().optional(),
  last_message_id: z.string().optional(),
  unread_count: z.number().optional(),
});

export const AuthResponseSchema = z.object({
  user: UserSchema,
  access_token: z.string(),
  expires_in: z.number(),
});

export type User = z.infer<typeof UserSchema>;
export type Reaction = z.infer<typeof ReactionSchema>;
export type Attachment = z.infer<typeof AttachmentSchema>;
export type PinnedContent = z.infer<typeof PinnedContentSchema>;
export interface Message extends z.infer<typeof MessageSchema> {
  source?: () => void;
}
export type Chat = z.infer<typeof ChatSchema>;
export type AuthResponse = z.infer<typeof AuthResponseSchema>;

export interface StreamSource {
  type?: 'mock' | 'sse';
  fn?: () => void;
  url?: string;
}

export function validate<T>(schema: z.ZodType<T>, data: unknown, label: string): T {
  const result = schema.safeParse(data);
  if (!result.success) {
    console.error(`[zod] ${label} validation failed:`, result.error.issues);
    throw new Error(`API response validation failed: ${label}`);
  }
  return result.data;
}
