package com.xiaoyi.xinyuenovel;

import android.content.ContentValues;
import android.content.Context;
import android.content.Intent;
import android.os.Build;
import android.os.Environment;
import android.provider.MediaStore;
import android.util.Log;
import android.webkit.JavascriptInterface;
import android.webkit.WebView;
import android.widget.Toast;

import org.json.JSONArray;
import org.json.JSONObject;

import java.io.BufferedReader;
import java.io.File;
import java.io.FileOutputStream;
import java.io.InputStream;
import java.io.InputStreamReader;
import java.io.OutputStream;
import java.net.HttpURLConnection;
import java.net.URL;
import java.net.URLEncoder;
import java.nio.charset.StandardCharsets;
import java.util.Map;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.Future;

public class NativeBridge {
    private static final String TAG = "XinyueNovel";
    private final Context context;
    private final WebView webView;
    private final SecureStore secureStore;
    private final ExecutorService pool = Executors.newCachedThreadPool();
    private final Map<String, Future<?>> jobs = new ConcurrentHashMap<>();
    private final Map<String, HttpURLConnection> connections = new ConcurrentHashMap<>();

    NativeBridge(Context context, WebView webView) {
        this.context = context;
        this.webView = webView;
        this.secureStore = new SecureStore(context);
    }

    @JavascriptInterface
    public void appReady() {
        Log.i(TAG, "XINYUE_APP_READY");
    }

    @JavascriptInterface
    public String getAppInfo() {
        try {
            JSONObject o = new JSONObject();
            o.put("version", "6.0.0-multiai");
            o.put("sdk", Build.VERSION.SDK_INT);
            return o.toString();
        } catch (Exception e) {
            return "{}";
        }
    }

    @JavascriptInterface
    public String saveApiKey(String provider, String key) {
        try {
            secureStore.put("api_" + safeProvider(provider), key == null ? "" : key.trim());
            return "ok";
        } catch (Exception e) {
            return "error:" + e.getMessage();
        }
    }

    @JavascriptInterface
    public boolean hasApiKey(String provider) {
        return secureStore.has("api_" + safeProvider(provider));
    }

    @JavascriptInterface
    public void clearApiKey(String provider) {
        try {
            secureStore.put("api_" + safeProvider(provider), "");
        } catch (Exception ignored) {}
    }

    @JavascriptInterface
    public void cancelRequest(String requestId) {
        HttpURLConnection c = connections.remove(requestId);
        if (c != null) c.disconnect();
        Future<?> f = jobs.remove(requestId);
        if (f != null) f.cancel(true);
    }

    @JavascriptInterface
    public void aiRequest(String requestId, String provider, String endpoint, String model,
                          String systemPrompt, String userPrompt, double temperature, int maxTokens) {
        if (requestId == null || requestId.isEmpty()) return;
        cancelRequest(requestId);
        Future<?> future = pool.submit(() -> {
            try {
                String p = safeProvider(provider);
                String apiKey = secureStore.get("api_" + p);
                if (apiKey.isEmpty()) throw new IllegalStateException("请先在设置中保存该 AI 的 API Key");
                String text = performRequest(requestId, p, endpoint, model, apiKey,
                        systemPrompt, userPrompt, temperature, maxTokens);
                callback("onNativeAiResult", requestId, true, text);
            } catch (Exception e) {
                String msg = e.getMessage() == null ? e.toString() : e.getMessage();
                if (!Thread.currentThread().isInterrupted()) callback("onNativeAiResult", requestId, false, msg);
            } finally {
                connections.remove(requestId);
                jobs.remove(requestId);
            }
        });
        jobs.put(requestId, future);
    }

    @JavascriptInterface
    public void testProvider(String requestId, String provider, String endpoint, String model) {
        aiRequest(requestId, provider, endpoint, model,
                "你是连接测试助手。", "只回复两个字：成功", 0.1, 32);
    }

