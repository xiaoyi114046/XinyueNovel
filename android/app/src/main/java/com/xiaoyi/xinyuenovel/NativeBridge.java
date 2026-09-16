package com.xiaoyi.xinyuenovel;

import android.app.Activity;
import android.content.ClipData;
import android.content.ClipboardManager;
import android.content.ContentResolver;
import android.content.ContentValues;
import android.content.Context;
import android.content.Intent;
import android.content.SharedPreferences;
import android.net.Uri;
import android.os.Build;
import android.os.Environment;
import android.provider.MediaStore;
import android.security.keystore.KeyGenParameterSpec;
import android.security.keystore.KeyProperties;
import android.webkit.JavascriptInterface;
import android.webkit.WebView;

import org.json.JSONObject;

import java.io.ByteArrayOutputStream;
import java.io.File;
import java.io.FileOutputStream;
import java.io.InputStream;
import java.io.OutputStream;
import java.net.HttpURLConnection;
import java.net.URL;
import java.nio.charset.StandardCharsets;
import java.security.KeyStore;
import android.util.Base64;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;

import javax.crypto.Cipher;
import javax.crypto.KeyGenerator;
import javax.crypto.SecretKey;
import javax.crypto.spec.GCMParameterSpec;

public final class NativeBridge {
    private static final String PREFS = "xinyue_secure";
    private static final String KEY_ALIAS = "xinyue_api_key_v1";
    private static final String PREF_CIPHER = "api_key_cipher";

    private final Activity activity;
    private final WebView webView;
    private final ExecutorService executor = Executors.newSingleThreadExecutor();
    private volatile HttpURLConnection activeConnection;
    private volatile String activeRequestId;

    NativeBridge(Activity activity, WebView webView) {
        this.activity = activity;
        this.webView = webView;
    }

    @JavascriptInterface
    public String appVersion() {
        return "4.1.3-android.1";
    }

    @JavascriptInterface
    public void postAsync(String requestId, String endpoint, String apiKey, String body) {
        final String id = requestId == null ? "" : requestId;
        executor.execute(() -> {
            int status = 0;
            String response = "";
            String error = "";
            HttpURLConnection conn = null;
            try {
                URL url = new URL(endpoint);
                String scheme = url.getProtocol();
                if (!"https".equalsIgnoreCase(scheme) && !"http".equalsIgnoreCase(scheme)) {
                    throw new IllegalArgumentException("API 地址只支持 http/https");
                }
                conn = (HttpURLConnection) url.openConnection();
                activeConnection = conn;
                activeRequestId = id;
                conn.setRequestMethod("POST");
                conn.setConnectTimeout(30_000);
                conn.setReadTimeout(8 * 60_000);
                conn.setDoOutput(true);
                conn.setUseCaches(false);
                conn.setRequestProperty("Content-Type", "application/json; charset=utf-8");
                conn.setRequestProperty("Authorization", "Bearer " + apiKey);
                conn.setRequestProperty("User-Agent", "XinyueNovel-Android/4.1.3");
                byte[] payload = body.getBytes(StandardCharsets.UTF_8);
                conn.setFixedLengthStreamingMode(payload.length);
                try (OutputStream os = conn.getOutputStream()) {
                    os.write(payload);
                }
                status = conn.getResponseCode();
                InputStream in = status >= 200 && status < 300 ? conn.getInputStream() : conn.getErrorStream();
                response = readLimited(in, 32 * 1024 * 1024);
            } catch (Exception e) {
                error = e.getClass().getSimpleName() + ": " + String.valueOf(e.getMessage());
            } finally {
                if (conn != null) conn.disconnect();
                if (id.equals(activeRequestId)) {
                    activeConnection = null;
                    activeRequestId = null;
                }
            }
            callbackHttp(id, status, response, error);
        });
    }

    @JavascriptInterface
    public void cancelRequest(String requestId) {
        HttpURLConnection c = activeConnection;
        String active = activeRequestId;
        if (c != null && (requestId == null || requestId.equals(active))) {
            c.disconnect();
        }
    }

    private static String readLimited(InputStream in, int max) throws Exception {
        if (in == null) return "";
        try (InputStream input = in; ByteArrayOutputStream out = new ByteArrayOutputStream()) {
            byte[] buf = new byte[8192];
            int total = 0;
            for (;;) {
                int n = input.read(buf);
                if (n < 0) break;
                total += n;
                if (total > max) throw new IllegalStateException("API 返回内容超过 32MB");
                out.write(buf, 0, n);
            }
            return out.toString(StandardCharsets.UTF_8.name());
        }
    }

    private void callbackHttp(String id, int status, String response, String error) {
        final String js = "window.__nativeHttpResult(" + JSONObject.quote(id) + "," + status + "," +
                JSONObject.quote(response == null ? "" : response) + "," +
                JSONObject.quote(error == null ? "" : error) + ");";
        webView.post(() -> webView.evaluateJavascript(js, null));
    }

    @JavascriptInterface
    public void copyText(String text) {
        activity.runOnUiThread(() -> {
            ClipboardManager cm = (ClipboardManager) activity.getSystemService(Context.CLIPBOARD_SERVICE);
            if (cm != null) cm.setPrimaryClip(ClipData.newPlainText("心阅小说", text == null ? "" : text));
        });
    }

