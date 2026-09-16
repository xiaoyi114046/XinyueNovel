package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

type LibraryIndex struct {
	CurrentID string           `json:"current_id"`
	Projects  []ProjectSummary `json:"projects"`
}
type ProjectSummary struct {
	ID, Name  string
	Chapters  int       `json:"chapters"`
	UpdatedAt time.Time `json:"updated_at"`
}

func appBaseDir() string {
	if v := strings.TrimSpace(os.Getenv("LOCALAPPDATA")); v != "" {
		return filepath.Join(v, "XinyueNovel")
	}
	h, _ := os.UserHomeDir()
	if h == "" {
		h = "."
	}
	return filepath.Join(h, ".xinyuenovel")
}
func libraryDir() string           { return filepath.Join(appBaseDir(), "LocalLibrary") }
func projectsDir() string          { return filepath.Join(libraryDir(), "projects") }
func settingsPath() string         { return filepath.Join(appBaseDir(), "settings.json") }
func projectDir(id string) string  { return filepath.Join(projectsDir(), id) }
func projectJSON(id string) string { return filepath.Join(projectDir(id), "project.json") }
func projectTXT(id string) string  { return filepath.Join(projectDir(id), "小说全文.txt") }

func ensureDirs() error { return os.MkdirAll(projectsDir(), 0755) }

func atomicWrite(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if _, err := os.Stat(path); err == nil {
		old, _ := os.ReadFile(path)
		_ = os.WriteFile(path+".bak", old, 0644)
	}
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	_ = os.Remove(path)
	return os.Rename(tmp, path)
}
func SaveProject(p *Project) error {
	if p == nil {
		return errors.New("项目为空")
	}
	if p.ID == "" {
		return errors.New("项目ID为空")
	}
	p.UpdatedAt = time.Now()
	if p.CreatedAt.IsZero() {
		p.CreatedAt = p.UpdatedAt
	}
	SortChapters(p.Chapters)
	b, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	if err = atomicWrite(projectJSON(p.ID), b); err != nil {
		return err
	}
	return atomicWrite(projectTXT(p.ID), []byte(FullNovelText(*p)))
}
func LoadProject(id string) (Project, error) {
	b, err := os.ReadFile(projectJSON(id))
	if err != nil {
		return Project{}, err
	}
	var p Project
	if err = json.Unmarshal(b, &p); err != nil {
		return Project{}, err
	}
	SortChapters(p.Chapters)
	return p, nil
}
func DeleteProjectData(id string) error {
	if id == "" {
		return nil
	}
	return os.RemoveAll(projectDir(id))
}
func ListProjects() ([]Project, error) {
	if err := ensureDirs(); err != nil {
		return nil, err
	}
	es, err := os.ReadDir(projectsDir())
	if err != nil {
		return nil, err
	}
	var ps []Project
	for _, e := range es {
		if !e.IsDir() {
			continue
		}
		p, er := LoadProject(e.Name())
		if er == nil {
			ps = append(ps, p)
		}
	}
	sort.SliceStable(ps, func(i, j int) bool { return ps[i].UpdatedAt.After(ps[j].UpdatedAt) })
	return ps, nil
}
func ExportFullNovel(p Project, path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return errors.New("导出路径为空")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(FullNovelText(p)), 0644)
}

func ExportProjectBackup(p Project, path string) error {
	b, e := json.MarshalIndent(p, "", "  ")
	if e != nil {
		return e
	}
	return atomicWrite(path, b)
}
func ImportProjectBackup(path string) (Project, error) {
	b, e := os.ReadFile(path)
	if e != nil {
		return Project{}, e
	}
	var p Project
	if e = json.Unmarshal(b, &p); e != nil {
		return Project{}, e
	}
	if p.ID == "" {
		p.ID = fmt.Sprintf("%d", time.Now().UnixNano())
	}
	p.UpdatedAt = time.Now()
	return p, SaveProject(&p)
}

func LoadSettings() (APISettings, error) {
	var s APISettings
	b, e := os.ReadFile(settingsPath())
	if e != nil {
		if os.IsNotExist(e) {
			s.Endpoint = DefaultEndpoint
			s.Model = "deepseek-chat"
			s.Thinking = "关闭"
			return s, nil
		}
		return s, e
	}
	if e = json.Unmarshal(b, &s); e != nil {
		return s, e
	}
	if s.Endpoint == "" {
		s.Endpoint = DefaultEndpoint
	}
	if s.Model == "" {
		s.Model = "deepseek-chat"
	}
	if s.Thinking == "" {
		s.Thinking = "关闭"
	}
	return s, nil
}
func SaveSettings(s APISettings) error {
	b, e := json.MarshalIndent(s, "", "  ")
	if e != nil {
		return e
	}
	return atomicWrite(settingsPath(), b)
}

