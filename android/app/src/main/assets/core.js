(function (root, factory) {
  const api = factory();
  if (typeof module !== 'undefined' && module.exports) module.exports = api;
  root.XinyueCore = api;
})(typeof globalThis !== 'undefined' ? globalThis : this, function () {
  'use strict';

  const APP_VERSION = '4.1.3-android.1';
  const DEFAULT_ENDPOINT = 'https://api.deepseek.com/chat/completions';

  function nowISO() { return new Date().toISOString(); }
  function trim(s) { return (s == null ? '' : String(s)).trim(); }
  function uid() { return Date.now().toString(36) + '_' + Math.random().toString(36).slice(2, 10); }

  function newProject(name) {
    const now = nowISO();
    return {
      id: uid(), name: trim(name) || '我的小说', genre: '玄幻', flow: '幕后流', style: '情感细腻风',
      pov: '第三人称', pace: '张弛有度', protagonist: '', characters: '', plot: '', world: '', realms: '',
      current_realm: '', outline: '', volume_outline: '', chapter_outline: '', draft: '', chapters: [],
      created_at: now, updated_at: now
    };
  }

  function normalizeProject(p) {
    const base = newProject((p && p.name) || '我的小说');
    const out = Object.assign(base, p || {});
    out.id = trim(out.id) || uid();
    out.name = trim(out.name) || '我的小说';
    out.chapters = Array.isArray(out.chapters) ? out.chapters.map(c => ({
      no: Number(c.no) || 0,
      title: trim(c.title),
      body: c.body == null ? '' : String(c.body),
      created_at: c.created_at || nowISO(),
      updated_at: c.updated_at || c.created_at || nowISO()
    })) : [];
    sortChapters(out.chapters);
    return out;
  }

  function sortChapters(chapters) { chapters.sort((a, b) => (Number(a.no) || 0) - (Number(b.no) || 0)); return chapters; }
  function nextChapterNo(p) { return (p.chapters || []).reduce((m, c) => Math.max(m, Number(c.no) || 0), 0) + 1; }

  function upsertChapter(p, no, title, body) {
    if (!p) throw new Error('项目为空');
    body = trim(body);
    if (!body) throw new Error('章节正文为空');
    no = Number(no) || nextChapterNo(p);
    title = trim(title) || `第${no}章`;
    const now = nowISO();
    const existing = (p.chapters || []).find(c => Number(c.no) === no);
    if (existing) {
      existing.title = title; existing.body = body; existing.updated_at = now;
    } else {
      if (!Array.isArray(p.chapters)) p.chapters = [];
      p.chapters.push({ no, title, body, created_at: now, updated_at: now });
    }
    p.updated_at = now;
    sortChapters(p.chapters);
    return p;
  }

  function deleteChapter(p, no) {
    if (!p || !Array.isArray(p.chapters)) return false;
    const before = p.chapters.length;
    p.chapters = p.chapters.filter(c => Number(c.no) !== Number(no));
    if (p.chapters.length !== before) { p.updated_at = nowISO(); return true; }
    return false;
  }

  function renameChapter(p, no, title) {
    title = trim(title);
    if (!title || !p) return false;
    const c = (p.chapters || []).find(x => Number(x.no) === Number(no));
    if (!c) return false;
    c.title = title; c.updated_at = nowISO(); p.updated_at = nowISO(); return true;
  }

  function recentContext(p, n) {
    n = Number(n) || 3;
    const ch = [...(p.chapters || [])]; sortChapters(ch);
    return ch.slice(Math.max(0, ch.length - n)).map(c => `${trim(c.title)}\n\n${trim(c.body)}`).join('\n\n').trim();
  }

  function fullNovelText(p) {
    const ch = [...(p.chapters || [])]; sortChapters(ch);
    if (!ch.length) return '';
    return ch.map(c => `${trim(c.title)}\n\n${trim(c.body)}`).join('\n\n') + '\n';
  }

  function safeFileName(s) {
    let out = trim(s) || '小说';
    out = out.replace(/[\\/:*?"<>|\u0000-\u001f]/g, '_').replace(/[. ]+$/g, '').trim();
    return out || '小说';
  }

  function styleGuidance(style) {
    switch (trim(style)) {
      case '情感细腻风': return '情感细腻风：重点描写人物细微心理波动、微表情、肢体反应、欲言又止和关系张力；情绪要有铺垫与递进，避免直白喊口号；在关键情绪节点用环境、动作和感官细节承托，让情感真实、克制又有余韵。';
      case '宏大玄幻·古意磅礴': return '宏大玄幻：世界尺度大，场景有史诗感，语言凝练有古意，战斗与天地异象具有层次感。';
      case '稳健升级·情感清晰': return '稳健升级：成长线清楚，冲突与收获形成闭环，人物情感关系明确而不过度拖沓。';
      case '快节奏爽文·强钩子': return '快节奏爽文：开篇迅速进入事件，每个场景都有目标、阻力和结果，段尾尽量留下悬念或爽点。';
      case '轻松吐槽·反差喜剧': return '轻松反差：人物对话自然诙谐，利用身份差、认知差制造笑点，但不破坏主线推进。';
      default: return trim(style) || '自然流畅，人物口吻稳定，避免模板化表达。';
    }
  }

  function flowGuidance(flow) {
    switch (trim(flow)) {
      case '迪化流': return '迪化流：主角行为常被他人过度解读，误会逐层升级但逻辑自洽，读者能同时看到真实动机与旁人脑补。';
      case '幕后流': return '幕后流：主角通过组织、化身、布局或信息差在幕后推动事件，逐步展现影响力，并保留身份与底牌悬念。';
      case '系统流': return '系统流：系统规则清楚，奖励与限制有因果，避免机械报数，重点写选择带来的剧情变化。';
      case '苟道流': return '苟道流：主角重视风险控制与隐藏实力，爽点来自稳健布局、信息优势和关键时刻的爆发。';
      case '无敌流': return '无敌流：实力优势明确，冲突重点放在身份、格局、关系和未知规则，而不是重复碾压。';
      default: return trim(flow);
    }
  }

  function buildNovelPrompt(r) {
    const p = r.project || {};
    const target = Number(r.targetChars) > 0 ? Number(r.targetChars) : 3500;
    const system = '你是一名专业中文网络小说作者。请根据项目设定和上下文直接续写正文。\n要求：只输出小说正文；严格保持人物、境界、世界观、时间线与既有事实一致；从上下文末尾自然承接；不要复述设定，不要解释写作过程，不要突然完结。';
    const user = `【项目】${p.name || ''}\n【类型】${p.genre || ''}\n【流派】${p.flow || ''}\n【文风】${p.style || ''}\n【叙事视角】${p.pov || ''}\n【节奏】${p.pace || ''}\n` +
      `【流派执行】${flowGuidance(p.flow)}\n【文风执行】${styleGuidance(p.style)}\n` +
      `【主角】${p.protagonist || ''}\n【人物关系】\n${p.characters || ''}\n\n【剧情脉络】\n${p.plot || ''}\n\n【世界观】\n${p.world || ''}\n\n` +
      `【境界体系】\n${p.realms || ''}\n【当前境界】${p.current_realm || ''}\n\n【总纲/卷纲】\n${p.outline || ''}\n${p.volume_outline || ''}\n\n` +
      `【本章小纲】\n${p.chapter_outline || ''}\n\n【本章标题】${r.chapterTitle || ''}\n\n【本章特殊要求】\n${r.instructions || ''}\n\n` +
      `【续写上下文】\n${r.context || ''}\n\n本次目标约 ${target} 个中文字符。内容完整优先，可适度浮动。现在直接续写正文：`;
    return { system, user, target };
  }

  function thinkingPayload(mode) {
    switch (trim(mode).toLowerCase()) {
      case 'low': return { type: 'enabled', effort: 'low' };
      case 'high': return { type: 'enabled', effort: 'high' };
      case 'max': return { type: 'enabled', effort: 'max' };
      case '开启': case 'enabled': return { type: 'enabled' };
      default: return { type: 'disabled' };
    }
  }

  function maxTokensForTarget(n) {
    n = Number(n) > 0 ? Number(n) : 3500;
    return Math.min(24000, Math.max(1024, n * 2));
  }

  function chatPayload(settings, system, user, maxTokens) {
    settings = settings || {};
    return {
      model: trim(settings.model) || 'deepseek-chat',
      messages: [{ role: 'system', content: system }, { role: 'user', content: user }],
      temperature: 0.82,
      max_tokens: maxTokens,
      stream: false,
      thinking: thinkingPayload(settings.thinking)
    };
  }

  function outlinePrompt(p) {
    return {
      system: '你是中文网文策划编辑。请根据用户设定输出可直接用于写作的总纲与卷纲，结构清晰、冲突递进、避免空泛。',
      user: `项目：${p.name || ''}\n类型：${p.genre || ''}\n流派：${p.flow || ''}\n文风：${p.style || ''}\n主角：${p.protagonist || ''}\n剧情脉络：${p.plot || ''}\n世界观：${p.world || ''}\n境界：${p.realms || ''}\n请输出总纲、主要人物弧线、核心冲突、分卷规划与关键转折。`
    };
  }

  function realmsPrompt(p) {
    return {
      system: '你是玄幻/仙侠世界观设计师。请设计简洁、递进清晰、能支撑长期网文升级的境界体系。',
      user: `小说类型：${p.genre || ''}\n流派：${p.flow || ''}\n世界观：${p.world || ''}\n剧情脉络：${p.plot || ''}\n请生成境界名称、每境界核心变化、寿命/战力或能力边界，以及跨境界差异。`
    };
  }

  return { APP_VERSION, DEFAULT_ENDPOINT, newProject, normalizeProject, sortChapters, nextChapterNo, upsertChapter,
    deleteChapter, renameChapter, recentContext, fullNovelText, safeFileName, styleGuidance, flowGuidance,
    buildNovelPrompt, thinkingPayload, maxTokensForTarget, chatPayload, outlinePrompt, realmsPrompt };
});
