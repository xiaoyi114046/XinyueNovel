package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEmotionalStylePrompt(t *testing.T) {
	p := NewProject("测试")
	p.Style = "情感细腻风"
	p.Flow = "幕后流"
	p.Characters = "男女主关系复杂"
	sys, user := BuildNovelPrompt(NovelRequest{Project: p, ChapterTitle: "第1章", Context: "前文", TargetChars: 1200})
	if !strings.Contains(sys, "专业中文网络小说作者") || !strings.Contains(user, "微表情") || !strings.Contains(user, "关系张力") {
		t.Fatalf("情感细腻风未注入提示词: %s", user)
	}
}
func TestChapterCRUDAndFullText(t *testing.T) {
	p := NewProject("书")
	if e := UpsertChapter(&p, 1, "第一章：开始", "正文一"); e != nil {
		t.Fatal(e)
	}
	if e := UpsertChapter(&p, 2, "第二章：继续", "正文二"); e != nil {
		t.Fatal(e)
	}
	if !RenameChapter(&p, 2, "第二章：新标题") {
		t.Fatal("rename")
	}
	if e := UpsertChapter(&p, 2, "第二章：新标题", "新正文"); e != nil {
		t.Fatal(e)
	}
	txt := FullNovelText(p)
	if !strings.Contains(txt, "第一章：开始") || !strings.Contains(txt, "新正文") {
		t.Fatal(txt)
	}
	if !DeleteChapter(&p, 1) || len(p.Chapters) != 1 {
		t.Fatal("delete")
	}
}
func TestRecentContext(t *testing.T) {
	p := NewProject("书")
	for i := 1; i <= 5; i++ {
		_ = UpsertChapter(&p, i, "第"+string(rune('0'+i))+"章", "body")
	}
	s := RecentContext(p, 3)
	if strings.Count(s, "body") != 3 {
		t.Fatal(s)
	}
}
func TestParseFullText(t *testing.T) {
	s := "第1章 开始\n\nAAA\n\n第2章 继续\n\nBBB"
	c := ParseFullTextChapters(s)
	if len(c) != 2 || c[1].No != 2 || !strings.Contains(c[1].Body, "BBB") {
		t.Fatalf("%+v", c)
	}
}
func TestStorageRoundTrip(t *testing.T) {
	dir := t.TempDir()
	old := os.Getenv("LOCALAPPDATA")
	defer os.Setenv("LOCALAPPDATA", old)
	os.Setenv("LOCALAPPDATA", dir)
	p := NewProject("本地库")
	_ = UpsertChapter(&p, 1, "第1章", "hello")
	if e := SaveProject(&p); e != nil {
		t.Fatal(e)
	}
	q, e := LoadProject(p.ID)
	if e != nil {
		t.Fatal(e)
	}
	if q.Name != "本地库" || len(q.Chapters) != 1 {
		t.Fatalf("%+v", q)
	}
	b, e := os.ReadFile(projectTXT(p.ID))
	if e != nil || !strings.Contains(string(b), "hello") {
		t.Fatal(e, string(b))
	}
	if !strings.HasPrefix(projectJSON(p.ID), filepath.Join(dir, "XinyueNovel")) {
		t.Fatal(projectJSON(p.ID))
	}
}
func TestSettingsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	old := os.Getenv("LOCALAPPDATA")
	defer os.Setenv("LOCALAPPDATA", old)
	os.Setenv("LOCALAPPDATA", dir)
	enc, _ := protectKey("secret")
	s := APISettings{Endpoint: DefaultEndpoint, Model: "deepseek-chat", Thinking: "关闭", APIKeyCipher: enc, RememberKey: true}
	if e := SaveSettings(s); e != nil {
		t.Fatal(e)
	}
	x, e := LoadSettings()
	if e != nil {
		t.Fatal(e)
	}
	k, e := unprotectKey(x.APIKeyCipher)
	if e != nil || k != "secret" {
		t.Fatal(k, e)
	}
}
func TestMockAPI(t *testing.T) {
	var got map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer k" {
			t.Errorf("auth")
		}
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"续写正文"},"finish_reason":"stop"}],"usage":{"total_tokens":12}}`))
	}))
	defer ts.Close()
	p := NewProject("书")
	s := APISettings{Endpoint: ts.URL, Model: "deepseek-chat", Thinking: "关闭"}
	text, u, e := GenerateNovel(context.Background(), ts.Client(), s, "k", NovelRequest{Project: p, Context: "前文", TargetChars: 1000})
	if e != nil || text != "续写正文" || u.TotalTokens != 12 {
		t.Fatal(text, u, e)
	}
	if got["model"] != "deepseek-chat" {
		t.Fatalf("%v", got)
	}
}
func TestAPIErrors(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"error":{"message":"bad key"}}`))
	}))
	defer ts.Close()
	_, _, e := callDeepSeek(context.Background(), ts.Client(), APISettings{Endpoint: ts.URL, Model: "x", Thinking: "关闭"}, "k", "s", "u", 128)
	if e == nil || !strings.Contains(e.Error(), "bad key") {
		t.Fatal(e)
	}
}

