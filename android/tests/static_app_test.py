from pathlib import Path
import re
root = Path(__file__).resolve().parents[1]
html = (root/'app/src/main/assets/index.html').read_text(encoding='utf-8')
js = (root/'app/src/main/assets/app.js').read_text(encoding='utf-8')
core = (root/'app/src/main/assets/core.js').read_text(encoding='utf-8')
java = (root/'app/src/main/java/com/xiaoyi/xinyuenovel/NativeBridge.java').read_text(encoding='utf-8')
main_java = (root/'app/src/main/java/com/xiaoyi/xinyuenovel/MainActivity.java').read_text(encoding='utf-8')

for pid in ['deepseek','doubao','zhipu','qwen','openai','gemini','grok','claude','custom']:
    assert re.search(rf'\b{pid}:\s*\{{', core), f'missing provider {pid}'

for id_ in ['deleteChapterBtn','providerSelect','providerEndpoint','providerModel','apiKey','testProviderBtn']:
    assert f'id="{id_}"' in html, f'missing UI id {id_}'
    assert id_ in js, f'unreferenced UI id {id_}'

for method in ['saveApiKey','hasApiKey','aiRequest','testProvider','cancelRequest','exportText','shareText','appReady']:
    assert re.search(r'@JavascriptInterface\s+public\s+[^\s]+\s+'+method+r'\s*\(', java), f'missing bridge {method}'

assert 'C.deleteCurrentChapter(p)' in js
assert 'project.currentChapterId = project.chapters[nextIndex].id' in core
print('static_app_test: PASS')

assert 'setWebChromeClient(new WebChromeClient())' in main_java, 'WebChromeClient required for confirm/prompt dialogs'
