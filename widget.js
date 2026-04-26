/* Chat Widget — CDN embed
 * Usage: <script src="https://cdn.jsdelivr.net/gh/Hana-ame/chat-app@main/widget.js"></script>
 */
(function () {
  var API = "https://wsl-8000.moonchan.xyz";
  var ROOM = 1;
  var token = "";
  var username = "";
  var lastId = 0;
  var pollTimer = null;
  var msgIds = {};
  var open = false;

  try { token = localStorage.getItem("chat_xtoken"); username = localStorage.getItem("chat_xuser"); } catch (e) {}

  /* ── CSS ──────────────────────────────────────────────────────── */
  var css = document.createElement("style");
  css.textContent = [
    "*{margin:0;padding:0;box-sizing:border-box}",
    ".cw-ball{position:fixed;width:50px;height:50px;border-radius:50%;background:#0f3460;color:#fff;display:flex;align-items:center;justify-content:center;cursor:pointer;z-index:2147483647;box-shadow:0 4px 12px rgba(0,0,0,.35);user-select:none;touch-action:none;font-size:20px;transition:opacity .2s,transform .15s}",
    ".cw-ball:hover{opacity:1!important;transform:scale(1.1)!important}",
    ".cw-win{position:fixed;width:360px;height:520px;background:#1a1a2e;color:#eee;border-radius:12px;box-shadow:0 5px 30px rgba(0,0,0,.4);z-index:2147483646;display:flex;flex-direction:column;overflow:hidden;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;font-size:14px;line-height:1.5}",
    ".cw-hdr{padding:10px 14px;background:#16213e;display:flex;justify-content:space-between;align-items:center;border-bottom:1px solid #0f3460;min-height:44px}",
    ".cw-hdr span{font-weight:bold;font-size:15px}",
    ".cw-hdr button{background:none;border:none;color:#e74c3c;cursor:pointer;font-size:13px}",
    ".cw-msgs{flex:1;overflow-y:auto;padding:12px;display:flex;flex-direction:column;gap:6px}",
    ".cw-b{max-width:82%;padding:8px 12px;border-radius:12px;word-break:break-word;font-size:14px;line-height:1.4;position:relative}",
    ".cw-bs{align-self:flex-end;background:#0f3460;color:#eee;border-bottom-right-radius:3px}",
    ".cw-bo{align-self:flex-start;background:#16213e;color:#ddd;border-bottom-left-radius:3px}",
    ".cw-by{align-self:center;background:transparent;color:#666;font-size:12px;padding:4px 8px}",
    ".cw-aut{font-size:11px;color:#aaa;margin-bottom:2px;margin-left:4px}",
    ".cw-tm{font-size:10px;color:#666;float:right;margin-left:8px;margin-top:2px}",
    ".cw-inp{border-top:1px solid #0f3460;padding:10px;display:flex;gap:8px;background:#16213e}",
    ".cw-inp input{flex:1;padding:10px 14px;border-radius:20px;border:none;background:#0f3460;color:#eee;font-size:14px;outline:none}",
    ".cw-inp button{padding:0 20px;border-radius:20px;border:none;background:#e94560;color:#fff;font-size:14px;font-weight:bold;cursor:pointer}",
    ".cw-login{height:100%;display:flex;flex-direction:column;justify-content:center;align-items:center;gap:16px;padding:20px}",
    ".cw-login h3{font-size:18px;color:#eee}",
    ".cw-login p{color:#999;font-size:13px}",
    ".cw-login input{width:80%;padding:10px 14px;border-radius:20px;border:none;background:#0f3460;color:#eee;font-size:14px;outline:none;text-align:center}",
    ".cw-login button{padding:10px 30px;border-radius:20px;border:none;background:#e94560;color:#fff;font-size:15px;font-weight:bold;cursor:pointer}",
    ".cw-load{text-align:center;color:#666;font-size:12px;padding:4px}",
    ".cw-img{max-width:200px;max-height:200px;border-radius:8px;cursor:pointer;display:block}",
    ".cw-flnk{color:#e94560;text-decoration:none;font-size:13px;word-break:break-all}",
    ".cw-flnk:hover{text-decoration:underline}",
    ".cw-drop{position:absolute;inset:0;background:rgba(233,69,96,.12);border:2px dashed #e94560;display:flex;align-items:center;justify-content:center;color:#e94560;font-size:16px;z-index:10;pointer-events:none;border-radius:12px}",
    ".cw-upld{position:absolute;top:40px;left:50%;transform:translateX(-50%);background:#0f3460;color:#fff;padding:6px 16px;border-radius:20px;font-size:12px;z-index:11}",
    ".cw-fbtn{background:none;border:none;color:#aaa;cursor:pointer;font-size:18px;padding:0 4px}",
  ].join("\n");
  document.head.appendChild(css);

  /* ── DOM ────────────────────────────────────────────────────────── */
  function h(tag, cls, props, children) {
    var el = document.createElement(tag);
    if (cls) el.className = cls;
    if (props) for (var k in props) { el[k] = props[k]; if (k === "style") for (var s in props.style) el.style[s] = props.style[s]; }
    if (children) children.forEach(function (c) { el.appendChild(typeof c === "string" ? document.createTextNode(c) : c); });
    return el;
  }

  var ball = h("div", "cw-ball", { style: { left: window.innerWidth - 65 + "px", top: window.innerHeight - 120 + "px" } }, [
    h("svg", "", { innerHTML: '<path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/>', style: { width: "20px", height: "20px" }, setAttribute: function () { this.setAttribute("viewBox", "0 0 24 24"); this.setAttribute("fill", "none"); this.setAttribute("stroke", "currentColor"); this.setAttribute("stroke-width", "2"); } }),
  ]);
  ball.firstChild.setAttribute("viewBox", "0 0 24 24");

  var closeSvg = '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>';
  var chatSvg = '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/></svg>';

  function loadMore() {
    var msgsEl = document.querySelector(".cw-msgs");
    if (!msgsEl || msgsEl.querySelector(".cw-load")) return;
    var loadEl = h("div", "cw-load", {}, ["Loading..."]);
    msgsEl.insertBefore(loadEl, msgsEl.firstChild);
    var afterId = Math.max(0, lastId - 50);
    api("/api/history/" + ROOM + "?token=" + token + "&after_id=" + afterId + "&limit=50").then(function (d) {
      try { loadEl.remove(); } catch (e) { }
      if (d.messages && d.messages.length) addMsgs(d.messages, true);
    });
  }

  var winEl;
  function buildWin() {
    if (winEl) return winEl;

    var loginBox = h("div", "cw-login", {}, [
      h("h3", "", {}, ["Chat Room"]),
      h("p", "", {}, ["Enter nickname"]),
      h("input", "", { placeholder: "User" + Math.random().toString(36).substring(2, 6), onkeydown: function (e) { if (e.key === "Enter") doLogin(this.value); } }),
      h("button", "", { onclick: function () { doLogin(this.parentNode.querySelector("input").value); } }, ["Enter"]),
    ]);

    var chatBox = h("div", "", { style: { flex: "1", display: "none", flexDirection: "column" } }, [
      h("div", "cw-msgs"),
      h("div", "cw-inp", {}, [
        h("input", "", { type: "file", id: "cw-file-inp", style: { display: "none" }, multiple: "", onchange: function () { handleFiles(this.files); this.value = ""; } }),
        h("button", "cw-fbtn", { title: "Send file", onclick: function () { document.getElementById("cw-file-inp").click(); } }, ["\uD83D\uDCCE"]),
        h("input", "", { placeholder: "Type...", onkeydown: function (e) { if (e.key === "Enter") doSend(this.value, this); } }),
        h("button", "", { onclick: function () { var inp = this.parentNode.querySelector("input:not([type=file])"); doSend(inp.value, inp); } }, ["Send"]),
      ]),
    ]);

    var logoutBtn = h("button", "", { style: { display: "none" }, onclick: logout }, ["Exit"]);

    winEl = h("div", "cw-win", {
      style: (function () {
        var w = 360, hh = 520, s = 50, gap = 15;
        var l = ball.offsetLeft - w + s, t = ball.offsetTop - hh - gap;
        if (l < 0) l = ball.offsetLeft;
        if (t < 0) t = ball.offsetTop + s + gap;
        return { left: l + "px", top: t + "px", display: "none" };
      })(),
    }, [
      h("div", "cw-hdr", {}, [h("span", "", {}, ["Chat Room"]), logoutBtn]),
      loginBox,
      chatBox,
    ]);

    document.body.appendChild(winEl);

    winEl.querySelector(".cw-msgs").addEventListener("scroll", function () {
      if (this.scrollTop === 0) loadMore();
    });

    return winEl;
  }

  /* ── API ────────────────────────────────────────────────────────── */
  function api(path, opts) {
    opts = opts || {};
    var headers = {};
    if (opts.body && typeof opts.body === "object") {
      opts.body = JSON.stringify(opts.body);
      headers["Content-Type"] = "application/json";
    }
    return fetch(API + path, { method: opts.method || "GET", headers: headers, body: opts.body }).then(function (r) { return r.json(); });
  }

  function doLogin(name) {
    name = name.trim() || "User" + Math.random().toString(36).substring(2, 6);
    api("/api/login", { method: "POST", body: { username: name, password: "" } }).then(function (d) {
      if (d.error) return alert(d.error);
      token = d.token; username = d.username;
      try { localStorage.setItem("chat_xtoken", token); localStorage.setItem("chat_xuser", username); } catch (e) { }
      showChat();
    });
  }

  function logout() {
    token = ""; username = ""; lastId = 0; msgIds = {};
    try { localStorage.removeItem("chat_xtoken"); localStorage.removeItem("chat_xuser"); } catch (e) { }
    clearInterval(pollTimer);
    var w = buildWin();
    w.querySelector(".cw-msgs").innerHTML = "";
    w.querySelector(".cw-login").style.display = "flex";
    w.querySelector(".cw-login").nextElementSibling.style.display = "none";
    w.querySelector(".cw-hdr button").style.display = "none";
    w.querySelector(".cw-login input").value = "";
  }

  function showChat() {
    var w = buildWin();
    w.querySelector(".cw-login").style.display = "none";
    w.querySelector(".cw-login").nextElementSibling.style.display = "flex";
    w.querySelector(".cw-hdr button").style.display = "inline";
    loadHistory();
    startPoll();
  }

  function loadHistory() {
    api("/api/history/" + ROOM + "?token=" + token + "&limit=50").then(function (d) {
      if (d.messages) addMsgs(d.messages);
    });
  }

  function ts(d) { return d ? new Date(d).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" }) : ""; }

  function fmtSize(bytes) {
    if (!bytes) return "";
    if (bytes < 1024) return bytes + " B";
    if (bytes < 1048576) return (bytes / 1024).toFixed(1) + " KB";
    return (bytes / 1048576).toFixed(1) + " MB";
  }

  function addMsgs(arr, prepend) {
    var fresh = arr.filter(function (m) { return !msgIds[m.id]; });
    if (!fresh.length) return;
    fresh.forEach(function (m) { msgIds[m.id] = true; });
    fresh.sort(function (a, b) { return a.id - b.id; });
    var msgsEl = buildWin().querySelector(".cw-msgs");
    fresh.forEach(function (m) {
      if (m.msg_type === "system" || m.type === "system") {
        msgsEl[prepend ? "insertBefore" : "appendChild"](
          h("div", "cw-b cw-by", {}, [m.content]),
          prepend ? msgsEl.firstChild : null
        );
        return;
      }
      if (m.msg_type === "file") {
        var fd = null;
        try { fd = JSON.parse(m.content); } catch (e) {}
        var isSelf = m.username === username;
        var t = ts(m.created_at);
        if (fd && fd.url) {
          var isImg = fd.mime && fd.mime.indexOf("image/") === 0;
          var el = h("div", "cw-b " + (isSelf ? "cw-bs" : "cw-bo"), {}, [
            h("div", "", {},
              !isSelf ? [h("span", "cw-aut", { style: { color: m.avatar_color || "#aaa" } }, [(m.is_bot ? "[Bot] " : "") + (m.username || "")])] : []
            ),
            h("div", "", {},
              isImg
                ? [h("a", "", { href: fd.url, target: "_blank" }, [h("img", "cw-img", { src: fd.url, alt: fd.name })])]
                : [h("a", "cw-flnk", { href: fd.url, target: "_blank" }, ["\uD83D\uDCCE " + fd.name + " (" + fmtSize(fd.size) + ")"])],
            ),
            h("span", "cw-tm", { style: { textAlign: "right", display: "block" } }, [t]),
          ]);
          msgsEl[prepend ? "insertBefore" : "appendChild"](el, prepend ? msgsEl.firstChild : null);
          lastId = m.id;
        }
        return;
      }
      var isSelf = m.username === username;
      var t = ts(m.created_at);
      var el = h("div", "cw-b " + (isSelf ? "cw-bs" : "cw-bo"), {}, [
        h("div", "", {},
          !isSelf ? [h("span", "cw-aut", { style: { color: m.avatar_color || "#aaa" } }, [(m.is_bot ? "[Bot] " : "") + (m.username || "")])] : []
        ),
        h("div", "", {}, [
          m.content || "",
          h("span", "cw-tm", {}, [t]),
        ]),
      ]);
      msgsEl[prepend ? "insertBefore" : "appendChild"](el, prepend ? msgsEl.firstChild : null);
      lastId = m.id;
    });
    if (!prepend) msgsEl.scrollTop = msgsEl.scrollHeight;
  }

  function startPoll() {
    clearInterval(pollTimer);
    pollTimer = setInterval(function () {
      api("/api/poll?room_id=" + ROOM + "&token=" + token + "&after_id=" + lastId + "&timeout=3").then(function (d) {
        if (d.messages && d.messages.length) addMsgs(d.messages);
      });
    }, 2000);
  }

  function doSend(text, inp) {
    text = text.trim();
    if (!text) return;
    inp.value = "";
    api("/api/msg", { method: "POST", body: { token: token, room_id: ROOM, content: text } }).then(function (d) {
      if (d.id) addMsgs([d]);
    });
  }

  function sendPayload(payload) {
    return api("/api/msg", { method: "POST", body: Object.assign({ token: token, room_id: ROOM }, payload) }).then(function (d) {
      if (d.id) addMsgs([d]);
    });
  }

  function uploadFile(file) {
    return fetch("https://upload.moonchan.xyz/api/upload", { method: "PUT", body: file }).then(function (r) {
      if (!r.ok) throw new Error("Upload failed: " + r.statusText);
      return r.json();
    }).then(function (d) {
      return {
        url: "https://upload.moonchan.xyz/api/" + d.id + "/" + encodeURIComponent(file.name),
        name: file.name,
        size: file.size,
        mime: file.type,
      };
    });
  }

  function doSendFile(file) {
    var w = buildWin();
    var toast = h("div", "cw-upld", {}, ["Uploading..."]);
    w.appendChild(toast);
    return uploadFile(file).then(function (fd) {
      return sendPayload({ content: JSON.stringify(fd), msg_type: "file" });
    }).finally(function () {
      try { toast.remove(); } catch (e) {}
    });
  }

  function handleFiles(files) {
    if (!files || !files.length) return;
    for (var i = 0; i < files.length; i++) {
      doSendFile(files[i]).catch(function (e) { console.error("Upload failed:", e); });
    }
  }

  /* ── Drag ────────────────────────────────────────────────────────── */
  var dragging = false;
  var moved = false;
  var startPos = {};
  var initPos = {};
  var lastClick = 0;

  ball.addEventListener("mousedown", onStart);
  ball.addEventListener("touchstart", onStart);

  function onStart(e) {
    var cx = e.touches ? e.touches[0].clientX : e.clientX;
    var cy = e.touches ? e.touches[0].clientY : e.clientY;
    dragging = true; moved = false;
    startPos = { x: cx, y: cy };
    initPos = { x: ball.offsetLeft, y: ball.offsetTop };
  }

  function onMove(e) {
    if (!dragging) return;
    if (e.cancelable && e.type === "touchmove") e.preventDefault();
    var cx = e.touches ? e.touches[0].clientX : e.clientX;
    var cy = e.touches ? e.touches[0].clientY : e.clientY;
    var dx = cx - startPos.x, dy = cy - startPos.y;
    if (Math.abs(dx) > 3 || Math.abs(dy) > 3) moved = true;
    var s = 50;
    ball.style.left = Math.max(0, Math.min(initPos.x + dx, window.innerWidth - s)) + "px";
    ball.style.top = Math.max(0, Math.min(initPos.y + dy, window.innerHeight - s)) + "px";
  }

  function onEnd() {
    dragging = false;
    if (!moved) {
      var now = Date.now();
      if (now - lastClick < 500) return;
      lastClick = now;
      toggleWin();
    }
  }

  document.addEventListener("mousemove", onMove);
  document.addEventListener("mouseup", onEnd);
  document.addEventListener("touchmove", onMove, { passive: false });
  document.addEventListener("touchend", onEnd);

  /* ── Toggle ──────────────────────────────────────────────────────── */
  function toggleWin() {
    open = !open;
    var w = buildWin();
    w.style.display = open ? "flex" : "none";
    ball.innerHTML = open ? closeSvg : chatSvg;
    if (open && token && username) showChat();
  }

  ball.addEventListener("click", function (e) {
    if (moved) { moved = false; return; }
  });

  /* ── File paste / drop ─────────────────────────────────────────────── */
  var dropOverlay = null;
  var dragCounter = 0;

  document.addEventListener("paste", function (e) {
    if (!token) return;
    var items = e.clipboardData && e.clipboardData.items;
    if (!items) return;
    var files = [];
    for (var i = 0; i < items.length; i++) {
      if (items[i].kind === "file") files.push(items[i].getAsFile());
    }
    if (files.length) handleFiles(files);
  });

  function showDropOverlay(w) {
    if (!dropOverlay) dropOverlay = h("div", "cw-drop", {}, ["Drop files here"]);
    if (!dropOverlay.parentNode) w.appendChild(dropOverlay);
  }

  function hideDropOverlay() {
    if (dropOverlay && dropOverlay.parentNode) dropOverlay.parentNode.removeChild(dropOverlay);
  }

  function onWinDragOver(e) {
    e.preventDefault(); e.stopPropagation();
    var w = buildWin();
    showDropOverlay(w);
  }

  function onWinDrop(e) {
    e.preventDefault(); e.stopPropagation();
    hideDropOverlay();
    handleFiles(e.dataTransfer.files);
  }

  function bindFileDrop() {
    var w = buildWin();
    w.addEventListener("dragover", onWinDragOver);
    w.addEventListener("dragleave", function (e) { hideDropOverlay(); });
    w.addEventListener("drop", onWinDrop);
  }

  /* ── Init ────────────────────────────────────────────────────────── */
  document.body.appendChild(ball);
  bindFileDrop();
  if (token && username) {
    // pre-warm: build window hidden, ready to open
    buildWin();
    showChat();
  }
})();