func TestLegacyMigrationRepairsInternalProjectNamesAndMetadata(t *testing.T) {
	dir := t.TempDir()
	old := os.Getenv("LOCALAPPDATA")
	defer os.Setenv("LOCALAPPDATA", old)
	os.Setenv("LOCALAPPDATA", dir)

	legacyID := "p_1789542681834439700"
	legacy := NewProject("我的测试小说")
	legacy.ID = legacyID
	legacy.World = "旧版世界观"
	legacy.Realms = "炼体、筑基、金丹"
	legacy.CurrentRealm = "筑基"
	legacy.Outline = "旧版总纲"
	legacy.VolumeOutline = "旧版卷纲"
	legacy.ChapterOutline = "旧版本章小纲"
	legacy.Characters = "主角：林舟"
	legacy.Plot = "主角踏上修行路"
	_ = UpsertChapter(&legacy, 1, "第1章", "旧版正文")
	legacyDir := filepath.Join(dir, "DeepSeekNovelStudio", "LocalLibrary", "projects", legacyID)
	if err := os.MkdirAll(legacyDir, 0755); err != nil {
		t.Fatal(err)
	}
	b, _ := json.MarshalIndent(legacy, "", "  ")
	if err := os.WriteFile(filepath.Join(legacyDir, "project.json"), b, 0644); err != nil {
		t.Fatal(err)
	}

	// Simulate the bad v4.1.0/4.1.1 TXT-only migration: new ID, old folder ID used as display name.
	bad := NewProject(legacyID)
	bad.World = ""
	bad.Realms = ""
	bad.Outline = ""
	bad.Chapters = nil
	_ = UpsertChapter(&bad, 1, "第1章", "临时迁移正文")
	if err := SaveProject(&bad); err != nil {
		t.Fatal(err)
	}

	migrateLegacyBestEffort()
	ps, err := ListProjects()
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) != 1 {
		t.Fatalf("expected 1 repaired project, got %d: %+v", len(ps), ps)
	}
	got := ps[0]
	if got.Name != "我的测试小说" {
		t.Fatalf("name not repaired: %q", got.Name)
	}
	if got.World != "旧版世界观" || got.Realms != "炼体、筑基、金丹" || got.Outline != "旧版总纲" || got.Characters != "主角：林舟" {
		t.Fatalf("metadata not restored: %+v", got)
	}
	if len(got.Chapters) != 1 || got.Chapters[0].Body != "旧版正文" {
		t.Fatalf("chapters not restored: %+v", got.Chapters)
	}
}