    private String performRequest(String requestId, String provider, String endpoint, String model,
                                  String apiKey, String systemPrompt, String userPrompt,
                                  double temperature, int maxTokens) throws Exception {
        if ("openai".equals(provider) || "doubao".equals(provider)) {
            return requestOpenAIResponses(requestId, endpoint, model, apiKey, systemPrompt, userPrompt, maxTokens);
        }
        if ("claude".equals(provider)) {
            return requestClaude(requestId, endpoint, model, apiKey, systemPrompt, userPrompt, temperature, maxTokens);
        }
        if ("gemini".equals(provider)) {
            return requestGemini(requestId, endpoint, model, apiKey, systemPrompt, userPrompt, temperature, maxTokens);
        }
        return requestOpenAICompatible(requestId, endpoint, model, apiKey, systemPrompt, userPrompt, temperature, maxTokens);
    }


    private String requestOpenAIResponses(String requestId, String endpoint, String model, String apiKey,
                                          String systemPrompt, String userPrompt, int maxTokens) throws Exception {
        JSONObject body = new JSONObject();
        body.put("model", require(model, "模型不能为空"));
        if (systemPrompt != null && !systemPrompt.isEmpty()) body.put("instructions", systemPrompt);
        body.put("input", userPrompt == null ? "" : userPrompt);
        body.put("max_output_tokens", Math.max(1, maxTokens));
        body.put("store", false);

        JSONObject json = postJson(requestId, require(endpoint, "接口地址不能为空"), body,
                new String[][]{{"Authorization", "Bearer " + apiKey}});
        JSONArray output = json.optJSONArray("output");
        if (output == null || output.length() == 0) throw new IllegalStateException("OpenAI 返回内容为空");
        StringBuilder out = new StringBuilder();
        for (int i = 0; i < output.length(); i++) {
            JSONObject item = output.optJSONObject(i);
            if (item == null) continue;
            JSONArray content = item.optJSONArray("content");
            if (content == null) continue;
            for (int j = 0; j < content.length(); j++) {
                JSONObject part = content.optJSONObject(j);
                if (part != null && "output_text".equals(part.optString("type"))) {
                    out.append(part.optString("text", ""));
                }
            }
        }
        if (out.length() == 0) throw new IllegalStateException("OpenAI 返回内容为空");
        return out.toString();
    }

    private String requestOpenAICompatible(String requestId, String endpoint, String model, String apiKey,
                                           String systemPrompt, String userPrompt, double temperature,
                                           int maxTokens) throws Exception {
        JSONObject body = new JSONObject();
        body.put("model", require(model, "模型不能为空"));
        JSONArray messages = new JSONArray();
        if (systemPrompt != null && !systemPrompt.isEmpty()) {
            messages.put(new JSONObject().put("role", "system").put("content", systemPrompt));
        }
        messages.put(new JSONObject().put("role", "user").put("content", userPrompt == null ? "" : userPrompt));
        body.put("messages", messages);
        body.put("temperature", temperature);
        body.put("max_tokens", Math.max(1, maxTokens));
        body.put("stream", false);

        JSONObject json = postJson(requestId, require(endpoint, "接口地址不能为空"), body,
                new String[][]{{"Authorization", "Bearer " + apiKey}});
        JSONArray choices = json.optJSONArray("choices");
        if (choices == null || choices.length() == 0) throw new IllegalStateException("AI 返回中没有 choices");
        JSONObject message = choices.getJSONObject(0).optJSONObject("message");
        if (message == null) throw new IllegalStateException("AI 返回中没有 message");
        String content = message.optString("content", "");
        if (content.isEmpty()) {
            content = message.optString("reasoning_content", "");
        }
        if (content.isEmpty()) throw new IllegalStateException("AI 返回内容为空");
        return content;
    }

    private String requestClaude(String requestId, String endpoint, String model, String apiKey,
                                 String systemPrompt, String userPrompt, double temperature,
                                 int maxTokens) throws Exception {
        JSONObject body = new JSONObject();
        body.put("model", require(model, "模型不能为空"));
        body.put("max_tokens", Math.max(1, maxTokens));
        body.put("temperature", temperature);
        if (systemPrompt != null && !systemPrompt.isEmpty()) body.put("system", systemPrompt);
        JSONArray messages = new JSONArray();
        messages.put(new JSONObject().put("role", "user").put("content", userPrompt == null ? "" : userPrompt));
        body.put("messages", messages);

        JSONObject json = postJson(requestId, require(endpoint, "接口地址不能为空"), body,
                new String[][]{{"x-api-key", apiKey}, {"anthropic-version", "2023-06-01"}});
        JSONArray content = json.optJSONArray("content");
        if (content == null || content.length() == 0) throw new IllegalStateException("Claude 返回内容为空");
        StringBuilder out = new StringBuilder();
        for (int i = 0; i < content.length(); i++) {
            JSONObject item = content.optJSONObject(i);
            if (item != null && "text".equals(item.optString("type"))) out.append(item.optString("text"));
        }
        if (out.length() == 0) throw new IllegalStateException("Claude 返回内容为空");
        return out.toString();
    }

