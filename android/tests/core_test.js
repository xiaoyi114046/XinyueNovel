'use strict';
const assert = require('assert');
const C = require('../app/src/main/assets/core.js');

(function testDefaults(){
  const p=C.newProject('书'); assert.equal(p.name,'书'); assert.equal(p.genre,'玄幻'); assert.equal(p.flow,'幕后流'); assert.equal(p.style,'情感细腻风');
})();
(function testCrud(){
  const p=C.newProject('书'); C.upsertChapter(p,1,'第一章：开始','正文一'); C.upsertChapter(p,2,'第二章：继续','正文二');
  assert(C.renameChapter(p,2,'第二章：新标题')); C.upsertChapter(p,2,'第二章：新标题','新正文');
  const txt=C.fullNovelText(p); assert(txt.includes('第一章：开始')); assert(txt.includes('新正文')); assert(C.deleteChapter(p,1)); assert.equal(p.chapters.length,1);
})();
(function testRecent(){
  const p=C.newProject('书'); for(let i=1;i<=5;i++)C.upsertChapter(p,i,`第${i}章`,`body${i}`); const s=C.recentContext(p,3); assert(!s.includes('body2')); assert(s.includes('body3')&&s.includes('body5'));
})();
(function testPrompt(){
  const p=C.newProject('测试'); p.style='情感细腻风'; p.flow='幕后流'; p.characters='男女主关系复杂';
  const r=C.buildNovelPrompt({project:p,chapterTitle:'第1章',context:'前文',targetChars:1200}); assert(r.system.includes('专业中文网络小说作者')); assert(r.user.includes('微表情')); assert(r.user.includes('关系张力')); assert(r.user.includes('幕后'));
})();
(function testMaxTokens(){ assert.equal(C.maxTokensForTarget(100),1024); assert.equal(C.maxTokensForTarget(3500),7000); assert.equal(C.maxTokensForTarget(20000),24000); })();
(function testFilename(){ const s=C.safeFileName(' 妈妈:的/性*教育?<>|. '); assert(!/[\\/:*?"<>|]/.test(s)); assert(s.length>0); })();
(function testPayload(){ const p=C.chatPayload({model:'deepseek-chat',thinking:'High'},'s','u',2048); assert.equal(p.model,'deepseek-chat'); assert.equal(p.thinking.type,'enabled'); assert.equal(p.thinking.effort,'high'); assert.equal(p.max_tokens,2048); })();
console.log('Android JS core tests: PASS');