func TestLegacyMigrationImportsCompleteProject(t *testing.T) {
	dir := t.TempDir()
	old := os.Getenv("LOCALAPPDATA")
	defer os.Setenv("LOCALAPPDATA", old)
	os.Setenv("LOCALAPPDATA", dir)

	legacy := NewProject("另一部小说")
	legacy.ID = "20260915T015555_f9d64719"
	legacy.World = "完整世界"
	legacy.Outline = "完整大纲"
	legacyDir := filepath.Join(dir, "DeepSeekNovelStudio", "LocalLibrary", legacy.ID)
	if err := os.MkdirAll(legacyDir, 0755); err != nil {
		t.Fatal(err)
	}
	b, _ := json.MarshalIndent(legacy, "", "  ")
	if err := os.WriteFile(filepath.Join(legacyDir, "project.json"), b, 0644); err != nil {
		t.Fatal(err)
	}

	migrateLegacyBestEffort()
	ps, err := ListProjects()
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) != 1 || ps[0].Name != "另一部小说" || ps[0].World != "完整世界" || ps[0].Outline != "完整大纲" {
		t.Fatalf("legacy project not imported fully: %+v", ps)
	}
}

func TestSettingsRememberLastProject(t *testing.T) {
	dir := t.TempDir()
	old := os.Getenv("LOCALAPPDATA")
	defer os.Setenv("LOCALAPPDATA", old)
	os.Setenv("LOCALAPPDATA", dir)
	s := APISettings{Endpoint: DefaultEndpoint, Model: "deepseek-chat", Thinking: "关闭", LastProjectID: "project-123"}
	if err := SaveSettings(s); err != nil {
		t.Fatal(err)
	}
	got, err := LoadSettings()
	if err != nil {
		t.Fatal(err)
	}
	if got.LastProjectID != "project-123" {
		t.Fatalf("last project not persisted: %+v", got)
	}
}

func TestWin32FilterEncodingAllowsEmbeddedNUL(t *testing.T) {
	f := buildWin32FilterUTF16("文本文件 (*.txt)\x00*.txt")
	if len(f) < 4 || f[len(f)-1] != 0 || f[len(f)-2] != 0 {
		t.Fatalf("filter is not double-NUL terminated: %#v", f)
	}
	seenInternal := false
	for i := 0; i < len(f)-2; i++ {
		if f[i] == 0 {
			seenInternal = true
			break
		}
	}
	if !seenInternal {
		t.Fatalf("filter lost embedded NUL separator: %#v", f)
	}
}

func TestSafeWindowsFileName(t *testing.T) {
	got := safeWindowsFileName("  妈妈:的/性*教育?<>|\x00.  ")
	if strings.ContainsAny(got, `\\/:*?"<>|`) || strings.ContainsRune(got, '\x00') {
		t.Fatalf("unsafe filename: %q", got)
	}
	if strings.TrimSpace(got) == "" || strings.HasSuffix(got, ".") || strings.HasSuffix(got, " ") {
		t.Fatalf("bad normalized filename: %q", got)
	}
}

func TestExportFullNovel(t *testing.T) {
	p := NewProject("导出测试")
	_ = UpsertChapter(&p, 1, "第1章 开始", "正文一")
	_ = UpsertChapter(&p, 2, "第2章 继续", "正文二")
	out := filepath.Join(t.TempDir(), "小说.txt")
	if err := ExportFullNovel(p, out); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != FullNovelText(p) || !strings.Contains(string(b), "正文二") {
		t.Fatalf("bad export: %q", string(b))
	}
}

func TestExportFullNovelRejectsEmptyPath(t *testing.T) {
	if err := ExportFullNovel(NewProject("x"), "  "); err == nil {
		t.Fatal("expected empty path error")
	}
}