    private String requestGemini(String requestId, String endpoint, String model, String apiKey,
                                 String systemPrompt, String userPrompt, double temperature,
                                 int maxTokens) throws Exception {
        String url = require(endpoint, "接口地址不能为空");
        if (url.contains("{model}")) {
            url = url.replace("{model}", URLEncoder.encode(require(model, "模型不能为空"), "UTF-8"));
        } else if (!url.contains(":generateContent")) {
            if (!url.endsWith("/")) url += "/";
            url += "models/" + URLEncoder.encode(require(model, "模型不能为空"), "UTF-8") + ":generateContent";
        }
        JSONObject body = new JSONObject();
        if (systemPrompt != null && !systemPrompt.isEmpty()) {
            body.put("systemInstruction", new JSONObject().put("parts",
                    new JSONArray().put(new JSONObject().put("text", systemPrompt))));
        }
        JSONArray contents = new JSONArray();
        contents.put(new JSONObject().put("role", "user").put("parts",
                new JSONArray().put(new JSONObject().put("text", userPrompt == null ? "" : userPrompt))));
        body.put("contents", contents);
        // Newer Gemini models deprecate some sampling parameters; maxOutputTokens is
        // sufficient for this app and avoids model-version-specific temperature errors.
        body.put("generationConfig", new JSONObject()
                .put("maxOutputTokens", Math.max(1, maxTokens)));

        JSONObject json = postJson(requestId, url, body, new String[][]{{"x-goog-api-key", apiKey}});
        JSONArray candidates = json.optJSONArray("candidates");
        if (candidates == null || candidates.length() == 0) throw new IllegalStateException("Gemini 返回内容为空");
        JSONObject content = candidates.getJSONObject(0).optJSONObject("content");
        if (content == null) throw new IllegalStateException("Gemini 返回内容为空");
        JSONArray parts = content.optJSONArray("parts");
        if (parts == null) throw new IllegalStateException("Gemini 返回内容为空");
        StringBuilder out = new StringBuilder();
        for (int i = 0; i < parts.length(); i++) {
            JSONObject part = parts.optJSONObject(i);
            if (part != null) out.append(part.optString("text", ""));
        }
        if (out.length() == 0) throw new IllegalStateException("Gemini 返回内容为空");
        return out.toString();
    }

    private JSONObject postJson(String requestId, String endpoint, JSONObject body, String[][] headers) throws Exception {
        HttpURLConnection conn = null;
        try {
            conn = (HttpURLConnection) new URL(endpoint).openConnection();
            connections.put(requestId, conn);
            conn.setRequestMethod("POST");
            conn.setConnectTimeout(30000);
            conn.setReadTimeout(180000);
            conn.setDoOutput(true);
            conn.setRequestProperty("Content-Type", "application/json; charset=utf-8");
            conn.setRequestProperty("Accept", "application/json");
            for (String[] h : headers) if (h.length == 2) conn.setRequestProperty(h[0], h[1]);

            byte[] data = body.toString().getBytes(StandardCharsets.UTF_8);
            conn.setFixedLengthStreamingMode(data.length);
            try (OutputStream os = conn.getOutputStream()) {
                os.write(data);
            }

            int code = conn.getResponseCode();
            InputStream input = code >= 200 && code < 300 ? conn.getInputStream() : conn.getErrorStream();
            String text = readAll(input);
            if (code < 200 || code >= 300) {
                throw new IllegalStateException("HTTP " + code + "：" + compactError(text));
            }
            return new JSONObject(text);
        } finally {
            if (conn != null) conn.disconnect();
        }
    }

