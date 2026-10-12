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
  var VERSION = '0.1.0';
  var EFB_API_VERSION = '1.0.3';
  var BASE_URL = 'coui://html_ui/efb_ui/efb_apps/' + INTERNAL_NAME;

  // 127.0.0.1 on purpose (not "localhost"): avoids IPv6/::1 resolution, and the
  // server only listens on 127.0.0.1. Change the port here AND in the server (PORT).
  var SERVER_URL = 'http://127.0.0.1:8080/';

  // Page of the server shown in the frame: '' = base map (index.html),
  // 'charts.html' = aircraft over the georeferenced chart (charts/library).
  var PAGE_PATH = 'viewer.html';

  var HEALTH_POLL_MS = 2000;
  var STYLE_ID = 'lmm-style';

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
        this.destroyed = false;
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
        this.createFrame();
        this.startHealthCheck();
      }

      createFrame() {
        var host = this.frameHost.instance;
        if (!host || this.destroyed) return;
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
        xhr.timeout = 1500;
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
          var wasOk = self.serverOk;
          self.serverOk = ok;
          self.simInfo = info;
          // Server came up after the app was opened: (re)load the page.
          if (ok && wasOk === false) self.createFrame();
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
      onOpen() { this.startHealthCheck(); }
      onResume() { this.startHealthCheck(); }
      onPause() { this.stopHealthCheck(); }
      onClose() { this.stopHealthCheck(); }
      onUpdate() {}
      routeGamepadInteractionEvent() {}
      handlePageKeyAction() {}

      destroy() {
        this.destroyed = true;
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
