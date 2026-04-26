(function () {
  var API_HOST = new URL(document.currentScript.src).origin;
  var LOADER_URL = API_HOST + "/static/loader.js";
  var WIDGET_URL = API_HOST + "/static/widget.jsx";
  window.__CHAT_API_HOST__ = API_HOST;

  if (document.querySelector('script[src="' + LOADER_URL + '"]')) return;

  var loadScript = function (src) {
    return new Promise(function (resolve, reject) {
      var s = document.createElement("script");
      s.src = src;
      s.onload = resolve;
      s.onerror = reject;
      document.head.appendChild(s);
    });
  };

  loadScript("https://unpkg.com/react@18.3.1/umd/react.production.min.js")
    .then(function () {
      return loadScript("https://unpkg.com/react-dom@18.3.1/umd/react-dom.production.min.js");
    })
    .then(function () {
      return loadScript("https://unpkg.com/@babel/standalone@7.28.5/babel.min.js");
    })
    .then(function () {
      return fetch(WIDGET_URL);
    })
    .then(function (r) {
      if (!r.ok) throw new Error("widget fetch failed: " + r.status);
      return r.text();
    })
    .then(function (jsx) {
      var code = Babel.transform(jsx, { presets: ["env", "react"] }).code;
      var fn = new Function("React", "ReactDOM", code);
      fn(window.React, window.ReactDOM);
    })
    .catch(function (err) {
      console.error("[Chat Widget] Load error:", err);
    });
})();
