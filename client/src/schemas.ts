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
  thinking: z.string().optional(),
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

// 【本地改动 2026-08-31】持久化通知 occurrence（后端 models.NotificationOccurrence
// 的镜像；移植 chatto FDR-012 通知机制到前端契约）。与后端 JSON 字段一一对应。
export const NotificationOccurrenceSchema = z.object({
  id: z.string(),
  user_id: z.string(),
  kind: z.string(),
  chat_id: z.string(),
  message_id: z.string(),
  actor_id: z.string(),
  title: z.string(),
  body: z.string(),
  read: z.boolean(),
  created_at: z.string(),
  expires_at: z.string(),
});

export type NotificationOccurrence = z.infer<typeof NotificationOccurrenceSchema>;

// 【本地改动 2026-08-31】Web Push 订阅（后端 models.PushSubscription 的镜像；
// 移植 chatto 的 push 机制到前端契约）。endpoint/p256dh/auth 与浏览器
// PushSubscription.toJSON() 字段一一对应；created_at 为后端注册时间。
export const PushSubscriptionSchema = z.object({
  id: z.string(),
  user_id: z.string(),
  endpoint: z.string(),
  p256dh: z.string(),
  auth: z.string(),
  created_at: z.string(),
});

export type PushSubscription = z.infer<typeof PushSubscriptionSchema>;

// VAPIDPublicKeyResponse 是 /api/push/vapid-public-key 的响应契约（未配置
// 时后端返回 503，调用方捕获后静默跳过推送注册）。
export const VAPIDPublicKeyResponseSchema = z.object({
  vapid_public_key: z.string(),
});
export type VAPIDPublicKeyResponse = z.infer<typeof VAPIDPublicKeyResponseSchema>;

export function validate<T>(schema: z.ZodType<T>, data: unknown, label: string): T {
  const result = schema.safeParse(data);
  if (!result.success) {
    console.error(`[zod] ${label} validation failed:`, result.error.issues);
    throw new Error(`API response validation failed: ${label}`);
  }
  return result.data;
}