    private String readAll(InputStream in) throws Exception {
        if (in == null) return "";
        StringBuilder sb = new StringBuilder();
        try (BufferedReader br = new BufferedReader(new InputStreamReader(in, StandardCharsets.UTF_8))) {
            String line;
            while ((line = br.readLine()) != null) sb.append(line).append('\n');
        }
        return sb.toString();
    }

    private String compactError(String text) {
        if (text == null || text.isEmpty()) return "请求失败";
        try {
            JSONObject j = new JSONObject(text);
            Object err = j.opt("error");
            if (err instanceof JSONObject) {
                String msg = ((JSONObject) err).optString("message", "");
                if (!msg.isEmpty()) return msg;
            }
            if (err != null) return String.valueOf(err);
            String msg = j.optString("message", "");
            if (!msg.isEmpty()) return msg;
        } catch (Exception ignored) {}
        text = text.replaceAll("\\s+", " ").trim();
        return text.length() > 500 ? text.substring(0, 500) : text;
    }

    @JavascriptInterface
    public void exportText(String fileName, String content) {
        pool.submit(() -> {
            try {
                String safeName = sanitizeFileName(fileName);
                if (!safeName.endsWith(".txt")) safeName += ".txt";
                if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
                    ContentValues values = new ContentValues();
                    values.put(MediaStore.Downloads.DISPLAY_NAME, safeName);
                    values.put(MediaStore.Downloads.MIME_TYPE, "text/plain");
                    values.put(MediaStore.Downloads.RELATIVE_PATH, Environment.DIRECTORY_DOWNLOADS + "/XinyueNovel");
                    android.net.Uri uri = context.getContentResolver().insert(MediaStore.Downloads.EXTERNAL_CONTENT_URI, values);
                    if (uri == null) throw new IllegalStateException("无法创建导出文件");
                    try (OutputStream os = context.getContentResolver().openOutputStream(uri)) {
                        if (os == null) throw new IllegalStateException("无法写入导出文件");
                        os.write((content == null ? "" : content).getBytes(StandardCharsets.UTF_8));
                    }
                } else {
                    File dir = new File(Environment.getExternalStoragePublicDirectory(Environment.DIRECTORY_DOWNLOADS), "XinyueNovel");
                    if (!dir.exists() && !dir.mkdirs()) throw new IllegalStateException("无法创建下载目录");
                    try (FileOutputStream fos = new FileOutputStream(new File(dir, safeName))) {
                        fos.write((content == null ? "" : content).getBytes(StandardCharsets.UTF_8));
                    }
                }
                toast("已导出到 下载/XinyueNovel/" + safeName);
            } catch (Exception e) {
                toast("导出失败：" + e.getMessage());
            }
        });
    }

    @JavascriptInterface
    public void shareText(String title, String content) {
        Intent send = new Intent(Intent.ACTION_SEND);
        send.setType("text/plain");
        send.putExtra(Intent.EXTRA_SUBJECT, title == null ? "心阅小说" : title);
        send.putExtra(Intent.EXTRA_TEXT, content == null ? "" : content);
        Intent chooser = Intent.createChooser(send, "分享小说内容");
        chooser.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK);
        context.startActivity(chooser);
    }

    private void callback(String fn, String requestId, boolean ok, String payload) {
        String js = "window." + fn + "(" + JSONObject.quote(requestId) + "," + ok + "," + JSONObject.quote(payload == null ? "" : payload) + ")";
        webView.post(() -> webView.evaluateJavascript(js, null));
    }

    private void toast(String text) {
        webView.post(() -> Toast.makeText(context, text, Toast.LENGTH_LONG).show());
    }

    private String safeProvider(String provider) {
        if (provider == null) return "custom";
        String p = provider.toLowerCase().replaceAll("[^a-z0-9_-]", "");
        return p.isEmpty() ? "custom" : p;
    }

    private String require(String value, String message) {
        if (value == null || value.trim().isEmpty()) throw new IllegalArgumentException(message);
        return value.trim();
    }

    private String sanitizeFileName(String name) {
        String n = (name == null || name.trim().isEmpty()) ? "心阅小说" : name.trim();
        n = n.replaceAll("[\\\\/:*?\"<>|]", "_");
        return n.length() > 80 ? n.substring(0, 80) : n;
    }

    void shutdown() {
        for (String id : jobs.keySet()) cancelRequest(id);
        pool.shutdownNow();
    }
}