// Portable fallback used in tests/non-Windows. Windows builds replace encryption via dpapi_windows.go.
func encodePlainKey(k string) string { return base64.StdEncoding.EncodeToString([]byte(k)) }
func decodePlainKey(s string) string { b, _ := base64.StdEncoding.DecodeString(s); return string(b) }

var legacyIDLikeRE = regexp.MustCompile(`^(?:p_[0-9A-Za-z_-]{8,}|\d{8}T\d{6}_[0-9A-Za-z_-]{4,}|\d{16,})$`)

func looksLikeInternalProjectName(name, id string) bool {
	name = strings.TrimSpace(name)
	id = strings.TrimSpace(id)
	if name == "" {
		return true
	}
	if id != "" && name == id {
		return true
	}
	return legacyIDLikeRE.MatchString(name)
}

func mergeLegacyProject(current Project, legacy Project) Project {
	// Keep the current-library ID/folder stable, but recover the human-readable
	// metadata and content from the legacy project whenever the current project
	// was created by the old TXT-only migration.
	out := legacy
	out.ID = current.ID
	if strings.TrimSpace(out.Name) == "" {
		out.Name = current.Name
	}
	if len(out.Chapters) == 0 && len(current.Chapters) > 0 {
		out.Chapters = current.Chapters
	}
	if strings.TrimSpace(out.Draft) == "" {
		out.Draft = current.Draft
	}
	if out.CreatedAt.IsZero() {
		out.CreatedAt = current.CreatedAt
	}
	if out.CreatedAt.IsZero() {
		out.CreatedAt = time.Now()
	}
	out.UpdatedAt = time.Now()
	return out
}

func mergeMissingLegacyFields(current Project, legacy Project) Project {
	out := current
	if strings.TrimSpace(out.Name) == "" || looksLikeInternalProjectName(out.Name, out.ID) {
		out.Name = legacy.Name
	}
	if strings.TrimSpace(out.Genre) == "" {
		out.Genre = legacy.Genre
	}
	if strings.TrimSpace(out.Flow) == "" {
		out.Flow = legacy.Flow
	}
	if strings.TrimSpace(out.Style) == "" {
		out.Style = legacy.Style
	}
	if strings.TrimSpace(out.POV) == "" {
		out.POV = legacy.POV
	}
	if strings.TrimSpace(out.Pace) == "" {
		out.Pace = legacy.Pace
	}
	if strings.TrimSpace(out.Protagonist) == "" {
		out.Protagonist = legacy.Protagonist
	}
	if strings.TrimSpace(out.Characters) == "" {
		out.Characters = legacy.Characters
	}
	if strings.TrimSpace(out.Plot) == "" {
		out.Plot = legacy.Plot
	}
	if strings.TrimSpace(out.World) == "" {
		out.World = legacy.World
	}
	if strings.TrimSpace(out.Realms) == "" {
		out.Realms = legacy.Realms
	}
	if strings.TrimSpace(out.CurrentRealm) == "" {
		out.CurrentRealm = legacy.CurrentRealm
	}
	if strings.TrimSpace(out.Outline) == "" {
		out.Outline = legacy.Outline
	}
	if strings.TrimSpace(out.VolumeOutline) == "" {
		out.VolumeOutline = legacy.VolumeOutline
	}
	if strings.TrimSpace(out.ChapterOutline) == "" {
		out.ChapterOutline = legacy.ChapterOutline
	}
	if strings.TrimSpace(out.Draft) == "" {
		out.Draft = legacy.Draft
	}
	if len(out.Chapters) == 0 && len(legacy.Chapters) > 0 {
		out.Chapters = legacy.Chapters
	}
	if out.CreatedAt.IsZero() {
		out.CreatedAt = legacy.CreatedAt
	}
	out.UpdatedAt = time.Now()
	return out
}

func readLegacyProject(path string) (Project, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Project{}, err
	}
	var p Project
	if err = json.Unmarshal(b, &p); err != nil {
		return Project{}, err
	}
	if strings.TrimSpace(p.ID) == "" {
		p.ID = filepath.Base(filepath.Dir(path))
	}
	if strings.TrimSpace(p.Name) == "" {
		p.Name = p.ID
	}
	SortChapters(p.Chapters)
	return p, nil
}

