const ME = { id: 'dev-self', username: 'Alice', avatar_color: '#5865F2', status: 'online', last_seen: new Date().toISOString() };
const USERS = [
  ME,
  { id: 'dev-bob', username: 'Bob', avatar_color: '#23a559', status: 'online', last_seen: new Date().toISOString() },
  { id: 'dev-carol', username: 'Carol', avatar_color: '#f0b232', status: 'online', last_seen: new Date().toISOString() },
  { id: 'dev-dave', username: 'Dave', avatar_color: '#ed4245', status: 'online', last_seen: new Date(Date.now() - 60000).toISOString() },
  { id: 'dev-eve', username: 'Eve', avatar_color: '#9b59b6', status: 'online', last_seen: new Date().toISOString() },
  { id: 'dev-frank', username: 'Frank', avatar_color: '#1abc9c', status: 'offline', last_seen: new Date(Date.now() - 86400000).toISOString() },
];

let seqId = 1;
function id() { return `chat-${seqId++}`; }
function mid() { return `msg-${seqId++}`; }

function timeAgo(seconds) {
  return new Date(Date.now() - seconds * 1000).toISOString();
}

const GM = { id: 'dev-gm', username: 'System', avatar_color: '#4f545c' };

const GROUP_TOPICS = {
  General: [
    ['Hey everyone! How was your weekend?', 'dev-eve'],
    ['Pretty good, finished a new game', 'dev-bob'],
    ['Nice! Which one?', 'dev-dave'],
    ['Elden Ring, finally beat Malenia', 'dev-bob'],
    ['Wow nice, I\'m still stuck on the fire giant', 'dev-carol'],
    ['Has anyone tried the new update?', 'dev-frank'],
    ['Yeah, the UI is much smoother now', 'dev-self'],
    ['Can we add more emoji reactions?', 'dev-eve'],
    ['I\'ll check with the team', 'dev-dave'],
    ['Sounds great! We need more cat emojis 🐱', 'dev-carol'],
    ['**bold** and *italic* test here', 'dev-frank'],
    ['~~strikethrough test~~ works now', 'dev-bob'],
    ['Here is a code snippet:\n```js\nconst x = 42;\nconsole.log(x);\n```', 'dev-self'],
    ['Meeting at 3pm today to discuss the roadmap', 'dev-dave'],
    ['I\'ll prepare the slides', 'dev-eve'],
    ['Can someone share the design mockups?', 'dev-frank'],
    ['Sure, I\'ll upload them', 'dev-carol'],
    ['LGTM 👍', 'dev-bob'],
    ['This is a very long message that goes on and on and should wrap to multiple lines to test text wrapping behavior in the chat view with long unbroken strings or many words repeated over and over again to fill up the space properly.', 'dev-eve'],
    ['Check out this link: https://example.com', 'dev-frank'],
    ['Anyone tried the new dark mode?', 'dev-self'],
    ['Yes, it looks great!', 'dev-self'],
    ['Love the contrast ratio', 'dev-self'],
    ['Also the animations are smooth', 'dev-self'],
  ],
  Random: [
    ['Did anyone watch the new movie?', 'dev-dave'],
    ['Yeah it was amazing! The plot twist blew my mind', 'dev-carol'],
    ['I preferred the original honestly', 'dev-bob'],
    ['Hot take: the soundtrack carried the whole film', 'dev-frank'],
    ['Anyone playing the new season?', 'dev-eve'],
    ['I\'m grinding ranked, almost diamond', 'dev-self'],
    ['Nice, add me: Bob#1234', 'dev-bob'],
    ['I saw a funny meme about that', 'dev-carol'],
    ['😂😂😂', 'dev-dave'],
    ['Check this out: **super cool** announcement soon', 'dev-eve'],
    ['When is the next patch dropping?', 'dev-frank'],
    ['Probably next Tuesday', 'dev-bob'],
    ['Another ~~terrible~~ take from Bob', 'dev-carol'],
    ['How dare you 😂', 'dev-bob'],
    ['```py\ndef hello():\n    print("hello")\n```', 'dev-dave'],
    ['I made this, thoughts?', 'dev-frank'],
    ['Looks clean! Maybe add more comments', 'dev-self'],
    ['Will there be pizza at the next event?', 'dev-eve'],
    ['Always pizza 🍕', 'dev-carol'],
  ],
  'Dev Team': [
    ['PR is ready for review: `feat/add-dark-mode`', 'dev-self'],
    ['I\'ll take a look after standup', 'dev-bob'],
    ['Found a bug in the auth flow', 'dev-dave'],
    ['Can you file an issue?', 'dev-eve'],
    ['Already did, #142', 'dev-dave'],
    ['The CI is failing on main', 'dev-frank'],
    ['Looking into it now', 'dev-bob'],
    ['It was a missing env var, fixed', 'dev-bob'],
    ['Ship it 🚀', 'dev-carol'],
    ['We need to refactor the message store', 'dev-eve'],
    ['Agreed, it\'s getting bloated', 'dev-self'],
    ['I\'ll draft a proposal', 'dev-frank'],
    ['```go\nfunc main() {\n    fmt.Println("hello")\n}\n```', 'dev-dave'],
    ['Coverage dropped by 2%, need more tests', 'dev-bob'],
    ['On it', 'dev-carol'],
    ['New endpoint: `GET /api/health`', 'dev-frank'],
    ['Nice, that was on the roadmap', 'dev-eve'],
    ['~~old approach is deprecated~~', 'dev-bob'],
    ['Fixed the pagination bug finally', 'dev-dave'],
    ['*applause*', 'dev-self'],
  ],
  Gaming: [
    ['Anyone up for Valorant tonight?', 'dev-dave'],
    ['I\'m in, what rank?', 'dev-bob'],
    ['Plat 3', 'dev-dave'],
    ['Nice, I\'m Diamond 1', 'dev-frank'],
    ['Carry me 😂', 'dev-dave'],
    ['New map looks sick', 'dev-eve'],
    ['Have you tried the new agent?', 'dev-carol'],
    ['Yeah, she\'s OP', 'dev-bob'],
    ['Nerf incoming for sure', 'dev-self'],
    ['GG last night team', 'dev-frank'],
    ['That clutch round was insane', 'dev-dave'],
    ['I panicked and somehow won 😂', 'dev-frank'],
    ['```\nkda: 25/3/10\n```', 'dev-bob'],
    ['Not bad!', 'dev-carol'],
    ['When is the tournament?', 'dev-eve'],
    ['Next Saturday 8pm', 'dev-dave'],
    ['I\'ll bring snacks', 'dev-carol'],
    ['**sign me up**', 'dev-self'],
    ['~~bot Bob~~ just kidding', 'dev-dave'],
  ],
  'Music Club': [
    ['New album dropped today!', 'dev-eve'],
    ['Which one?', 'dev-carol'],
    ['The one by that indie band, you know the one', 'dev-eve'],
    ['Send the link', 'dev-frank'],
    ['https://example.com/album', 'dev-eve'],
    ['I\'m more into electronic lately', 'dev-bob'],
    ['Check out this track: **banger** 🔥', 'dev-dave'],
    ['Added to my playlist', 'dev-self'],
    ['Anyone going to the concert Friday?', 'dev-carol'],
    ['I have tickets! Section B', 'dev-frank'],
    ['Lucky! Sold out in 5 minutes', 'dev-eve'],
    ['I know, I was sweating', 'dev-frank'],
    ['We should form a band', 'dev-dave'],
    ['I call drums', 'dev-bob'],
    ['I\'ll sing ~~badly~~', 'dev-carol'],
    ['```guitar\nAm  G  C  F\n```', 'dev-dave'],
    ['What genre?', 'dev-eve'],
    ['Jazz fusion obviously', 'dev-frank'],
    ['The new music video is wild', 'dev-bob'],
    ['The cinematography is **gorgeous**', 'dev-self'],
  ],
  'Movie Night': [
    ['Suggestions for this week?', 'dev-frank'],
    ['I heard the new horror movie is good', 'dev-eve'],
    ['Not a fan of horror', 'dev-carol'],
    ['How about a classic?', 'dev-bob'],
    ['We could do a Marvel marathon', 'dev-dave'],
    ['That\'s 20 hours...', 'dev-carol'],
    ['Pick the best 3', 'dev-frank'],
    ['Winter Soldier, Infinity War, No Way Home', 'dev-bob'],
    ['Good picks!', 'dev-self'],
    ['I\'ll bring popcorn 🍿', 'dev-eve'],
    ['What time Friday?', 'dev-dave'],
    ['7pm works for me', 'dev-carol'],
    ['Same', 'dev-bob'],
    ['~~I can\'t make it~~ actually I can!', 'dev-frank'],
    ['The ~~sequel was terrible~~', 'dev-eve'],
    ['Controversial opinion: the original is overrated', 'dev-dave'],
    ['*gasps*', 'dev-carol'],
    ['Let\'s vote on the movie', 'dev-self'],
    ['I\'ll set up a poll', 'dev-frank'],
    ['This is a very long message that goes on and on and should wrap to multiple lines to test text wrapping behavior in the chat view with long unbroken strings or many words repeated over and over again to fill up the space properly.', 'dev-dave'],
  ],
  'Food & Cooking': [
    ['Anyone know a good pasta recipe?', 'dev-carol'],
    ['Carbonara is easy and delicious', 'dev-bob'],
    ['Send recipe please!', 'dev-carol'],
    ['```recipe\n- spaghetti 200g\n- eggs 3\n- pancetta 100g\n- pecorino 50g\n- black pepper\n```', 'dev-bob'],
    ['Mouth is watering 🤤', 'dev-dave'],
    ['I made ramen from scratch last weekend', 'dev-eve'],
    ['That\'s impressive', 'dev-frank'],
    ['Took 6 hours but worth it', 'dev-eve'],
    ['**best** ramen I\'ve ever had', 'dev-frank'],
    ['I\'ll stick to instant noodles 😂', 'dev-self'],
    ['Nothing wrong with instant', 'dev-carol'],
    ['Add an egg and some green onions, instant gourmet', 'dev-bob'],
    ['Pro tip: sesame oil', 'dev-dave'],
    ['Anyone tried the new Korean place downtown?', 'dev-frank'],
    ['Yes! The bibimbap is amazing', 'dev-eve'],
    ['I\'m going there tomorrow', 'dev-carol'],
    ['Get the fried chicken too', 'dev-bob'],
    ['~~skip the kimchi~~ EVERYONE GET THE KIMCHI', 'dev-dave'],
    ['Let\'s do a potluck next week!', 'dev-carol'],
  ],
  'Travel Pics': [
    ['Just got back from Japan 🇯🇵', 'dev-eve'],
    ['Amazing! How was Tokyo?', 'dev-self'],
    ['Incredible, the food, the culture, everything', 'dev-eve'],
    ['Did you visit Kyoto?', 'dev-frank'],
    ['Yeah, the bamboo forest was magical', 'dev-eve'],
    ['I\'m planning a trip to Europe next summer', 'dev-dave'],
    ['Highly recommend Barcelona', 'dev-carol'],
    ['Paris is overrated tbh', 'dev-bob'],
    ['**controversial** but true', 'dev-frank'],
    ['Italy is where it\'s at', 'dev-eve'],
    ['Rome, Florence, Venice - all stunning', 'dev-carol'],
    ['I want to go to New Zealand', 'dev-self'],
    ['Me too! The landscapes look unreal', 'dev-dave'],
    ['~~travel is expensive~~ worth every penny', 'dev-bob'],
    ['Anyone been to Iceland?', 'dev-frank'],
    ['Yes! Northern lights are breathtaking', 'dev-eve'],
    ['Adding to my bucket list', 'dev-carol'],
    ['Let\'s plan a group trip!', 'dev-dave'],
    ['That would be epic', 'dev-self'],
  ],
  'Pet Lovers': [
    ['Look at my new puppy!', 'dev-carol'],
    ['So cute! What breed?', 'dev-dave'],
    ['Golden retriever', 'dev-carol'],
    ['I have two cats, they\'re siblings', 'dev-eve'],
    ['Cat tax please 🐱', 'dev-self'],
    ['My dog learned a new trick today', 'dev-frank'],
    ['What trick?', 'dev-bob'],
    ['Roll over! He\'s so smart', 'dev-frank'],
    ['~~~smart~~ treat-motivated', 'dev-bob'],
    ['Aren\'t we all 😂', 'dev-frank'],
    ['Pets make life better', 'dev-eve'],
    ['**fact**', 'dev-carol'],
    ['Anyone else have exotic pets?', 'dev-dave'],
    ['I have a bearded dragon', 'dev-bob'],
    ['No way! What\'s its name?', 'dev-dave'],
    ['Spike', 'dev-bob'],
    ['Perfect name', 'dev-frank'],
    ['I\'m thinking of adopting from the shelter', 'dev-self'],
    ['Do it! Rescue animals are the best', 'dev-carol'],
  ],
};
const GROUP_NAMES = Object.keys(GROUP_TOPICS);

