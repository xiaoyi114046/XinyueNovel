package main

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"
)

const (
	AppName         = "心阅小说"
	AppVersion      = "4.1.3"
	DefaultEndpoint = "https://api.deepseek.com/chat/completions"
)

// cleanNUL removes embedded NUL characters before text is passed to Win32
// APIs such as SetWindowTextW/StringToUTF16Ptr. Project text files themselves
// are not modified by this helper.
func cleanNUL(s string) string {
	return strings.ReplaceAll(s, "\x00", "")
}

// safeWindowsFileName makes a user/project supplied name safe as a default
// Windows file name. It intentionally keeps Chinese and other Unicode text.
func safeWindowsFileName(s string) string {
	s = cleanNUL(strings.TrimSpace(s))
	if s == "" {
		return "小说"
	}
	bad := `\/:*?"<>|`
	var b strings.Builder
	for _, r := range s {
		if r < 32 || strings.ContainsRune(bad, r) {
			b.WriteRune('_')
		} else {
			b.WriteRune(r)
		}
	}
	out := strings.TrimRight(strings.TrimSpace(b.String()), ". ")
	if out == "" {
		out = "小说"
	}
	return out
}

// buildWin32FilterUTF16 encodes an OPENFILENAME filter. Unlike
// syscall.StringToUTF16 it deliberately supports embedded NUL separators and
// always returns a double-NUL terminated buffer required by Win32.
func buildWin32FilterUTF16(s string) []uint16 {
	u := utf16.Encode([]rune(s))
	for len(u) < 2 || u[len(u)-1] != 0 || u[len(u)-2] != 0 {
		u = append(u, 0)
	}
	return u
}

type Chapter struct {
	No        int       `json:"no"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Project struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Genre          string    `json:"genre"`
	Flow           string    `json:"flow"`
	Style          string    `json:"style"`
	POV            string    `json:"pov"`
	Pace           string    `json:"pace"`
	Protagonist    string    `json:"protagonist"`
	Characters     string    `json:"characters"`
	Plot           string    `json:"plot"`
	World          string    `json:"world"`
	Realms         string    `json:"realms"`
	CurrentRealm   string    `json:"current_realm"`
	Outline        string    `json:"outline"`
	VolumeOutline  string    `json:"volume_outline"`
	ChapterOutline string    `json:"chapter_outline"`
	Draft          string    `json:"draft"`
	Chapters       []Chapter `json:"chapters"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type APISettings struct {
	Endpoint      string `json:"endpoint"`
	Model         string `json:"model"`
	Thinking      string `json:"thinking"`
	APIKeyCipher  string `json:"api_key_cipher"`
	RememberKey   bool   `json:"remember_key"`
	LastProjectID string `json:"last_project_id,omitempty"`
}

type NovelRequest struct {
	Project      Project
	ChapterTitle string
	Instructions string
	Context      string
	TargetChars  int
}

func NewProject(name string) Project {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "我的小说"
	}
	now := time.Now()
	return Project{
		ID: fmt.Sprintf("%d", now.UnixNano()), Name: name,
		Genre: "玄幻", Flow: "幕后流", Style: "情感细腻风", POV: "第三人称", Pace: "张弛有度",
		CreatedAt: now, UpdatedAt: now,
	}
}

func NextChapterNo(p Project) int {
	m := 0
	for _, c := range p.Chapters {
		if c.No > m {
			m = c.No
		}
	}
	return m + 1
}

func SortChapters(ch []Chapter) {
	sort.SliceStable(ch, func(i, j int) bool { return ch[i].No < ch[j].No })
}

func UpsertChapter(p *Project, no int, title, body string) error {
	if p == nil {
		return fmt.Errorf("项目为空")
	}
	body = strings.TrimSpace(body)
	if body == "" {
		return fmt.Errorf("章节正文为空")
	}
	if no <= 0 {
		no = NextChapterNo(*p)
	}
	title = strings.TrimSpace(title)
	if title == "" {
		title = fmt.Sprintf("第%d章", no)
	}
	now := time.Now()
	for i := range p.Chapters {
		if p.Chapters[i].No == no {
			p.Chapters[i].Title = title
			p.Chapters[i].Body = body
			p.Chapters[i].UpdatedAt = now
			p.UpdatedAt = now
			SortChapters(p.Chapters)
			return nil
		}
	}
	p.Chapters = append(p.Chapters, Chapter{No: no, Title: title, Body: body, CreatedAt: now, UpdatedAt: now})
	p.UpdatedAt = now
	SortChapters(p.Chapters)
	return nil
}

func DeleteChapter(p *Project, no int) bool {
	if p == nil {
		return false
	}
	for i, c := range p.Chapters {
		if c.No == no {
			p.Chapters = append(p.Chapters[:i], p.Chapters[i+1:]...)
			p.UpdatedAt = time.Now()
			return true
		}
	}
	return false
}

func RenameChapter(p *Project, no int, title string) bool {
	title = strings.TrimSpace(title)
	if p == nil || title == "" {
		return false
	}
	for i := range p.Chapters {
		if p.Chapters[i].No == no {
			p.Chapters[i].Title = title
			p.Chapters[i].UpdatedAt = time.Now()
			p.UpdatedAt = time.Now()
			return true
		}
	}
	return false
}

func RecentContext(p Project, n int) string {
	if n <= 0 {
		n = 3
	}
	ch := append([]Chapter(nil), p.Chapters...)
	SortChapters(ch)
	if len(ch) > n {
		ch = ch[len(ch)-n:]
	}
	var b strings.Builder
	for _, c := range ch {
		fmt.Fprintf(&b, "%s\n\n%s\n\n", c.Title, c.Body)
	}
	return strings.TrimSpace(b.String())
}

