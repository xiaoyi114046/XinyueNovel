(() => {
  'use strict';
  const C = window.XinyueCore;
  const STORAGE_KEY = 'xinyue_state_v4_1_3_android';
  const $ = id => document.getElementById(id);
  let state;
  let selectedChapterNo = null;
  let pending = new Map();
  let activeRequestId = null;
  let statusTimer = null;

  function defaultState() {
    const p = C.newProject('我的小说');
    return {
      version: 1,
      currentId: p.id,
      projects: [p],
      settings: { endpoint: C.DEFAULT_ENDPOINT, model: 'deepseek-chat', thinking: '关闭', rememberKey: true }
    };
  }

  function loadState() {
    try {
      const raw = localStorage.getItem(STORAGE_KEY);
      const x = raw ? JSON.parse(raw) : defaultState();
      x.projects = Array.isArray(x.projects) && x.projects.length ? x.projects.map(C.normalizeProject) : [C.newProject('我的小说')];
      x.currentId = x.projects.some(p => p.id === x.currentId) ? x.currentId : x.projects[0].id;
      x.settings = Object.assign({ endpoint: C.DEFAULT_ENDPOINT, model: 'deepseek-chat', thinking: '关闭', rememberKey: true }, x.settings || {});
      return x;
    } catch (e) {
      return defaultState();
    }
  }

  function saveState() {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(state));
  }

  function project() {
    return state.projects.find(p => p.id === state.currentId) || state.projects[0];
  }

  function markUpdated(p) { p.updated_at = new Date().toISOString(); }

  function showStatus(text, isError = false, persistent = false) {
    const s = $('status');
    s.textContent = text;
    s.classList.toggle('error', !!isError);
    s.classList.add('show');
    if (statusTimer) clearTimeout(statusTimer);
    if (!persistent) statusTimer = setTimeout(() => s.classList.remove('show'), 4200);
  }

  function nav(page) {
    document.querySelectorAll('.page').forEach(el => el.classList.toggle('active', el.dataset.page === page));
    document.querySelectorAll('[data-nav]').forEach(el => el.classList.toggle('active', el.dataset.nav === page));
    window.scrollTo(0, 0);
  }

  function fillSelect(select, value) {
    if (![...select.options].some(o => o.value === value || o.text === value)) {
      const o = document.createElement('option'); o.value = value; o.textContent = value; select.appendChild(o);
    }
    select.value = value;
  }

  function refreshProjectSelects() {
    const selects = [$('homeProject'), $('novelProject'), $('worldProject'), $('projectSelect')];
    for (const sel of selects) {
      sel.innerHTML = '';
      state.projects.forEach(p => {
        const o = document.createElement('option');
        o.value = p.id; o.textContent = `${p.name}（${(p.chapters || []).length}章）`; sel.appendChild(o);
      });
      sel.value = state.currentId;
    }
    const p = project();
    $('projectPill').textContent = `当前：${p.name} · ${(p.chapters || []).length}章 · ${p.genre} · ${p.style}`;
  }

  function loadProjectToUI() {
    const p = project();
    refreshProjectSelects();
    fillSelect($('genre'), p.genre || '玄幻'); fillSelect($('flow'), p.flow || '幕后流'); fillSelect($('style'), p.style || '情感细腻风');
    fillSelect($('pov'), p.pov || '第三人称'); fillSelect($('pace'), p.pace || '张弛有度');
    $('protagonist').value = p.protagonist || ''; $('characters').value = p.characters || ''; $('plot').value = p.plot || '';
    $('world').value = p.world || ''; $('realms').value = p.realms || ''; $('currentRealm').value = p.current_realm || '';
    $('outline').value = p.outline || ''; $('volumeOutline').value = p.volume_outline || ''; $('chapterOutline').value = p.chapter_outline || '';
    $('homeOutput').value = p.draft || '';
    $('homeTitle').value = `第${C.nextChapterNo(p)}章`;
    $('renameProjectName').value = p.name || '';
    selectedChapterNo = null;
    refreshChapterList();
  }

  function saveNovelFields(silent = false) {
    const p = project();
    p.genre = $('genre').value; p.flow = $('flow').value; p.style = $('style').value; p.pov = $('pov').value; p.pace = $('pace').value;
    p.protagonist = $('protagonist').value.trim(); p.characters = $('characters').value; p.plot = $('plot').value; markUpdated(p); saveState(); refreshProjectSelects();
    if (!silent) showStatus('小说设定已保存');
    return true;
  }

  function saveWorldFields(silent = false) {
    const p = project(); p.world = $('world').value; p.realms = $('realms').value; p.current_realm = $('currentRealm').value.trim(); p.outline = $('outline').value;
    p.volume_outline = $('volumeOutline').value; p.chapter_outline = $('chapterOutline').value; markUpdated(p); saveState();
    if (!silent) showStatus('世界观与大纲已保存');
    return true;
  }

  function refreshChapterList() {
    const list = $('chapterList'); list.innerHTML = '';
    const p = project();
    C.sortChapters(p.chapters);
    if (!p.chapters.length) {
      const span = document.createElement('span'); span.className = 'hint'; span.textContent = '暂无已保存章节'; list.appendChild(span);
      $('chapterTitle').value = ''; $('chapterBody').value = ''; return;
    }
    p.chapters.forEach(c => {
      const b = document.createElement('button'); b.className = 'chapter-chip' + (Number(c.no) === Number(selectedChapterNo) ? ' active' : '');
      b.textContent = `${c.no}. ${c.title}`; b.onclick = () => { selectedChapterNo = c.no; refreshChapterList(); showSelectedChapter(); }; list.appendChild(b);
    });
  }

  function showSelectedChapter() {
    const c = project().chapters.find(x => Number(x.no) === Number(selectedChapterNo));
    if (!c) return;
    $('chapterTitle').value = c.title || ''; $('chapterBody').value = c.body || '';
  }

  function switchProject(id) {
    if (!state.projects.some(p => p.id === id)) return;
    state.currentId = id; saveState(); loadProjectToUI(); showStatus('已切换项目：' + project().name);
  }

  function settingsFromUI() {
    return { endpoint: $('endpoint').value.trim() || C.DEFAULT_ENDPOINT, model: $('model').value.trim() || 'deepseek-chat', thinking: $('thinking').value, rememberKey: $('rememberKey').checked };
  }

  function getApiKey() { return $('apiKey').value.trim(); }

  function saveSettings(show = true) {
    state.settings = settingsFromUI();
    saveState();
    const key = getApiKey();
    if (state.settings.rememberKey && key && window.Native && Native.saveApiKey) {
      const r = Native.saveApiKey(key);
      if (String(r).startsWith('ERR|')) { showStatus('API Key 加密保存失败：' + String(r).slice(4), true); return false; }
    } else if (!state.settings.rememberKey && window.Native && Native.clearApiKey) {
      Native.clearApiKey();
    }
    if (show) showStatus('API 设置已保存');
    return true;
  }

  function loadSettingsUI() {
    const s = state.settings;
    $('endpoint').value = s.endpoint || C.DEFAULT_ENDPOINT; fillSelect($('model'), s.model || 'deepseek-chat'); fillSelect($('thinking'), s.thinking || '关闭'); $('rememberKey').checked = s.rememberKey !== false;
    if ($('rememberKey').checked && window.Native && Native.loadApiKey) {
      const key = Native.loadApiKey(); if (key) $('apiKey').value = key;
    }
    $('versionText').textContent = (window.Native && Native.appVersion) ? Native.appVersion() : C.APP_VERSION;
  }

  function nativePost(endpoint, key, body) {
    return new Promise((resolve, reject) => {
      if (!window.Native || !Native.postAsync) { reject(new Error('当前环境没有 Android 原生网络桥接')); return; }
      const id = 'r_' + Date.now() + '_' + Math.random().toString(36).slice(2, 7);
      pending.set(id, { resolve, reject }); activeRequestId = id;
      Native.postAsync(id, endpoint, key, body);
    });
  }

  window.__nativeHttpResult = (id, status, body, error) => {
    const p = pending.get(id); if (!p) return;
    pending.delete(id); if (activeRequestId === id) activeRequestId = null;
    if (error) p.reject(new Error(error)); else p.resolve({ status: Number(status), body: body || '' });
  };

  function cancelActive() {
    if (activeRequestId && window.Native && Native.cancelRequest) Native.cancelRequest(activeRequestId);
    showStatus('正在停止…', false, true);
  }

  async function callDeepSeek(system, user, maxTokens) {
    const key = getApiKey(); if (!key) throw new Error('请先在设置页填写 DeepSeek API Key');
    const s = settingsFromUI();
    const payload = C.chatPayload(s, system, user, maxTokens);
    const r = await nativePost(s.endpoint || C.DEFAULT_ENDPOINT, key, JSON.stringify(payload));
    let data = null; try { data = JSON.parse(r.body); } catch (_) {}
    if (r.status < 200 || r.status >= 300) {
      const msg = data && data.error && data.error.message ? data.error.message : (r.body || `HTTP ${r.status}`);
      throw new Error(`DeepSeek API 错误（HTTP ${r.status}）：${String(msg).slice(0, 800)}`);
    }
    if (!data || !Array.isArray(data.choices) || !data.choices.length) throw new Error('API 没有返回 choices');
    const text = String((data.choices[0].message || {}).content || '').trim();
    if (!text) throw new Error('API 返回正文为空，可尝试关闭思考模式或切换模型');
    return { text, usage: data.usage || {} };
  }

  async function withBusy(kind, task) {
    const ids = ['btnGenerate','btnTestApi','btnGenRealms','btnGenOutline']; ids.forEach(id => $(id).disabled = true); $('btnStop').disabled = false;
    showStatus(kind + '处理中…', false, true);
    try { await task(); }
    catch (e) {
      const msg = String(e && e.message || e);
      if (/disconnect|Socket|cancel|aborted|unexpected end/i.test(msg)) showStatus('已停止生成'); else showStatus(msg, true);
    } finally {
      ids.forEach(id => $(id).disabled = false); $('btnStop').disabled = true; activeRequestId = null;
    }
  }

  async function generateNovel() {
    saveNovelFields(true); saveWorldFields(true); saveSettings(false);
    const p = project();
    const prompt = C.buildNovelPrompt({ project: p, chapterTitle: $('homeTitle').value, instructions: $('homeInstructions').value, context: $('homeContext').value, targetChars: Number($('homeTarget').value) || 3500 });
    await withBusy('正在续写', async () => {
      const r = await callDeepSeek(prompt.system, prompt.user, C.maxTokensForTarget(prompt.target));
      $('homeOutput').value = r.text; p.draft = r.text; markUpdated(p); saveState();
      showStatus(`续写完成 · ${Number(r.usage.total_tokens) || 0} tokens`);
    });
  }

  async function generateRealms() {
    saveNovelFields(true); saveWorldFields(true); saveSettings(false);
    const pr = C.realmsPrompt(project());
    await withBusy('正在生成境界体系', async () => {
      const r = await callDeepSeek(pr.system, pr.user, 3500); $('realms').value = r.text; project().realms = r.text; markUpdated(project()); saveState(); showStatus('AI 境界体系已生成并保存');
    });
  }

  async function generateOutline() {
    saveNovelFields(true); saveWorldFields(true); saveSettings(false);
    const pr = C.outlinePrompt(project());
    await withBusy('正在生成总纲/卷纲', async () => {
      const r = await callDeepSeek(pr.system, pr.user, 5000); $('outline').value = r.text; project().outline = r.text; markUpdated(project()); saveState(); showStatus('AI 总纲已生成并保存');
    });
  }

  async function testApi() {
    saveSettings(false);
    await withBusy('正在测试 API', async () => {
      const r = await callDeepSeek('你是API连接测试助手。', '只回复四个字：连接成功', 128);
      if ($('rememberKey').checked && window.Native && Native.saveApiKey) Native.saveApiKey(getApiKey());
      showStatus('API 连接成功：' + r.text);
    });
  }

  function saveGeneratedChapter() {
    const p = project(); const body = $('homeOutput').value.trim(); if (!body) return showStatus('续写结果为空，无法保存', true);
    const no = C.nextChapterNo(p); const title = $('homeTitle').value.trim() || `第${no}章`;
    try { C.upsertChapter(p, no, title, body); p.draft = ''; $('homeOutput').value = ''; $('homeTitle').value = `第${C.nextChapterNo(p)}章`; saveState(); refreshProjectSelects(); refreshChapterList(); showStatus(`第 ${no} 章已保存`); }
    catch (e) { showStatus(e.message, true); }
  }

  function exportNovel() {
    const p = project(); const text = C.fullNovelText(p); if (!text) return showStatus('当前项目没有已保存章节', true);
    const filename = C.safeFileName(p.name) + '_完整小说.txt';
    if (window.Native && Native.saveTextFile) {
      const r = String(Native.saveTextFile(filename, text));
      if (r.startsWith('OK|')) showStatus('TXT 已导出：' + r.slice(3)); else showStatus('导出失败：' + r.replace(/^ERR\|/, ''), true);
    } else showStatus('当前环境不支持文件导出', true);
  }

  function bind() {
    document.querySelectorAll('[data-nav]').forEach(b => b.onclick = () => nav(b.dataset.nav));
    [$('homeProject'),$('novelProject'),$('worldProject'),$('projectSelect')].forEach(sel => sel.onchange = () => switchProject(sel.value));
    $('btnRecent').onclick = () => { $('homeContext').value = C.recentContext(project(), Number($('homeRecent').value)); showStatus('最近章节已载入续写上下文'); };
    $('btnGenerate').onclick = generateNovel; $('btnStop').onclick = cancelActive; $('btnSaveChapter').onclick = saveGeneratedChapter;
    $('btnCopy').onclick = () => { if (window.Native && Native.copyText) Native.copyText($('homeOutput').value); showStatus('续写结果已复制'); };
    $('btnClear').onclick = () => { if ($('homeOutput').value.trim() && !confirm('只清空当前未入库的续写结果，确定吗？')) return; $('homeOutput').value=''; project().draft=''; saveState(); showStatus('续写结果已清空'); };
    $('homeOutput').oninput = () => { project().draft = $('homeOutput').value; saveState(); };
    $('btnSaveNovel').onclick = () => saveNovelFields(false); $('btnSaveWorld').onclick = () => saveWorldFields(false); $('btnGenRealms').onclick = generateRealms; $('btnGenOutline').onclick = generateOutline;
    $('btnNewProject').onclick = () => { const name=$('newProjectName').value.trim(); if(!name)return showStatus('请填写项目名称',true); const p=C.newProject(name); state.projects.unshift(p); state.currentId=p.id; $('newProjectName').value=''; saveState(); loadProjectToUI(); showStatus('已新建项目 '+name); };
    $('btnRenameProject').onclick = () => { const name=$('renameProjectName').value.trim(); if(!name)return showStatus('项目名称不能为空',true); project().name=name; markUpdated(project()); saveState(); refreshProjectSelects(); showStatus('项目已重命名为 '+name); };
    $('btnDeleteProject').onclick = () => { if(state.projects.length<=1)return showStatus('至少保留一个小说项目',true); if(!confirm(`确定删除整个项目「${project().name}」吗？`))return; state.projects=state.projects.filter(p=>p.id!==state.currentId); state.currentId=state.projects[0].id; saveState(); loadProjectToUI(); showStatus('项目已删除'); };
    $('btnRenameChapter').onclick = () => { if(selectedChapterNo==null)return showStatus('请先选择章节',true); if(!C.renameChapter(project(),selectedChapterNo,$('chapterTitle').value))return showStatus('章节标题不能为空',true); saveState(); refreshChapterList(); showStatus('章节标题已修改'); };
    $('btnSaveChapterEdit').onclick = () => { if(selectedChapterNo==null)return showStatus('请先选择章节',true); try{C.upsertChapter(project(),selectedChapterNo,$('chapterTitle').value,$('chapterBody').value);saveState();refreshChapterList();refreshProjectSelects();showStatus('章节修改已保存');}catch(e){showStatus(e.message,true);} };
    $('btnDeleteChapter').onclick = () => { if(selectedChapterNo==null)return showStatus('请先选择章节',true); const c=project().chapters.find(x=>Number(x.no)===Number(selectedChapterNo)); if(!c)return; if(!confirm(`确定删除「${c.title}」吗？`))return; C.deleteChapter(project(),selectedChapterNo);selectedChapterNo=null;saveState();refreshChapterList();refreshProjectSelects();showStatus('章节已删除'); };
    $('btnAsContext').onclick = () => { const c=project().chapters.find(x=>Number(x.no)===Number(selectedChapterNo)); if(!c)return showStatus('请先选择章节',true); $('homeContext').value=c.body;nav('home');showStatus('已将所选章节放入续写上下文'); };
    $('btnExport').onclick = exportNovel;
    $('btnShareNovel').onclick = () => { const t=C.fullNovelText(project()); if(!t)return showStatus('当前没有已保存章节',true); if(window.Native&&Native.shareText)Native.shareText(project().name,t); };
    $('btnShowKey').onclick = () => { const i=$('apiKey'); const show=i.type==='password';i.type=show?'text':'password';$('btnShowKey').textContent=show?'隐藏':'显示'; };
    $('btnSaveSettings').onclick = () => saveSettings(true); $('btnTestApi').onclick = testApi;
    $('btnClearKey').onclick = () => { if(!confirm('确定清除本机保存的 API Key 吗？'))return; $('apiKey').value=''; if(window.Native&&Native.clearApiKey)Native.clearApiKey();showStatus('本机 API Key 已清除'); };
  }

  document.addEventListener('DOMContentLoaded', () => {
    state = loadState(); bind(); loadSettingsUI(); loadProjectToUI();
    if (window.Native && Native.reportReady) Native.reportReady();
  });
})();