const REACTION_EMOJIS = ['👍','❤️','😂','🎉','😢','😡','👀'];

function expandMessages(topicMsgs) {
  const result = [];
  for (const [content, userId] of topicMsgs) {
    result.push({ content, userId, isSystem: userId === 'GM' });
  }
  return result;
}

const aliceImgUrl = 'https://proxy.moonchan.xyz/mw2000/78318f19gy1id3yg7ubx0j20k00hkdhr.jpg?proxy_host=wx2.sinaimg.cn&proxy_referer=https%3A%2F%2Fweibo.com%2F';
const dummyFiles = [
  { name: 'project-spec.pdf', mime: 'application/pdf', size: 512000, url: 'https://upload.moonchan.xyz/api/test/spec.pdf' },
  { name: 'archive.zip', mime: 'application/zip', size: 2048000, url: 'https://upload.moonchan.xyz/api/test/arc.zip' },
  { name: 'meeting-notes.txt', mime: 'text/plain', size: 10240, url: 'https://upload.moonchan.xyz/api/test/notes.txt' },
  { name: 'budget.xlsx', mime: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet', size: 307200, url: 'https://upload.moonchan.xyz/api/test/budget.xlsx' },
];

export function generateDummyData({ chatCount = 10, msgPerChat = 65 } = {}) {
  const chats = [];
  const allMessages = [];

  for (let ci = 0; ci < chatCount; ci++) {
    const chatId = id();
    const groupName = GROUP_NAMES[ci % GROUP_NAMES.length];
    const topicMembers = GROUP_TOPICS[groupName].map(([, uid]) => USERS.find(u => u.id === uid)).filter(Boolean);
    const unique = [...new Map(topicMembers.map(m => [m.id, m])).values()].slice(0, Math.min(topicMembers.length, 6));
    const members = unique.map(u => ({ ...u }));
    const ownerMember = members[Math.floor(Math.random() * members.length)];
    ownerMember.role = 'admin';
    const adminCount = Math.floor(Math.random() * Math.max(1, Math.floor((members.length - 1) / 2)));
    const candidates = members.filter(m => m.id !== ownerMember.id).sort(() => Math.random() - 0.5);
    for (let i = 0; i < adminCount && i < candidates.length; i++) {
      const idx = members.findIndex(m => m.id === candidates[i].id);
      members[idx].role = 'admin';
    }

    const meIdx = members.findIndex(m => m.id === ME.id);
    if (meIdx >= 0) {
      members[meIdx].pinned_last_read_at = ci < 2 ? timeAgo(1200) : null;
    }

    const chat = {
      id: chatId,
      name: groupName,
      type: 'group',
      visibility: ci < 3 ? 'private' : 'public',
      pinned: ci === 0,
          pinned_message: ci < 2 ? { id: mid(), content: '📢 公告测试消息', pinned_at: timeAgo(3600) } : null,
      pinned_updated_at: ci < 2 ? timeAgo(1800) : null,
      member_count: members.length,
      members,
      owner_id: ownerMember.id,
      created_at: timeAgo(86400 * (chatCount - ci)),
      last_message_at: timeAgo(60 * (ci + 1)),
      unread_count: ci < 3 ? (ci + 1) * 2 : 0,
      last_message: null,
    };
    chats.push(chat);

    const expandedTopics = expandMessages(GROUP_TOPICS[groupName]);
    for (let mi = 0; mi < msgPerChat; mi++) {
      const src = expandedTopics[mi % expandedTopics.length];
      const { content, userId, isSystem } = src;
      const author = isSystem ? GM : USERS.find(u => u.id === userId) || members[0];
      const createdAt = timeAgo((msgPerChat - mi) * 180 + ci * 3600);
      const isDeleted = mi === 1;
      const msg = {
        id: mid(),
        chat_id: chatId,
        content: isDeleted ? '' : content,
        user_id: author.id,
        author: { ...author },
        created_at: createdAt,
        edited_at: !isDeleted && mi === 3 ? createdAt : null,
        deleted: isDeleted,
        attachments: mi === 4 ? [
          { id: id(), filename: 'photo.png', mime_type: 'image/png', size: 204800, url: 'https://upload.moonchan.xyz/api/test/photo.png' },
          { id: id(), filename: 'document.pdf', mime_type: 'application/pdf', size: 1024000, url: 'https://upload.moonchan.xyz/api/test/doc.pdf' },
        ] : [],
        reactions: [],
      };
      if (!isDeleted && ci === 0 && mi > 5 && mi % 3 === 0) {
        if (mi % 6 === 0) {
          msg.reactions = [
            { emoji: '👍', count: 2, user_ids: [ME.id, 'dev-bob'] },
            { emoji: '🎉', count: 2, user_ids: ['dev-carol', 'dev-dave'] },
          ];
        } else {
          msg.reactions = [
            { emoji: '👍', count: 2, user_ids: [ME.id, 'dev-bob'] },
            { emoji: '🎉', count: 1, user_ids: [ME.id] },
          ];
        }
      } else if (!isDeleted && mi > 5 && mi % 3 === 0) {
        if (mi % 6 === 0) {
          msg.reactions = [{ emoji: '😂', count: 3, user_ids: ['dev-bob', 'dev-carol', 'dev-dave'] }];
        } else {
          msg.reactions = [{ emoji: '😂', count: 1, user_ids: [ME.id] }];
        }
      }
      allMessages.push(msg);
      if (mi === msgPerChat - 1) chat.last_message = msg;
    }
  }

  const activeChatId = chats[1]?.id;
  const onlineUserIds = USERS.filter(u => u.status === 'online').map(u => u.id);
  return { chats, messages: allMessages, activeChatId, onlineUserIds };
}
