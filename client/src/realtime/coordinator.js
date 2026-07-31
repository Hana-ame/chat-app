import { createWsTransport } from './transports/ws';
import { createSseTransport } from './transports/sse';
import { createPollTransport } from './transports/poll';
import { createMockTransport } from './transports/mock';

const STATE = { IDLE: 0, CONNECTING: 1, CONNECTED: 2, DISCONNECTING: 3 };

const MAX_RETRY_DELAY = 30000;
const BASE_RETRY_DELAY = 1000;

class RealtimeCoordinator {
  constructor() {
    this._state = STATE.IDLE;
    this._mode = null;
    this._token = null;
    this._transport = null;
    this._handlers = {};
    this._reconnectTimer = null;
    this._reconnectAttempt = 0;
    this._gen = 0;
  }

  setHandlers(h) { this._handlers = h; }

  get mode() { return this._mode; }
  get ready() { return this._state === STATE.CONNECTED; }
  get token() { return this._token; }

  connect(mode, token) {
    this._cancelReconnect();
    this._gen++;
    // Always restart with the requested transport, even if a previous
    // connect is still in flight (e.g. mode switch before WS handshake
    // completes). Old transport callbacks are gated by _gen.
    this._teardown();
    this._reconnectAttempt = 0;
    this._state = STATE.CONNECTING;
    this._mode = mode;
    this._token = token;
    this._initTransport(mode, token);
  }

  disconnect() {
    this._cancelReconnect();
    this._gen++;
    this._teardown();
  }

  _cancelReconnect() {
    if (this._reconnectTimer) {
      clearTimeout(this._reconnectTimer);
      this._reconnectTimer = null;
    }
  }

  _scheduleReconnect() {
    this._reconnectAttempt++;
    const delay = Math.min(BASE_RETRY_DELAY * Math.pow(2, this._reconnectAttempt - 1), MAX_RETRY_DELAY);
    const gen = this._gen;
    this._reconnectTimer = setTimeout(() => {
      this._reconnectTimer = null;
      if (this._gen !== gen) return;
      if (this._state === STATE.IDLE && this._mode && this._token)
        this.connect(this._mode, this._token);
    }, delay);
  }

  _teardown() {
    const wasConnected = this._state === STATE.CONNECTED;
    this._state = STATE.DISCONNECTING;
    if (this._transport) { this._transport.disconnect(); this._transport = null; }
    this._state = STATE.IDLE;
    if (wasConnected) this._handlers.onClose?.();
  }

  _initTransport(mode, token) {
    const gen = this._gen;
    const ctx = {
      token,
      onReady: (data) => {
        if (this._gen !== gen) return;
        if (this._state !== STATE.CONNECTING) return;
        this._state = STATE.CONNECTED;
        this._reconnectAttempt = 0;
        this._handlers.onReady?.(data);
      },
      onEvent: (op, payload) => {
        if (this._gen !== gen) return;
        this._handlers.onEvent?.(op, payload);
      },
      onClose: () => {
        if (this._gen !== gen) return;
        this._state = STATE.IDLE;
        this._transport = null;
        this._handlers.onClose?.();
        this._scheduleReconnect();
      },
    };

    if (mode === 'mock') { this._transport = createMockTransport(ctx); return; }

    switch (mode) {
      case 'ws':
        this._transport = createWsTransport(ctx);
        break;
      case 'sse':
        this._transport = createSseTransport(ctx);
        break;
      case 'poll':
        this._transport = createPollTransport({
          token,
          onChats: (chats) => {
            if (this._gen !== gen) return;
            this._handlers.onEvent?.('poll:chats', chats);
          },
          onMessages: (chatId, msgs) => {
            if (this._gen !== gen) return;
            // Drop responses for a chat that is no longer active (stale in-flight fetch)
            if (this._handlers.getActiveChatId?.() !== chatId) return;
            this._handlers.onEvent?.('poll:messages', msgs);
          },
          getActiveChatId: () => this._handlers.getActiveChatId?.(),
          onClose: ctx.onClose,
        });
        this._state = STATE.CONNECTED;
        break;
      default:
        console.error('[Realtime] Unknown mode:', mode);
        this._state = STATE.IDLE;
    }
  }

  sendTyping(chatId) { this._transport?.sendTyping?.(chatId); }
  subscribe(chatId) { this._transport?.subscribe?.(chatId); }
  wsRequest(op, payload) {
    if (!this._transport?.wsRequest) return Promise.reject(new Error('WS not available'));
    return this._transport.wsRequest(op, payload);
  }
}

let _instance = null;
export function getCoordinator() {
  if (!_instance) _instance = new RealtimeCoordinator();
  return _instance;
}