func TestAllProjectMetadataRoundTrip(t *testing.T) {
	dir := t.TempDir()
	old := os.Getenv("LOCALAPPDATA")
	defer os.Setenv("LOCALAPPDATA", old)
	os.Setenv("LOCALAPPDATA", dir)
	p := NewProject("完整设定")
	p.Genre = "都市"
	p.Flow = "迪化流"
	p.Style = "情感细腻风"
	p.POV = "第一人称"
	p.Pace = "慢热细腻"
	p.Protagonist = "林舟"
	p.Characters = "人物关系"
	p.Plot = "剧情脉络"
	p.World = "世界观"
	p.Realms = "境界体系"
	p.CurrentRealm = "第三境"
	p.Outline = "总纲"
	p.VolumeOutline = "卷纲"
	p.ChapterOutline = "本章小纲"
	p.Draft = "未入库草稿"
	_ = UpsertChapter(&p, 3, "第3章", "正文")
	if err := SaveProject(&p); err != nil {
		t.Fatal(err)
	}
	got, err := LoadProject(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != p.Name || got.Genre != p.Genre || got.Flow != p.Flow || got.Style != p.Style || got.POV != p.POV || got.Pace != p.Pace || got.Protagonist != p.Protagonist || got.Characters != p.Characters || got.Plot != p.Plot || got.World != p.World || got.Realms != p.Realms || got.CurrentRealm != p.CurrentRealm || got.Outline != p.Outline || got.VolumeOutline != p.VolumeOutline || got.ChapterOutline != p.ChapterOutline || got.Draft != p.Draft || len(got.Chapters) != 1 {
		t.Fatalf("metadata mismatch: %+v", got)
	}
}

func TestProjectListRenameDeleteWorkflow(t *testing.T) {
	dir := t.TempDir()
	old := os.Getenv("LOCALAPPDATA")
	defer os.Setenv("LOCALAPPDATA", old)
	os.Setenv("LOCALAPPDATA", dir)
	p1 := NewProject("第一本")
	p2 := NewProject("第二本")
	if err := SaveProject(&p1); err != nil {
		t.Fatal(err)
	}
	if err := SaveProject(&p2); err != nil {
		t.Fatal(err)
	}
	p1.Name = "重命名后"
	if err := SaveProject(&p1); err != nil {
		t.Fatal(err)
	}
	got, err := LoadProject(p1.ID)
	if err != nil || got.Name != "重命名后" {
		t.Fatal(got.Name, err)
	}
	ps, err := ListProjects()
	if err != nil || len(ps) != 2 {
		t.Fatalf("projects=%d err=%v", len(ps), err)
	}
	if err := DeleteProjectData(p2.ID); err != nil {
		t.Fatal(err)
	}
	ps, err = ListProjects()
	if err != nil || len(ps) != 1 || ps[0].ID != p1.ID {
		t.Fatalf("after delete: %+v %v", ps, err)
	}
}

func TestProjectBackupRoundTrip(t *testing.T) {
	dir := t.TempDir()
	old := os.Getenv("LOCALAPPDATA")
	defer os.Setenv("LOCALAPPDATA", old)
	os.Setenv("LOCALAPPDATA", dir)
	p := NewProject("备份测试")
	p.World = "世界"
	_ = UpsertChapter(&p, 1, "第1章", "正文")
	backup := filepath.Join(t.TempDir(), "backup.dsnproj")
	if err := ExportProjectBackup(p, backup); err != nil {
		t.Fatal(err)
	}
	q, err := ImportProjectBackup(backup)
	if err != nil {
		t.Fatal(err)
	}
	if q.Name != p.Name || q.World != p.World || len(q.Chapters) != 1 {
		t.Fatalf("%+v", q)
	}
}

func TestThinkingModesRequestBody(t *testing.T) {
	modes := map[string]struct{ typ, effort string }{
		"关闭": {"disabled", ""}, "Low": {"enabled", "low"}, "High": {"enabled", "high"}, "Max": {"enabled", "max"},
	}
	for mode, want := range modes {
		t.Run(mode, func(t *testing.T) {
			var got apiReq
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
					t.Error(err)
				}
				_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
			}))
			defer ts.Close()
			_, _, err := callDeepSeek(context.Background(), ts.Client(), APISettings{Endpoint: ts.URL, Model: "m", Thinking: mode}, "k", "s", "u", 128)
			if err != nil {
				t.Fatal(err)
			}
			if got.Thinking == nil || got.Thinking.Type != want.typ || got.Thinking.Effort != want.effort {
				t.Fatalf("got=%+v want=%+v", got.Thinking, want)
			}
		})
	}
}

