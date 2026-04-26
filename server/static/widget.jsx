var API = window.__CHAT_API_HOST__ || "http://localhost:8000";
var ROOM_ID = 1;

var _a = React,
  useState = _a.useState,
  useEffect = _a.useEffect,
  useRef = _a.useRef,
  useCallback = _a.useCallback,
  useMemo = _a.useMemo;

/* ── Helpers ───────────────────────────────────────────────────────── */

var ts = function (d) {
  if (!d) return "";
  return new Date(d).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
};

var fetchAPI = function (path, opts) {
  return fetch(API + path, opts).then(function (r) { return r.json(); });
};

/* ── Styles ────────────────────────────────────────────────────────── */

var S = {
  reset: {
    boxSizing: "border-box",
    lineHeight: 1.5,
    fontFamily: '-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,sans-serif',
    fontSize: "14px",
    color: "#333",
    textAlign: "left",
  },
  ball: {
    position: "fixed", width: 50, height: 50, borderRadius: "50%",
    background: "#0f3460", color: "#fff",
    display: "flex", alignItems: "center", justifyContent: "center",
    cursor: "pointer", zIndex: 2147483647,
    boxShadow: "0 4px 12px rgba(0,0,0,0.35)",
    userSelect: "none", touchAction: "none",
    transition: "opacity 0.2s,transform 0.15s",
    fontSize: 20,
  },
  win: {
    position: "fixed", width: 360, height: 520,
    background: "#1a1a2e", color: "#eee",
    borderRadius: 12,
    boxShadow: "0 5px 30px rgba(0,0,0,0.4)",
    zIndex: 2147483646,
    display: "flex", flexDirection: "column",
    overflow: "hidden",
  },
  winMb: {
    position: "fixed", top: 0, left: 0, right: 0, bottom: 0,
    background: "#1a1a2e", color: "#eee",
    zIndex: 2147483646,
    display: "flex", flexDirection: "column",
  },
  hdr: {
    padding: "10px 14px", background: "#16213e",
    display: "flex", justifyContent: "space-between",
    alignItems: "center", borderBottom: "1px solid #0f3460",
    minHeight: 44,
  },
  title: { fontWeight: "bold", fontSize: 15, color: "#eee" },
  hdrRight: { display: "flex", alignItems: "center", gap: 12 },
  iconBtn: {
    cursor: "pointer", color: "#999", fontSize: 18,
    display: "flex", alignItems: "center", justifyContent: "center",
  },
  msgList: {
    flex: 1, overflowY: "auto", padding: 12,
    display: "flex", flexDirection: "column", gap: 6,
  },
  bubble: {
    padding: "8px 12px", borderRadius: 12,
    maxWidth: "82%", wordBreak: "break-word",
    fontSize: 14, lineHeight: 1.4, position: "relative",
  },
  bubbleSelf: {
    alignSelf: "flex-end", background: "#0f3460", color: "#eee",
    borderBottomRightRadius: 3,
  },
  bubbleOther: {
    alignSelf: "flex-start", background: "#16213e", color: "#ddd",
    borderBottomLeftRadius: 3,
  },
  bubbleSys: {
    alignSelf: "center", background: "transparent", color: "#666",
    fontSize: 12, padding: "4px 8px",
  },
  author: { fontSize: 11, color: "#aaa", marginBottom: 2, marginLeft: 4 },
  time: { fontSize: 10, color: "#666", float: "right", marginLeft: 8, marginTop: 2 },
  dot: { fontSize: 10, marginRight: 3 },
  inputRow: {
    padding: 10, borderTop: "1px solid #0f3460",
    display: "flex", gap: 8, background: "#16213e",
  },
  inp: {
    flex: 1, padding: "10px 14px", borderRadius: 20,
    border: "none", background: "#0f3460", color: "#eee",
    fontSize: 14, outline: "none",
  },
  btn: {
    padding: "0 20px", borderRadius: 20, border: "none",
    background: "#e94560", color: "#fff", fontSize: 14,
    fontWeight: "bold", cursor: "pointer",
  },
  loginBox: {
    height: "100%", display: "flex", flexDirection: "column",
    justifyContent: "center", alignItems: "center",
    gap: 16, padding: 20,
  },
};

/* ── Components ────────────────────────────────────────────────────── */

