/* eslint-disable */
;(function () {
  if (window.Telegram && window.Telegram.WebApp && window.Telegram.WebApp.initData) return

  var cfg = window.__TG_MOCK_CONFIG__ || {}
  var initData = cfg.initData || ''
  var colorScheme = cfg.colorScheme || 'light'
  var version = cfg.version || '8.0'
  var platform = cfg.platform || 'tdesktop'
  var confirmAnswer = cfg.confirmAnswer !== undefined ? cfg.confirmAnswer : true

  var lightTheme = {
    bg_color: '#ffffff',
    text_color: '#000000',
    hint_color: '#707579',
    link_color: '#2B3EF0',
    button_color: '#2B3EF0',
    button_text_color: '#ffffff',
    secondary_bg_color: '#f1f1f4',
    section_bg_color: '#ffffff',
    section_header_text_color: '#707579',
    destructive_text_color: '#e5484d',
  }
  var darkTheme = {
    bg_color: '#17212b',
    text_color: '#f5f5f5',
    hint_color: '#7d8b99',
    link_color: '#6ab7ff',
    button_color: '#2B3EF0',
    button_text_color: '#ffffff',
    secondary_bg_color: '#232e3c',
    section_bg_color: '#17212b',
    section_header_text_color: '#7d8b99',
    destructive_text_color: '#ff595a',
  }
  var themeParams = cfg.themeParams || (colorScheme === 'dark' ? darkTheme : lightTheme)

  // Разбор initDataUnsafe из строки initData.
  var initDataUnsafe = {}
  try {
    var sp = new URLSearchParams(initData)
    var u = sp.get('user')
    if (u) initDataUnsafe.user = JSON.parse(u)
    var start = sp.get('start_param')
    if (start) initDataUnsafe.start_param = start
    if (sp.get('query_id')) initDataUnsafe.query_id = sp.get('query_id')
    if (sp.get('auth_date')) initDataUnsafe.auth_date = Number(sp.get('auth_date'))
  } catch (e) {}
  if (cfg.startParam && !initDataUnsafe.start_param) initDataUnsafe.start_param = cfg.startParam

  function toKebab(k) {
    return k.replace(/_/g, '-')
  }
  function applyThemeVars(tp, scheme) {
    var root = document.documentElement
    for (var k in tp) {
      if (Object.prototype.hasOwnProperty.call(tp, k)) {
        root.style.setProperty('--tg-theme-' + toKebab(k), tp[k])
      }
    }
    root.style.setProperty('--tg-viewport-height', window.innerHeight + 'px')
    root.style.setProperty('--tg-viewport-stable-height', window.innerHeight + 'px')
    var insets = ['top', 'right', 'bottom', 'left']
    for (var i = 0; i < insets.length; i++) {
      root.style.setProperty('--tg-safe-area-inset-' + insets[i], '0px')
      root.style.setProperty('--tg-content-safe-area-inset-' + insets[i], '0px')
    }
    root.dataset.theme = scheme
  }
  applyThemeVars(themeParams, colorScheme)

  var log = {
    alerts: [],
    confirms: [],
    haptics: [],
    closingConfirmation: false,
    setTheme: function (s) {
      colorScheme = s
      WebApp.colorScheme = s
      WebApp.themeParams = s === 'dark' ? darkTheme : lightTheme
      applyThemeVars(WebApp.themeParams, s)
      fire('themeChanged')
    },
  }
  window.__tgMock = log

  var listeners = {}
  function fire(name) {
    var ls = listeners[name] || []
    for (var i = 0; i < ls.length; i++) {
      try {
        ls[i]()
      } catch (e) {}
    }
  }

  var backBtnEl, mainBtnEl
  function buildChrome() {
    if (document.querySelector('[data-testid=tg-back-button]')) return
    var top = document.createElement('div')
    top.style.cssText =
      'position:fixed;top:0;left:0;right:0;height:44px;z-index:9998;display:flex;align-items:center;padding:0 12px;background:' +
      themeParams.secondary_bg_color
    backBtnEl = document.createElement('button')
    backBtnEl.setAttribute('data-testid', 'tg-back-button')
    backBtnEl.textContent = '‹ Назад'
    backBtnEl.style.cssText = 'display:none;border:0;background:transparent;color:' + themeParams.link_color + ';font-size:16px'
    top.appendChild(backBtnEl)
    document.body.appendChild(top)

    mainBtnEl = document.createElement('button')
    mainBtnEl.setAttribute('data-testid', 'tg-main-button')
    mainBtnEl.style.cssText =
      'position:fixed;bottom:0;left:0;right:0;height:56px;z-index:9998;border:0;display:none;font-size:16px;color:#fff;background:#2B3EF0'
    document.body.appendChild(mainBtnEl)

    document.body.style.paddingTop = '44px'
    document.body.style.paddingBottom = '56px'

    if (backHandler) backBtnEl.onclick = backHandler
    backBtnEl.style.display = BackButton.isVisible ? 'block' : 'none'
    if (mainHandler) mainBtnEl.onclick = mainHandler
    renderMain()
  }
  function ensureChrome() {
    if (document.body) buildChrome()
    else document.addEventListener('DOMContentLoaded', buildChrome)
  }

  var backHandler = null
  var BackButton = {
    isVisible: false,
    show: function () {
      this.isVisible = true
      if (backBtnEl) backBtnEl.style.display = 'block'
      return this
    },
    hide: function () {
      this.isVisible = false
      if (backBtnEl) backBtnEl.style.display = 'none'
      return this
    },
    onClick: function (cb) {
      backHandler = cb
      if (backBtnEl) backBtnEl.onclick = cb
      return this
    },
    offClick: function () {
      backHandler = null
      if (backBtnEl) backBtnEl.onclick = null
      return this
    },
  }

  var mainHandler = null
  var mainState = { text: '', color: '#2B3EF0', text_color: '#fff', is_active: true, is_visible: false, progress: false }
  function renderMain() {
    if (!mainBtnEl) return
    mainBtnEl.textContent = mainState.progress ? '…' : mainState.text
    mainBtnEl.style.display = mainState.is_visible ? 'block' : 'none'
    mainBtnEl.style.background = mainState.color
    mainBtnEl.style.color = mainState.text_color
    mainBtnEl.style.opacity = mainState.is_active ? '1' : '0.5'
    mainBtnEl.disabled = !mainState.is_active || mainState.progress
    mainBtnEl.setAttribute('data-progress', mainState.progress ? '1' : '0')
    mainBtnEl.setAttribute('data-active', mainState.is_active ? '1' : '0')
  }
  var MainButton = {
    text: '',
    color: '#2B3EF0',
    textColor: '#fff',
    isVisible: false,
    isActive: true,
    isProgressVisible: false,
    setText: function (t) {
      mainState.text = t
      this.text = t
      renderMain()
      return this
    },
    setParams: function (p) {
      if (p.text !== undefined) mainState.text = p.text
      if (p.color !== undefined) mainState.color = p.color
      if (p.text_color !== undefined) mainState.text_color = p.text_color
      if (p.is_active !== undefined) mainState.is_active = p.is_active
      if (p.is_visible !== undefined) mainState.is_visible = p.is_visible
      this.text = mainState.text
      this.isVisible = mainState.is_visible
      this.isActive = mainState.is_active
      renderMain()
      return this
    },
    show: function () {
      mainState.is_visible = true
      this.isVisible = true
      renderMain()
      return this
    },
    hide: function () {
      mainState.is_visible = false
      this.isVisible = false
      renderMain()
      return this
    },
    enable: function () {
      mainState.is_active = true
      renderMain()
      return this
    },
    disable: function () {
      mainState.is_active = false
      renderMain()
      return this
    },
    showProgress: function () {
      mainState.progress = true
      this.isProgressVisible = true
      renderMain()
      return this
    },
    hideProgress: function () {
      mainState.progress = false
      this.isProgressVisible = false
      renderMain()
      return this
    },
    onClick: function (cb) {
      mainHandler = cb
      if (mainBtnEl) mainBtnEl.onclick = cb
      return this
    },
    offClick: function () {
      mainHandler = null
      if (mainBtnEl) mainBtnEl.onclick = null
      return this
    },
  }

  var HapticFeedback = {
    impactOccurred: function (style) {
      log.haptics.push({ type: 'impact', style: style })
      return this
    },
    notificationOccurred: function (type) {
      log.haptics.push({ type: 'notification', notification: type })
      return this
    },
    selectionChanged: function () {
      log.haptics.push({ type: 'selection' })
      return this
    },
  }

  var WebApp = {
    initData: initData,
    initDataUnsafe: initDataUnsafe,
    version: version,
    platform: platform,
    colorScheme: colorScheme,
    themeParams: themeParams,
    isVersionAtLeast: function (v) {
      function parse(s) {
        return String(s).split('.').map(Number)
      }
      var a = parse(version)
      var b = parse(v)
      for (var i = 0; i < Math.max(a.length, b.length); i++) {
        var x = a[i] || 0
        var y = b[i] || 0
        if (x > y) return true
        if (x < y) return false
      }
      return true
    },
    ready: function () {},
    expand: function () {},
    close: function () {
      log.closed = true
    },
    MainButton: MainButton,
    BackButton: BackButton,
    HapticFeedback: HapticFeedback,
    showAlert: function (msg, cb) {
      log.alerts.push(msg)
      if (cb) cb()
    },
    showConfirm: function (msg, cb) {
      log.confirms.push(msg)
      if (cb) cb(confirmAnswer)
    },
    enableClosingConfirmation: function () {
      log.closingConfirmation = true
    },
    disableClosingConfirmation: function () {
      log.closingConfirmation = false
    },
    setHeaderColor: function () {},
    setBackgroundColor: function () {},
    disableVerticalSwipes: function () {},
    enableVerticalSwipes: function () {},
    onEvent: function (name, cb) {
      ;(listeners[name] = listeners[name] || []).push(cb)
    },
    offEvent: function (name, cb) {
      var ls = listeners[name] || []
      var i = ls.indexOf(cb)
      if (i >= 0) ls.splice(i, 1)
    },
    safeAreaInset: { top: 0, right: 0, bottom: 0, left: 0 },
    contentSafeAreaInset: { top: 0, right: 0, bottom: 0, left: 0 },
  }

  window.Telegram = { WebApp: WebApp }

  ensureChrome()
})()