func legacyLibraryDirs() []string {
	root := strings.TrimSpace(os.Getenv("LOCALAPPDATA"))
	if root == "" {
		return nil
	}
	return []string{
		filepath.Join(root, "DeepSeekNovelStudio", "LocalLibrary"),
		filepath.Join(root, "XinyueNovelStudio", "LocalLibrary"),
	}
}

func migrateLegacyBestEffort() {
	_ = ensureDirs()
	currentProjects, _ := ListProjects()

	// Index current projects by both their real ID and their displayed name.  The
	// v4.1.0/4.1.1 TXT-only migration used the old folder ID as the *display name*,
	// so matching by name is required to repair already-migrated installations.
	byID := map[string]int{}
	byName := map[string]int{}
	for i, p := range currentProjects {
		byID[p.ID] = i
		byName[strings.TrimSpace(p.Name)] = i
	}
	matchedLegacy := map[string]bool{}

	for _, base := range legacyLibraryDirs() {
		if st, err := os.Stat(base); err != nil || !st.IsDir() {
			continue
		}
		_ = filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
			if err != nil || info == nil || info.IsDir() {
				return nil
			}
			if info.Name() != "project.json" && info.Name() != "project.json.bak" {
				return nil
			}
			lp, er := readLegacyProject(path)
			if er != nil || strings.TrimSpace(lp.ID) == "" {
				return nil
			}
			if matchedLegacy[lp.ID] {
				return nil
			}

			idx, ok := byID[lp.ID]
			if !ok {
				// Repair projects created by the old TXT-only migration: their current
				// ID is new, but their visible name equals the legacy folder/ID.
				idx, ok = byName[lp.ID]
			}
			if !ok && strings.TrimSpace(lp.Name) != "" {
				// Also avoid duplicating a project that the user already recreated with
				// the correct human-readable name in a newer version.
				idx, ok = byName[strings.TrimSpace(lp.Name)]
			}
			if ok && idx >= 0 && idx < len(currentProjects) {
				cp := currentProjects[idx]
				var fixed Project
				if looksLikeInternalProjectName(cp.Name, cp.ID) || strings.TrimSpace(cp.Name) == lp.ID {
					fixed = mergeLegacyProject(cp, lp)
				} else {
					fixed = mergeMissingLegacyFields(cp, lp)
				}
				_ = SaveProject(&fixed)
				currentProjects[idx] = fixed
				delete(byName, strings.TrimSpace(cp.Name))
				byName[strings.TrimSpace(fixed.Name)] = idx
				matchedLegacy[lp.ID] = true
				return nil
			}

			// Legacy project has never been imported: bring the complete project in,
			// not just 小说全文.txt. Keep its old ID if it does not collide.
			imported := lp
			if _, exists := byID[imported.ID]; exists {
				imported.ID = fmt.Sprintf("%d", time.Now().UnixNano())
			}
			imported.UpdatedAt = time.Now()
			if imported.CreatedAt.IsZero() {
				imported.CreatedAt = imported.UpdatedAt
			}
			if er = SaveProject(&imported); er == nil {
				currentProjects = append(currentProjects, imported)
				idx = len(currentProjects) - 1
				byID[imported.ID] = idx
				byName[strings.TrimSpace(imported.Name)] = idx
				matchedLegacy[lp.ID] = true
			}
			return nil
		})
	}

	// Final fallback for very old installations that only have 小说全文.txt and
	// no readable project.json. This preserves chapters, while avoiding duplicate
	// imports when a project.json migration above already represented the folder.
	if len(currentProjects) == 0 {
		for _, base := range legacyLibraryDirs() {
			_ = filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
				if err != nil || info == nil || info.IsDir() || info.Name() != "小说全文.txt" {
					return nil
				}
				folderID := filepath.Base(filepath.Dir(path))
				if matchedLegacy[folderID] {
					return nil
				}
				txt, e := os.ReadFile(path)
				if e != nil {
					return nil
				}
				chs := ParseFullTextChapters(string(txt))
				if len(chs) == 0 {
					return nil
				}
				p := NewProject(folderID)
				p.Chapters = chs
				_ = SaveProject(&p)
				return filepath.SkipDir
			})
			ps, _ := ListProjects()
			if len(ps) > 0 {
				break
			}
		}
	}
}