func TestGenerateOutlineAndRealmsMock(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req apiReq
		_ = json.NewDecoder(r.Body).Decode(&req)
		ans := "OUTLINE_OK"
		if len(req.Messages) > 0 && strings.Contains(req.Messages[0].Content, "世界观设计师") {
			ans = "REALMS_OK"
		}
		_, _ = fmt.Fprintf(w, `{"choices":[{"message":{"content":%q}}]}`, ans)
	}))
	defer ts.Close()
	s := APISettings{Endpoint: ts.URL, Model: "deepseek-chat", Thinking: "关闭"}
	p := NewProject("书")
	o, err := GenerateOutline(context.Background(), ts.Client(), s, "k", p)
	if err != nil || o != "OUTLINE_OK" {
		t.Fatal(o, err)
	}
	r, err := GenerateRealms(context.Background(), ts.Client(), s, "k", p)
	if err != nil || r != "REALMS_OK" {
		t.Fatal(r, err)
	}
}

func TestGenerateCancellation(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer ts.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := callDeepSeek(ctx, ts.Client(), APISettings{Endpoint: ts.URL, Model: "m", Thinking: "关闭"}, "k", "s", "u", 128)
	if err == nil || !strings.Contains(err.Error(), "停止") {
		t.Fatalf("expected stop error, got %v", err)
	}
}

func TestEndToEndLocalWritingWorkflow(t *testing.T) {
	dir := t.TempDir()
	old := os.Getenv("LOCALAPPDATA")
	defer os.Setenv("LOCALAPPDATA", old)
	os.Setenv("LOCALAPPDATA", dir)
	p := NewProject("端到端")
	p.World = "W"
	p.Realms = "R"
	p.Outline = "O"
	p.Style = "情感细腻风"
	for i := 1; i <= 5; i++ {
		if err := UpsertChapter(&p, i, fmt.Sprintf("第%d章 标题", i), fmt.Sprintf("正文%d", i)); err != nil {
			t.Fatal(err)
		}
	}
	if err := SaveProject(&p); err != nil {
		t.Fatal(err)
	}
	if !RenameChapter(&p, 2, "第2章 新标题") {
		t.Fatal("rename")
	}
	if err := UpsertChapter(&p, 2, "第2章 新标题", "修改后的正文2"); err != nil {
		t.Fatal(err)
	}
	if !DeleteChapter(&p, 4) {
		t.Fatal("delete")
	}
	if err := SaveProject(&p); err != nil {
		t.Fatal(err)
	}
	ctx := RecentContext(p, 3)
	if strings.Contains(ctx, "正文1") || !strings.Contains(ctx, "正文5") {
		t.Fatalf("bad recent ctx: %s", ctx)
	}
	out := filepath.Join(t.TempDir(), safeWindowsFileName(p.Name)+"_完整小说.txt")
	if err := ExportFullNovel(p, out); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(out)
	if strings.Contains(string(b), "第4章") || !strings.Contains(string(b), "修改后的正文2") {
		t.Fatalf("bad full text: %s", string(b))
	}
	q, err := LoadProject(p.ID)
	if err != nil || len(q.Chapters) != 4 || q.Outline != "O" {
		t.Fatalf("reload: %+v %v", q, err)
	}
}

func TestSettingsSerializationDoesNotPutPlainAPIKeyInProject(t *testing.T) {
	p := NewProject("书")
	b, _ := json.Marshal(p)
	if strings.Contains(string(b), "api_key") || strings.Contains(string(b), "secret") {
		t.Fatalf("project unexpectedly contains api material: %s", string(b))
	}
}

