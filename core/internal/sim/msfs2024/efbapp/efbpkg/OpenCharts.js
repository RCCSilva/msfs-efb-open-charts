/*
  Open Charts - MSFS 2024 EFB app

  A deliberately tiny shell: it only embeds a page served by the local
  server (SERVER_URL) in an <iframe>. All map logic lives in that page, so it
  can be developed/tested in a normal browser and reloaded without
  restarting the simulator.

  NAMES THAT MUST STAY CONSISTENT (the sim breaks or silently ignores the app if not):
    - folder   html_ui/efb_ui/efb_apps/OpenCharts/
    - file     OpenCharts.js   (same as the folder name)
    - BASE_URL coui://html_ui/efb_ui/efb_apps/OpenCharts
    - INTERNAL_NAME 'OpenCharts'  (no whitespace - EFB rejects it)
  Do NOT use "TemplateApp": it collides with other EFB add-ons.

  Plain JS, no build step. Uses the globals the EFB provides:
    window.EFB_API   (EFB container, exposes use())
    window.msfssdk   (@microsoft/msfs-sdk: DisplayComponent, FSComponent, Subject)
*/
(function () {
  'use strict';

  var APP_NAME = 'Open Charts';
  var INTERNAL_NAME = 'OpenCharts';
  var VERSION = '0.5.0';
  var EFB_API_VERSION = '1.0.3';
  var BASE_URL = 'coui://html_ui/efb_ui/efb_apps/' + INTERNAL_NAME;

  // 127.0.0.1 on purpose (not "localhost"): avoids IPv6/::1 resolution, and the
  // server only listens on 127.0.0.1. Change the port here AND in the server (PORT).
  var SERVER_URL = 'http://127.0.0.1:43210/';

  // Page of the server shown in the frame: '' = base map (index.html),
  // 'charts.html' = aircraft over the georeferenced chart (charts/library).
  var PAGE_PATH = 'viewer.html';

  var HEALTH_POLL_MS = 2000;
  var STYLE_ID = 'lmm-style';

  // Tell the server what this app is doing (only the log of the program; nothing about the user):
  // the page used to be reloaded for no visible reason, and this shows why.
  function trace(msg) {
    try {
      var x = new XMLHttpRequest();
      x.open('POST', SERVER_URL + 'api/client-perf');
      x.setRequestHeader('Content-Type', 'application/json');
      x.send(JSON.stringify({ ua: navigator.userAgent, seconds: 0, mode: 'efb-app', counts: {}, note: 'efb: ' + msg }));
    } catch (e) { /* tracing must never break the app */ }
  }

  function getSdk() {
    return window.msfssdk || (typeof msfssdk !== 'undefined' ? msfssdk : null);
  }

  function injectStyle() {
    if (document.getElementById(STYLE_ID)) return;
    var css =
      '.lmm-root{position:absolute;left:0;top:0;right:0;bottom:0;width:100%;height:100%;' +
      'overflow:hidden;background:#1b2630;}' +
      '.lmm-frame-host{position:absolute;left:0;top:0;right:0;bottom:0;overflow:hidden;}' +
      '.lmm-frame{position:absolute;left:0;top:0;width:100%;height:100%;border:0;' +
      'background:#cfd8dc;}' +
      '.lmm-bar{display:none;position:absolute;left:0;right:0;bottom:0;height:24px;width:100%;' +
      'line-height:24px;padding:0 10px;box-sizing:border-box;font:12px/24px Arial,sans-serif;' +
      'color:#dfe7ee;background:#111a22;white-space:nowrap;overflow:hidden;}' +
      '.lmm-msg{position:absolute;left:50%;top:50%;width:70%;margin-left:-35%;margin-top:-50px;' +
      'padding:16px;box-sizing:border-box;text-align:center;font:16px/22px Arial,sans-serif;' +
      'color:#fff;background:rgba(160,30,30,.92);border-radius:8px;z-index:5;}' +
      '.lmm-hidden{display:none;}';
    var el = document.createElement('style');
    el.id = STYLE_ID;
    el.type = 'text/css';
    el.appendChild(document.createTextNode(css));
    document.head.appendChild(el);
  }

  function makeViewClass(sdk) {
    return class OpenChartsView extends sdk.DisplayComponent {
      constructor(props) {
        super(props);
        this.rootRef = sdk.FSComponent.createRef();
        this.frameHost = sdk.FSComponent.createRef();
        this.frame = null;
        this.frameLoaded = false;
        this.serverOk = null; // null = unknown yet
        this.simInfo = '';
        this.healthTimer = null;
        this.healthFails = 0;   // consecutive failed checks
        this.destroyed = false;
        this.textFocused = false;   // the page inside the frame has a text field in use
        this.onFrameMessage = this.handleFrameMessage.bind(this);
        this.onGameBlur = this.blurTextField.bind(this, true);
        this.onTextFromGame = this.handleTextFromGame.bind(this);
        this.barEl = null;
        this.msgEl = null;
        // The EFB may route gamepad/page-key events to the view; harmless no-ops.
        this.pageKeyActions = new Map();
      }

      render() {
        var h = sdk.FSComponent.buildComponent;
        return h('div', { class: 'lmm-root', ref: this.rootRef },
          h('div', { class: 'lmm-frame-host', ref: this.frameHost }),
          h('div', { class: 'lmm-msg lmm-hidden' }),
          h('div', { class: 'lmm-bar' }, APP_NAME + ' ' + VERSION)
        );
      }

      onAfterRender() {
        var root = this.rootRef.instance;
        if (!root) return;
        this.barEl = root.querySelector('.lmm-bar');
        this.msgEl = root.querySelector('.lmm-msg');
        trace('view rendered (frame exists: ' + !!this.frame + ')');
        window.addEventListener('message', this.onFrameMessage);
        this.createFrame('after-render');
        this.startHealthCheck();
      }

      // ---- keyboard: while a text field of the page has focus, tell the game, so the keys typed go
      // to the field and do not trigger commands of the simulator (gear, flaps, camera...). This is
      // the same FOCUS_INPUT_FIELD / UNFOCUS_INPUT_FIELD pair the EFB's own text boxes use. The page
      // runs in a frame from another origin, so it asks for it with postMessage.
      handleFrameMessage(ev) {
        var d = ev && ev.data;
        if (!d || d.oc !== 'text-input' || !this.frame || ev.source !== this.frame.contentWindow) return;
        if (d.state === 'focus') this.focusTextField(d.text);
        else this.blurTextField(false);
      }

      focusTextField(text) {
        var C = window.Coherent;
        if (this.textFocused || !C || typeof C.trigger !== 'function') return;
        this.textFocused = true;
        C.trigger('FOCUS_INPUT_FIELD', { title: APP_NAME, description: '', default_text: String(text || ''), is_numeric: false, is_efb: true });
        if (typeof C.on === 'function') {
          C.on('mousePressOutsideView', this.onGameBlur);
          C.on('SetInputTextFromOS', this.onTextFromGame);
        }
      }

      // fromGame: the game itself asked to leave the field (a click outside the view): the page's
      // field must lose focus too.
      blurTextField(fromGame) {
        var C = window.Coherent;
        if (!this.textFocused) return;
        this.textFocused = false;
        if (C && typeof C.trigger === 'function') C.trigger('UNFOCUS_INPUT_FIELD', 'open-charts-search');
        if (C && typeof C.off === 'function') {
          C.off('mousePressOutsideView', this.onGameBlur);
          C.off('SetInputTextFromOS', this.onTextFromGame);
        }
        if (fromGame) this.sendToFrame({ oc: 'text-blur' });
      }

      // Text entered with the game's own keyboard (VR, gamepad): hand it to the page.
      handleTextFromGame(text) {
        this.sendToFrame({ oc: 'text-value', text: String(text || '') });
        this.blurTextField(true);
      }

      sendToFrame(msg) {
        try {
          if (this.frame && this.frame.contentWindow) this.frame.contentWindow.postMessage(msg, SERVER_URL.slice(0, -1));
        } catch (e) { /* the frame is gone */ }
      }

      createFrame(reason) {
        var host = this.frameHost.instance;
        if (!host || this.destroyed) return;
        trace('page (re)loaded because: ' + (reason || 'unknown'));
        if (this.frame && this.frame.parentNode) this.frame.parentNode.removeChild(this.frame);

        var self = this;
        var f = document.createElement('iframe');
        f.className = 'lmm-frame';
        f.setAttribute('frameborder', '0');
        f.onload = function () {
          self.frameLoaded = true;
          self.updateBar();
        };
        this.frameLoaded = false;
        this.frame = f;
        host.appendChild(f);
        f.src = SERVER_URL + PAGE_PATH;
        this.updateBar();
      }

      startHealthCheck() {
        var self = this;
        this.stopHealthCheck();
        this.checkHealth();
        this.healthTimer = window.setInterval(function () { self.checkHealth(); }, HEALTH_POLL_MS);
      }

      stopHealthCheck() {
        if (this.healthTimer !== null) {
          window.clearInterval(this.healthTimer);
          this.healthTimer = null;
        }
      }

      checkHealth() {
        if (this.destroyed) return;
        var self = this;
        var xhr = new XMLHttpRequest();
        xhr.open('GET', SERVER_URL + 'api/health?t=' + Date.now());
        xhr.timeout = 4000;
        xhr.onreadystatechange = function () {
          if (xhr.readyState !== 4 || self.destroyed) return;
          var ok = false;
          var info = '';
          if (xhr.status === 200) {
            try {
              var j = JSON.parse(xhr.responseText);
              ok = !!j.ok;
              info = j.simulated ? 'simulated' : (j.simConnected ? 'sim connected' : 'sim not connected');
            } catch (e) { ok = false; }
          }
          // One slow or lost answer is not an outage (the simulator can stall its own scripts for a
          // moment): only three failures in a row count, and only then is the page reloaded when the
          // server answers again. Reloading the page at every hiccup made the map flash.
          if (ok) {
            self.healthFails = 0;
          } else {
            self.healthFails += 1;
            if (self.healthFails < 3) return;
          }
          var wasOk = self.serverOk;
          self.serverOk = ok;
          self.simInfo = info;
          if (ok && wasOk === false) self.createFrame('server back after an outage');
          self.updateBar();
        };
        try { xhr.send(); } catch (e) {
          this.serverOk = false;
          this.updateBar();
        }
      }

      updateBar() {
        if (this.barEl) {
          var server = this.serverOk === null ? 'checking...' : (this.serverOk ? 'OK (' + this.simInfo + ')' : 'UNREACHABLE');
          this.barEl.textContent = APP_NAME + ' ' + VERSION + '  |  server: ' + server +
            '  |  frame: ' + (this.frameLoaded ? 'loaded' : 'loading') + '  |  ' + SERVER_URL;
        }
        if (this.msgEl) {
          if (this.serverOk === false) {
            this.msgEl.textContent = 'Cannot reach Open Charts at ' + SERVER_URL +
              '. Start the Open Charts program (tray icon) and this page will load automatically.';
            this.msgEl.className = 'lmm-msg';
          } else {
            this.msgEl.className = 'lmm-msg lmm-hidden';
          }
        }
      }

      // EFB lifecycle (see AppView.d.ts): open/resume/pause/close/update.
      onOpen() { trace('onOpen'); this.startHealthCheck(); }
      onResume() { trace('onResume'); this.startHealthCheck(); }
      onPause() { trace('onPause'); this.blurTextField(false); this.stopHealthCheck(); }
      onClose() { trace('onClose'); this.blurTextField(false); this.stopHealthCheck(); }
      onUpdate() {}
      routeGamepadInteractionEvent() {}
      handlePageKeyAction() {}

      destroy() {
        trace('view destroyed');
        this.destroyed = true;
        this.blurTextField(false);
        window.removeEventListener('message', this.onFrameMessage);
        this.stopHealthCheck();
        if (this.frame) {
          this.frame.onload = null;
          this.frame.src = 'about:blank';
        }
        super.destroy();
      }
    };
  }

  function register() {
    var sdk = getSdk();
    var efb = window.EFB_API;
    if (!sdk || !efb || typeof efb.use !== 'function') return false;

    injectStyle();
    var View = makeViewClass(sdk);

    class OpenChartsApp {
      constructor() {
        this.available = sdk.Subject.create(true);
        this.BootMode = 0;     // AppBootMode.COLD
        this.SuspendMode = 0;  // AppSuspendMode.SLEEP
        this._isReady = false;
        this._favoriteIndex = -1;
        this.bus = null;
      }

      get name() { return APP_NAME; }
      get internalName() { return INTERNAL_NAME; }
      get icon() { return BASE_URL + '/Assets/app-icon.svg'; }
      get isReady() { return this._isReady; }
      get favoriteIndex() { return this._favoriteIndex; }
      set favoriteIndex(value) { this._favoriteIndex = value; }
      get compatibleAircraftModels() { return undefined; }
      get unitsSettingsManager() { return this._unitsSettingsManager; }
      getIsSearchable() { return true; }
      getIsFavoritable() { return true; }
      getVersion() { return VERSION; }

      async install() { /* nothing to load: everything comes from the server */ }

      async _install(props) {
        this.bus = props && props.bus;
        this._unitsSettingsManager = props && props.unitsSettingManager;
        this.notificationManager = props && props.notificationManager;
        this.efbSettingsManager = props && props.efbSettingsManager;
        this.options = props && props.options;
        await this.install(props);
        this._isReady = true;
        try {
          if (window.Coherent && typeof window.Coherent.trigger === 'function') {
            window.Coherent.trigger('EFB_APP_INSTALLED', APP_NAME, INTERNAL_NAME, EFB_API_VERSION, VERSION);
          }
        } catch (_) { /* informational only */ }
      }

      render() {
        return sdk.FSComponent.buildComponent(View, { bus: this.bus });
      }
    }

    // use() throws until the EFB has handed over its bus; the caller retries.
    efb.use(OpenChartsApp);
    return true;
  }

  // The EFB OS and this script load independently: retry until both are ready.
  var tries = 0;
  var retry = window.setInterval(function () {
    tries += 1;
    var done = false;
    try { done = register(); } catch (e) { done = false; }
    if (done || tries > 200) window.clearInterval(retry);
  }, 50);
})();