var LoginScreen = function (_a2) {
  var onLogin = _a2.onLogin;
  var _b = useState(""), name = _b[0], setName = _b[1];
  var ref = useRef(null);
  useEffect(function () { if (ref.current) ref.current.focus(); }, []);

  var randName = useMemo(function () {
    return "User" + Math.random().toString(36).substring(2, 6);
  }, []);

  var go = function () {
    onLogin(name.trim() || randName);
  };

  return React.createElement("div", { style: Object.assign({}, S.reset, S.loginBox) },
    React.createElement("h3", { style: { margin: 0, fontSize: 18, color: "#eee" } }, "Chat Room"),
    React.createElement("p", { style: { margin: 0, color: "#999", fontSize: 13 } }, "Enter a nickname"),
    React.createElement("input", {
      ref: ref, placeholder: randName, value: name,
      onChange: function (e) { return setName(e.target.value); },
      onKeyDown: function (e) { if (e.key === "Enter") go(); },
      style: Object.assign({}, S.inp, { width: "80%", flex: "none", textAlign: "center" }),
    }),
    React.createElement("button", { onClick: go, style: S.btn }, "Start Chat")
  );
};

var MessageList = function (_a2) {
  var msgs = _a2.msgs, token = _a2.token, onScrollTop = _a2.onScrollTop, loading = _a2.loading;
  return React.createElement("div", {
    style: S.msgList,
    onScroll: function (e) { if (e.target.scrollTop === 0) onScrollTop(); },
  },
    loading && React.createElement("div", { style: Object.assign({}, S.bubbleSys, { textAlign: "center" }) }, "Loading..."),
    msgs.map(function (m) {
      if (m.msg_type === "system" || m.type === "system") {
        return React.createElement("div", { key: m.id || m.content, style: S.bubbleSys }, m.content);
      }
      return React.createElement(MessageItem, { key: m.id, msg: m, token: token });
    }),
    React.createElement("div", { id: "chat-widget-end" })
  );
};

var MessageItem = function (_a2) {
  var msg = _a2.msg, token = _a2.token;
  var isSelf = msg.token === token;
  var isBot = msg.is_bot;
  var time = ts(msg.created_at);

  return React.createElement("div", {
    style: Object.assign({}, S.bubble, isSelf ? S.bubbleSelf : S.bubbleOther),
  },
    !isSelf && React.createElement("div", { style: Object.assign({}, S.author, { color: msg.avatar_color || "#aaa" }) },
      isBot ? "[Bot] " : "", msg.username
    ),
    React.createElement("div", null,
      msg.content,
      React.createElement("span", { style: S.time }, time)
    )
  );
};

var MessageInput = function (_a2) {
  var onSend = _a2.onSend, sending = _a2.sending;
  var _b = useState(""), val = _b[0], setVal = _b[1];

  var send = function () {
    if (!val.trim() || sending) return;
    onSend(val);
    setVal("");
  };

  return React.createElement("div", { style: S.inputRow },
    React.createElement("input", {
      style: S.inp, value: val, placeholder: "Type a message...",
      onChange: function (e) { return setVal(e.target.value); },
      onKeyDown: function (e) { if (e.key === "Enter") { e.preventDefault(); send(); } },
      disabled: sending,
    }),
    React.createElement("button", {
      onClick: send, disabled: sending || !val.trim(),
      style: Object.assign({}, S.btn, {
        background: sending || !val.trim() ? "#555" : "#e94560",
        cursor: sending || !val.trim() ? "default" : "pointer",
      }),
    }, sending ? "..." : "Send")
  );
};

/* ── Main App ──────────────────────────────────────────────────────── */

