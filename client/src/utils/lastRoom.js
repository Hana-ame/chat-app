// lastRoom.js — 最近聊天记忆（【本地改动 2026-09-03】对齐 chatto FDR-026）
//
// 背景：登录/刷新后应回到上次所在的聊天，而不是固定落到通知页。
// chatto FDR-026 用 localStorage 按服务器记录 last room；chat-app 是单服务器，
// 故用一个 key 即可。本模块只含纯函数（读写 localStorage），可单测。
//
// 边界：
//   - 只记录真实聊天（非 notify 通知聊天；notifications 是特殊视图，不应覆盖记忆）。
//   - 记录在登出/聊天不可达时清除（由调用方触发 clearLastRoom）。
//   - localStorage 不可用（隐私模式/被禁用）时静默降级为无记忆。

const LAST_ROOM_KEY = 'chatapp:lastRoom';

/** @returns {string} 保存的最近聊天 id（无则 ''） */
export function getLastRoom() {
  try {
    return localStorage.getItem(LAST_ROOM_KEY) || '';
  } catch {
    return '';
  }
}

/** 记录最近聊天 id。 */
export function setLastRoom(id) {
  if (!id) return;
  try {
    localStorage.setItem(LAST_ROOM_KEY, id);
  } catch {
    // 隐私模式等场景静默失败，不打扰
  }
}

/** 清除最近聊天记录（登出 / 聊天不可达时）。 */
export function clearLastRoom() {
  try {
    localStorage.removeItem(LAST_ROOM_KEY);
  } catch {
    // ignore
  }
}

export { LAST_ROOM_KEY };
