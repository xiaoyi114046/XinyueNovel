(() => {
  'use strict';
  const C = window.XinyueCore;
  const STATE_KEY = 'xinyue_state_v6';
  const PROVIDER_KEY = 'xinyue_provider_configs_v6';
  const $ = id => document.getElementById(id);
  const qsa = sel => Array.from(document.querySelectorAll(sel));

  let state = loadState();
  let providerConfigs = loadProviderConfigs();
  let activeRequestId = '';
  let activeRequestKind = '';

  function android() { return typeof window.Android !== 'undefined' ? window.Android : null; }
  function nowId(prefix) { return prefix + '_' + Date.now().toString(36) + '_' + Math.random().toString(36).slice(2,7); }
  function safeParse(v, fallback) { try { return JSON.parse(v); } catch (_) { return fallback; } }
  function escapeHtml(s) { return String(s ?? '').replace(/[&<>"']/g, m => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[m])); }

  function loadState() {
    const existing = safeParse(localStorage.getItem(STATE_KEY), null);
    if (existing && Array.isArray(existing.projects)) return normalizeState(existing);

    // 尽量迁移旧版：扫描 localStorage，寻找包含 projects 数组的对象。
    for (let i = 0; i < localStorage.length; i++) {
      const key = localStorage.key(i);
      if (!key || key === PROVIDER_KEY) continue;
      const candidate = safeParse(localStorage.getItem(key), null);
      if (candidate && Array.isArray(candidate.projects) && candidate.projects.length) {
        const migrated = normalizeState(candidate);
        localStorage.setItem(STATE_KEY, JSON.stringify(migrated));
        return migrated;
      }
    }
    const p = C.createProject('我的小说');
    return { projects: [p], currentProjectId: p.id, schemaVersion: 6 };
  }

  function normalizeState(s) {
    const projects = (s.projects || []).map(C.normalizeProject);
    if (!projects.length) projects.push(C.createProject('我的小说'));
    let currentProjectId = s.currentProjectId || s.activeProjectId || projects[0].id;
    if (!projects.some(p => p.id === currentProjectId)) currentProjectId = projects[0].id;
    return { projects, currentProjectId, schemaVersion: 6 };
  }

  function loadProviderConfigs() {
    const saved = safeParse(localStorage.getItem(PROVIDER_KEY), {});
    const out = {};
    Object.keys(C.PROVIDERS).forEach(id => { out[id] = C.providerConfig(saved, id); });
    return out;
  }

  function saveState() {
    state.schemaVersion = 6;
    localStorage.setItem(STATE_KEY, JSON.stringify(state));
  }
  function saveProviderConfigs() {
    localStorage.setItem(PROVIDER_KEY, JSON.stringify(providerConfigs));
  }
  function project() { return state.projects.find(p => p.id === state.currentProjectId) || state.projects[0]; }
  function chapter() { const p = project(); return p.chapters.find(c => c.id === p.currentChapterId) || p.chapters[0]; }

  function syncEditorToState() {
    const p = project(), c = chapter();
    if (!p || !c) return;
    c.title = $('chapterTitle').value.trim() || c.title || '未命名章节';
    c.outline = $('chapterOutline').value;
    c.content = $('chapterContent').value;
    c.updatedAt = Date.now();
    p.aiProvider = $('writeProvider').value || p.aiProvider || 'deepseek';
    qsa('[data-project-field]').forEach(el => { p[el.dataset.projectField] = el.value; });
    p.updatedAt = Date.now();
  }

  function renderAll() {
    renderProviderOptions();
    renderProjectOptions();
    renderProjectFields();
    renderChapter();
    renderChapterList();
    renderProviderSettings();
    $('projectBadge').textContent = project().name + ' · ' + chapter().title;
  }

  function renderProviderOptions() {
    const ids = Object.keys(C.PROVIDERS);
    const html = ids.map(id => `<option value="${id}">${escapeHtml(C.PROVIDERS[id].name)}</option>`).join('');
    const currentWrite = project().aiProvider || 'deepseek';
    $('writeProvider').innerHTML = html;
    $('writeProvider').value = C.PROVIDERS[currentWrite] ? currentWrite : 'deepseek';
    const currentSetting = $('providerSelect').value || currentWrite;
    $('providerSelect').innerHTML = html;
    $('providerSelect').value = C.PROVIDERS[currentSetting] ? currentSetting : 'deepseek';
    $('providerCards').innerHTML = ids.filter(id => id !== 'custom').map(id => {
      const p = C.PROVIDERS[id];
      return `<div class="provider-card"><b>${escapeHtml(p.name)}</b><span>${escapeHtml(p.model)}</span></div>`;
    }).join('');
  }

  function renderProjectOptions() {
    $('projectSelect').innerHTML = state.projects.map(p => `<option value="${p.id}">${escapeHtml(p.name)}</option>`).join('');
    $('projectSelect').value = project().id;
  }

  function renderProjectFields() {
    const p = project();
    qsa('[data-project-field]').forEach(el => { el.value = p[el.dataset.projectField] || ''; });
  }

  function renderChapter() {
    const c = chapter();
    $('chapterTitle').value = c.title || '';
    $('chapterOutline').value = c.outline || '';
    $('chapterContent').value = c.content || '';
  }

  function renderChapterList() {
    const p = project();
    $('chapterList').innerHTML = p.chapters.map((c, i) => `
      <div class="chapter-item ${c.id === p.currentChapterId ? 'active' : ''}" data-chapter-id="${c.id}">
        <div><b>${escapeHtml(c.title)}</b><div class="chapter-meta">${(c.content || '').length} 字符</div></div>
        <span>›</span>
      </div>`).join('');
    qsa('.chapter-item').forEach(el => el.addEventListener('click', () => {
      syncEditorToState();
      p.currentChapterId = el.dataset.chapterId;
      saveState(); renderAll(); showTab('write');
    }));
  }

  function renderProviderSettings() {
    const id = $('providerSelect').value || project().aiProvider || 'deepseek';
    const cfg = providerConfigs[id] || C.providerConfig({}, id);
    $('providerEndpoint').value = cfg.endpoint || '';
    $('providerModel').value = cfg.model || '';
    $('providerHelp').textContent = cfg.help || C.PROVIDERS[id]?.help || '';
    const a = android();
    let has = false;
    try { has = !!(a && a.hasApiKey(id)); } catch (_) {}
    $('keyState').textContent = has ? '✓ 已安全保存 API Key' : '尚未保存 API Key';
    $('keyState').className = 'status ' + (has ? 'ok' : '');
    $('apiKey').value = '';
  }

  function showTab(name) {
    qsa('.tab').forEach(x => x.classList.toggle('active', x.id === 'tab-' + name));
    qsa('.bottom-nav button').forEach(x => x.classList.toggle('active', x.dataset.tab === name));
  }

  function persistAndToast(msg) {
    syncEditorToState(); saveState();
    $('aiStatus').textContent = msg || '已保存';
    $('aiStatus').className = 'status ok';
    setTimeout(() => { if ($('aiStatus').textContent === msg) $('aiStatus').textContent = ''; }, 1300);
  }

  function addChapter() {
    syncEditorToState();
    const p = project();
    const ch = C.createChapter(p.chapters.length + 1);
    p.chapters.push(ch); p.currentChapterId = ch.id; p.updatedAt = Date.now();
    saveState(); renderAll(); showTab('write');
  }

  function deleteCurrentChapter() {
    syncEditorToState();
    const p = project(), c = chapter();
    if (!c) return alert('当前没有可删除章节');
    if (!confirm(`确定删除“${c.title}”吗？\n删除后无法恢复。`)) return;
    const result = C.deleteCurrentChapter(p);
    if (!result.ok) return alert(result.reason || '删除失败');
    saveState();
    renderAll();
    $('aiStatus').textContent = `已删除：${result.deleted.title}`;
    $('aiStatus').className = 'status ok';
  }

  function buildWritePrompt(kind) {
    syncEditorToState();
    const p = project(), c = chapter();
    if (kind === 'realm') {
      return `请根据以下小说设定设计完整、清晰、有递进感的境界/力量体系。\n小说类型：${p.genre}\n世界观：${p.world}\n主角：${p.protagonist}\n请直接输出可粘贴到“境界体系”的正文。`;
    }
    if (kind === 'outline') {
      return `请根据以下信息生成小说总纲与当前卷纲，包含核心矛盾、阶段目标、反转和伏笔。\n类型：${p.genre}\n流派：${p.webGenre}\n世界观：${p.world}\n主角：${p.protagonist}\n剧情脉络：${p.plot}\n请用“【总纲】”和“【卷纲】”分段输出。`;
    }
    const context = C.recentContext(p, parseInt($('contextCount').value || '2', 10));
    return [
      `请续写当前章节“${c.title}”。`,
      `本章小纲：${c.outline || '无'}`,
      `目标约 ${parseInt($('targetWords').value || '2200', 10)} 字。`,
      $('extraPrompt').value && ('特殊要求：' + $('extraPrompt').value),
      context && ('最近章节正文：\n' + context),
      c.content && ('当前章节已有正文（从末尾继续，勿重复）：\n' + c.content),
      '只输出续写正文，不要解释，不要使用“以下是续写”等前缀。'
    ].filter(Boolean).join('\n\n');
  }

  function startAi(kind) {
    if (activeRequestId) return;
    syncEditorToState(); saveState();
    const p = project();
    const providerId = p.aiProvider || $('writeProvider').value || 'deepseek';
    const cfg = providerConfigs[providerId] || C.providerConfig({}, providerId);
    const a = android();
    if (!a) return setAiError('当前不是 Android 原生环境，无法发起 API 请求');
    try {
      if (!a.hasApiKey(providerId)) return setAiError(`请先在“AI 接口”中保存 ${C.PROVIDERS[providerId]?.name || providerId} 的 API Key`);
    } catch (_) {}
    if (!cfg.endpoint || !cfg.model) return setAiError('请先设置接口地址和模型');

    activeRequestId = nowId(kind);
    activeRequestKind = kind;
    $('generateBtn').disabled = true; $('stopBtn').classList.remove('hidden');
    $('aiStatus').textContent = `${C.PROVIDERS[providerId]?.name || providerId} 正在生成……`;
    $('aiStatus').className = 'status';
    const maxTokens = kind === 'write' ? Math.max(512, Math.ceil(parseInt($('targetWords').value || '2200', 10) * 1.8)) : 4096;
    const system = C.buildSystemPrompt(p);
    const user = buildWritePrompt(kind);
    try {
      a.aiRequest(activeRequestId, providerId, cfg.endpoint, cfg.model, system, user,
        parseFloat($('temperature').value || '0.8'), maxTokens);
    } catch (e) {
      finishAi(); setAiError(String(e));
    }
  }

  window.onNativeAiResult = function(requestId, ok, payload) {
    if (requestId !== activeRequestId && !requestId.startsWith('test_')) return;
    if (requestId.startsWith('test_')) {
      $('providerTestStatus').textContent = ok ? ('✓ 连接成功：' + payload.slice(0,80)) : ('连接失败：' + payload);
      $('providerTestStatus').className = 'status ' + (ok ? 'ok' : 'err');
      return;
    }
    const kind = activeRequestKind;
    finishAi();
    if (!ok) return setAiError(payload);
    const p = project();
    if (kind === 'write') {
      const c = chapter();
      c.content = [c.content.trim(), payload.trim()].filter(Boolean).join('\n\n');
      c.updatedAt = Date.now();
      $('chapterContent').value = c.content;
    } else if (kind === 'realm') {
      p.realms = payload.trim();
      const el = document.querySelector('[data-project-field="realms"]'); if (el) el.value = p.realms;
    } else if (kind === 'outline') {
      const text = payload.trim();
      const m = text.match(/【总纲】([\s\S]*?)(?:【卷纲】|$)/);
      const v = text.match(/【卷纲】([\s\S]*)/);
      p.masterOutline = (m ? m[1] : text).trim();
      p.volumeOutline = (v ? v[1] : p.volumeOutline).trim();
      renderProjectFields();
    }
    saveState();
    $('aiStatus').textContent = '生成完成并已保存';
    $('aiStatus').className = 'status ok';
  };

  function finishAi() {
    activeRequestId = ''; activeRequestKind = '';
    $('generateBtn').disabled = false; $('stopBtn').classList.add('hidden');
  }
  function setAiError(msg) { $('aiStatus').textContent = msg; $('aiStatus').className = 'status err'; }

  function stopAi() {
    if (!activeRequestId) return;
    try { android()?.cancelRequest(activeRequestId); } catch (_) {}
    finishAi(); $('aiStatus').textContent = '已停止生成'; $('aiStatus').className = 'status';
  }

  function fullNovelText() {
    syncEditorToState();
    const p = project();
    return `${p.name}\n\n` + p.chapters.map(c => `${c.title}\n\n${c.content || ''}`).join('\n\n\n');
  }

  function bind() {
    qsa('.bottom-nav button').forEach(b => b.addEventListener('click', () => showTab(b.dataset.tab)));
    $('saveAllBtn').addEventListener('click', () => persistAndToast('已保存'));
    $('saveChapterBtn').addEventListener('click', () => persistAndToast('本章已保存'));
    $('newChapterBtn').addEventListener('click', addChapter);
    $('projectNewChapterBtn').addEventListener('click', addChapter);
    $('deleteChapterBtn').addEventListener('click', deleteCurrentChapter);
    $('renameChapterBtn').addEventListener('click', () => {
      syncEditorToState(); const c = chapter(); const n = prompt('新的章节标题', c.title); if (!n?.trim()) return;
      c.title = n.trim(); saveState(); renderAll();
    });
    $('projectSelect').addEventListener('change', e => { syncEditorToState(); state.currentProjectId = e.target.value; saveState(); renderAll(); });
    $('newProjectBtn').addEventListener('click', () => {
      syncEditorToState(); const name = prompt('项目名称', '新小说'); if (!name?.trim()) return;
      const p = C.createProject(name.trim()); state.projects.push(p); state.currentProjectId = p.id; saveState(); renderAll();
    });
    $('renameProjectBtn').addEventListener('click', () => {
      const p = project(); const name = prompt('新的项目名称', p.name); if (!name?.trim()) return;
      p.name = name.trim(); saveState(); renderAll();
    });
    $('deleteProjectBtn').addEventListener('click', () => {
      if (state.projects.length <= 1) return alert('至少保留一个项目');
      const p = project(); if (!confirm(`确定删除项目“${p.name}”及其全部章节吗？`)) return;
      state.projects = state.projects.filter(x => x.id !== p.id); state.currentProjectId = state.projects[0].id; saveState(); renderAll();
    });
    $('writeProvider').addEventListener('change', e => { project().aiProvider = e.target.value; saveState(); });
    $('generateBtn').addEventListener('click', () => startAi('write'));
    $('stopBtn').addEventListener('click', stopAi);
    $('aiRealmBtn').addEventListener('click', () => startAi('realm'));
    $('aiOutlineBtn').addEventListener('click', () => startAi('outline'));

    $('providerSelect').addEventListener('change', renderProviderSettings);
    $('saveProviderBtn').addEventListener('click', () => {
      const id = $('providerSelect').value;
      providerConfigs[id] = Object.assign({}, providerConfigs[id] || C.PROVIDERS[id], {
        endpoint: $('providerEndpoint').value.trim(), model: $('providerModel').value.trim()
      });
      saveProviderConfigs(); $('providerTestStatus').textContent = '接口配置已保存'; $('providerTestStatus').className = 'status ok';
    });
    $('resetProviderBtn').addEventListener('click', () => {
      const id = $('providerSelect').value; providerConfigs[id] = C.providerConfig({}, id); saveProviderConfigs(); renderProviderSettings();
    });
    $('saveKeyBtn').addEventListener('click', () => {
      const id = $('providerSelect').value, key = $('apiKey').value.trim();
      if (!key) return alert('请输入 API Key');
      const a = android(); if (!a) return alert('当前不是 Android 原生环境');
      const result = a.saveApiKey(id, key);
      $('apiKey').value = '';
      if (String(result).startsWith('ok')) renderProviderSettings(); else alert('保存失败：' + result);
    });
    $('testProviderBtn').addEventListener('click', () => {
      const id = $('providerSelect').value, cfg = providerConfigs[id];
      const a = android(); if (!a) return alert('当前不是 Android 原生环境');
      $('providerTestStatus').textContent = '正在测试连接……'; $('providerTestStatus').className = 'status';
      a.testProvider('test_' + Date.now(), id, cfg.endpoint, cfg.model);
    });
    $('exportBtn').addEventListener('click', () => { android()?.exportText(project().name + '.txt', fullNovelText()); });
    $('shareBtn').addEventListener('click', () => { android()?.shareText(project().name, fullNovelText()); });

    ['chapterTitle','chapterOutline','chapterContent'].forEach(id => $(id).addEventListener('change', () => { syncEditorToState(); saveState(); renderChapterList(); }));
    qsa('[data-project-field]').forEach(el => el.addEventListener('change', () => { syncEditorToState(); saveState(); }));
  }

  renderAll(); bind();
  setTimeout(() => { try { android()?.appReady(); } catch (_) {} }, 250);
})();
