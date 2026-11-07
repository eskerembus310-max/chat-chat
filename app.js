'use strict';
(function () {
  /* ───────── утилиты ───────── */
  const $ = (s, r) => (r || document).querySelector(s);
  function h(tag, attrs, ...kids) {
    const e = document.createElement(tag);
    if (attrs) for (const k of Object.keys(attrs)) {
      const v = attrs[k]; if (v == null || v === false) continue;
      if (k === 'class') e.className = v;
      else if (k === 'text') e.textContent = v;
      else if (k.startsWith('on')) e.addEventListener(k.slice(2), v);
      else if (k === 'value') e.value = v;
      else e.setAttribute(k, v === true ? '' : v);
    }
    for (const kid of kids.flat(Infinity)) { if (kid == null || kid === false) continue; e.append(kid.nodeType ? kid : document.createTextNode(String(kid))); }
    return e;
  }
  const PALETTE = ['#0F7B8A', '#6C4BD1', '#B83280', '#2F7D32', '#C2570C', '#2563EB'];
  const hashColor = (s) => { let x = 0; for (const c of String(s)) x = (x * 31 + c.charCodeAt(0)) >>> 0; return PALETTE[x % PALETTE.length]; };
  const dayKey = (t) => { const d = new Date(t); return d.getFullYear() + '-' + d.getMonth() + '-' + d.getDate(); };
  const coarse = window.matchMedia && matchMedia('(pointer:coarse)').matches;
  function onlyEmoji(t) {
    const s = t.trim(); if (!s || s.length > 24) return false;
    if (!/^[\p{Extended_Pictographic}‍️\s]+$/u.test(s)) return false;
    return (s.match(/\p{Extended_Pictographic}/gu) || []).length <= 3;
  }
  function toast(msg) { const t = $('#toast'); if (!t) return; t.textContent = msg; t.hidden = false; clearTimeout(toast.t); toast.t = setTimeout(() => { t.hidden = true; }, 3800); }
  const NS = 'http://www.w3.org/2000/svg';
  function svg(d, w) {
    const s = document.createElementNS(NS, 'svg'); w = w || 20;
    s.setAttribute('width', w); s.setAttribute('height', w); s.setAttribute('viewBox', '0 0 20 20'); s.setAttribute('fill', 'none');
    s.setAttribute('stroke', 'currentColor'); s.setAttribute('stroke-width', '2'); s.setAttribute('stroke-linecap', 'round'); s.setAttribute('stroke-linejoin', 'round');
    const p = document.createElementNS(NS, 'path'); p.setAttribute('d', d); s.append(p); return s;
  }
  const ICON = { plus: 'M10 4v12M4 10h12', back: 'M12 4l-6 6 6 6', close: 'M4 4l12 12M16 4L4 16', send: 'M3 3l14 7-14 7 2-7z', smile: 'M10 18a8 8 0 1 0 0-16 8 8 0 0 0 0 16zM6.5 11.5c.8 1.6 2 2.4 3.5 2.4s2.7-.8 3.5-2.4M7.5 7.8v.4M12.5 7.8v.4', next: 'M8 4l6 6-6 6' };
  function logoSvg(w) {
    const d = document.createElement('div');
    d.innerHTML = '<svg width="' + w + '" height="' + w + '" viewBox="0 0 34 34" aria-hidden="true"><path d="M4 7a5 5 0 0 1 5-5h12a5 5 0 0 1 5 5v8a5 5 0 0 1-5 5h-8l-6 5v-5a5 5 0 0 1-3-4.6z" fill="var(--accent)"/><path d="M12 15a5 5 0 0 1 5-5h9a5 5 0 0 1 5 5v7a5 5 0 0 1-2 4v4.5l-5-3.5h-7a5 5 0 0 1-5-5z" fill="var(--ink)" stroke="var(--pane)" stroke-width="2"/></svg>';
    return d.firstChild;
  }
  async function copyText(text, input) {
    try { await navigator.clipboard.writeText(text); return true; } catch (_) {}
    if (input) { input.focus(); input.select(); try { return document.execCommand('copy'); } catch (_) {} }
    return false;
  }

  /* ───────── язык ───────── */
  const I = window.I18N;
  function guessLang() {
    const l = (navigator.language || 'ru').slice(0, 2).toLowerCase();
    return I.langs[l] ? l : (l === 'en' ? 'en' : 'ru');
  }
  const L = () => I.langs[S.set.lang] || I.langs.ru;
  function raw(key) { let v = L().d[key]; if (v == null) v = I.langs.ru.d[key]; return v; }
  function t(key, vars) {
    let v = raw(key); if (v == null) return key; if (Array.isArray(v)) v = v[0];
    return vars ? v.replace(/\{(\w+)\}/g, (m, k) => (vars[k] != null ? vars[k] : m)) : v;
  }
  function tn(key, n) {
    let v = raw(key); if (v == null) return key + ' ' + n;
    if (Array.isArray(v)) {
      const rule = L().rule; let i = 0;
      if (rule === 'slavic') { const a = n % 10, b = n % 100; i = b > 10 && b < 15 ? 2 : a === 1 ? 0 : a > 1 && a < 5 ? 1 : 2; }
      else if (rule === 'two') i = n === 1 ? 0 : 1;
      v = v[Math.min(i, v.length - 1)];
    }
    return v.replace('{n}', n);
  }
  const locale = () => L().locale || 'ru-RU';
  const timeStr = (ts) => new Date(ts).toLocaleTimeString(locale(), { hour: '2-digit', minute: '2-digit' });
  function dayLabel(ts) {
    const n = new Date(), d = new Date(ts);
    if (dayKey(ts) === dayKey(n)) return t('day.today');
    const y = new Date(n); y.setDate(n.getDate() - 1);
    if (dayKey(ts) === dayKey(y)) return t('day.yesterday');
    const o = { day: 'numeric', month: 'long' }; if (d.getFullYear() !== n.getFullYear()) o.year = 'numeric';
    return d.toLocaleDateString(locale(), o);
  }
  const listTime = (ts) => (dayKey(ts) === dayKey(Date.now()) ? timeStr(ts) : new Date(ts).toLocaleDateString(locale(), { day: 'numeric', month: 'short' }));
  function errMsg(e) {
    if (!e) return t('err.generic');
    const k = 'err.' + e.code; const v = raw(k);
    return v != null ? t(k) : (e.message || t('err.generic'));
  }

  /* ───────── настройки внешнего вида ───────── */
  const DEFAULTS = { theme: 'auto', accent: '', wallpaper: 'none', font: 'system', fontSize: 15, radius: 16, dim: 25, lang: '' };
  const DARK = { online: '#3DD598', danger: '#FF7A80', scrim: 'rgba(0,0,0,.6)', shade: 'rgba(0,0,0,.35)' };
  const LIGHT = { online: '#17935F', danger: '#B3202A', scrim: 'rgba(15,27,39,.45)', shade: 'rgba(15,27,39,.12)' };
  const THEMES = [
    { id: 'auto' },
    { id: 'classic', dark: false, v: { bg: '#D9E3EC', pane: '#F8FAFC', pane2: '#EBF1F6', chatbg: '#E4ECF3', ink: '#0F1B27', ink2: '#55697B', line: '#D2DDE7', in: '#FFFFFF', 'in-ink': '#0F1B27', accent: '#C93D08' } },
    { id: 'mint', dark: false, v: { bg: '#D3E7DD', pane: '#F5FBF8', pane2: '#E5F2EB', chatbg: '#DCEDE4', ink: '#10261C', ink2: '#4F6B5D', line: '#C8DFD3', in: '#FFFFFF', 'in-ink': '#10261C', accent: '#0E8A5F' } },
    { id: 'lilac', dark: false, v: { bg: '#E0D9EE', pane: '#FAF8FE', pane2: '#EEE9F7', chatbg: '#E8E2F4', ink: '#1E1633', ink2: '#645A80', line: '#DAD2EA', in: '#FFFFFF', 'in-ink': '#1E1633', accent: '#7B4CD6' } },
    { id: 'sand', dark: false, v: { bg: '#E6DCCB', pane: '#FBF8F2', pane2: '#F0E9DC', chatbg: '#EBE2D2', ink: '#2A2013', ink2: '#7A6A52', line: '#E0D5C0', in: '#FFFFFF', 'in-ink': '#2A2013', accent: '#B5651D' } },
    { id: 'night', dark: true, v: { bg: '#070D13', pane: '#101A24', pane2: '#172431', chatbg: '#0B141C', ink: '#E7EDF3', ink2: '#8DA0B2', line: '#213242', in: '#1A2938', 'in-ink': '#E7EDF3', accent: '#F0662A' } },
    { id: 'graphite', dark: true, v: { bg: '#0E0F11', pane: '#1B1D21', pane2: '#25282D', chatbg: '#141518', ink: '#ECEEF1', ink2: '#9AA0A8', line: '#2E3238', in: '#2A2D33', 'in-ink': '#ECEEF1', accent: '#4DA3FF' } },
    { id: 'ocean', dark: true, v: { bg: '#050E1A', pane: '#0B1B2E', pane2: '#12263D', chatbg: '#08162A', ink: '#E4F0FB', ink2: '#86A3BE', line: '#1B3552', in: '#14304F', 'in-ink': '#E4F0FB', accent: '#2DD4BF' } },
    { id: 'forest', dark: true, v: { bg: '#07110C', pane: '#0F1D16', pane2: '#16291F', chatbg: '#0A1610', ink: '#E5F2E9', ink2: '#8DAA98', line: '#1F3A2B', in: '#183427', 'in-ink': '#E5F2E9', accent: '#7BD88F' } },
    { id: 'dusk', dark: true, v: { bg: '#0D0816', pane: '#171028', pane2: '#211838', chatbg: '#120C22', ink: '#EEE8FA', ink2: '#9F93BD', line: '#2D2349', in: '#2A1F47', 'in-ink': '#EEE8FA', accent: '#C084FC' } },
    { id: 'amoled', dark: true, v: { bg: '#000000', pane: '#0A0A0A', pane2: '#151515', chatbg: '#000000', ink: '#F2F2F2', ink2: '#8E8E8E', line: '#222222', in: '#1A1A1A', 'in-ink': '#F2F2F2', accent: '#FF5C8A' } },
  ];
  const ACCENTS = ['#C93D08', '#E11D48', '#DB2777', '#9333EA', '#4F46E5', '#2563EB', '#0891B2', '#0E8A5F', '#65A30D', '#CA8A04', '#EA580C', '#475569'];
  const WALLS = [
    { id: 'none' },
    { id: 'dots', img: 'radial-gradient(color-mix(in srgb,var(--ink) 15%,transparent) 1.3px,transparent 1.7px)', size: '22px 22px' },
    { id: 'grid', img: 'linear-gradient(color-mix(in srgb,var(--ink) 8%,transparent) 1px,transparent 1px),linear-gradient(90deg,color-mix(in srgb,var(--ink) 8%,transparent) 1px,transparent 1px)', size: '26px 26px' },
    { id: 'stripes', img: 'repeating-linear-gradient(135deg,color-mix(in srgb,var(--ink) 6%,transparent) 0 10px,transparent 10px 20px)', size: 'auto' },
    { id: 'sunrise', img: 'linear-gradient(160deg,color-mix(in srgb,#ff7a3d 32%,var(--chatbg)),color-mix(in srgb,#ffd166 20%,var(--chatbg)) 55%,color-mix(in srgb,#ef476f 18%,var(--chatbg)))', size: 'auto' },
    { id: 'sea', img: 'linear-gradient(160deg,color-mix(in srgb,#2b8cff 32%,var(--chatbg)),color-mix(in srgb,#19d3c5 20%,var(--chatbg)))', size: 'auto' },
    { id: 'grove', img: 'linear-gradient(160deg,color-mix(in srgb,#3cb371 30%,var(--chatbg)),color-mix(in srgb,#b5e48c 18%,var(--chatbg)))', size: 'auto' },
    { id: 'aurora', img: 'linear-gradient(200deg,color-mix(in srgb,#7c3aed 34%,var(--chatbg)),color-mix(in srgb,#22d3ee 22%,var(--chatbg)) 50%,color-mix(in srgb,#4ade80 22%,var(--chatbg)))', size: 'auto' },
    { id: 'candy', img: 'linear-gradient(160deg,color-mix(in srgb,#ff7eb6 32%,var(--chatbg)),color-mix(in srgb,#7ec8ff 26%,var(--chatbg)))', size: 'auto' },
    { id: 'accent', img: 'linear-gradient(160deg,color-mix(in srgb,var(--accent) 30%,var(--chatbg)),color-mix(in srgb,var(--accent) 6%,var(--chatbg)))', size: 'auto' },
    { id: 'custom' },
  ];
  const FONTS = [
    { id: 'system', stack: 'system-ui,-apple-system,"Segoe UI",Roboto,"Helvetica Neue",Arial,sans-serif' },
    { id: 'inter', name: 'Inter', stack: '"CC Inter",system-ui,sans-serif' },
    { id: 'carlito', name: 'Carlito', stack: '"CC Carlito",Calibri,system-ui,sans-serif' },
    { id: 'serif', name: 'DejaVu Serif', stack: '"CC Serif",Georgia,"Times New Roman",serif' },
    { id: 'mono', name: 'DejaVu Mono', stack: '"CC Mono",Consolas,"Courier New",monospace' },
    { id: 'georgia', name: 'Georgia', stack: 'Georgia,"Times New Roman",serif' },
    { id: 'palatino', name: 'Palatino', stack: '"Palatino Linotype",Palatino,"Book Antiqua",serif' },
    { id: 'verdana', name: 'Verdana', stack: 'Verdana,Geneva,sans-serif' },
    { id: 'trebuchet', name: 'Trebuchet', stack: '"Trebuchet MS","Lucida Grande",sans-serif' },
    { id: 'rounded', name: 'Rounded', stack: '"Arial Rounded MT Bold","Varela Round",Nunito,system-ui,sans-serif' },
    { id: 'comic', name: 'Comic', stack: '"Comic Sans MS","Comic Neue","Chalkboard SE",cursive' },
    { id: 'hand', name: 'Print', stack: '"Segoe Print","Bradley Hand","Chalkduster",cursive' },
    { id: 'impact', name: 'Impact', stack: 'Impact,"Arial Narrow Bold",sans-serif' },
  ];
  const SYNC_KEYS = ['theme', 'accent', 'wallpaper', 'font', 'fontSize', 'radius', 'dim', 'lang'];
  function loadSettings() { try { return Object.assign({}, DEFAULTS, JSON.parse(localStorage.getItem('cc_settings') || '{}')); } catch (_) { return Object.assign({}, DEFAULTS); } }
  function lum(hex) { const n = parseInt(hex.slice(1), 16), c = [(n >> 16) & 255, (n >> 8) & 255, n & 255].map((v) => { v /= 255; return v <= 0.03928 ? v / 12.92 : Math.pow((v + 0.055) / 1.055, 2.4); }); return 0.2126 * c[0] + 0.7152 * c[1] + 0.0722 * c[2]; }
  function customWall() { try { return localStorage.getItem('cc_wall') || ''; } catch (_) { return ''; } }
  function wallCss(id) {
    if (id === 'custom') {
      const u = customWall(); if (!u) return null; const d = (S.set.dim || 0) / 100;
      return { img: 'linear-gradient(rgba(0,0,0,' + d + '),rgba(0,0,0,' + d + ')),url("' + u + '")', size: 'auto,cover' };
    }
    const w = WALLS.find((x) => x.id === id); return w && w.img ? w : null;
  }
  function applySettings() {
    const root = document.documentElement, st = root.style;
    const th = THEMES.find((x) => x.id === S.set.theme) || THEMES[0];
    ['bg', 'pane', 'pane2', 'chatbg', 'ink', 'ink2', 'line', 'in', 'in-ink', 'online', 'danger', 'scrim', 'shade'].forEach((k) => st.removeProperty('--' + k));
    st.removeProperty('color-scheme');
    if (th.v) {
      Object.keys(th.v).forEach((k) => { if (k !== 'accent') st.setProperty('--' + k, th.v[k]); });
      const x = th.dark ? DARK : LIGHT; Object.keys(x).forEach((k) => st.setProperty('--' + k, x[k]));
      st.setProperty('color-scheme', th.dark ? 'dark' : 'light');
    }
    const ac = S.set.accent || (th.v && th.v.accent) || '';
    if (/^#[0-9a-f]{6}$/i.test(ac)) { st.setProperty('--accent', ac); st.setProperty('--accent-ink', lum(ac) > 0.5 ? '#10151B' : '#FFFFFF'); }
    else { st.removeProperty('--accent'); st.removeProperty('--accent-ink'); }
    const w = wallCss(S.set.wallpaper);
    if (w) { st.setProperty('--wall-image', w.img); st.setProperty('--wall-size', w.size); } else { st.removeProperty('--wall-image'); st.removeProperty('--wall-size'); }
    const f = FONTS.find((x) => x.id === S.set.font) || FONTS[0];
    st.setProperty('--font', f.stack); st.setProperty('--fs', S.set.fontSize + 'px'); st.setProperty('--rad', S.set.radius + 'px');
    root.lang = S.set.lang === 'en' ? 'en' : (S.set.lang === 'be' ? 'be' : (S.set.lang === 'uk' ? 'uk' : 'ru'));
    const m = $('meta[name="theme-color"]'); if (m) m.setAttribute('content', ac || '#C93D08');
  }
  let saveT = null;
  function saveSettings() {
    try { localStorage.setItem('cc_settings', JSON.stringify(S.set)); } catch (_) {}
    if (!S.token) return;
    clearTimeout(saveT);
    saveT = setTimeout(() => {
      const o = {}; SYNC_KEYS.forEach((k) => { o[k] = S.set[k]; });
      if (o.wallpaper === 'custom') o.wallpaper = 'none';
      api('PUT', '/api/me/settings', { settings: o }).catch(() => {});
    }, 700);
  }

  /* ───────── состояние ───────── */
  const S = {
    token: null, me: null, chats: new Map(), users: new Map(), emojis: new Map(), msgs: new Map(), more: new Map(), contacts: [], cByUser: new Map(),
    active: null, filter: '', ws: null, wsOk: false, typing: {}, emojiOpen: false, emojiTab: 'smileys', emojiMode: 'text',
    selMsg: null, installEvt: null, config: { email: true, phone: false }, set: loadSettings(),
  };
  try { S.hadLocal = !!localStorage.getItem('cc_settings'); } catch (_) {}
  if (!S.set.lang || !I.langs[S.set.lang]) S.set.lang = guessLang();
  try { S.token = localStorage.getItem('cc_token'); } catch (_) {}
  applySettings();

  async function api(method, path, body) {
    let r;
    try {
      r = await fetch(path, { method, headers: Object.assign({ 'Content-Type': 'application/json' }, S.token ? { Authorization: 'Bearer ' + S.token } : {}), body: body ? JSON.stringify(body) : undefined });
    } catch (_) { throw { code: 'network', message: t('err.network') }; }
    const j = await r.json().catch(() => ({}));
    if (!r.ok) {
      if (r.status === 401 && S.token) logout(true);
      throw { code: j.error || 'error', message: j.message || t('err.generic'), status: r.status };
    }
    return j;
  }

  /* ───────── данные ───────── */
  const user = (id) => S.users.get(id);
  const dn = (id) => { const c = S.cByUser.get(id); if (c) return c.name; const u = user(id); return (u && u.name) || t('msg.someone'); };
  const emojiByName = () => { const m = new Map(); for (const e of S.emojis.values()) m.set(e.name, e); return m; };
  const peerOf = (c) => c.members.find((id) => id !== S.me.id);
  function titleOf(c) { if (c.type === 'dm') { const p = peerOf(c); return p ? dn(p) : t('msg.someone'); } return c.title || '…'; }
  function putUsers(us) { if (us) for (const k of Object.keys(us)) S.users.set(Number(k), us[k]); }
  const lastTime = (c) => (c.last && c.last.createdAt) || c.createdAt || 0;
  const sortedChats = () => [...S.chats.values()].sort((a, b) => lastTime(b) - lastTime(a));
  function lastSeenText(u) {
    if (!u) return '';
    if (u.online) return t('p.online');
    if (!u.lastSeen) return t('p.seenLong');
    return dayKey(u.lastSeen) === dayKey(Date.now()) ? t('p.seenAt', { time: timeStr(u.lastSeen) }) : t('p.seenOn', { date: new Date(u.lastSeen).toLocaleDateString(locale(), { day: 'numeric', month: 'short' }) });
  }
  function previewOf(c) {
    const m = c.last; if (!m) return c.type === 'group' ? t('msg.groupCreated') : t('msg.firstMsg');
    const body = m.kind === 'sticker' ? t('msg.sticker') : m.text;
    const who = m.userId === S.me.id ? t('msg.youColon') : (c.type === 'group' ? dn(m.userId) + ': ' : '');
    return who + body;
  }
  async function loadContacts() {
    try { const r = await api('GET', '/api/contacts'); setContacts(r.contacts); } catch (_) {}
  }
  function setContacts(list) {
    S.contacts = list; S.cByUser = new Map();
    list.forEach((c) => { if (c.userId) { S.cByUser.set(c.userId, c); if (c.user) S.users.set(c.userId, c.user); } });
  }

  /* ───────── аватарки ───────── */
  function avatar(u, cls, nameOverride) {
    const a = h('span', { class: 'av' + (cls ? ' ' + cls : '') }); a.style.setProperty('--c', hashColor((u && u.id) || nameOverride || '0'));
    if (u && u.avatar) a.append(h('img', { class: 'ph', alt: '', src: u.avatar })); else a.append(((nameOverride || (u && u.name) || '?').trim().charAt(0).toUpperCase()) || '?');
    if (u && u.online && !cls) a.append(h('i', { class: 'dot' }));
    return a;
  }
  const chatAvatar = (c, cls) => (c.type === 'dm' ? avatar(user(peerOf(c)), cls, titleOf(c)) : avatar({ id: 'g' + c.id, name: c.title }, cls));

  /* ───────── картинки: уменьшаем перед загрузкой ───────── */
  function fileToDataUrl(file, max, kind) {
    return new Promise((resolve, reject) => {
      if (!file || !/^image\//.test(file.type)) return reject({ message: t('file.notImage') });
      const url = URL.createObjectURL(file), img = new Image();
      img.onerror = () => { URL.revokeObjectURL(url); reject({ message: t('file.openFail') }); };
      img.onload = () => {
        URL.revokeObjectURL(url); let size = max;
        for (let i = 0; i < 4; i++) {
          const cv = document.createElement('canvas'), ctx = cv.getContext('2d');
          if (kind === 'avatar') {
            cv.width = cv.height = size; const s = Math.min(img.width, img.height);
            ctx.drawImage(img, (img.width - s) / 2, (img.height - s) / 2, s, s, 0, 0, size, size);
          } else {
            const k = Math.min(1, size / Math.max(img.width, img.height));
            cv.width = Math.max(1, Math.round(img.width * k)); cv.height = Math.max(1, Math.round(img.height * k));
            ctx.drawImage(img, 0, 0, cv.width, cv.height);
          }
          const out = kind === 'emoji' ? cv.toDataURL('image/png') : cv.toDataURL('image/jpeg', kind === 'wall' ? 0.8 : 0.88);
          if (out.length * 0.75 < (kind === 'wall' ? 900 : 550) * 1024) return resolve(out);
          size = Math.round(size * 0.7);
        }
        reject({ message: t('file.tooComplex') });
      };
      img.src = url;
    });
  }
  function pickFile(cb) {
    const inp = h('input', { type: 'file', accept: 'image/*', hidden: true });
    inp.addEventListener('change', () => { const f = inp.files && inp.files[0]; inp.remove(); if (f) cb(f); });
    document.body.append(inp); inp.click();
  }

  /* ───────── окна ───────── */
  function openSheet(title, bodyEls, opts) {
    const sh = $('#sheet'), keep = sh.hidden ? 0 : (($('.card-body', sh) || {}).scrollTop || 0);
    sh.textContent = ''; sh.hidden = false;
    const body = h('div', { class: 'card-body' }, bodyEls);
    sh.append(h('div', { class: 'card', role: 'dialog', 'aria-label': title },
      h('div', { class: 'card-top' }, h('span', { text: title }),
        (opts && opts.noClose) ? null : h('button', { class: 'iconbtn', type: 'button', 'aria-label': t('common.close'), onclick: closeSheet }, svg(ICON.close, 18))),
      body));
    if (opts && opts.keep) body.scrollTop = keep;
    sh.onclick = (e) => { if (e.target === sh && !(opts && opts.noClose)) closeSheet(); };
    S.sheetOpen = true;
  }
  function closeSheet() { const sh = $('#sheet'); sh.hidden = true; sh.textContent = ''; S.sheetOpen = false; S.sheetLocked = false; }
  document.addEventListener('keydown', (e) => {
    if (e.key !== 'Escape') return;
    if (S.sheetOpen && !S.sheetLocked) closeSheet(); else if (S.emojiOpen) { S.emojiOpen = false; renderEmoji(); }
  });
  const field = (id, label, input, hint) => h('div', { class: 'field' }, h('label', { for: id, text: label }), input, hint ? h('span', { class: 'hint', text: hint }) : null);

  /* ───────── вход ───────── */
  const A = { kind: 'email', step: 'target', target: '', devCode: '', busy: false, err: '' };
  function langButton() {
    return h('button', { class: 'link', type: 'button', onclick: () => langSheet() }, '🌐 ' + L().name);
  }
  function renderAuth() {
    const root = $('#root'); root.textContent = '';
    const form = h('form', { onsubmit: (e) => { e.preventDefault(); A.step === 'target' ? requestCode() : verifyCode(); } });
    if (A.step === 'target') {
      form.append(
        h('div', { class: 'seg' },
          h('button', { type: 'button', class: A.kind === 'email' ? 'on' : '', disabled: !S.config.email, text: t('auth.email'), onclick: () => { A.kind = 'email'; A.target = ''; A.err = ''; renderAuth(); } }),
          h('button', { type: 'button', class: A.kind === 'phone' ? 'on' : '', disabled: !S.config.phone, text: t('auth.phone'), onclick: () => { A.kind = 'phone'; A.target = ''; A.err = ''; renderAuth(); } })),
        field('target', A.kind === 'email' ? t('auth.emailLabel') : t('auth.phoneLabel'),
          h('input', { id: 'target', name: 'target', type: A.kind === 'email' ? 'email' : 'tel', autocomplete: A.kind === 'email' ? 'email' : 'tel', inputmode: A.kind === 'email' ? 'email' : 'tel', placeholder: A.kind === 'email' ? 'name@example.com' : '+375 29 123 45 67', value: A.target, required: true, oninput: (e) => { A.target = e.target.value; } }),
          A.kind === 'email' ? t('auth.emailHint') : t('auth.phoneHint')));
      if (!S.config.email && !S.config.phone) form.append(h('p', { class: 'err', text: t('auth.notConfigured') }));
    } else {
      form.append(
        h('p', { class: 'sub', text: t('auth.codeSent', { target: A.target }) }),
        A.devCode ? h('div', { class: 'devbox' }, t('auth.dev') + ' ', h('b', { text: A.devCode })) : null,
        field('code', t('auth.codeLabel'), h('input', { id: 'code', name: 'code', class: 'code-in', type: 'text', inputmode: 'numeric', autocomplete: 'one-time-code', maxlength: '6', pattern: '[0-9]{6}', required: true })));
    }
    if (A.err) form.append(h('p', { class: 'err', role: 'alert', text: A.err }));
    form.append(h('button', { class: 'btn', type: 'submit', disabled: A.busy || (!S.config.email && !S.config.phone), text: A.busy ? t('auth.wait') : (A.step === 'target' ? t('auth.getCode') : t('auth.login')) }));
    if (A.step === 'code') form.append(
      h('button', { class: 'link', type: 'button', text: t('auth.resend'), onclick: requestCode }),
      h('button', { class: 'link', type: 'button', text: t('auth.change'), onclick: () => { A.step = 'target'; A.err = ''; A.devCode = ''; renderAuth(); } }));
    root.append(h('div', { class: 'auth' }, h('div', { class: 'auth-card' },
      h('div', { class: 'logo' }, logoSvg(38), 'Чат-Чат'),
      h('p', { class: 'sub', text: t('auth.tagline') }), form, h('div', { style: 'text-align:center;margin-top:8px' }, langButton()))));
    const f = $(A.step === 'target' ? '#target' : '#code'); if (f && !coarse) f.focus();
  }
  async function requestCode() {
    A.busy = true; A.err = ''; renderAuth();
    try { const r = await api('POST', '/api/auth/request', { kind: A.kind, target: A.target }); A.step = 'code'; A.devCode = r.devCode || ''; }
    catch (e) { if (e.code === 'wait') { A.step = 'code'; } A.err = errMsg(e); }
    A.busy = false; renderAuth();
  }
  async function verifyCode() {
    const code = $('#code').value.trim(); A.busy = true; A.err = ''; $('.btn').disabled = true;
    try {
      const r = await api('POST', '/api/auth/verify', { kind: A.kind, target: A.target, code });
      S.token = r.token; try { localStorage.setItem('cc_token', r.token); } catch (_) {}
      S.me = r.user; A.busy = false; A.step = 'target'; A.devCode = '';
      adoptServerSettings(r.user.settings);
      await startApp(r.isNew);
    } catch (e) { A.err = errMsg(e); A.busy = false; renderAuth(); }
  }
  function adoptServerSettings(st) {
    if (S.hadLocal) { saveSettings(); return; }
    if (st && Object.keys(st).length) {
      Object.keys(st).forEach((k) => { if (SYNC_KEYS.includes(k) && !(k === 'wallpaper' && st[k] === 'none' && S.set.wallpaper === 'custom' && customWall())) S.set[k] = st[k]; });
      if (!I.langs[S.set.lang]) S.set.lang = guessLang();
      try { localStorage.setItem('cc_settings', JSON.stringify(S.set)); } catch (_) {}
      applySettings();
    } else saveSettings();
  }
  function logout(silent) {
    if (!silent && S.token) api('POST', '/api/auth/logout', {}).catch(() => {});
    S.token = null; S.me = null; try { localStorage.removeItem('cc_token'); } catch (_) {}
    if (S.ws) { try { S.ws.onclose = null; S.ws.close(); } catch (_) {} S.ws = null; }
    S.chats.clear(); S.msgs.clear(); S.contacts = []; S.cByUser = new Map(); S.active = null; closeSheet(); A.step = 'target'; A.err = ''; renderAuth();
  }

  /* ───────── настройки (главное меню) ───────── */
  function menuItem(icon, title, sub, onclick) {
    return h('button', { class: 'mitem', type: 'button', onclick }, h('span', { class: 'mi', text: icon }), h('div', { style: 'min-width:0' }, h('div', { class: 'mt', text: title }), sub ? h('div', { class: 'ms', text: sub }) : null), h('span', { class: 'go' }, svg(ICON.next, 16)));
  }
  function settingsSheet() {
    const me = S.me;
    const items = [
      menuItem('👤', t('settings.profile'), t('settings.profileSub'), () => profileSheet()),
      menuItem('📒', t('settings.contacts'), t('settings.contactsSub'), () => contactsSheet()),
      menuItem('🎨', t('settings.appearance'), t('settings.appearanceSub'), () => appearanceSheet()),
      menuItem('🌐', t('settings.language'), L().name, () => langSheet()),
      menuItem('😀', t('settings.emojis'), t('settings.emojisSub'), () => emojiManageSheet()),
    ];
    if ('Notification' in window && Notification.permission === 'default') items.push(menuItem('🔔', t('settings.notifications'), '', async () => { const p = await Notification.requestPermission(); toast(p === 'granted' ? t('settings.notifOn') : t('settings.notifOff')); settingsSheet(); }));
    if (S.installEvt) items.push(menuItem('📲', t('settings.install'), '', async () => { S.installEvt.prompt(); try { await S.installEvt.userChoice; } catch (_) {} S.installEvt = null; settingsSheet(); }));
    openSheet(t('settings.title'), [
      h('div', { class: 'mini-me' }, avatar(me, 'lg'), h('div', { style: 'min-width:0' }, h('div', { class: 'pname', text: me.name }), h('div', { class: 'hint', text: me.username ? '@' + me.username : (me.email || me.phone || '') }))),
      h('div', { class: 'menu' }, items), h('div', { class: 'sep' }),
      h('button', { class: 'btn danger', type: 'button', text: t('settings.logout'), onclick: () => logout() })]);
  }

  /* ───────── профиль ───────── */
  function profileSheet(setup) {
    S.sheetLocked = !!setup;
    const st = { name: S.me.name, username: S.me.username || '', bio: S.me.bio || '' }; let err = '';
    const draw = () => {
      const avBox = h('div', { class: 'avwrap' }, avatar(S.me, 'lg'),
        h('div', { class: 'row2' },
          h('button', { class: 'link', type: 'button', text: S.me.avatar ? t('profile.changePhoto') : t('profile.addPhoto'), onclick: () => pickFile(async (f) => {
            try { const d = await fileToDataUrl(f, 320, 'avatar'); const r = await api('PUT', '/api/me/avatar', { data: d }); S.me = r.user; draw(); renderList(); renderMe(); } catch (e) { toast(errMsg(e) || t('profile.photoFail')); }
          }) }),
          S.me.avatar ? h('button', { class: 'link', type: 'button', text: t('profile.removePhoto'), onclick: async () => { try { const r = await api('DELETE', '/api/me/avatar'); S.me = r.user; draw(); renderList(); renderMe(); } catch (e) { toast(errMsg(e)); } } }) : null));
      const form = h('form', { onsubmit: async (e) => {
        e.preventDefault(); err = '';
        try {
          const r = await api('PATCH', '/api/me', { name: st.name, username: st.username, bio: st.bio });
          S.me = r.user; S.users.set(S.me.id, S.me); S.sheetLocked = false; closeSheet(); renderMe();
          if (setup) await startApp(false, true); else { renderList(); renderHead(); toast(t('profile.saved')); }
        } catch (e2) { err = errMsg(e2); draw(); }
      } },
        field('pname', t('profile.name'), h('input', { id: 'pname', value: st.name, maxlength: '50', required: true, autocomplete: 'name', oninput: (e) => { st.name = e.target.value; } })),
        field('puser', t('profile.username'), h('input', { id: 'puser', value: st.username, maxlength: '33', placeholder: '@anna_k', autocapitalize: 'none', autocomplete: 'off', oninput: (e) => { st.username = e.target.value; } }), t('profile.usernameHint')),
        field('pbio', t('profile.bio'), h('textarea', { id: 'pbio', maxlength: '200', oninput: (e) => { st.bio = e.target.value; } }, st.bio)),
        h('div', { class: 'field' }, h('label', { text: t('profile.via') }), h('div', { text: S.me.email || S.me.phone || '' })),
        err ? h('p', { class: 'err', role: 'alert', text: err }) : null,
        h('button', { class: 'btn', type: 'submit', text: setup ? t('profile.done') : t('profile.save') }));
      openSheet(setup ? t('profile.setup') : t('settings.profile'), [avBox, form], { noClose: !!setup });
    };
    draw();
  }

  /* ───────── внешний вид ───────── */
  function appearanceSheet() {
    const draw = () => {
      const set = (k, v) => { S.set[k] = v; applySettings(); saveSettings(); draw(); };
      const themeTile = (th) => {
        const v = th.v || THEMES[1].v, sw = th.id === 'auto' ? [THEMES[1].v, THEMES[5].v] : null;
        const inner = h('span', { class: 'sw' });
        if (sw) { inner.style.display = 'block'; inner.style.background = 'linear-gradient(135deg,' + sw[0].pane + ' 50%,' + sw[1].pane + ' 50%)'; }
        else [v.pane, v.chatbg, v.in, v.accent, v.ink, v.pane2].forEach((c) => inner.append(h('i', { style: 'background:' + c })));
        return h('button', { class: 'tile' + (S.set.theme === th.id ? ' on' : ''), type: 'button', onclick: () => set('theme', th.id) }, inner, h('span', { class: 'lab', text: t('theme.' + th.id) }));
      };
      const wallTile = (w) => {
        const inner = h('span', { style: 'position:absolute;inset:0;background-color:var(--chatbg)' });
        const css = wallCss(w.id); if (css) { inner.style.backgroundImage = css.img; inner.style.backgroundSize = css.size; inner.style.backgroundPosition = 'center'; }
        return h('button', { class: 'tile' + (S.set.wallpaper === w.id ? ' on' : ''), type: 'button', onclick: () => {
          if (w.id === 'custom') pickFile(async (f) => { try { const d = await fileToDataUrl(f, 1280, 'wall'); try { localStorage.setItem('cc_wall', d); } catch (_) { toast(t('err.generic')); return; } set('wallpaper', 'custom'); } catch (e) { toast(errMsg(e)); } });
          else set('wallpaper', w.id);
        } }, inner, h('span', { class: 'lab', text: t('wall.' + w.id) }));
      };
      const slider = (id, label, k, min, max, step) => {
        const out = h('output', { text: String(S.set[k]) });
        return h('div', { class: 'slide' }, h('label', { for: id, text: label }),
          h('input', { id, type: 'range', min, max, step: step || 1, value: S.set[k], oninput: (e) => { S.set[k] = Number(e.target.value); out.textContent = e.target.value; applySettings(); }, onchange: () => { saveSettings(); draw(); } }), out);
      };
      const prevBox = h('div', { class: 'preview' },
        h('div', { class: 'row first' }, h('div', { class: 'bub tail' }, h('span', { class: 'txt', text: t('look.prevIn') }), h('span', { class: 't', text: '12:30' }))),
        h('div', { class: 'row mine' }, h('div', { class: 'bub tail' }, h('span', { class: 'txt', text: t('look.prevOut') }), h('span', { class: 't', text: '12:31' }))));
      const accents = h('div', { class: 'swatches' }, ACCENTS.map((c) => h('button', { class: 'swc' + (S.set.accent.toLowerCase() === c.toLowerCase() ? ' on' : ''), type: 'button', 'aria-label': c, style: 'background:' + c, onclick: () => set('accent', c) })),
        h('span', { class: 'swc custom', title: t('look.custom') }, h('input', { type: 'color', 'aria-label': t('look.custom'), value: /^#[0-9a-f]{6}$/i.test(S.set.accent) ? S.set.accent : '#C93D08', onchange: (e) => set('accent', e.target.value) })),
        S.set.accent ? h('button', { class: 'link', type: 'button', text: t('look.accentAuto'), onclick: () => set('accent', '') }) : null);
      openSheet(t('settings.appearance'), [
        prevBox,
        h('div', { class: 'sect', text: t('look.theme') }), h('div', { class: 'grid' }, THEMES.map(themeTile)),
        h('div', { class: 'sect', text: t('look.accent') }), accents,
        h('div', { class: 'sect', text: t('look.wallpaper') }), h('div', { class: 'grid' }, WALLS.map(wallTile)),
        S.set.wallpaper === 'custom' ? slider('dim', t('look.dim'), 'dim', 0, 80, 5) : null,
        h('div', { class: 'sect', text: t('look.font') }),
        FONTS.map((f) => h('button', { class: 'fontrow' + (S.set.font === f.id ? ' on' : ''), type: 'button', style: 'font-family:' + f.stack, onclick: () => set('font', f.id) }, h('span', { text: f.id === 'system' ? t('font.system') : f.name }), h('span', { class: 'ck', text: S.set.font === f.id ? '✓' : '' }))),
        h('p', { class: 'hint', style: 'font-size:12.5px;color:var(--ink2);margin:8px 0 0', text: t('look.fontNote') }),
        h('div', { class: 'sect', text: t('look.size') }), slider('fs', t('look.size'), 'fontSize', 12, 22),
        slider('rad', t('look.radius'), 'radius', 0, 28),
        h('div', { class: 'sep' }),
        h('button', { class: 'btn ghost', type: 'button', text: t('look.reset'), onclick: () => { const lang = S.set.lang; S.set = Object.assign({}, DEFAULTS, { lang }); try { localStorage.removeItem('cc_wall'); } catch (_) {} applySettings(); saveSettings(); draw(); } }),
      ], { keep: true });
    };
    draw();
  }

  /* ───────── язык ───────── */
  function langSheet() {
    const draw = () => {
      openSheet(t('settings.language'), [
        h('p', { class: 'hint', style: 'font-size:13px;color:var(--ink2);margin:0 0 8px', text: t('lang.hint') }),
        I.order.map((id) => { const l = I.langs[id]; return h('button', { class: 'langrow' + (S.set.lang === id ? ' on' : ''), type: 'button', onclick: () => { S.set.lang = id; applySettings(); saveSettings(); refreshUI(); draw(); } },
          h('span', { class: 'fl', text: l.flag }), h('div', { style: 'min-width:0' }, h('div', { class: 'ln', text: l.name }), l.sub ? h('div', { class: 'ls', text: l.sub }) : null)); }),
      ], { keep: true });
    };
    draw();
  }

  /* ───────── контакты ───────── */
  function contactAvatar(c) { return c.user ? avatar(c.user, 'sm', c.name) : avatar({ id: 'c' + c.id }, 'sm', c.name); }
  function contactSub(c) {
    if (c.user) return c.user.username ? '@' + c.user.username : lastSeenText(c.user);
    return (c.target || '') + ' · ' + t('contacts.notIn');
  }
  async function contactsSheet() {
    await loadContacts();
    const st = { q: '' }; const box = h('div');
    const draw = () => {
      box.textContent = '';
      const list = S.contacts.filter((c) => !st.q || c.name.toLowerCase().includes(st.q) || (c.target || '').includes(st.q));
      if (!list.length) box.append(h('p', { class: 'hint', style: 'color:var(--ink2);font-size:13.5px', text: S.contacts.length ? t('chats.none') : t('contacts.empty') }));
      list.forEach((c) => box.append(h('button', { class: 'person', type: 'button', onclick: () => contactSheet(c.id) }, contactAvatar(c),
        h('div', { style: 'min-width:0' }, h('div', { class: 'pn', text: c.name }), h('div', { class: 'ps', text: contactSub(c) })), h('span'))));
    };
    openSheet(t('contacts.title'), [
      h('button', { class: 'btn', type: 'button', text: '+ ' + t('contacts.add'), onclick: () => addContactSheet() }),
      h('div', { class: 'field', style: 'margin-top:12px' }, h('input', { id: 'cq', type: 'search', placeholder: t('contacts.search'), autocomplete: 'off', oninput: (e) => { st.q = e.target.value.trim().toLowerCase(); draw(); } })),
      box]);
    draw();
  }
  function addContactSheet(prefill) {
    const st = Object.assign({ name: '', target: '' }, prefill); let err = '';
    const draw = () => {
      openSheet(t('contacts.add'), [h('form', { onsubmit: async (e) => {
        e.preventDefault(); err = '';
        try { const r = await api('POST', '/api/contacts', { name: st.name, target: st.target }); await loadContacts(); renderList(); renderHead(); renderMsgs(); toast(t('contacts.saved')); contactSheet(r.contact.id); }
        catch (e2) { err = errMsg(e2); draw(); }
      } },
        field('cname', t('contacts.name'), h('input', { id: 'cname', value: st.name, maxlength: '50', required: true, autocomplete: 'off', oninput: (e) => { st.name = e.target.value; } })),
        field('ctarget', t('contacts.target'), h('input', { id: 'ctarget', value: st.target, required: true, autocomplete: 'off', placeholder: '+375 29 123 45 67', oninput: (e) => { st.target = e.target.value; } }), t('contacts.targetHint')),
        err ? h('p', { class: 'err', role: 'alert', text: err }) : null,
        h('button', { class: 'btn', type: 'submit', text: t('contacts.save') }))]);
      const f = $('#cname'); if (f && !coarse) f.focus();
    };
    draw();
  }
  function inviteBox() {
    const text = t('contacts.invite', { url: location.origin });
    const inp = h('input', { readonly: true, value: text, 'aria-label': t('contacts.inviteTitle'), style: 'width:100%;border:1px solid var(--line);background:var(--pane2);border-radius:10px;padding:8px 10px;margin-top:8px' });
    return h('div', { class: 'invite' }, h('b', { text: t('contacts.inviteTitle') }), inp,
      h('button', { class: 'btn ghost', type: 'button', style: 'margin-top:8px;height:40px', text: t('contacts.copy'), onclick: async () => { toast((await copyText(text, inp)) ? t('contacts.copied') : text); } }));
  }
  function contactSheet(id) {
    const c = S.contacts.find((x) => x.id === id); if (!c) return contactsSheet();
    let editing = false;
    const draw = () => {
      const body = [h('div', { class: 'prof-head' }, contactAvatar(Object.assign({}, c, {})), h('div', { class: 'pname', text: c.name }),
        c.user ? h('div', { class: 'puser', text: t('contacts.inProfile', { name: c.user.name }) + (c.user.username ? ' · @' + c.user.username : '') }) : null,
        c.target ? h('div', { class: 'hint', text: c.target }) : null,
        c.user ? h('div', { class: 'hint', text: lastSeenText(c.user) }) : null,
        c.user && c.user.bio ? h('div', { class: 'pbio', text: c.user.bio }) : null)];
      if (!c.user) body.push(h('p', { class: 'hint', style: 'color:var(--ink2);font-size:13.5px', text: t('contacts.notRegistered') }), inviteBox());
      if (editing) {
        let nm = c.name;
        body.push(h('form', { onsubmit: async (e) => { e.preventDefault(); try { await api('PATCH', '/api/contacts/' + c.id, { name: nm }); await loadContacts(); renderList(); renderHead(); renderMsgs(); contactSheet(c.id); } catch (e2) { toast(errMsg(e2)); } } },
          field('rn', t('contacts.name'), h('input', { id: 'rn', value: nm, maxlength: '50', required: true, oninput: (e) => { nm = e.target.value; } })),
          h('button', { class: 'btn', type: 'submit', text: t('contacts.save') })));
      } else {
        const acts = [];
        if (c.user) acts.push(h('button', { class: 'btn', type: 'button', text: t('contacts.write'), onclick: async () => { try { const r = await api('POST', '/api/chats', { type: 'dm', userId: c.userId }); putUsers(r.users); S.chats.set(r.chat.id, r.chat); closeSheet(); openChat(r.chat.id); } catch (e) { toast(errMsg(e)); } } }));
        acts.push(h('button', { class: 'btn ghost', type: 'button', text: t('contacts.rename'), onclick: () => { editing = true; draw(); } }),
          h('button', { class: 'btn danger', type: 'button', text: t('contacts.delete'), onclick: async () => { try { await api('DELETE', '/api/contacts/' + c.id); await loadContacts(); renderList(); renderHead(); renderMsgs(); toast(t('contacts.deleted')); contactsSheet(); } catch (e) { toast(errMsg(e)); } } }));
        body.push(h('div', { style: 'display:flex;flex-direction:column;gap:0' }, acts));
      }
      openSheet(t('contacts.title'), body);
    };
    draw();
  }
  function nameContactSheet(userId) {
    const u = user(userId); if (!u) return;
    const ex = S.cByUser.get(userId); let nm = ex ? ex.name : u.name;
    openSheet(ex ? t('contacts.editName') : t('contacts.saveFromProfile'), [h('form', { onsubmit: async (e) => {
      e.preventDefault();
      try { await api('POST', '/api/contacts', { userId, name: nm }); await loadContacts(); renderList(); renderHead(); renderMsgs(); toast(t('contacts.saved')); userSheet(userId); } catch (e2) { toast(errMsg(e2)); }
    } }, field('nn', t('contacts.name'), h('input', { id: 'nn', value: nm, maxlength: '50', required: true, oninput: (e) => { nm = e.target.value; } })), h('button', { class: 'btn', type: 'submit', text: t('contacts.save') }))]);
  }

  /* ───────── смайлики ───────── */
  function emojiManageSheet() {
    const draw = (name, preview, err) => {
      const mine = [...S.emojis.values()].filter((e) => e.ownerId === S.me.id);
      const dataUrl = preview || '';
      const nameInp = h('input', { id: 'ename', value: name || '', maxlength: '24', placeholder: 'kotik', autocapitalize: 'none', autocomplete: 'off' });
      const prev = h('div', { class: 'myem' }, dataUrl ? h('img', { alt: '', src: dataUrl }) : h('span'), h('span', { class: 'hint', text: dataUrl ? t('em.picked') : t('em.pickHint') }),
        h('button', { class: 'link', type: 'button', text: dataUrl ? t('em.other') : t('em.pick'), onclick: () => pickFile(async (f) => { try { const d = await fileToDataUrl(f, 192, 'emoji'); draw(nameInp.value, d, ''); } catch (e) { draw(nameInp.value, dataUrl, errMsg(e)); } }) }));
      const form = h('form', { onsubmit: async (e) => {
        e.preventDefault();
        if (!dataUrl) return draw(nameInp.value, '', t('em.needPic'));
        try { const r = await api('POST', '/api/emojis', { name: nameInp.value, data: dataUrl }); S.emojis.set(r.emoji.id, r.emoji); renderEmoji(); renderMsgs(); draw('', '', ''); toast(t('em.added', { name: r.emoji.name })); }
        catch (e2) { draw(nameInp.value, dataUrl, errMsg(e2)); }
      } }, field('ename', t('em.name'), nameInp, t('em.nameHint')), prev,
        err ? h('p', { class: 'err', role: 'alert', text: err }) : null, h('button', { class: 'btn', type: 'submit', text: t('em.addBtn') }));
      openSheet(t('settings.emojis'), [form, h('div', { class: 'sep' }), mine.length ? mine.map((e) => h('div', { class: 'myem' }, h('img', { alt: '', src: e.url }), h('span', { text: ':' + e.name + ':' }),
        h('button', { class: 'link', type: 'button', text: t('em.del'), onclick: async () => { try { await api('DELETE', '/api/emojis/' + e.id); S.emojis.delete(e.id); renderEmoji(); renderMsgs(); draw('', '', ''); } catch (e3) { toast(errMsg(e3)); } } }))) : h('p', { class: 'hint', text: t('em.noneMine') })]);
    };
    draw('', '', '');
  }
  function userSheet(id) {
    const u = user(id); if (!u) return; const c = S.cByUser.get(id);
    openSheet(t('user.profile'), [h('div', { class: 'prof-head' }, avatar(u, 'lg', dn(id)), h('div', { class: 'pname', text: dn(id) }),
      c ? h('div', { class: 'puser', text: t('contacts.inProfile', { name: u.name }) }) : null,
      u.username ? h('div', { class: 'puser', text: '@' + u.username }) : null, h('div', { class: 'hint', text: lastSeenText(u) }), u.bio ? h('div', { class: 'pbio', text: u.bio }) : null),
    h('button', { class: 'btn ghost', type: 'button', text: c ? t('contacts.editName') : t('contacts.saveFromProfile'), onclick: () => nameContactSheet(id) })]);
  }
  function groupSheet(c) {
    const ids = c.members;
    openSheet(c.title || '…', [h('p', { class: 'hint', style: 'color:var(--ink2);font-size:13.5px', text: tn('members', ids.length) }),
      ids.map((id) => { const u = user(id) || { id, name: '…' }; return h('button', { class: 'person', type: 'button', onclick: () => { if (id !== S.me.id) userSheet(id); } }, avatar(u, 'sm', dn(id)), h('div', { style: 'min-width:0' }, h('div', { class: 'pn', text: id === S.me.id ? u.name + t('group.you') : dn(id) }), h('div', { class: 'ps', text: u.username ? '@' + u.username : lastSeenText(u) })), h('span')); }),
      h('div', { class: 'sep' }),
      h('button', { class: 'btn danger', type: 'button', text: t('group.leave'), onclick: async () => { try { await api('POST', '/api/chats/' + c.id + '/leave'); S.chats.delete(c.id); closeSheet(); closeChat(); renderList(); } catch (e) { toast(errMsg(e)); } } })]);
  }

  /* ───────── новый чат ───────── */
  function newChatSheet() {
    const st = { tab: 'dm', q: '', res: [], sel: new Map(), title: '' }; let timer = null, seq = 0;
    const search = (v) => {
      st.q = v; clearTimeout(timer);
      timer = setTimeout(async () => {
        const my = ++seq;
        if (v.trim().length < 2) { st.res = []; return drawList(); }
        try { const r = await api('GET', '/api/users/search?q=' + encodeURIComponent(v.trim())); if (my !== seq) return; r.users.forEach((u) => S.users.set(u.id, u)); st.res = r.users; drawList(); } catch (_) {}
      }, 250);
    };
    const listBox = h('div'), chipsBox = h('div', { class: 'chips' });
    const makeBtn = h('button', { class: 'btn', type: 'button', text: t('new.create'), disabled: true, onclick: async () => {
      try { const r = await api('POST', '/api/chats', { type: 'group', title: st.title, members: [...st.sel.keys()] }); putUsers(r.users); S.chats.set(r.chat.id, r.chat); closeSheet(); openChat(r.chat.id); } catch (e) { toast(errMsg(e)); }
    } });
    async function pick(u) {
      if (st.tab === 'dm') { try { const r = await api('POST', '/api/chats', { type: 'dm', userId: u.id }); putUsers(r.users); S.chats.set(r.chat.id, r.chat); closeSheet(); openChat(r.chat.id); } catch (e) { toast(errMsg(e)); } }
      else { if (st.sel.has(u.id)) st.sel.delete(u.id); else st.sel.set(u.id, u); drawList(); }
    }
    function row(u, sub) {
      const sel = st.sel.has(u.id);
      return h('button', { class: 'person' + (sel ? ' sel' : ''), type: 'button', onclick: () => pick(u) }, avatar(u, 'sm', dn(u.id)),
        h('div', { style: 'min-width:0' }, h('div', { class: 'pn', text: dn(u.id) }), h('div', { class: 'ps', text: sub })), st.tab === 'group' ? h('span', { class: 'chk', text: sel ? '✓' : '' }) : h('span'));
    }
    function drawList() {
      listBox.textContent = '';
      if (st.q.trim().length < 2) {
        const mine = S.contacts.filter((c) => c.user);
        if (mine.length) { listBox.append(h('div', { class: 'sect', text: t('new.contacts') })); mine.forEach((c) => listBox.append(row(c.user, c.user.username ? '@' + c.user.username : lastSeenText(c.user)))); }
        else listBox.append(h('p', { class: 'hint', style: 'color:var(--ink2);font-size:13.5px', text: t('new.hintType') }));
      } else if (!st.res.length) listBox.append(h('p', { class: 'hint', style: 'color:var(--ink2);font-size:13.5px', text: t('new.notFound') }));
      st.res.forEach((u) => listBox.append(row(u, (S.cByUser.has(u.id) ? u.name + ' · ' : '') + (u.username ? '@' + u.username : ''))));
      chipsBox.textContent = ''; st.sel.forEach((u) => chipsBox.append(h('span', { class: 'chip', text: dn(u.id) })));
      makeBtn.disabled = !(st.title.trim() && st.sel.size);
    }
    function frame() {
      openSheet(t('new.title'), [
        h('div', { class: 'seg' },
          h('button', { type: 'button', class: st.tab === 'dm' ? 'on' : '', text: t('new.dm'), onclick: () => { st.tab = 'dm'; frame(); } }),
          h('button', { type: 'button', class: st.tab === 'group' ? 'on' : '', text: t('new.group'), onclick: () => { st.tab = 'group'; frame(); } })),
        st.tab === 'group' ? field('gt', t('new.groupName'), h('input', { id: 'gt', maxlength: '60', value: st.title, oninput: (e) => { st.title = e.target.value; makeBtn.disabled = !(st.title.trim() && st.sel.size); } })) : null,
        field('us', t('new.find'), h('input', { id: 'us', type: 'search', value: st.q, autocomplete: 'off', placeholder: t('new.findPh'), oninput: (e) => search(e.target.value) })),
        st.tab === 'group' ? chipsBox : null, listBox, st.tab === 'group' ? makeBtn : null,
        h('button', { class: 'link', type: 'button', text: '+ ' + t('contacts.add'), onclick: () => addContactSheet() })]);
      drawList(); const f = $('#us'); if (f && !coarse) f.focus();
    }
    frame();
  }

  /* ───────── каркас приложения ───────── */
  function buildApp() {
    const draft = $('#text') ? $('#text').value : '';
    const root = $('#root'); root.textContent = '';
    root.append(h('div', { id: 'app', 'data-view': S.active ? 'chat' : 'list' },
      h('aside', { class: 'side' },
        h('div', { id: 'offline', class: 'off', hidden: true, text: t('offline') }),
        h('header', { class: 'side-top' },
          h('button', { class: 'meBtn', id: 'meBtn', type: 'button', 'aria-label': t('settings.title'), onclick: settingsSheet }),
          h('div', { class: 'brand', text: 'Чат-Чат' }),
          h('button', { class: 'iconbtn', type: 'button', 'aria-label': t('new.title'), onclick: newChatSheet }, svg(ICON.plus))),
        h('div', { class: 'search' }, h('input', { id: 'q', type: 'search', placeholder: t('chats.search'), autocomplete: 'off', value: S.filter, oninput: (e) => { S.filter = e.target.value.trim().toLowerCase(); renderList(); } })),
        h('div', { id: 'list', class: 'list' })),
      h('main', { class: 'conv' },
        h('div', { id: 'empty', class: 'empty', hidden: !!S.active }, h('div', {}, logoSvg(56), h('b', { text: t('chats.pickTitle') }), t('chats.pickText'))),
        h('section', { id: 'convbox', class: 'convbox', hidden: !S.active },
          h('header', { class: 'conv-top' },
            h('button', { class: 'iconbtn back', type: 'button', 'aria-label': t('chats.back'), onclick: closeChat }, svg(ICON.back)),
            h('button', { class: 'titlebtn', id: 'titlebtn', type: 'button', onclick: () => { const c = S.chats.get(S.active); if (!c) return; c.type === 'dm' ? userSheet(peerOf(c)) : groupSheet(c); } },
              h('span', { id: 'headav' }), h('div', { class: 'titles' }, h('div', { class: 'ctitle', id: 'ctitle' }), h('div', { class: 'csub', id: 'csub' })))),
          h('div', { id: 'msgs', class: 'msgs', onscroll: onMsgScroll }),
          h('div', { id: 'emoji', class: 'emoji', hidden: true }),
          h('form', { id: 'composer', class: 'composer', autocomplete: 'off', onsubmit: onSend },
            h('button', { class: 'iconbtn', type: 'button', 'aria-label': t('msg.emoji'), onclick: () => { S.emojiOpen = !S.emojiOpen; renderEmoji(); } }, svg(ICON.smile, 22)),
            h('textarea', { id: 'text', rows: '1', maxlength: '2000', placeholder: t('msg.placeholder'), 'aria-label': t('msg.placeholder'), oninput: () => { autosize(); typingPing(); }, onkeydown: (e) => { if (e.key === 'Enter' && !e.shiftKey && !coarse && !e.isComposing) { e.preventDefault(); $('#composer').requestSubmit(); } } }),
            h('button', { class: 'send', type: 'submit', 'aria-label': t('msg.send') }, svg(ICON.send)))))));
    if (draft) { $('#text').value = draft; autosize(); }
    renderMe();
  }
  function renderMe() { const b = $('#meBtn'); if (!b || !S.me) return; b.textContent = ''; b.append(avatar(S.me, 'sm')); }
  function refreshUI() {
    if (!S.me) { renderAuth(); return; }
    buildApp(); renderList();
    if (S.active && S.chats.has(S.active)) { renderHead(); renderMsgs(); renderEmoji(); }
  }
  function renderList() {
    const box = $('#list'); if (!box) return; box.textContent = '';
    const items = sortedChats().filter((c) => !S.filter || titleOf(c).toLowerCase().includes(S.filter));
    if (!items.length) { box.append(h('div', { class: 'none', text: S.filter ? t('chats.none') : t('chats.noChats') })); return; }
    for (const c of items) {
      const ty = typingNames(c);
      box.append(h('button', { class: 'item' + (c.id === S.active ? ' on' : ''), type: 'button', onclick: () => openChat(c.id) },
        chatAvatar(c),
        h('div', { style: 'min-width:0' },
          h('div', { class: 'l1' }, h('span', { class: 'name', text: titleOf(c) }), c.last ? h('span', { class: 'time', text: listTime(c.last.createdAt) }) : null),
          h('div', { class: 'l2' }, h('span', { class: 'prev' + (ty.length ? ' typing' : ''), text: ty.length ? t('msg.typing') : previewOf(c) }), c.unread > 0 ? h('span', { class: 'count', text: c.unread > 99 ? '99+' : String(c.unread) }) : null))));
    }
  }
  function typingNames(c) {
    const out = []; const now = Date.now();
    for (const k of Object.keys(S.typing)) { const [cid, uid] = k.split(':').map(Number); if (cid === c.id && now - S.typing[k] < 4000 && uid !== S.me.id) out.push(dn(uid)); }
    return out;
  }
  function renderHead() {
    const c = S.chats.get(S.active); if (!c || !$('#headav')) return;
    const ty = typingNames(c); let sub, typing = false;
    if (ty.length) { typing = true; sub = c.type === 'dm' ? t('msg.typing') : ty.join(', ') + ' ' + (ty.length > 1 ? t('msg.typingMany') : t('msg.typing')); }
    else if (c.type === 'dm') sub = lastSeenText(user(peerOf(c)));
    else { const on = c.members.filter((id) => (user(id) || {}).online).length; sub = tn('members', c.members.length) + (on ? ' · ' + t('members.online', { n: on }) : ''); }
    const av = $('#headav'); av.textContent = ''; av.append(chatAvatar(c, 'sm'));
    $('#ctitle').textContent = titleOf(c);
    const cs = $('#csub'); cs.textContent = sub; cs.className = 'csub' + (typing ? ' typing' : '');
  }
  function richText(text) {
    const frag = document.createDocumentFragment(); const byName = emojiByName(); let last = 0; const re = /:([a-z0-9_]{2,24}):/g; let m;
    while ((m = re.exec(text))) {
      const e = byName.get(m[1]); if (!e) continue;
      if (m.index > last) frag.append(text.slice(last, m.index));
      frag.append(h('img', { class: 'ce', alt: ':' + e.name + ':', title: ':' + e.name + ':', src: e.url })); last = m.index + m[0].length;
    }
    if (last < text.length) frag.append(text.slice(last));
    return frag;
  }
  function renderMsgs() {
    const box = $('#msgs'); if (!box) return; const c = S.chats.get(S.active);
    const near = box.scrollHeight - box.scrollTop - box.clientHeight < 140 || S.forceBottom;
    box.textContent = ''; if (!c) return;
    const list = S.msgs.get(c.id) || [];
    if (!list.length) { box.append(h('div', { class: 'hint-c', text: t('msg.quiet') })); return; }
    let lastDay = '', prev = null;
    list.forEach((m, i) => {
      const dk = dayKey(m.createdAt);
      if (dk !== lastDay) { box.append(h('div', { class: 'day' }, h('span', { text: dayLabel(m.createdAt) }))); lastDay = dk; prev = null; }
      const mine = m.userId === S.me.id, first = !prev || prev.userId !== m.userId || m.createdAt - prev.createdAt > 300000;
      const nx = list[i + 1], last = !nx || nx.userId !== m.userId || nx.createdAt - m.createdAt > 300000 || dayKey(nx.createdAt) !== dk;
      const body = [];
      if (c.type === 'group' && first && !mine) { const w = h('div', { class: 'who', text: dn(m.userId) }); w.style.color = hashColor(m.userId); body.push(w); }
      let cls = '';
      if (m.kind === 'sticker') { cls = ' sticker'; const e = S.emojis.get(m.emojiId); body.push(e ? h('img', { class: 'st', alt: ':' + e.name + ':', src: e.url }) : h('div', { class: 'gone', text: t('msg.stickerGone') })); }
      else { if (onlyEmoji(m.text)) cls = ' bigemoji'; body.push(h('span', { class: 'txt' }, richText(m.text))); }
      body.push(h('span', { class: 't', text: timeStr(m.createdAt) }));
      const bub = h('div', { class: 'bub' + (last ? ' tail' : '') + cls, onclick: mine ? () => { S.selMsg = S.selMsg === m.id ? null : m.id; renderMsgs(); } : null }, body);
      const row = h('div', { class: 'row' + (mine ? ' mine' : '') + (first ? ' first' : '') }, bub);
      if (mine && S.selMsg === m.id) row.append(h('button', { class: 'del', type: 'button', text: t('msg.delete'), onclick: async () => { S.selMsg = null; try { await api('DELETE', '/api/messages/' + m.id); removeMsg(c.id, m.id); } catch (e) { toast(errMsg(e)); } } }));
      box.append(row); prev = m;
    });
    if (near) { box.scrollTop = box.scrollHeight; S.forceBottom = false; }
  }
  const EMOJI = {
    smileys: '😀 😃 😄 😁 😆 😅 😂 🤣 😊 😇 🙂 😉 😍 🥰 😘 😋 😛 😜 🤪 🤔 🤨 😐 😶 🙄 😏 😴 😮 😲 😳 🥺 😢 😭 😤 😡 🥳 😎 🤓 🤯 😱 🤗'.split(' '),
    gestures: '👍 👎 👌 ✌️ 🤞 🤟 🤘 👏 🙌 🙏 🤝 💪 👋 ✋ 👀 🫶 🫡'.split(' '),
    hearts: '❤️ 🧡 💛 💚 💙 💜 🖤 🤍 💔 💕 💖 💯 🔥 ✨ 🎉 🎊'.split(' '),
    misc: '🐱 🐶 🦜 🐼 🦊 🍕 🍔 🍟 🍩 🍪 ☕ 🍺 🎮 🎧 📷 💻 📱 🚀 ⚽ 🎂 🌈 ☀️ 🌙 ⭐'.split(' '),
  };
  function renderEmoji() {
    const box = $('#emoji'); if (!box) return; box.hidden = !S.emojiOpen; if (!S.emojiOpen) return; box.textContent = '';
    const tabs = Object.keys(EMOJI).concat(['mine']);
    box.append(h('div', { class: 'etabs' }, tabs.map((k) => h('button', { type: 'button', class: k === S.emojiTab ? 'on' : '', text: k === 'mine' ? t('em.tabMine') : t('et.' + k), onclick: () => { S.emojiTab = k; renderEmoji(); } }))));
    if (S.emojiTab === 'mine') {
      const g = h('div', { class: 'egrid cust' },
        h('div', { class: 'mode' }, t('em.mode'), h('button', { type: 'button', class: S.emojiMode === 'text' ? 'on' : '', text: t('em.modeText'), onclick: () => { S.emojiMode = 'text'; renderEmoji(); } }), h('button', { type: 'button', class: S.emojiMode === 'sticker' ? 'on' : '', text: t('em.modeSticker'), onclick: () => { S.emojiMode = 'sticker'; renderEmoji(); } })),
        h('button', { class: 'addtile', type: 'button', 'aria-label': t('em.add'), text: '+', onclick: emojiManageSheet }),
        [...S.emojis.values()].map((e) => h('button', { type: 'button', title: ':' + e.name + ':', 'aria-label': e.name, onclick: () => { if (S.emojiMode === 'sticker') { S.emojiOpen = false; renderEmoji(); sendMsg({ emojiId: e.id }); } else insertText(':' + e.name + ':'); } }, h('img', { alt: '', src: e.url }))));
      if (!S.emojis.size) g.append(h('div', { class: 'full', text: t('em.none') }));
      box.append(g);
    } else box.append(h('div', { class: 'egrid' }, EMOJI[S.emojiTab].map((e) => h('button', { type: 'button', text: e, onclick: () => insertText(e) }))));
  }
  function insertText(s) {
    const ta = $('#text'), a = ta.selectionStart == null ? ta.value.length : ta.selectionStart, b = ta.selectionEnd == null ? a : ta.selectionEnd;
    ta.setRangeText(s, a, b, 'end'); if (!coarse) ta.focus(); autosize(); typingPing();
  }
  const autosize = () => { const ta = $('#text'); if (!ta) return; ta.style.height = 'auto'; ta.style.height = Math.min(ta.scrollHeight, 140) + 'px'; };
  let typingSent = 0;
  function typingPing() { const n = Date.now(); if (S.active && S.wsOk && n - typingSent > 2500) { typingSent = n; S.ws.send(JSON.stringify({ t: 'typing', chat: S.active })); } }

  /* ───────── чаты и сообщения ───────── */
  async function loadChats() { const r = await api('GET', '/api/chats'); putUsers(r.users); S.chats = new Map(r.chats.map((c) => [c.id, c])); renderList(); renderHead(); }
  async function loadEmojis() { const r = await api('GET', '/api/emojis'); S.emojis = new Map(r.emojis.map((e) => [e.id, e])); renderEmoji(); renderMsgs(); }
  async function openChat(id) {
    S.active = id; S.selMsg = null; S.emojiOpen = false; S.forceBottom = true;
    $('#app').dataset.view = 'chat'; $('#empty').hidden = true; $('#convbox').hidden = false;
    renderHead(); renderList(); renderEmoji(); renderMsgs();
    try {
      const r = await api('GET', '/api/chats/' + id + '/messages?limit=50');
      if (S.active !== id) return;
      S.msgs.set(id, r.messages); S.more.set(id, r.more); S.forceBottom = true; renderMsgs(); markRead(id);
    } catch (e) { toast(errMsg(e)); }
    if (!coarse) $('#text').focus();
  }
  function closeChat() { S.active = null; $('#app').dataset.view = 'list'; $('#convbox').hidden = true; $('#empty').hidden = false; renderList(); }
  let loadingOlder = false;
  async function onMsgScroll(e) {
    const box = e.target, id = S.active; if (!id || loadingOlder || box.scrollTop > 60 || !S.more.get(id)) return;
    const list = S.msgs.get(id) || []; if (!list.length) return;
    loadingOlder = true;
    try {
      const r = await api('GET', '/api/chats/' + id + '/messages?limit=50&before=' + list[0].id);
      if (S.active !== id) return; const h0 = box.scrollHeight;
      S.msgs.set(id, r.messages.concat(list)); S.more.set(id, r.more); renderMsgs(); box.scrollTop = box.scrollHeight - h0;
    } catch (_) {} finally { loadingOlder = false; }
  }
  function markRead(id) {
    const c = S.chats.get(id), list = S.msgs.get(id) || []; if (!c) return;
    const last = list.length ? list[list.length - 1].id : (c.last ? c.last.id : 0);
    if (c.unread > 0) { c.unread = 0; renderList(); }
    if (last) api('POST', '/api/chats/' + id + '/read', { upTo: last }).catch(() => {});
  }
  function addMsg(m) {
    const c = S.chats.get(m.chatId);
    if (!c) { loadChats().then(() => addMsg(m)); return; }
    const list = S.msgs.get(m.chatId);
    if (list) { if (list.some((x) => x.id === m.id)) return; list.push(m); list.sort((a, b) => a.id - b.id); }
    c.last = m;
    const mine = m.userId === S.me.id, viewing = S.active === m.chatId && document.visibilityState === 'visible';
    if (!mine && !viewing) { c.unread = (c.unread || 0) + 1; notify(c, m); }
    renderList();
    if (S.active === m.chatId) { renderMsgs(); if (viewing && !mine) markRead(m.chatId); }
  }
  function removeMsg(chatId, id) {
    const list = S.msgs.get(chatId), c = S.chats.get(chatId);
    if (list) { const i = list.findIndex((x) => x.id === id); if (i > -1) list.splice(i, 1); }
    if (c && c.last && c.last.id === id) c.last = list && list.length ? list[list.length - 1] : null;
    renderList(); if (S.active === chatId) renderMsgs();
  }
  function notify(c, m) {
    if (!('Notification' in window) || Notification.permission !== 'granted') return;
    try { new Notification(c.type === 'group' ? titleOf(c) : dn(m.userId), { body: (c.type === 'group' ? dn(m.userId) + ': ' : '') + (m.kind === 'sticker' ? t('msg.sticker') : m.text), icon: '/icons/icon-192.png', tag: 'chat' + c.id }); } catch (_) {}
  }
  async function sendMsg(payload) {
    const id = S.active; if (!id) return; S.forceBottom = true;
    try { const r = await api('POST', '/api/chats/' + id + '/messages', payload); addMsg(r.message); }
    catch (e) { toast(errMsg(e)); }
  }
  function onSend(e) {
    e.preventDefault(); const ta = $('#text'), v = ta.value.trim(); if (!v) return;
    ta.value = ''; autosize(); sendMsg({ text: v });
  }

  /* ───────── соединение в реальном времени ───────── */
  let retry = 0;
  function connectWs() {
    if (!S.token) return;
    const ws = new WebSocket((location.protocol === 'https:' ? 'wss://' : 'ws://') + location.host + '/ws'); S.ws = ws;
    ws.onopen = () => ws.send(JSON.stringify({ t: 'auth', token: S.token }));
    ws.onmessage = (ev) => {
      let m; try { m = JSON.parse(ev.data); } catch (_) { return; }
      switch (m.t) {
        case 'ready': S.wsOk = true; retry = 0; if ($('#offline')) $('#offline').hidden = true; if (S.reconnected) { loadChats(); loadEmojis(); loadContacts(); if (S.active) openChatSilent(S.active); } S.reconnected = true; break;
        case 'auth_fail': logout(true); break;
        case 'msg': addMsg(m.message); break;
        case 'del': removeMsg(m.chatId, m.id); break;
        case 'chat': { putUsers(m.users); const old = S.chats.get(m.chat.id); S.chats.set(m.chat.id, Object.assign({}, m.chat, old && S.active === m.chat.id ? { unread: 0 } : {})); renderList(); if (S.active === m.chat.id) renderHead(); break; }
        case 'user': S.users.set(m.user.id, m.user); if (m.user.id === S.me.id) S.me = Object.assign({}, S.me, m.user); renderList(); renderHead(); renderMsgs(); break;
        case 'presence': { const u = S.users.get(m.userId); if (u) { u.online = m.online; u.lastSeen = m.lastSeen; } renderList(); renderHead(); break; }
        case 'typing': S.typing[m.chat + ':' + m.userId] = Date.now(); renderList(); renderHead(); setTimeout(() => { renderList(); renderHead(); }, 4100); break;
        case 'emoji': S.emojis.set(m.emoji.id, m.emoji); renderEmoji(); renderMsgs(); break;
        case 'emoji_del': S.emojis.delete(m.id); renderEmoji(); renderMsgs(); break;
      }
    };
    ws.onclose = () => { S.wsOk = false; if (!S.token) return; const o = $('#offline'); if (o) o.hidden = false; retry = Math.min(retry + 1, 6); setTimeout(connectWs, Math.min(1000 * 2 ** retry, 20000)); };
    ws.onerror = () => { try { ws.close(); } catch (_) {} };
  }
  async function openChatSilent(id) { try { const r = await api('GET', '/api/chats/' + id + '/messages?limit=50'); S.msgs.set(id, r.messages); S.more.set(id, r.more); renderMsgs(); } catch (_) {} }

  /* ───────── запуск ───────── */
  async function startApp(isNew, afterSetup) {
    if (!S.me.name && !afterSetup) { renderAuthBackdrop(); profileSheet(true); return; }
    buildApp(); S.users.set(S.me.id, S.me);
    try { await Promise.all([loadChats(), loadEmojis(), loadContacts()]); } catch (_) {}
    renderList(); renderHead();
    connectWs(); S.reconnected = false;
    if (matchMedia('(min-width:761px)').matches) { const f = sortedChats()[0]; if (f) openChat(f.id); }
  }
  function renderAuthBackdrop() { const root = $('#root'); root.textContent = ''; root.append(h('div', { class: 'auth' }, h('div', { class: 'logo' }, logoSvg(38), 'Чат-Чат'))); }
  document.addEventListener('visibilitychange', () => { if (document.visibilityState === 'visible' && S.active) { const c = S.chats.get(S.active); if (c && c.unread) markRead(S.active); } });
  window.addEventListener('beforeinstallprompt', (e) => { e.preventDefault(); S.installEvt = e; });
  window.addEventListener('online', () => { if (S.token && !S.wsOk) { try { S.ws && S.ws.close(); } catch (_) {} } });
  window.addEventListener('error', (e) => { if (e && e.message && !/ResizeObserver/.test(e.message)) toast('⚠ ' + e.message); });
  window.addEventListener('unhandledrejection', (e) => { const r = e && e.reason; if (r && r.message && r.code == null) toast('⚠ ' + r.message); });
  if ('serviceWorker' in navigator) navigator.serviceWorker.register('/sw.js').catch(() => {});

  (async function init() {
    try { S.config = await (await fetch('/api/config')).json(); } catch (_) {}
    if (!S.config.email && S.config.phone) A.kind = 'phone';
    if (S.token) {
      try { const r = await api('GET', '/api/me'); S.me = r.user; adoptServerSettings(r.user.settings); await startApp(false); return; } catch (e) { if (e.code === 'network') { renderAuthBackdrop(); toast(e.message); return; } }
    }
    renderAuth();
  })();
})();
