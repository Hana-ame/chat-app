// realtime 事件 → store 动作的分发器。
//
// 设计:本模块不 import store,由装配方(store/chat.js)注入桥接对象
// { set, get, actions },actions 是惰性函数(调用时返回 store 实例)——
// 避免 store ↔ dispatcher 的模块级循环依赖,分发逻辑独立可测。
//
// 为什么存在:realtime 事件(op + payload)与 store 动作之间存在一张
// 映射表;把它从 store 里拆出来,store 只负责状态与动作,coordinator
// 只负责连接与事件,这里的 switch 是两者之间唯一的翻译层。

/**
 * @param {{ set: Function, get: Function, actions: () => Object }} bridge
 */
export function createOpHandlers(bridge) {
  const { set, get, actions } = bridge;

  const onReady = ({ onlineUserIds, chats }) => {
    set({ onlineUserIds, wsReady: true, sseReady: true });
    actions().setChats(chats || []);
  };

  const onEvent = (op, payload) => {
    const a = actions();
    switch (op) {
      case 'message_create': a.onMessageCreate(payload); break;
      case 'message_update': a.onMessageUpdate(payload); break;
      case 'message_delete': a.onMessageDelete(payload); break;
      case 'reaction_add': a.onReaction(payload, true); break;
      case 'reaction_remove': a.onReaction(payload, false); break;
      case 'chat_create': case 'chat_update': a.onChatUpdate(payload); break;
      case 'chat_delete': a.onChatDelete(payload); break;
      case 'chat_remove': a.onChatRemove(payload); break;
      case 'presence_update': {
        set(s => {
          const ids = new Set(s.onlineUserIds);
          if (payload.status === 'online') ids.add(payload.user_id);
          else ids.delete(payload.user_id);
          return { onlineUserIds: [...ids] };
        });
        break;
      }
      case 'user_update': {
        set(s => ({
          chats: s.chats.map(c => ({
            ...c,
            members: c.members?.map(m => m.id === payload.id ? { ...m, ...payload } : m),
          })),
          userUpdateVer: (s.userUpdateVer || 0) + 1,
        }));
        break;
      }
      case 'poll:chats': a.setChats(payload); break;
      case 'poll:messages': set({ messages: (payload || []).map(m => get()._normalize(m)) }); break;
      // 已知无 UI 消费者的事件(typing 等)静默;其余忽略以保持原有行为。
      default: break;
    }
  };

  const onClose = () => {
    set({ wsReady: false, sseReady: false });
  };

  return {
    onReady,
    onEvent,
    onClose,
    getActiveChatId: () => get().activeChatId,
  };
}
