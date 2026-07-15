import { createWsTransport } from './transports/ws';
import { createSseTransport } from './transports/sse';
import { createPollTransport } from './transports/poll';
import { createMockTransport } from './transports/mock';

const STATE = { IDLE: 0, CONNECTING: 1, CONNECTED: 2, DISCONNECTING: 3 };

class RealtimeCoordinator {
  constructor() {
    this._state = STATE.IDLE;
    this._mode = null;
    this._token = null;
    this._transport = null;
    this._handlers = {};
    this._closeGuard = false;
  }

  setHandlers(h) { this._handlers = h; }

  get mode() { return this._mode; }
  get ready() { return this._state === STATE.CONNECTED; }
  get token() { return this._token; }

  connect(mode, token) {
    if (this._state === STATE.CONNECTING || this._state === STATE.DISCONNECTING) return;
    this._closeGuard = false;
    this._teardown();
    this._state = STATE.CONNECTING;
    this._mode = mode;
    this._token = token;
    this._initTransport(mode, token);
  }

  disconnect() {
    this._closeGuard = true;
    this._teardown();
  }

  _teardown() {
    const wasConnected = this._state === STATE.CONNECTED;
    this._state = STATE.DISCONNECTING;
    if (this._transport) { this._transport.disconnect(); this._transport = null; }
    this._state = STATE.IDLE;
    if (wasConnected) this._handlers.onClose?.();
  }

  _initTransport(mode, token) {
    const ctx = {
      token,
      onReady: (data) => this._handlers.onReady?.(data),
      onEvent: (op, payload) => this._handlers.onEvent?.(op, payload),
      onClose: () => {
        if (this._closeGuard) return;
        this._state = STATE.IDLE;
        this._transport = null;
        this._handlers.onClose?.();
        setTimeout(() => {
          if (this._state === STATE.IDLE && this._mode && this._token)
            this.connect(this._mode, this._token);
        }, 3000);
      },
    };

    if (mode === 'mock') { this._transport = createMockTransport(ctx); this._state = STATE.CONNECTED; return; }

    switch (mode) {
      case 'ws':
        this._transport = createWsTransport(ctx);
        this._state = STATE.CONNECTED;
        break;
      case 'sse':
        this._transport = createSseTransport(ctx);
        this._state = STATE.CONNECTED;
        break;
      case 'poll':
        this._transport = createPollTransport({
          token,
          onChats: (chats) => this._handlers.onEvent?.('poll:chats', chats),
          onMessages: (msgs) => this._handlers.onEvent?.('poll:messages', msgs),
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