var ChatApp = function () {
  var _a2 = useState(false), open = _a2[0], setOpen = _a2[1];
  var _b = useState(window.innerWidth <= 768), mobile = _b[0], setMobile = _b[1];
  var _c = useState(false), hovered = _c[0], setHovered = _c[1];
  var _d = useState({ x: window.innerWidth - 65, y: window.innerHeight - 120 }), pos = _d[0], setPos = _d[1];
  var _e = useState(false), dragging = _e[0], setDragging = _e[1];
  var dragStart = useRef({ x: 0, y: 0 });
  var initPos = useRef({ x: 0, y: 0 });
  var moved = useRef(false);
  var lastClick = useRef(0);

  var _f = useState(""), token = _f[0], setToken = _f[1];
  var _g = useState(""), username = _g[0], setUsername = _g[1];
  var _h = useState([]), msgs = _h[0], setMsgs = _h[1];
  var _j = useState(false), loading = _j[0], setLoading = _j[1];
  var _k = useState(false), sending = _k[0], setSending = _k[1];
  var poller = useRef(null);
  var lastId = useRef(0);

  useEffect(function () {
    var t = localStorage.getItem("chat_token");
    var u = localStorage.getItem("chat_username");
    if (t && u) { setToken(t); setUsername(u); }
  }, []);

  useEffect(function () {
    var cb = function () {
      setMobile(window.innerWidth <= 768);
      setPos(function (p) { return ({ x: Math.min(p.x, window.innerWidth - 55), y: Math.min(p.y, window.innerHeight - 55) }); });
    };
    window.addEventListener("resize", cb);
    return function () { return window.removeEventListener("resize", cb); };
  }, []);

  var login = function (name) {
    var pw = "";
    fetchAPI("/api/login", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ username: name, password: pw }),
    }).then(function (d) {
      if (d.error) return alert(d.error);
      localStorage.setItem("chat_token", d.token);
      localStorage.setItem("chat_username", d.username);
      setToken(d.token);
      setUsername(d.username);
      loadHistory();
    });
  };

  var logout = function () {
    localStorage.removeItem("chat_token");
    localStorage.removeItem("chat_username");
    setToken(""); setUsername(""); setMsgs([]); lastId.current = 0;
  };

  var addMsgs = useCallback(function (arr) {
    if (!arr || arr.length === 0) return;
    setMsgs(function (prev) {
      var ids = new Set(prev.map(function (m) { return m.id; }));
      var fresh = arr.filter(function (m) { return !ids.has(m.id); });
      if (fresh.length === 0) return prev;
      var mix = prev.concat(fresh).sort(function (a, b) { return a.id - b.id; });
      lastId.current = mix[mix.length - 1].id;
      return mix;
    });
  }, []);

  var loadHistory = function () {
    var t = localStorage.getItem("chat_token");
    fetchAPI("/api/history/" + ROOM_ID + "?token=" + t + "&limit=50").then(function (d) {
      if (d.messages) addMsgs(d.messages);
    });
  };

  var loadMore = function () {
    if (msgs.length === 0 || loading) return;
    setLoading(true);
    var t = localStorage.getItem("chat_token");
    var oldest = msgs[0] ? msgs[0].id : lastId.current;
    fetchAPI("/api/history/" + ROOM_ID + "?token=" + t + "&after_id=" + (oldest - 50) + "&limit=50").then(function (d) {
      if (d.messages && d.messages.length > 0) {
        var ids = new Set(msgs.map(function (m) { return m.id; }));
        var newMsgs = d.messages.filter(function (m) { return !ids.has(m.id); });
        if (newMsgs.length > 0) {
          setMsgs(function (prev) { return newMsgs.concat(prev).sort(function (a, b) { return a.id - b.id; }); });
        }
      }
      setLoading(false);
    });
  };

  useEffect(function () {
    if (!token || !open) return;
    poller.current = setInterval(function () {
      fetchAPI("/api/poll?room_id=" + ROOM_ID + "&token=" + token + "&after_id=" + lastId.current + "&timeout=5").then(function (d) {
        if (d.messages && d.messages.length > 0) addMsgs(d.messages);
      });
    }, 2000);
    return function () { if (poller.current) clearInterval(poller.current); };
  }, [token, open, addMsgs]);

  var sendMsg = function (text) {
    if (!text.trim()) return;
    setSending(true);
    fetchAPI("/api/msg", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ token: token, room_id: ROOM_ID, content: text }),
    }).then(function (d) {
      if (d.id) addMsgs([d]);
      setSending(false);
    }).catch(function () { setSending(false); });
  };

  /* ── Dragging ──────────────────────────────────────────────────── */

  var onStart = function (e) {
    var cx = e.touches ? e.touches[0].clientX : e.clientX;
    var cy = e.touches ? e.touches[0].clientY : e.clientY;
    setDragging(true); moved.current = false;
    dragStart.current = { x: cx, y: cy };
    initPos.current = Object.assign({}, pos);
    setHovered(true);
  };
  var onMove = function (e) {
    if (!dragging) return;
    if (e.cancelable && e.type === "touchmove") e.preventDefault();
    var cx = e.touches ? e.touches[0].clientX : e.clientX;
    var cy = e.touches ? e.touches[0].clientY : e.clientY;
    var dx = cx - dragStart.current.x;
    var dy = cy - dragStart.current.y;
    if (Math.abs(dx) > 3 || Math.abs(dy) > 3) moved.current = true;
    var s = 50;
    setPos({
      x: Math.max(0, Math.min(initPos.current.x + dx, window.innerWidth - s)),
      y: Math.max(0, Math.min(initPos.current.y + dy, window.innerHeight - s)),
    });
  };
  var onEnd = function () {
    setDragging(false); setHovered(false);
    if (!moved.current) {
      var now = Date.now();
      if (now - lastClick.current < 500) return;
      lastClick.current = now;
      setOpen(function (o) { return !o; });
    }
  };

  useEffect(function () {
    if (dragging) {
      window.addEventListener("mousemove", onMove);
      window.addEventListener("mouseup", onEnd);
      window.addEventListener("touchmove", onMove, { passive: false });
      window.addEventListener("touchend", onEnd);
    }
    return function () {
      window.removeEventListener("mousemove", onMove);
      window.removeEventListener("mouseup", onEnd);
      window.removeEventListener("touchmove", onMove);
      window.removeEventListener("touchend", onEnd);
    };
  }, [dragging]);

  /* ── Window position ──────────────────────────────────────────── */

  var winStyle = function () {
    if (mobile) return Object.assign({}, S.winMb, S.reset);
    var w = 360, h = 520, s = 50, gap = 15;
    var left = pos.x - w + s;
    var top = pos.y - h - gap;
    if (left < 0) left = pos.x;
    if (top < 0) top = pos.y + s + gap;
    return Object.assign({}, S.win, S.reset, { left: left + "px", top: top + "px" });
  };

  var ballStyle = Object.assign({}, S.ball, {
    left: pos.x + "px", top: pos.y + "px",
    opacity: dragging || hovered || open ? 1 : 0.65,
    transform: dragging || hovered ? "scale(1.1)" : "scale(1)",
  });

  return React.createElement(React.Fragment, null,
    open && React.createElement("div", { style: winStyle() },
      React.createElement("div", { style: S.hdr },
        React.createElement("span", { style: S.title }, "Chat Room"),
        React.createElement("div", { style: S.hdrRight },
          token && React.createElement("div", {
            style: S.iconBtn, onClick: logout, title: "Logout",
          }, React.createElement("svg", { width: 16, height: 16, viewBox: "0 0 24 24", fill: "none", stroke: "currentColor", strokeWidth: 2 }, React.createElement("path", { d: "M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4M16 17l5-5-5-5M21 12H9" }))),
          React.createElement("div", {
            style: S.iconBtn, onClick: function () { return setOpen(false); }, title: "Close",
          }, React.createElement("svg", { width: 18, height: 18, viewBox: "0 0 24 24", fill: "none", stroke: "currentColor", strokeWidth: 2 }, React.createElement("line", { x1: 18, y1: 6, x2: 6, y2: 18 }), React.createElement("line", { x1: 6, y1: 6, x2: 18, y2: 18 })))
        )
      ),
      token
        ? React.createElement(React.Fragment, null,
            React.createElement(MessageList, { msgs: msgs, token: token, onScrollTop: loadMore, loading: loading }),
            React.createElement(MessageInput, { onSend: sendMsg, sending: sending })
          )
        : React.createElement(LoginScreen, { onLogin: login })
    ),
    React.createElement("div", {
      style: ballStyle,
      onMouseDown: onStart, onTouchStart: onStart,
      onMouseEnter: function () { return setHovered(true); },
      onMouseLeave: function () { return setHovered(false); },
    },
      open
        ? React.createElement("svg", { width: 20, height: 20, viewBox: "0 0 24 24", fill: "none", stroke: "currentColor", strokeWidth: 2 }, React.createElement("line", { x1: 18, y1: 6, x2: 6, y2: 18 }), React.createElement("line", { x1: 6, y1: 6, x2: 18, y2: 18 }))
        : React.createElement("svg", { width: 20, height: 20, viewBox: "0 0 24 24", fill: "none", stroke: "currentColor", strokeWidth: 2 }, React.createElement("path", { d: "M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z" }))
    )
  );
};

/* ── Mount ────────────────────────────────────────────────────────── */

var rootEl = document.createElement("div");
rootEl.id = "chat-widget-root";
rootEl.style.all = "initial";
document.body.appendChild(rootEl);
ReactDOM.createRoot(rootEl).render(React.createElement(ChatApp));