func TestWindowsSourceRegressionContracts(t *testing.T) {
	b, err := os.ReadFile("main_windows.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	checks := []struct {
		needle        string
		shouldContain bool
	}{
		{"syscall.StringToUTF16(filter", false},
		{"buildWin32FilterUTF16(filter)", true},
		{"ExportFullNovel(current, p)", true},
		{"if pOpenClipboard.Call(uintptr(mainHwnd)); false", false},
		{"enable(hHomeStop, false)", true},
		{"safeWindowsFileName(current.Name)", true},
	}
	for _, c := range checks {
		got := strings.Contains(s, c.needle)
		if got != c.shouldContain {
			t.Fatalf("source contract %q got=%v want=%v", c.needle, got, c.shouldContain)
		}
	}
}

func TestNextChapterAfterDeletionUsesMaxPlusOne(t *testing.T) {
	p := NewProject("书")
	for i := 1; i <= 5; i++ {
		if err := UpsertChapter(&p, i, fmt.Sprintf("第%d章", i), "正文"); err != nil {
			t.Fatal(err)
		}
	}
	if !DeleteChapter(&p, 3) {
		t.Fatal("delete failed")
	}
	if got := NextChapterNo(p); got != 6 {
		t.Fatalf("next=%d want=6", got)
	}
}

func TestDraftClearPersists(t *testing.T) {
	dir := t.TempDir()
	old := os.Getenv("LOCALAPPDATA")
	defer os.Setenv("LOCALAPPDATA", old)
	os.Setenv("LOCALAPPDATA", dir)
	p := NewProject("草稿")
	p.Draft = "旧草稿"
	if err := SaveProject(&p); err != nil {
		t.Fatal(err)
	}
	p.Draft = ""
	if err := SaveProject(&p); err != nil {
		t.Fatal(err)
	}
	q, err := LoadProject(p.ID)
	if err != nil || q.Draft != "" {
		t.Fatalf("draft=%q err=%v", q.Draft, err)
	}
}

func TestFullNovelTextOrderAndUnicode(t *testing.T) {
	p := NewProject("书")
	_ = UpsertChapter(&p, 10, "第10章 尾声", "十😊")
	_ = UpsertChapter(&p, 2, "第2章 中段", "二❤️")
	_ = UpsertChapter(&p, 1, "第1章 开始", "一中文")
	s := FullNovelText(p)
	if !(strings.Index(s, "第1章") < strings.Index(s, "第2章") && strings.Index(s, "第2章") < strings.Index(s, "第10章")) {
		t.Fatalf("order wrong: %s", s)
	}
	if !strings.Contains(s, "😊") || !strings.Contains(s, "❤️") {
		t.Fatalf("unicode lost: %s", s)
	}
}

func TestWindowsUserActionHandlersPresent(t *testing.T) {
	b, err := os.ReadFile("main_windows.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	actions := []string{
		"case idHomeRecentToContext:", "case idHomeGenerate:", "case idHomeStop:", "case idHomeSaveChapter:",
		"case idHomeCopy:", "case idHomeClear:", "case idSetSaveNovel:", "case idWorldSave:",
		"case idWorldGenRealms:", "case idWorldGenOutline:", "case idProjNew:", "case idProjRenameProject:",
		"case idProjRename:", "case idProjSaveEdit:", "case idProjDeleteChapter:", "case idProjAsContext:",
		"case idProjExport:", "case idProjOpenFolder:", "case idProjDeleteProject:", "case idAPISave:",
		"case idAPITest:", "case idAPIClear:",
	}
	for _, a := range actions {
		if !strings.Contains(s, a) {
			t.Fatalf("missing handler %s", a)
		}
	}
	projectSelectors := []string{"case idHomeProject:", "case idSetProject:", "case idWorldProject:", "case idProjCombo:"}
	for _, a := range projectSelectors {
		if !strings.Contains(s, a) {
			t.Fatalf("missing project selector %s", a)
		}
	}
}
