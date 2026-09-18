(function(root, factory) {
  const api = factory();
  if (typeof module !== 'undefined' && module.exports) module.exports = api;
  root.XinyueCore = api;
})(typeof globalThis !== 'undefined' ? globalThis : this, function() {
  const PROVIDERS = {
    deepseek: {
      id: 'deepseek', name: 'DeepSeek', type: 'openai',
      endpoint: 'https://api.deepseek.com/chat/completions', model: 'deepseek-flash',
      help: 'DeepSeek 官方 OpenAI 兼容接口；默认 deepseek-flash，也可改为你账号可用的模型。'
    },
    doubao: {
      id: 'doubao', name: '豆包 / 火山方舟', type: 'openai-responses',
      endpoint: 'https://ark.cn-beijing.volces.com/api/v3/responses', model: 'doubao-seed-2-0-lite-260215',
      help: '火山方舟 Responses API；模型可改为你的推理接入点 ID 或账号中已开通的豆包模型。'
    },
    zhipu: {
      id: 'zhipu', name: '智谱 GLM', type: 'openai',
      endpoint: 'https://open.bigmodel.cn/api/paas/v4/chat/completions', model: 'glm-5.2',
      help: '智谱 BigModel OpenAI 兼容接口；若账号尚未开放 GLM-5.2，可改成控制台中可用的 GLM 模型。'
    },
    qwen: {
      id: 'qwen', name: '通义千问', type: 'openai',
      endpoint: 'https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions', model: 'qwen-plus',
      help: '阿里云百炼 DashScope OpenAI 兼容接口。'
    },
    openai: {
      id: 'openai', name: 'GPT / OpenAI', type: 'openai-responses',
      endpoint: 'https://api.openai.com/v1/responses', model: 'gpt-5.6-terra',
      help: 'OpenAI Responses API；默认 GPT-5.6 Terra，模型名称可按你的账号权限修改。'
    },
    gemini: {
      id: 'gemini', name: 'Gemini', type: 'gemini',
      endpoint: 'https://generativelanguage.googleapis.com/v1beta/models/{model}:generateContent', model: 'gemini-3.8-flash',
      help: 'Google Gemini generateContent 接口。'
    },
    grok: {
      id: 'grok', name: 'Grok / xAI', type: 'openai',
      endpoint: 'https://api.x.ai/v1/chat/completions', model: 'grok-4.6',
      help: 'xAI OpenAI 兼容接口。'
    },
    claude: {
      id: 'claude', name: 'Claude / Anthropic', type: 'claude',
      endpoint: 'https://api.anthropic.com/v1/messages', model: 'claude-sonnet-4-6',
      help: 'Anthropic Messages API。'
    },
    custom: {
      id: 'custom', name: '自定义 OpenAI 兼容', type: 'openai',
      endpoint: '', model: '',
      help: '适用于任何 OpenAI Chat Completions 兼容接口。'
    }
  };

  function deepClone(v) { return JSON.parse(JSON.stringify(v)); }

  function createChapter(n) {
    return {
      id: 'ch_' + Date.now().toString(36) + '_' + Math.random().toString(36).slice(2, 8),
      title: '第' + n + '章',
      outline: '',
      content: '',
      createdAt: Date.now(), updatedAt: Date.now()
    };
  }

  function createProject(name) {
    const ch = createChapter(1);
    return {
      id: 'p_' + Date.now().toString(36) + '_' + Math.random().toString(36).slice(2, 8),
      name: name || '未命名小说',
      aiProvider: 'deepseek',
      genre: '', webGenre: '', style: '', pov: '第三人称', pace: '', protagonist: '', relations: '', plot: '',
      world: '', realms: '', currentRealm: '', masterOutline: '', volumeOutline: '',
      chapters: [ch], currentChapterId: ch.id,
      createdAt: Date.now(), updatedAt: Date.now()
    };
  }

  function normalizeProject(p) {
    p = p && typeof p === 'object' ? p : createProject();
    p.id = p.id || ('p_' + Date.now());
    p.name = p.name || p.title || '未命名小说';
    p.aiProvider = PROVIDERS[p.aiProvider] ? p.aiProvider : 'deepseek';
    p.chapters = Array.isArray(p.chapters) ? p.chapters : [];
    p.chapters = p.chapters.map((c, i) => ({
      id: c.id || ('ch_m_' + i + '_' + Date.now()),
      title: c.title || ('第' + (i + 1) + '章'),
      outline: c.outline || c.chapterOutline || '',
      content: c.content || c.text || '',
      createdAt: c.createdAt || Date.now(), updatedAt: c.updatedAt || Date.now()
    }));
    if (!p.chapters.length) p.chapters.push(createChapter(1));
    if (!p.currentChapterId || !p.chapters.some(c => c.id === p.currentChapterId)) {
      p.currentChapterId = p.chapters[0].id;
    }
    ['genre','webGenre','style','pov','pace','protagonist','relations','plot','world','realms','currentRealm','masterOutline','volumeOutline']
      .forEach(k => { if (typeof p[k] !== 'string') p[k] = ''; });
    return p;
  }

  function deleteCurrentChapter(project) {
    project = normalizeProject(project);
    const idx = project.chapters.findIndex(c => c.id === project.currentChapterId);
    if (idx < 0) return { ok: false, reason: '找不到当前章节', project };
    const deleted = project.chapters[idx];
    project.chapters.splice(idx, 1);
    if (!project.chapters.length) {
      const ch = createChapter(1);
      project.chapters.push(ch);
      project.currentChapterId = ch.id;
    } else {
      const nextIndex = Math.min(idx, project.chapters.length - 1);
      project.currentChapterId = project.chapters[nextIndex].id;
    }
    project.updatedAt = Date.now();
    return { ok: true, deleted, project };
  }

  function providerConfig(saved, providerId) {
    const preset = PROVIDERS[providerId] || PROVIDERS.custom;
    const custom = (saved && saved[providerId]) || {};
    return Object.assign({}, preset, custom, { id: providerId });
  }

  function buildSystemPrompt(project) {
    project = normalizeProject(project);
    const lines = [
      '你是一名专业中文网文作者。请严格延续既有设定、人物性格与剧情逻辑，避免总结式写法。',
      project.genre && ('小说类型：' + project.genre),
      project.webGenre && ('网文流派：' + project.webGenre),
      project.style && ('文风：' + project.style),
      project.pov && ('叙事视角：' + project.pov),
      project.pace && ('节奏：' + project.pace),
      project.protagonist && ('主角设定：' + project.protagonist),
      project.relations && ('人物关系：' + project.relations),
      project.plot && ('剧情脉络：' + project.plot),
      project.world && ('世界观：' + project.world),
      project.realms && ('境界体系：' + project.realms),
      project.currentRealm && ('当前境界：' + project.currentRealm),
      project.masterOutline && ('总纲：' + project.masterOutline),
      project.volumeOutline && ('卷纲：' + project.volumeOutline)
    ].filter(Boolean);
    return lines.join('\n');
  }

  function recentContext(project, count) {
    project = normalizeProject(project);
    const idx = project.chapters.findIndex(c => c.id === project.currentChapterId);
    const end = idx >= 0 ? idx : project.chapters.length;
    const start = Math.max(0, end - Math.max(0, count || 0));
    return project.chapters.slice(start, end)
      .filter(c => c.content)
      .map(c => '【' + c.title + '】\n' + c.content)
      .join('\n\n');
  }

  return { PROVIDERS, createChapter, createProject, normalizeProject, deleteCurrentChapter, providerConfig, buildSystemPrompt, recentContext, deepClone };
});