    @JavascriptInterface
    public void shareText(String title, String text) {
        activity.runOnUiThread(() -> {
            Intent send = new Intent(Intent.ACTION_SEND);
            send.setType("text/plain");
            send.putExtra(Intent.EXTRA_SUBJECT, title == null ? "心阅小说" : title);
            send.putExtra(Intent.EXTRA_TEXT, text == null ? "" : text);
            activity.startActivity(Intent.createChooser(send, "分享/导出小说"));
        });
    }

    @JavascriptInterface
    public String saveTextFile(String filename, String text) {
        try {
            String safe = safeFileName(filename);
            byte[] data = (text == null ? "" : text).getBytes(StandardCharsets.UTF_8);
            if (Build.VERSION.SDK_INT >= 29) {
                ContentResolver resolver = activity.getContentResolver();
                ContentValues values = new ContentValues();
                values.put(MediaStore.MediaColumns.DISPLAY_NAME, safe);
                values.put(MediaStore.MediaColumns.MIME_TYPE, "text/plain");
                values.put(MediaStore.MediaColumns.RELATIVE_PATH, Environment.DIRECTORY_DOWNLOADS + "/XinyueNovel");
                Uri uri = resolver.insert(MediaStore.Downloads.EXTERNAL_CONTENT_URI, values);
                if (uri == null) throw new IllegalStateException("无法创建下载文件");
                try (OutputStream os = resolver.openOutputStream(uri, "w")) {
                    if (os == null) throw new IllegalStateException("无法打开下载文件");
                    os.write(data);
                }
                return "OK|" + uri;
            }
            File base = activity.getExternalFilesDir(Environment.DIRECTORY_DOCUMENTS);
            if (base == null) base = activity.getFilesDir();
            File dir = new File(base, "XinyueNovel");
            if (!dir.exists() && !dir.mkdirs()) throw new IllegalStateException("无法创建导出目录");
            File file = new File(dir, safe);
            try (OutputStream os = new FileOutputStream(file)) {
                os.write(data);
            }
            return "OK|" + file.getAbsolutePath();
        } catch (Exception e) {
            return "ERR|" + e.getClass().getSimpleName() + ": " + String.valueOf(e.getMessage());
        }
    }

    private static String safeFileName(String s) {
        String out = s == null ? "小说.txt" : s.trim();
        if (out.isEmpty()) out = "小说.txt";
        out = out.replaceAll("[\\\\/:*?\"<>|\\p{Cntrl}]", "_");
        if (!out.toLowerCase().endsWith(".txt")) out += ".txt";
        return out;
    }

    @JavascriptInterface
    public String saveApiKey(String apiKey) {
        try {
            if (apiKey == null || apiKey.trim().isEmpty()) return "ERR|API Key 为空";
            SecretKey key = getOrCreateKey();
            Cipher cipher = Cipher.getInstance("AES/GCM/NoPadding");
            cipher.init(Cipher.ENCRYPT_MODE, key);
            byte[] encrypted = cipher.doFinal(apiKey.getBytes(StandardCharsets.UTF_8));
            String payload = Base64.encodeToString(cipher.getIV(), Base64.NO_WRAP) + ":" + Base64.encodeToString(encrypted, Base64.NO_WRAP);
            prefs().edit().putString(PREF_CIPHER, payload).apply();
            return "OK";
        } catch (Exception e) {
            return "ERR|" + e.getClass().getSimpleName() + ": " + String.valueOf(e.getMessage());
        }
    }

    @JavascriptInterface
    public String loadApiKey() {
        try {
            String payload = prefs().getString(PREF_CIPHER, "");
            if (payload == null || payload.isEmpty()) return "";
            String[] parts = payload.split(":", 2);
            if (parts.length != 2) return "";
            byte[] iv = Base64.decode(parts[0], Base64.NO_WRAP);
            byte[] encrypted = Base64.decode(parts[1], Base64.NO_WRAP);
            KeyStore ks = KeyStore.getInstance("AndroidKeyStore");
            ks.load(null);
            SecretKey key = (SecretKey) ks.getKey(KEY_ALIAS, null);
            if (key == null) return "";
            Cipher cipher = Cipher.getInstance("AES/GCM/NoPadding");
            cipher.init(Cipher.DECRYPT_MODE, key, new GCMParameterSpec(128, iv));
            return new String(cipher.doFinal(encrypted), StandardCharsets.UTF_8);
        } catch (Exception e) {
            return "";
        }
    }

    @JavascriptInterface
    public void clearApiKey() {
        prefs().edit().remove(PREF_CIPHER).apply();
    }

    private SharedPreferences prefs() {
        return activity.getSharedPreferences(PREFS, Context.MODE_PRIVATE);
    }

    private static SecretKey getOrCreateKey() throws Exception {
        KeyStore ks = KeyStore.getInstance("AndroidKeyStore");
        ks.load(null);
        SecretKey existing = (SecretKey) ks.getKey(KEY_ALIAS, null);
        if (existing != null) return existing;
        KeyGenerator generator = KeyGenerator.getInstance(KeyProperties.KEY_ALGORITHM_AES, "AndroidKeyStore");
        KeyGenParameterSpec spec = new KeyGenParameterSpec.Builder(
                KEY_ALIAS,
                KeyProperties.PURPOSE_ENCRYPT | KeyProperties.PURPOSE_DECRYPT)
                .setBlockModes(KeyProperties.BLOCK_MODE_GCM)
                .setEncryptionPaddings(KeyProperties.ENCRYPTION_PADDING_NONE)
                .build();
        generator.init(spec);
        return generator.generateKey();
    }

    void shutdown() {
        HttpURLConnection c = activeConnection;
        if (c != null) c.disconnect();
        executor.shutdownNow();
    }
}
