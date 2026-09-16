from pathlib import Path
import re

root = Path(__file__).resolve().parents[1]
html = (root/'app/src/main/assets/index.html').read_text('utf-8')
js = (root/'app/src/main/assets/app.js').read_text('utf-8')
bridge = (root/'app/src/main/java/com/xiaoyi/xinyuenovel/NativeBridge.java').read_text('utf-8')
manifest = (root/'app/src/main/AndroidManifest.xml').read_text('utf-8')

ids = set(re.findall(r'\bid=["\']([^"\']+)["\']', html))
refs = set(re.findall(r"\$\('([^']+)'\)", js)) | set(re.findall(r'\$\("([^\"]+)"\)', js))
missing = sorted(refs - ids)
assert not missing, f'JS references missing HTML ids: {missing}'

# Every direct Native.method reference in JS should have a @JavascriptInterface method.
native_refs = set(re.findall(r'\bNative\.([A-Za-z_]\w*)', js))
bridge_methods = set(re.findall(r'@JavascriptInterface\s+public\s+[\w<>\[\]]+\s+([A-Za-z_]\w*)\s*\(', bridge))
missing_native = sorted(native_refs - bridge_methods)
assert not missing_native, f'JS references missing native bridge methods: {missing_native}'

assert 'android.permission.INTERNET' in manifest
assert 'android:name=".MainActivity"' in manifest
assert 'android:exported="true"' in manifest
assert 'file:///android_asset/index.html' in (root/'app/src/main/java/com/xiaoyi/xinyuenovel/MainActivity.java').read_text('utf-8')
assert '<script src="core.js"></script><script src="app.js"></script>' in html
assert 'http://' not in html and 'https://' not in html, 'HTML should not load remote content'
assert 'eval(' not in js, 'Avoid dynamic eval in WebView app code'
print(f'Static Android app checks: PASS ({len(ids)} UI ids, {len(native_refs)} native methods)')
