const assert = require('assert');
const C = require('../app/src/main/assets/core.js');

const required = ['deepseek','doubao','zhipu','qwen','openai','gemini','grok','claude','custom'];
for (const id of required) assert(C.PROVIDERS[id], `missing provider ${id}`);

assert.equal(C.PROVIDERS.deepseek.model, 'deepseek-flash');
assert.equal(C.PROVIDERS.doubao.type, 'openai-responses');
assert(C.PROVIDERS.doubao.endpoint.endsWith('/responses'));
assert.equal(C.PROVIDERS.gemini.model, 'gemini-3.8-flash');

let p = C.createProject('测试');
assert.equal(p.chapters.length, 1);
const first = p.chapters[0].id;
p.chapters.push(C.createChapter(2));
p.currentChapterId = first;
let r = C.deleteCurrentChapter(p);
assert(r.ok);
assert.equal(p.chapters.length, 1);
assert.notEqual(p.currentChapterId, first);

p.currentChapterId = p.chapters[0].id;
r = C.deleteCurrentChapter(p);
assert(r.ok);
assert.equal(p.chapters.length, 1, 'deleting last chapter should leave a new blank chapter');
assert(p.currentChapterId);

p.world = '玄幻世界'; p.masterOutline = '主角成长';
const sys = C.buildSystemPrompt(p);
assert(sys.includes('玄幻世界'));

const cfg = C.providerConfig({openai:{model:'my-model'}}, 'openai');
assert.equal(cfg.model, 'my-model');
assert(cfg.endpoint.includes('openai.com'));

console.log('core_test: PASS');
