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
  author: UserSchema.optional(),
  created_at: z.string(),
  edited_at: z.string().nullable().optional(),
  deleted: z.boolean().optional(),
  attachments: z.array(AttachmentSchema).optional(),
  reactions: z.array(ReactionSchema).optional(),
  reaction_count: z.number().optional(),
  streaming: z.boolean().optional(),
  source: z.function().optional(),
});

export const ChatSchema = z.object({
  id: z.string(),
  type: z.string(),
  name: z.string().optional(),
  icon_color: z.string().optional(),
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
  unread_count: z.number().optional(),
});

export const AuthResponseSchema = z.object({
  user: UserSchema,
  access_token: z.string(),
  expires_in: z.number(),
});

export function validate<T>(schema: z.ZodType<T>, data: unknown, label: string): T {
  const result = schema.safeParse(data);
  if (!result.success) {
    console.error(`[zod] ${label} validation failed:`, result.error.issues);
    return data as T;
  }
  return result.data;
}
