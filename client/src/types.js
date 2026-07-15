/**
 * @typedef {Object} User
 * @property {string} id
 * @property {string} username
 * @property {string} [avatar_color]
 * @property {string} [avatar_url]
 * @property {string} [email]
 * @property {string} [status]
 * @property {string} [last_seen]
 * @property {string} [role]
 */

/**
 * @typedef {Object} Reaction
 * @property {string} emoji
 * @property {number} count
 * @property {string[]} [user_ids]
 * @property {boolean} [me]
 */

/**
 * @typedef {Object} Attachment
 * @property {string} id
 * @property {string} filename
 * @property {string} mime_type
 * @property {number} size
 * @property {string} url
 */

/**
 * @typedef {Object} PinnedContent
 * @property {string} id
 * @property {string} content
 * @property {string} pinned_at
 */

/**
 * @typedef {Object} Message
 * @property {string} id
 * @property {string} chat_id
 * @property {string} content
 * @property {string} user_id
 * @property {User} [author]
 * @property {string} created_at
 * @property {string|null} [edited_at]
 * @property {boolean} [deleted]
 * @property {Attachment[]} [attachments]
 * @property {Reaction[]} [reactions]
 * @property {number} [reaction_count]
 * @property {boolean} [streaming]
 * @property {Function} [source]
 */

/**
 * @typedef {Object} Chat
 * @property {string} id
 * @property {string} type
 * @property {string} [name]
 * @property {string} [icon_color]
 * @property {string} [owner_id]
 * @property {string} [visibility]
 * @property {boolean} [pinned]
 * @property {string} created_at
 * @property {string} [last_message_at]
 * @property {Message} [last_message]
 * @property {number} [member_count]
 * @property {User[]} [members]
 * @property {PinnedContent|null} [pinned_message]
 * @property {string|null} [pinned_updated_at]
 * @property {string|null} [pinned_last_read_at]
 * @property {number} [unread_count]
 */

/**
 * @typedef {Object} StreamSource
 * @property {'mock'|'sse'} [type]
 * @property {Function} [fn]
 * @property {string} [url]
 */

export default {};