func FullNovelText(p Project) string {
	ch := append([]Chapter(nil), p.Chapters...)
	SortChapters(ch)
	var b strings.Builder
	for i, c := range ch {
		if i > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(strings.TrimSpace(c.Title))
		b.WriteString("\n\n")
		b.WriteString(strings.TrimSpace(c.Body))
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String()) + "\n"
}

func styleGuidance(style string) string {
	switch strings.TrimSpace(style) {
	case "情感细腻风":
		return "情感细腻风：重点描写人物细微心理波动、微表情、肢体反应、欲言又止和关系张力；情绪要有铺垫与递进，避免直白喊口号；在关键情绪节点用环境、动作和感官细节承托，让情感真实、克制又有余韵。"
	case "宏大玄幻·古意磅礴":
		return "宏大玄幻：世界尺度大，场景有史诗感，语言凝练有古意，战斗与天地异象具有层次感。"
	case "稳健升级·情感清晰":
		return "稳健升级：成长线清楚，冲突与收获形成闭环，人物情感关系明确而不过度拖沓。"
	case "快节奏爽文·强钩子":
		return "快节奏爽文：开篇迅速进入事件，每个场景都有目标、阻力和结果，段尾尽量留下悬念或爽点。"
	case "轻松吐槽·反差喜剧":
		return "轻松反差：人物对话自然诙谐，利用身份差、认知差制造笑点，但不破坏主线推进。"
	default:
		if strings.TrimSpace(style) == "" {
			return "自然流畅，人物口吻稳定，避免模板化表达。"
		}
		return style
	}
}

func flowGuidance(flow string) string {
	switch strings.TrimSpace(flow) {
	case "迪化流":
		return "迪化流：主角行为常被他人过度解读，误会逐层升级但逻辑自洽，读者能同时看到真实动机与旁人脑补。"
	case "幕后流":
		return "幕后流：主角通过组织、化身、布局或信息差在幕后推动事件，逐步展现影响力，并保留身份与底牌悬念。"
	case "系统流":
		return "系统流：系统规则清楚，奖励与限制有因果，避免机械报数，重点写选择带来的剧情变化。"
	case "苟道流":
		return "苟道流：主角重视风险控制与隐藏实力，爽点来自稳健布局、信息优势和关键时刻的爆发。"
	case "无敌流":
		return "无敌流：实力优势明确，冲突重点放在身份、格局、关系和未知规则，而不是重复碾压。"
	default:
		return flow
	}
}

func BuildNovelPrompt(r NovelRequest) (string, string) {
	target := r.TargetChars
	if target <= 0 {
		target = 3500
	}
	p := r.Project
	system := `你是一名专业中文网络小说作者。请根据项目设定和上下文直接续写正文。
要求：只输出小说正文；严格保持人物、境界、世界观、时间线与既有事实一致；从上下文末尾自然承接；不要复述设定，不要解释写作过程，不要突然完结。`
	var b strings.Builder
	fmt.Fprintf(&b, "【项目】%s\n【类型】%s\n【流派】%s\n【文风】%s\n【叙事视角】%s\n【节奏】%s\n", p.Name, p.Genre, p.Flow, p.Style, p.POV, p.Pace)
	fmt.Fprintf(&b, "【流派执行】%s\n【文风执行】%s\n", flowGuidance(p.Flow), styleGuidance(p.Style))
	fmt.Fprintf(&b, "【主角】%s\n【人物关系】\n%s\n\n【剧情脉络】\n%s\n\n【世界观】\n%s\n\n【境界体系】\n%s\n【当前境界】%s\n", p.Protagonist, p.Characters, p.Plot, p.World, p.Realms, p.CurrentRealm)
	fmt.Fprintf(&b, "\n【总纲/卷纲】\n%s\n%s\n\n【本章小纲】\n%s\n\n【本章标题】%s\n\n【本章特殊要求】\n%s\n\n【续写上下文】\n%s\n\n", p.Outline, p.VolumeOutline, p.ChapterOutline, r.ChapterTitle, r.Instructions, r.Context)
	fmt.Fprintf(&b, "本次目标约 %d 个中文字符。内容完整优先，可适度浮动。现在直接续写正文：", target)
	return system, b.String()
}

var headingRE = regexp.MustCompile(`(?m)^\s*(第\s*[0-9一二三四五六七八九十百千万零〇两]+\s*章[^\r\n]*)\s*$`)

func ParseFullTextChapters(text string) []Chapter {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	loc := headingRE.FindAllStringSubmatchIndex(text, -1)
	if len(loc) == 0 {
		return nil
	}
	out := make([]Chapter, 0, len(loc))
	for i, m := range loc {
		title := strings.TrimSpace(text[m[2]:m[3]])
		start := m[1]
		end := len(text)
		if i+1 < len(loc) {
			end = loc[i+1][0]
		}
		body := strings.TrimSpace(text[start:end])
		no := i + 1
		if n := extractArabicChapterNo(title); n > 0 {
			no = n
		}
		now := time.Now()
		out = append(out, Chapter{No: no, Title: title, Body: body, CreatedAt: now, UpdatedAt: now})
	}
	return out
}

func extractArabicChapterNo(title string) int {
	re := regexp.MustCompile(`第\s*(\d+)\s*章`)
	m := re.FindStringSubmatch(title)
	if len(m) < 2 {
		return 0
	}
	n, _ := strconv.Atoi(m[1])
	return n
}
