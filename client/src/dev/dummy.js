const ME = { id: 'dev-self', username: 'Alice', avatar_color: '#5865F2' };
const BOB = { id: 'dev-bob', username: 'Bob', avatar_color: '#23a559' };
const CAROL = { id: 'dev-carol', username: 'Carol', avatar_color: '#f0b232' };

const USERS = [ME, BOB, CAROL];

let seqId = Date.now();
const id = (t) => `${t}-${seqId++}`;

function timeAgo(seconds) {
  return new Date(Date.now() - seconds * 1000).toISOString();
}

const MSG_CONTENTS = [
  'Hey team! How is everyone doing today?',
  'I just pushed a new feature, check it out 🚀',
  'Can someone review my PR?',
  'Sure, I\'ll take a look',
  'Meeting at 3pm, don\'t be late!',
  'LGTM 👍',
  'Can we add dark mode support?',
  'Already done, check settings',
  'This is a very long message that goes on and on and should wrap to multiple lines to test text wrapping behavior in the chat view with long unbroken strings or many words repeated over and over again to fill the space.',
  '# Heading\n\nSome **markdown** with `inline code` and a list:\n- item 1\n- item 2',
  '```js\nconsole.log("hello world");\n```',
  '> Blockquote test\n\nContinuing after quote',
  'Just a simple text message',
  '**bold** and *italic* and ~~strikethrough~~',
  '😂😂😂',
  'Message with an [external link](https://example.com)',
];

const REACTION_EMOJIS = ['👍','❤️','😂','🎉','😢','😡','👀'];

function pick(arr) {
  return arr[Math.floor(Math.random() * arr.length)];
}

export function generateDummyData({ chatCount = 3, msgPerChat = 30 } = {}) {
  const chats = [];
  const allMessages = [];

  const loginTime = Date.now();

  for (let ci = 0; ci < chatCount; ci++) {
    const chatId = id('chat');
    const isDM = ci === 0;
    const memberCount = isDM ? 2 : [3, 5][ci % 2];
    const members = USERS.slice(0, Math.min(memberCount, USERS.length));
    const chatName = isDM ? null : ['General', 'Random', 'Dev Team'][ci % 3];
    const chat = {
      id: chatId,
      name: chatName,
      type: isDM ? 'dm' : 'group',
      visibility: ci === 2 ? 'public' : 'private',
      pinned: ci === 1,
      members,
      owner_id: ME.id,
      created_at: timeAgo(86400 * (ci + 1)),
      last_message_at: timeAgo(60 * (ci + 1)),
      unread_count: ci === 0 ? 3 : 0,
      last_message: null,
    };
    chats.push(chat);

    const msgs = [];
    for (let mi = 0; mi < msgPerChat; mi++) {
      const author = pick(members);
      const content = pick(MSG_CONTENTS);
      const createdAt = timeAgo((msgPerChat - mi) * 120 + ci * 3600);
      const isDeleted = mi === 1;
      const msg = {
        id: id('msg'),
        chat_id: chatId,
        content: isDeleted ? '' : content,
        user_id: author.id,
        author: { ...author },
        created_at: createdAt,
        edited_at: mi === 3 ? createdAt : null,
        deleted: isDeleted,
        attachments: mi === 4 ? [
          { id: id('att'), filename: 'photo.png', mime_type: 'image/png', size: 204800, url: 'https://upload.moonchan.xyz/api/test/photo.png' },
          { id: id('att'), filename: 'document.pdf', mime_type: 'application/pdf', size: 1024000, url: 'https://upload.moonchan.xyz/api/test/doc.pdf' },
        ] : [],
        reactions: mi > 5 && mi % 3 === 0 ? (() => {
          const count = Math.floor(Math.random() * 3);
          const set = new Set();
          for (let i = 0; i < count; i++) set.add(pick(REACTION_EMOJIS));
          return [...set].map(emoji => ({
            emoji,
            count: Math.floor(Math.random() * 3) + 1,
            me: Math.random() > 0.7,
          }));
        })() : [],
      };
      msgs.push(msg);
    }
    chat.last_message = msgs[msgs.length - 1];
    allMessages.push(...msgs);
  }

  return { chats, messages: allMessages, activeChatId: chats[0]?.id };
}
