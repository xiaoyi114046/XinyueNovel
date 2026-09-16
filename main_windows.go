//go:build windows

package main

import (
	"context"
	_ "embed"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"
)

//go:embed assets/background.bmp
var bgBMP []byte

//go:embed assets/icon.ico
var iconICO []byte

const (
	WS_OVERLAPPED        = 0x00000000
	WS_CAPTION           = 0x00C00000
	WS_SYSMENU           = 0x00080000
	WS_THICKFRAME        = 0x00040000
	WS_MINIMIZEBOX       = 0x00020000
	WS_MAXIMIZEBOX       = 0x00010000
	WS_VISIBLE           = 0x10000000
	WS_CHILD             = 0x40000000
	WS_TABSTOP           = 0x00010000
	WS_VSCROLL           = 0x00200000
	WS_BORDER            = 0x00800000
	WS_CLIPCHILDREN      = 0x02000000
	WS_EX_CLIENTEDGE     = 0x00000200
	ES_LEFT              = 0x0000
	ES_MULTILINE         = 0x0004
	ES_AUTOVSCROLL       = 0x0040
	ES_WANTRETURN        = 0x1000
	ES_PASSWORD          = 0x0020
	ES_READONLY          = 0x0800
	BS_PUSHBUTTON        = 0x00000000
	BS_AUTOCHECKBOX      = 0x00000003
	BS_OWNERDRAW         = 0x0000000B
	SS_ICON              = 0x00000003
	STM_SETICON          = 0x0170
	CBS_DROPDOWN         = 0x0002
	CBS_DROPDOWNLIST     = 0x0003
	LBS_NOTIFY           = 0x0001
	LBS_NOINTEGRALHEIGHT = 0x0100
	SW_HIDE              = 0
	SW_SHOW              = 5
	WM_CREATE            = 0x0001
	WM_DESTROY           = 0x0002
	WM_SIZE              = 0x0005
	WM_PAINT             = 0x000F
	WM_CLOSE             = 0x0010
	WM_COMMAND           = 0x0111
	WM_DRAWITEM          = 0x002B
	WM_CTLCOLORSTATIC    = 0x0138
	WM_CTLCOLOREDIT      = 0x0133
	WM_CTLCOLORLISTBOX   = 0x0134
	WM_ERASEBKGND        = 0x0014
	WM_SETFONT           = 0x0030
	WM_SETICON           = 0x0080
	WM_GETTEXT           = 0x000D
	WM_GETTEXTLENGTH     = 0x000E
	WM_GETMINMAXINFO     = 0x0024
	WM_APP               = 0x8000
	WM_ASYNC_DONE        = WM_APP + 1
	BM_GETCHECK          = 0x00F0
	BM_SETCHECK          = 0x00F1
	BST_CHECKED          = 1
	EM_SETPASSWORDCHAR   = 0x00CC
	CB_ADDSTRING         = 0x0143
	CB_GETCURSEL         = 0x0147
	CB_SETCURSEL         = 0x014E
	CB_GETLBTEXTLEN      = 0x0149
	CB_GETLBTEXT         = 0x0148
	CB_RESETCONTENT      = 0x014B
	CB_SELECTSTRING      = 0x014D
	LB_ADDSTRING         = 0x0180
	LB_RESETCONTENT      = 0x0184
	LB_GETCURSEL         = 0x0188
	LB_SETCURSEL         = 0x0186
	LB_GETTEXT           = 0x0189
	BN_CLICKED           = 0
	CBN_SELCHANGE        = 1
	LBN_SELCHANGE        = 1
	MB_OK                = 0
	MB_ICONERROR         = 0x10
	MB_ICONINFORMATION   = 0x40
	MB_ICONQUESTION      = 0x20
	MB_YESNO             = 0x4
	IDYES                = 6
	COLOR_WINDOW         = 5
	IDC_ARROW            = 32512
	IMAGE_BITMAP         = 0
	IMAGE_ICON           = 1
	LR_LOADFROMFILE      = 0x10
	LR_DEFAULTSIZE       = 0x40
	SRCCOPY              = 0x00CC0020
	HALFTONE             = 4
	TRANSPARENT          = 1
	NULL_BRUSH           = 5
	NULL_PEN             = 8
	DT_CENTER            = 0x1
	DT_VCENTER           = 0x4
	DT_SINGLELINE        = 0x20
	DT_END_ELLIPSIS      = 0x8000
	CF_UNICODETEXT       = 13
	GMEM_MOVEABLE        = 0x0002
	OFN_OVERWRITEPROMPT  = 0x2
	OFN_PATHMUSTEXIST    = 0x800
	OFN_FILEMUSTEXIST    = 0x1000
	ICON_SMALL           = 0
	ICON_BIG             = 1
)

const (
	pageHome = iota
	pageNovel
	pageWorld
	pageProject
	pageSettings
	pageCount
)

const (
	idNavHome = 2001 + iota
	idNavNovel
	idNavWorld
	idNavProject
	idNavSettings
	idHomeProject
	idHomeTitle
	idHomeTarget
	idHomeRecent
	idHomeInstructions
	idHomeContext
	idHomeRecentToContext
	idHomeGenerate
	idHomeStop
	idHomeSaveChapter
	idHomeCopy
	idHomeClear
	idHomeOutput
	idSetProject
	idSetGenre
	idSetFlow
	idSetStyle
	idSetPOV
	idSetPace
	idSetProtagonist
	idSetCharacters
	idSetPlot
	idSetSaveNovel
	idWorldProject
	idWorldWorld
	idWorldRealms
	idWorldCurrent
	idWorldOutline
	idWorldVolume
	idWorldChapter
	idWorldGenRealms
	idWorldGenOutline
	idWorldSave
	idProjNewName
	idProjNew
	idProjCombo
	idProjList
	idProjChapterTitle
	idProjChapterBody
	idProjRename
	idProjSaveEdit
	idProjDeleteChapter
	idProjAsContext
	idProjExport
	idProjOpenFolder
	idProjDeleteProject
	idProjRenameProject
	idAPIKey
	idAPIShow
	idAPIEndpoint
	idAPIModel
	idAPIThinking
	idAPISave
	idAPITest
	idAPIClear
)

type POINT struct{ X, Y int32 }
type RECT struct{ Left, Top, Right, Bottom int32 }
type MSG struct {
	Hwnd           syscall.Handle
	Message        uint32
	WParam, LParam uintptr
	Time           uint32
	Pt             POINT
}
type WNDCLASSEX struct {
	CbSize, Style                            uint32
	LpfnWndProc                              uintptr
	CbClsExtra, CbWndExtra                   int32
	HInstance, HIcon, HCursor, HbrBackground syscall.Handle
	LpszMenuName, LpszClassName              *uint16
	HIconSm                                  syscall.Handle
}
type PAINTSTRUCT struct {
	Hdc                  syscall.Handle
	FErase               int32
	RcPaint              RECT
	FRestore, FIncUpdate int32
	RgbReserved          [32]byte
}
type MINMAXINFO struct{ PtReserved, PtMaxSize, PtMaxPosition, PtMinTrackSize, PtMaxTrackSize POINT }
type DRAWITEMSTRUCT struct {
	CtlType, CtlID                uint32
	ItemID, ItemAction, ItemState uint32
	HwndItem, HDC                 syscall.Handle
	RcItem                        RECT
	ItemData                      uintptr
}
type BITMAP struct {
	Type                      int32
	Width, Height, WidthBytes int32
	Planes, BitPixel          uint16
	Bits                      unsafe.Pointer
}
type OPENFILENAME struct {
	LStructSize                    uint32
	HwndOwner, HInstance           syscall.Handle
	LpstrFilter, LpstrCustomFilter *uint16
	NMaxCustFilter, NFilterIndex   uint32
	LpstrFile                      *uint16
	NMaxFile                       uint32
	LpstrFileTitle                 *uint16
	NMaxFileTitle                  uint32
	LpstrInitialDir, LpstrTitle    *uint16
	Flags                          uint32
	NFileOffset, NFileExtension    uint16
	LpstrDefExt                    *uint16
	LCustData, LpfnHook            uintptr
	LpTemplateName                 *uint16
	PvReserved                     unsafe.Pointer
	DwReserved, FlagsEx            uint32
}

var (
	user32                = syscall.NewLazyDLL("user32.dll")
	kernel32              = syscall.NewLazyDLL("kernel32.dll")
	gdi32                 = syscall.NewLazyDLL("gdi32.dll")
	comdlg32              = syscall.NewLazyDLL("comdlg32.dll")
	shell32               = syscall.NewLazyDLL("shell32.dll")
	pRegisterClassExW     = user32.NewProc("RegisterClassExW")
	pCreateWindowExW      = user32.NewProc("CreateWindowExW")
	pDefWindowProcW       = user32.NewProc("DefWindowProcW")
	pShowWindow           = user32.NewProc("ShowWindow")
	pUpdateWindow         = user32.NewProc("UpdateWindow")
	pGetMessageW          = user32.NewProc("GetMessageW")
	pTranslateMessage     = user32.NewProc("TranslateMessage")
	pDispatchMessageW     = user32.NewProc("DispatchMessageW")
	pPostQuitMessage      = user32.NewProc("PostQuitMessage")
	pPostMessageW         = user32.NewProc("PostMessageW")
	pSendMessageW         = user32.NewProc("SendMessageW")
	pSetWindowTextW       = user32.NewProc("SetWindowTextW")
	pGetWindowTextW       = user32.NewProc("GetWindowTextW")
	pGetWindowTextLengthW = user32.NewProc("GetWindowTextLengthW")
	pMessageBoxW          = user32.NewProc("MessageBoxW")
	pEnableWindow         = user32.NewProc("EnableWindow")
	pGetClientRect        = user32.NewProc("GetClientRect")
	pLoadCursorW          = user32.NewProc("LoadCursorW")
	pLoadImageW           = user32.NewProc("LoadImageW")
	pInvalidateRect       = user32.NewProc("InvalidateRect")
	pBeginPaint           = user32.NewProc("BeginPaint")
	pEndPaint             = user32.NewProc("EndPaint")
	pFillRect             = user32.NewProc("FillRect")
	pDrawTextW            = user32.NewProc("DrawTextW")
	pSetFocus             = user32.NewProc("SetFocus")
	pOpenClipboard        = user32.NewProc("OpenClipboard")
	pEmptyClipboard       = user32.NewProc("EmptyClipboard")
	pSetClipboardData     = user32.NewProc("SetClipboardData")
	pCloseClipboard       = user32.NewProc("CloseClipboard")
	pSetProcessDPIAware   = user32.NewProc("SetProcessDPIAware")
	pGetModuleHandleW     = kernel32.NewProc("GetModuleHandleW")
	pGlobalAlloc          = kernel32.NewProc("GlobalAlloc")
	pGlobalLock           = kernel32.NewProc("GlobalLock")
	pGlobalUnlock         = kernel32.NewProc("GlobalUnlock")
	pCreateFontW          = gdi32.NewProc("CreateFontW")
	pCreateSolidBrush     = gdi32.NewProc("CreateSolidBrush")
	pDeleteObject         = gdi32.NewProc("DeleteObject")
	pGetStockObject       = gdi32.NewProc("GetStockObject")
	pSetTextColor         = gdi32.NewProc("SetTextColor")
	pSetBkColor           = gdi32.NewProc("SetBkColor")
	pSetBkMode            = gdi32.NewProc("SetBkMode")
	pRoundRect            = gdi32.NewProc("RoundRect")
	pCreateCompatibleDC   = gdi32.NewProc("CreateCompatibleDC")
	pDeleteDC             = gdi32.NewProc("DeleteDC")
	pSelectObject         = gdi32.NewProc("SelectObject")
	pStretchBlt           = gdi32.NewProc("StretchBlt")
	pSetStretchBltMode    = gdi32.NewProc("SetStretchBltMode")
	pGetObjectW           = gdi32.NewProc("GetObjectW")
	pGetSaveFileNameW     = comdlg32.NewProc("GetSaveFileNameW")
	pGetOpenFileNameW     = comdlg32.NewProc("GetOpenFileNameW")
	pShellExecuteW        = shell32.NewProc("ShellExecuteW")
)

func rgb(r, g, b byte) uintptr { return uintptr(uint32(r) | uint32(g)<<8 | uint32(b)<<16) }
func ptr(s string) *uint16     { return syscall.StringToUTF16Ptr(cleanNUL(s)) }
func send(h syscall.Handle, m uint32, w, l uintptr) uintptr {
	r, _, _ := pSendMessageW.Call(uintptr(h), uintptr(m), w, l)
	return r
}
func setText(h syscall.Handle, s string) {
	pSetWindowTextW.Call(uintptr(h), uintptr(unsafe.Pointer(ptr(cleanNUL(s)))))
}
func getText(h syscall.Handle) string {
	n, _, _ := pGetWindowTextLengthW.Call(uintptr(h))
	if n == 0 {
		return ""
	}
	b := make([]uint16, int(n)+1)
	pGetWindowTextW.Call(uintptr(h), uintptr(unsafe.Pointer(&b[0])), n+1)
	return syscall.UTF16ToString(b)
}
func show(h syscall.Handle, v bool) {
	s := uintptr(SW_HIDE)
	if v {
		s = SW_SHOW
	}
	pShowWindow.Call(uintptr(h), s)
}
func enable(h syscall.Handle, v bool) {
	x := uintptr(0)
	if v {
		x = 1
	}
	pEnableWindow.Call(uintptr(h), x)
}
func msgBox(title, text string, flags uintptr) int {
	r, _, _ := pMessageBoxW.Call(uintptr(mainHwnd), uintptr(unsafe.Pointer(ptr(text))), uintptr(unsafe.Pointer(ptr(title))), flags)
	return int(r)
}

var mainHwnd syscall.Handle
var bgBitmap, appIcon syscall.Handle
var fontNormal, fontSmall, fontTitle, fontNav, fontBig syscall.Handle
var brushWhite, brushEdit, brushSidebar, brushPink, brushPinkDark, brushSoft, brushTransparent syscall.Handle
var currentPage = pageHome
var sidebarStatics = map[syscall.Handle]bool{}
var pageHandles [pageCount][]syscall.Handle
var navButtons []syscall.Handle
var hStatus, hProjectCard syscall.Handle

var (
	hHomeProject, hHomeTitle, hHomeTarget, hHomeRecent, hHomeInstructions, hHomeContext, hHomeGenerate, hHomeStop, hHomeSaveChapter, hHomeOutput syscall.Handle
	hSetProject, hSetGenre, hSetFlow, hSetStyle, hSetPOV, hSetPace, hSetProtagonist, hSetCharacters, hSetPlot                                    syscall.Handle
	hWorldProject, hWorldWorld, hWorldRealms, hWorldCurrent, hWorldOutline, hWorldVolume, hWorldChapter                                          syscall.Handle
	hProjNewName, hProjCombo, hProjRenameName, hProjList, hProjTitle, hProjBody                                                                  syscall.Handle
	hAPIKey, hAPIShow, hAPIEndpoint, hAPIModel, hAPIThinking                                                                                     syscall.Handle
)

var current Project
var projectList []Project
var settings APISettings
var apiKey string
var httpClient = &http.Client{Timeout: 8 * time.Minute}
var cancelMu sync.Mutex
var currentCancel context.CancelFunc
var asyncMu sync.Mutex
var asyncKind, asyncText string
var asyncErr error
var asyncUsage Usage

func addPageHandle(page int, h syscall.Handle) syscall.Handle {
	if page >= 0 && page < pageCount {
		pageHandles[page] = append(pageHandles[page], h)
	}
	return h
}
func createControl(ex uint32, class, text string, style uint32, x, y, w, h int32, parent syscall.Handle, id int) syscall.Handle {
	r, _, _ := pCreateWindowExW.Call(uintptr(ex), uintptr(unsafe.Pointer(ptr(class))), uintptr(unsafe.Pointer(ptr(text))), uintptr(style), uintptr(x), uintptr(y), uintptr(w), uintptr(h), uintptr(parent), uintptr(id), 0, 0)
	hw := syscall.Handle(r)
	if hw != 0 && fontNormal != 0 {
		send(hw, WM_SETFONT, uintptr(fontNormal), 1)
	}
	return hw
}
func label(page int, text string, x, y, w, h int32, big bool) syscall.Handle {
	hw := addPageHandle(page, createControl(0, "STATIC", text, WS_CHILD|WS_VISIBLE, x, y, w, h, mainHwnd, 0))
	if big {
		send(hw, WM_SETFONT, uintptr(fontTitle), 1)
	}
	return hw
}
func edit(page int, text string, x, y, w, h int32, multi bool, id int) syscall.Handle {
	st := uint32(WS_CHILD | WS_VISIBLE | WS_TABSTOP | WS_BORDER | ES_LEFT)
	if multi {
		st |= ES_MULTILINE | ES_AUTOVSCROLL | ES_WANTRETURN | WS_VSCROLL
	}
	return addPageHandle(page, createControl(0, "EDIT", text, st, x, y, w, h, mainHwnd, id))
}
func button(page int, text string, x, y, w, h int32, id int) syscall.Handle {
	return addPageHandle(page, createControl(0, "BUTTON", text, WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_OWNERDRAW, x, y, w, h, mainHwnd, id))
}
func combo(page int, x, y, w, h int32, id int, items []string, editable bool) syscall.Handle {
	st := uint32(WS_CHILD | WS_VISIBLE | WS_TABSTOP | CBS_DROPDOWNLIST)
	if editable {
		st = WS_CHILD | WS_VISIBLE | WS_TABSTOP | CBS_DROPDOWN
	}
	hw := addPageHandle(page, createControl(0, "COMBOBOX", "", st, x, y, w, h, mainHwnd, id))
	for _, s := range items {
		send(hw, CB_ADDSTRING, 0, uintptr(unsafe.Pointer(ptr(s))))
	}
	send(hw, CB_SETCURSEL, 0, 0)
	return hw
}
func listbox(page int, x, y, w, h int32, id int) syscall.Handle {
	return addPageHandle(page, createControl(WS_EX_CLIENTEDGE, "LISTBOX", "", WS_CHILD|WS_VISIBLE|WS_TABSTOP|WS_VSCROLL|LBS_NOTIFY|LBS_NOINTEGRALHEIGHT, x, y, w, h, mainHwnd, id))
}
func staticGlobal(text string, x, y, w, h int32, big bool) syscall.Handle {
	hw := createControl(0, "STATIC", text, WS_CHILD|WS_VISIBLE, x, y, w, h, mainHwnd, 0)
	if big {
		send(hw, WM_SETFONT, uintptr(fontBig), 1)
	}
	if x < 270 {
		sidebarStatics[hw] = true
	}
	return hw
}
func setComboText(h syscall.Handle, s string) {
	if s == "" {
		return
	}
	r := send(h, CB_SELECTSTRING, ^uintptr(0), uintptr(unsafe.Pointer(ptr(s))))
	if r == ^uintptr(0) {
		setText(h, s)
	}
}
func comboText(h syscall.Handle) string {
	idx := int(send(h, CB_GETCURSEL, 0, 0))
	if idx >= 0 && idx < 10000 {
		ln := send(h, CB_GETLBTEXTLEN, uintptr(idx), 0)
		if ln < 1<<20 {
			buf := make([]uint16, int(ln)+1)
			send(h, CB_GETLBTEXT, uintptr(idx), uintptr(unsafe.Pointer(&buf[0])))
			return syscall.UTF16ToString(buf)
		}
	}
	return getText(h)
}

func initFonts() {
	mk := func(size int, weight int, name string) syscall.Handle {
		r, _, _ := pCreateFontW.Call(uintptr(int32(-size)), 0, 0, 0, uintptr(weight), 0, 0, 0, 1, 0, 0, 5, 0, uintptr(unsafe.Pointer(ptr(name))))
		return syscall.Handle(r)
	}
	fontNormal = mk(17, 400, "Microsoft YaHei UI")
	fontSmall = mk(14, 400, "Microsoft YaHei UI")
	fontTitle = mk(28, 600, "Microsoft YaHei UI")
	fontBig = mk(32, 700, "Microsoft YaHei UI")
	fontNav = mk(19, 500, "Microsoft YaHei UI")
}
func ensureEmbeddedAssets() (string, string) {
	dir := filepath.Join(appBaseDir(), "assets")
	_ = os.MkdirAll(dir, 0755)
	bp := filepath.Join(dir, "background.bmp")
	ip := filepath.Join(dir, "icon.ico")
	_ = os.WriteFile(bp, bgBMP, 0644)
	_ = os.WriteFile(ip, iconICO, 0644)
	return bp, ip
}
func loadAssets() {
	bp, ip := ensureEmbeddedAssets()
	r, _, _ := pLoadImageW.Call(0, uintptr(unsafe.Pointer(ptr(bp))), IMAGE_BITMAP, 0, 0, LR_LOADFROMFILE)
	bgBitmap = syscall.Handle(r)
	r, _, _ = pLoadImageW.Call(0, uintptr(unsafe.Pointer(ptr(ip))), IMAGE_ICON, 64, 64, LR_LOADFROMFILE)
	appIcon = syscall.Handle(r)
}

func buildUI() {
	// Sidebar brand and navigation. Keep the brand on a clean, high-contrast
	// sidebar instead of placing it over the decorative reference artwork.
	if appIcon != 0 {
		brandIcon := createControl(0, "STATIC", "", WS_CHILD|WS_VISIBLE|SS_ICON, 24, 24, 56, 56, mainHwnd, 0)
		sidebarStatics[brandIcon] = true
		send(brandIcon, STM_SETICON, uintptr(appIcon), 0)
	}
	brand := staticGlobal("心阅小说", 92, 24, 150, 42, true)
	send(brand, WM_SETFONT, uintptr(fontBig), 1)
	ver := staticGlobal("v"+AppVersion, 94, 67, 120, 24, false)
	send(ver, WM_SETFONT, uintptr(fontSmall), 1)
	tagline := staticGlobal("每一次心动 · 都有故事", 28, 101, 214, 24, false)
	send(tagline, WM_SETFONT, uintptr(fontSmall), 1)
	navTexts := []string{"✦  续写主页", "⚙  小说设定", "▣  世界观 · 大纲", "▤  项目 · 章节", "⚙  设置"}
	navIDs := []int{idNavHome, idNavNovel, idNavWorld, idNavProject, idNavSettings}
	for i, t := range navTexts {
		h := createControl(0, "BUTTON", t, WS_CHILD|WS_VISIBLE|BS_OWNERDRAW, 20, int32(145+i*69), 232, 54, mainHwnd, navIDs[i])
		send(h, WM_SETFONT, uintptr(fontNav), 1)
		navButtons = append(navButtons, h)
	}
	staticGlobal("当前项目", 28, 506, 190, 28, false)
	hProjectCard = staticGlobal("尚未加载项目", 28, 542, 214, 104, false)
	send(hProjectCard, WM_SETFONT, uintptr(fontNormal), 1)
	hStatus = staticGlobal("状态：正在启动…", 286, 944, 1260, 28, false)
	send(hStatus, WM_SETFONT, uintptr(fontSmall), 1)

	// HOME
	label(pageHome, "✦ 续写主页", 300, 168, 250, 38, true)
	label(pageHome, "集中完成本章续写，保存后自动进入项目历史并更新小说全文 TXT。", 300, 210, 820, 26, false)
	label(pageHome, "项目", 300, 244, 60, 28, false)
	hHomeProject = combo(pageHome, 365, 238, 315, 220, idHomeProject, nil, false)
	label(pageHome, "章节标题", 710, 244, 90, 28, false)
	hHomeTitle = edit(pageHome, "", 800, 238, 250, 34, false, idHomeTitle)
	label(pageHome, "目标字数", 1080, 244, 80, 28, false)
	hHomeTarget = edit(pageHome, "3500", 1160, 238, 90, 34, false, idHomeTarget)
	label(pageHome, "最近", 1270, 244, 55, 28, false)
	hHomeRecent = combo(pageHome, 1325, 238, 130, 180, idHomeRecent, []string{"1章", "2章", "3章", "5章", "8章"}, false)
	send(hHomeRecent, CB_SETCURSEL, 2, 0)
	label(pageHome, "★ 本章特殊要求", 300, 294, 250, 28, false)
	hHomeInstructions = edit(pageHome, "", 300, 324, 1170, 88, true, idHomeInstructions)
	label(pageHome, "▤ 续写上下文（可手工编辑）", 300, 426, 320, 28, false)
	hHomeContext = edit(pageHome, "", 300, 456, 1170, 128, true, idHomeContext)
	button(pageHome, "最近章节 → 上下文", 300, 598, 220, 48, idHomeRecentToContext)
	hHomeGenerate = button(pageHome, "✦ 开始续写", 535, 598, 205, 48, idHomeGenerate)
	hHomeStop = button(pageHome, "■ 停止", 755, 598, 125, 48, idHomeStop)
	enable(hHomeStop, false)
	button(pageHome, "▣ 保存本章入库", 895, 598, 205, 48, idHomeSaveChapter)
	button(pageHome, "复制结果", 1115, 598, 165, 48, idHomeCopy)
	button(pageHome, "清空结果", 1295, 598, 175, 48, idHomeClear)
	label(pageHome, "▤ 续写结果", 300, 658, 180, 28, false)
	hHomeOutput = edit(pageHome, "", 300, 688, 1170, 230, true, idHomeOutput)

	// NOVEL SETTINGS
	label(pageNovel, "⚙ 小说设定", 300, 168, 260, 38, true)
	label(pageNovel, "选择项目后会立即载入该小说的流派、文风、人物与剧情设定。", 300, 210, 900, 26, false)
	label(pageNovel, "当前项目", 300, 248, 90, 26, false)
	hSetProject = combo(pageNovel, 400, 242, 520, 220, idSetProject, nil, false)
	label(pageNovel, "切换项目会同步更新所有页面", 950, 248, 360, 26, false)
	label(pageNovel, "小说类型", 300, 305, 90, 26, false)
	hSetGenre = combo(pageNovel, 400, 299, 240, 200, idSetGenre, []string{"玄幻", "仙侠", "都市", "科幻", "历史", "悬疑", "言情", "校园", "末世", "游戏", "轻小说", "自定义"}, true)
	label(pageNovel, "网文流派", 680, 305, 90, 26, false)
	hSetFlow = combo(pageNovel, 780, 299, 260, 240, idSetFlow, []string{"迪化流", "幕后流", "系统流", "苟道流", "无敌流", "反派流", "诸天流", "签到流", "凡人流", "经营流", "种田流", "扮猪吃虎", "重生流", "穿越流", "群像流"}, true)
	label(pageNovel, "文风", 1080, 305, 65, 26, false)
	hSetStyle = combo(pageNovel, 1150, 299, 320, 240, idSetStyle, []string{"情感细腻风", "宏大玄幻·古意磅礴", "稳健升级·情感清晰", "快节奏爽文·强钩子", "轻松吐槽·反差喜剧", "古典仙侠·意境留白", "自定义"}, true)
	label(pageNovel, "叙事视角", 300, 362, 90, 26, false)
	hSetPOV = combo(pageNovel, 400, 356, 240, 200, idSetPOV, []string{"第三人称", "第一人称", "多视角"}, true)
	label(pageNovel, "节奏", 680, 362, 60, 26, false)
	hSetPace = combo(pageNovel, 780, 356, 260, 200, idSetPace, []string{"张弛有度", "快节奏", "慢热细腻", "强冲突", "日常治愈"}, true)
	label(pageNovel, "主角", 1080, 362, 60, 26, false)
	hSetProtagonist = edit(pageNovel, "", 1150, 356, 320, 34, false, idSetProtagonist)
	label(pageNovel, "人物设定 / 关系", 300, 420, 220, 28, false)
	hSetCharacters = edit(pageNovel, "", 300, 453, 560, 185, true, idSetCharacters)
	label(pageNovel, "剧情脉络", 890, 420, 160, 28, false)
	hSetPlot = edit(pageNovel, "", 890, 453, 580, 185, true, idSetPlot)
	button(pageNovel, "保存小说设定", 300, 658, 200, 48, idSetSaveNovel)
	label(pageNovel, "情感细腻风提示", 300, 725, 220, 28, false)
	label(pageNovel, "强调心理波动、微表情、动作细节、欲言又止、关系张力与情绪递进；关键情绪节点用环境和感官细节承托，避免空泛煽情。", 300, 758, 1120, 62, false)

	// WORLD
	label(pageWorld, "▣ 世界观 · 大纲", 300, 168, 320, 38, true)
	label(pageWorld, "选择项目后会立即载入该小说保存过的世界观、境界、总纲、卷纲与本章小纲。", 300, 210, 1000, 26, false)
	label(pageWorld, "当前项目", 300, 248, 90, 26, false)
	hWorldProject = combo(pageWorld, 400, 242, 520, 220, idWorldProject, nil, false)
	label(pageWorld, "世界观", 300, 305, 100, 28, false)
	hWorldWorld = edit(pageWorld, "", 300, 336, 550, 145, true, idWorldWorld)
	label(pageWorld, "境界体系", 880, 305, 120, 28, false)
	hWorldRealms = edit(pageWorld, "", 880, 336, 590, 145, true, idWorldRealms)
	label(pageWorld, "当前境界", 880, 490, 90, 28, false)
	hWorldCurrent = edit(pageWorld, "", 980, 485, 250, 34, false, idWorldCurrent)
	button(pageWorld, "AI 生成境界", 1245, 482, 225, 42, idWorldGenRealms)
	label(pageWorld, "总纲", 300, 500, 100, 28, false)
	hWorldOutline = edit(pageWorld, "", 300, 532, 550, 150, true, idWorldOutline)
	label(pageWorld, "卷纲", 880, 535, 100, 28, false)
	hWorldVolume = edit(pageWorld, "", 880, 567, 590, 115, true, idWorldVolume)
	label(pageWorld, "本章小纲", 300, 700, 120, 28, false)
	hWorldChapter = edit(pageWorld, "", 300, 732, 880, 135, true, idWorldChapter)
	button(pageWorld, "AI 生成总纲/卷纲", 1200, 732, 270, 48, idWorldGenOutline)
	button(pageWorld, "保存世界观与大纲", 1200, 795, 270, 48, idWorldSave)

	// PROJECTS
	label(pageProject, "▤ 项目 · 章节", 300, 168, 300, 38, true)
	label(pageProject, "项目独立保存；可切换、重命名、删除，历史章节修改后会自动重建小说全文 TXT。", 300, 210, 1100, 26, false)
	label(pageProject, "新项目名称", 300, 244, 100, 28, false)
	hProjNewName = edit(pageProject, "", 410, 238, 330, 34, false, idProjNewName)
	button(pageProject, "新建项目", 755, 236, 145, 38, idProjNew)
	label(pageProject, "当前项目", 300, 292, 90, 28, false)
	hProjCombo = combo(pageProject, 400, 286, 350, 220, idProjCombo, nil, false)
	label(pageProject, "项目名称", 780, 292, 90, 28, false)
	hProjRenameName = edit(pageProject, "", 875, 286, 310, 34, false, 0)
	button(pageProject, "重命名项目", 1200, 284, 140, 38, idProjRenameProject)
	button(pageProject, "删除项目", 1350, 284, 120, 38, idProjDeleteProject)
	label(pageProject, "历史章节", 300, 345, 120, 28, false)
	hProjList = listbox(pageProject, 300, 379, 360, 455, idProjList)
	label(pageProject, "章节标题", 690, 345, 100, 28, false)
	hProjTitle = edit(pageProject, "", 790, 339, 680, 34, false, idProjChapterTitle)
	label(pageProject, "章节正文（可修改）", 690, 395, 220, 28, false)
	hProjBody = edit(pageProject, "", 690, 427, 780, 285, true, idProjChapterBody)
	button(pageProject, "仅重命名标题", 690, 730, 170, 44, idProjRename)
	button(pageProject, "保存章节修改", 875, 730, 170, 44, idProjSaveEdit)
	button(pageProject, "删除本章", 1060, 730, 140, 44, idProjDeleteChapter)
	button(pageProject, "作为续写上下文", 1215, 730, 180, 44, idProjAsContext)
	button(pageProject, "导出完整 TXT", 690, 790, 170, 44, idProjExport)
	button(pageProject, "打开项目目录", 875, 790, 170, 44, idProjOpenFolder)

	// SETTINGS
	label(pageSettings, "⚙ 设置", 300, 168, 260, 38, true)
	label(pageSettings, "DeepSeek API Key 可安全保存在本机；首次输入并测试成功后，以后会自动读取。", 300, 210, 1040, 26, false)
	label(pageSettings, "DeepSeek API Key", 300, 260, 150, 28, false)
	hAPIKey = edit(pageSettings, "", 465, 254, 720, 36, false, idAPIKey) // mask below
	style := uint32(send(hAPIKey, 0x00CB, 0, 0))
	_ = style
	send(hAPIKey, EM_SETPASSWORDCHAR, uintptr('●'), 0)
	hAPIShow = createControl(0, "BUTTON", "显示 Key", WS_CHILD|WS_VISIBLE|BS_AUTOCHECKBOX, 1205, 256, 130, 32, mainHwnd, idAPIShow)
	addPageHandle(pageSettings, hAPIShow)
	label(pageSettings, "API 地址", 300, 320, 100, 28, false)
	hAPIEndpoint = edit(pageSettings, DefaultEndpoint, 465, 314, 870, 36, false, idAPIEndpoint)
	label(pageSettings, "模型", 300, 380, 100, 28, false)
	hAPIModel = combo(pageSettings, 465, 374, 420, 220, idAPIModel, []string{"deepseek-chat", "deepseek-reasoner", "deepseek-flash", "deepseek-v4-pro"}, true)
	label(pageSettings, "思考模式", 930, 380, 100, 28, false)
	hAPIThinking = combo(pageSettings, 1040, 374, 295, 220, idAPIThinking, []string{"关闭", "Low", "High", "Max"}, false)
	button(pageSettings, "保存到本机", 465, 444, 180, 46, idAPISave)
	button(pageSettings, "测试 API 连接", 665, 444, 200, 46, idAPITest)
	button(pageSettings, "清除本机 Key", 885, 444, 180, 46, idAPIClear)
	label(pageSettings, "本机保存说明", 300, 535, 160, 28, false)
	label(pageSettings, "API Key 使用 Windows DPAPI 加密，只能由当前 Windows 用户解密；不会写入小说项目 JSON 或导出的 TXT。", 300, 570, 1035, 55, false)
	label(pageSettings, "版本", 300, 675, 80, 28, false)
	label(pageSettings, AppName+" v"+AppVersion+" · UI 参考你提供的粉紫梦幻设计，软件图标来自你提供的图标稿。", 390, 675, 980, 28, false)

	loadAllStateToUI()
	switchPage(pageHome)
}

func switchPage(p int) {
	if p < 0 || p >= pageCount {
		return
	}
	currentPage = p
	for pg := 0; pg < pageCount; pg++ {
		for _, h := range pageHandles[pg] {
			show(h, pg == p)
		}
	}
	for _, h := range navButtons {
		pInvalidateRect.Call(uintptr(h), 0, 1)
	}
	setStatus("状态：" + pageName(p))
	pInvalidateRect.Call(uintptr(mainHwnd), 0, 1)
}
func pageName(p int) string {
	switch p {
	case pageHome:
		return "续写主页"
	case pageNovel:
		return "小说设定"
	case pageWorld:
		return "世界观 · 大纲"
	case pageProject:
		return "项目 · 章节"
	case pageSettings:
		return "设置"
	}
	return ""
}
func setStatus(s string) { setText(hStatus, s) }

func refreshProjects() {
	projectList, _ = ListProjects()
	combos := []syscall.Handle{hHomeProject, hSetProject, hWorldProject, hProjCombo}
	for _, h := range combos {
		if h != 0 {
			send(h, CB_RESETCONTENT, 0, 0)
		}
	}
	for _, p := range projectList {
		name := strings.TrimSpace(p.Name)
		if name == "" {
			name = "未命名小说"
		}
		t := fmt.Sprintf("%s（%d章）", name, len(p.Chapters))
		for _, h := range combos {
			if h != 0 {
				send(h, CB_ADDSTRING, 0, uintptr(unsafe.Pointer(ptr(t))))
			}
		}
	}
	idx := 0
	for i, p := range projectList {
		if p.ID == current.ID {
			idx = i
			break
		}
	}
	if len(projectList) > 0 {
		for _, h := range combos {
			if h != 0 {
				send(h, CB_SETCURSEL, uintptr(idx), 0)
			}
		}
	}
	updateProjectCard()
}
func updateProjectCard() {
	if current.ID == "" {
		setText(hProjectCard, "尚未加载项目")
		return
	}
	setText(hProjectCard, fmt.Sprintf("%s\r\n\r\n共 %d 章\r\n%s · %s\r\n继续书写，让故事更精彩…", current.Name, len(current.Chapters), current.Genre, current.Style))
}
func loadAllStateToUI() {
	migrateLegacyBestEffort()
	settings, _ = LoadSettings()
	ps, _ := ListProjects()
	if len(ps) == 0 {
		p := NewProject("我的小说")
		_ = SaveProject(&p)
		ps = []Project{p}
	}
	current = ps[0]
	if settings.LastProjectID != "" {
		for _, p := range ps {
			if p.ID == settings.LastProjectID {
				current = p
				break
			}
		}
	}
	if settings.APIKeyCipher != "" {
		if k, e := unprotectKey(settings.APIKeyCipher); e == nil {
			apiKey = k
		}
	}
	setText(hAPIKey, apiKey)
	setText(hAPIEndpoint, settings.Endpoint)
	setComboText(hAPIModel, settings.Model)
	setComboText(hAPIThinking, settings.Thinking)
	refreshProjects()
	loadCurrentProjectToUI()
}
func loadCurrentProjectToUI() {
	setText(hHomeTitle, fmt.Sprintf("第%d章", NextChapterNo(current)))
	setText(hProjRenameName, current.Name)
	setText(hHomeInstructions, "")
	setText(hHomeContext, "")
	setText(hHomeOutput, current.Draft)
	setComboText(hSetGenre, current.Genre)
	setComboText(hSetFlow, current.Flow)
	setComboText(hSetStyle, current.Style)
	setComboText(hSetPOV, current.POV)
	setComboText(hSetPace, current.Pace)
	setText(hSetProtagonist, current.Protagonist)
	setText(hSetCharacters, current.Characters)
	setText(hSetPlot, current.Plot)
	setText(hWorldWorld, current.World)
	setText(hWorldRealms, current.Realms)
	setText(hWorldCurrent, current.CurrentRealm)
	setText(hWorldOutline, current.Outline)
	setText(hWorldVolume, current.VolumeOutline)
	setText(hWorldChapter, current.ChapterOutline)
	refreshChapterList()
	refreshProjects()
}
func saveNovelFields() bool {
	current.Genre = comboText(hSetGenre)
	current.Flow = comboText(hSetFlow)
	current.Style = comboText(hSetStyle)
	current.POV = comboText(hSetPOV)
	current.Pace = comboText(hSetPace)
	current.Protagonist = getText(hSetProtagonist)
	current.Characters = getText(hSetCharacters)
	current.Plot = getText(hSetPlot)
	if e := SaveProject(&current); e != nil {
		msgBox("保存失败", e.Error(), MB_OK|MB_ICONERROR)
		setStatus("状态：小说设定保存失败")
		return false
	}
	refreshProjects()
	setStatus("状态：小说设定已保存")
	return true
}
func saveWorldFields() bool {
	current.World = getText(hWorldWorld)
	current.Realms = getText(hWorldRealms)
	current.CurrentRealm = getText(hWorldCurrent)
	current.Outline = getText(hWorldOutline)
	current.VolumeOutline = getText(hWorldVolume)
	current.ChapterOutline = getText(hWorldChapter)
	if e := SaveProject(&current); e != nil {
		msgBox("保存失败", e.Error(), MB_OK|MB_ICONERROR)
		setStatus("状态：世界观与大纲保存失败")
		return false
	}
	setStatus("状态：世界观与大纲已保存")
	return true
}
func selectProjectByCombo(h syscall.Handle) {
	idx := int(send(h, CB_GETCURSEL, 0, 0))
	if idx < 0 || idx >= len(projectList) {
		return
	}
	p, e := LoadProject(projectList[idx].ID)
	if e != nil {
		msgBox("加载项目失败", e.Error(), MB_OK|MB_ICONERROR)
		return
	}
	current = p
	settings.LastProjectID = current.ID
	if e := SaveSettings(settings); e != nil {
		setStatus("状态：项目已加载，但记住上次项目失败：" + e.Error())
	}
	loadCurrentProjectToUI()
	setStatus("状态：已加载项目：" + current.Name)
}
func refreshChapterList() {
	send(hProjList, LB_RESETCONTENT, 0, 0)
	for _, c := range current.Chapters {
		t := fmt.Sprintf("%d. %s", c.No, c.Title)
		send(hProjList, LB_ADDSTRING, 0, uintptr(unsafe.Pointer(ptr(t))))
	}
	if len(current.Chapters) > 0 {
		send(hProjList, LB_SETCURSEL, uintptr(len(current.Chapters)-1), 0)
		showSelectedChapter()
	} else {
		setText(hProjTitle, "")
		setText(hProjBody, "")
	}
}
func selectedChapterIndex() int {
	i := int(send(hProjList, LB_GETCURSEL, 0, 0))
	if i < 0 || i >= len(current.Chapters) {
		return -1
	}
	return i
}
func showSelectedChapter() {
	i := selectedChapterIndex()
	if i < 0 {
		return
	}
	c := current.Chapters[i]
	setText(hProjTitle, c.Title)
	setText(hProjBody, c.Body)
}

func currentAPISettings() APISettings {
	settings.Endpoint = strings.TrimSpace(getText(hAPIEndpoint))
	settings.Model = strings.TrimSpace(comboText(hAPIModel))
	settings.Thinking = strings.TrimSpace(comboText(hAPIThinking))
	if settings.Endpoint == "" {
		settings.Endpoint = DefaultEndpoint
	}
	return settings
}
func saveKeyLocal(silent bool) error {
	apiKey = strings.TrimSpace(getText(hAPIKey))
	settings = currentAPISettings()
	settings.RememberKey = true
	enc, e := protectKey(apiKey)
	if e != nil {
		return e
	}
	settings.APIKeyCipher = enc
	if e = SaveSettings(settings); e != nil {
		return e
	}
	if !silent {
		msgBox("已保存", "API 设置已安全保存在本机。以后启动会自动读取。", MB_OK|MB_ICONINFORMATION)
	}
	return nil
}

func startAsync(kind string, fn func(context.Context) (string, Usage, error)) {
	cancelMu.Lock()
	if currentCancel != nil {
		currentCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	currentCancel = cancel
	cancelMu.Unlock()
	enable(hHomeGenerate, false)
	enable(hHomeStop, true)
	setStatus("状态：正在处理，请稍候…")
	go func() {
		t, u, e := fn(ctx)
		asyncMu.Lock()
		asyncKind = kind
		asyncText = t
		asyncErr = e
		asyncUsage = u
		asyncMu.Unlock()
		pPostMessageW.Call(uintptr(mainHwnd), WM_ASYNC_DONE, 0, 0)
	}()
}
func finishAsync() {
	asyncMu.Lock()
	kind, text, e, u := asyncKind, asyncText, asyncErr, asyncUsage
	asyncMu.Unlock()
	enable(hHomeGenerate, true)
	enable(hHomeStop, false)
	cancelMu.Lock()
	currentCancel = nil
	cancelMu.Unlock()
	if e != nil {
		setStatus("状态：" + e.Error())
		msgBox("操作失败", e.Error(), MB_OK|MB_ICONERROR)
		return
	}
	switch kind {
	case "novel":
		setText(hHomeOutput, text)
		current.Draft = text
		if e := SaveProject(&current); e != nil {
			msgBox("草稿保存失败", e.Error(), MB_OK|MB_ICONERROR)
			setStatus("状态：续写完成，但草稿保存失败")
			return
		}
		setStatus(fmt.Sprintf("状态：续写完成 · %d tokens", u.TotalTokens))
	case "test":
		if e := saveKeyLocal(true); e != nil {
			msgBox("API 测试成功，但保存 Key 失败", e.Error(), MB_OK|MB_ICONERROR)
			setStatus("状态：API 连接成功，但 Key 本地保存失败")
			return
		}
		msgBox("API 测试成功", "DeepSeek API 连接成功；API Key 已安全保存到本机。\n\n返回："+text, MB_OK|MB_ICONINFORMATION)
		setStatus("状态：API 测试成功，Key 已保存")
	case "outline":
		setText(hWorldOutline, text)
		current.Outline = text
		if e := SaveProject(&current); e != nil {
			msgBox("大纲保存失败", e.Error(), MB_OK|MB_ICONERROR)
			return
		}
		setStatus("状态：AI 大纲已生成并保存")
	case "realms":
		setText(hWorldRealms, text)
		current.Realms = text
		if e := SaveProject(&current); e != nil {
			msgBox("境界保存失败", e.Error(), MB_OK|MB_ICONERROR)
			return
		}
		setStatus("状态：AI 境界体系已生成并保存")
	}
}

func recentCount() int {
	s := comboText(hHomeRecent)
	n := 3
	fmt.Sscanf(s, "%d", &n)
	if n <= 0 {
		n = 3
	}
	return n
}
func beginGenerate() {
	if !saveNovelFields() || !saveWorldFields() {
		return
	}
	target, _ := strconv.Atoi(strings.TrimSpace(getText(hHomeTarget)))
	r := NovelRequest{Project: current, ChapterTitle: getText(hHomeTitle), Instructions: getText(hHomeInstructions), Context: getText(hHomeContext), TargetChars: target}
	s := currentAPISettings()
	key := apiKey
	if strings.TrimSpace(getText(hAPIKey)) != "" {
		key = getText(hAPIKey)
	}
	startAsync("novel", func(ctx context.Context) (string, Usage, error) { return GenerateNovel(ctx, httpClient, s, key, r) })
}
func saveGeneratedChapter() {
	body := strings.TrimSpace(getText(hHomeOutput))
	if body == "" {
		msgBox("无法保存", "续写结果为空。", MB_OK|MB_ICONERROR)
		return
	}
	no := NextChapterNo(current)
	title := strings.TrimSpace(getText(hHomeTitle))
	if title == "" {
		title = fmt.Sprintf("第%d章", no)
	}
	if e := UpsertChapter(&current, no, title, body); e != nil {
		msgBox("保存失败", e.Error(), MB_OK|MB_ICONERROR)
		return
	}
	current.Draft = ""
	if e := SaveProject(&current); e != nil {
		msgBox("保存失败", e.Error(), MB_OK|MB_ICONERROR)
		setStatus("状态：章节入库失败")
		return
	}
	setText(hHomeOutput, "")
	setText(hHomeTitle, fmt.Sprintf("第%d章", NextChapterNo(current)))
	refreshChapterList()
	refreshProjects()
	setStatus(fmt.Sprintf("状态：第 %d 章已保存，小说全文 TXT 已更新", no))
}
func copyText(s string) bool {
	r, _, _ := pOpenClipboard.Call(uintptr(mainHwnd))
	if r == 0 {
		return false
	}
	defer pCloseClipboard.Call()
	pEmptyClipboard.Call()
	u := utf16.Encode([]rune(cleanNUL(s) + "\x00"))
	sz := uintptr(len(u) * 2)
	h, _, _ := pGlobalAlloc.Call(GMEM_MOVEABLE, sz)
	if h == 0 {
		return false
	}
	m, _, _ := pGlobalLock.Call(h)
	if m == 0 {
		return false
	}
	copy(unsafe.Slice((*uint16)(unsafe.Pointer(m)), len(u)), u)
	pGlobalUnlock.Call(h)
	r, _, _ = pSetClipboardData.Call(CF_UNICODETEXT, h)
	return r != 0
}
func chooseSave(title, defext, filter, defaultName string) (string, bool) {
	buf := make([]uint16, 2048)
	nameUTF16 := utf16.Encode([]rune(cleanNUL(defaultName)))
	if len(nameUTF16) >= len(buf) {
		nameUTF16 = nameUTF16[:len(buf)-1]
	}
	copy(buf, nameUTF16)
	f := buildWin32FilterUTF16(filter)
	of := OPENFILENAME{LStructSize: uint32(unsafe.Sizeof(OPENFILENAME{})), HwndOwner: mainHwnd, LpstrFilter: &f[0], LpstrFile: &buf[0], NMaxFile: uint32(len(buf)), LpstrTitle: ptr(title), Flags: OFN_OVERWRITEPROMPT | OFN_PATHMUSTEXIST, LpstrDefExt: ptr(defext)}
	r, _, _ := pGetSaveFileNameW.Call(uintptr(unsafe.Pointer(&of)))
	if r == 0 {
		return "", false
	}
	return syscall.UTF16ToString(buf), true
}
func openFolder(path string) {
	pShellExecuteW.Call(uintptr(mainHwnd), uintptr(unsafe.Pointer(ptr("open"))), uintptr(unsafe.Pointer(ptr(path))), 0, 0, SW_SHOW)
}

func handleCommand(id int, code int) {
	switch id {
	case idNavHome:
		switchPage(pageHome)
	case idNavNovel:
		switchPage(pageNovel)
	case idNavWorld:
		switchPage(pageWorld)
	case idNavProject:
		switchPage(pageProject)
	case idNavSettings:
		switchPage(pageSettings)
	case idHomeProject:
		if code == CBN_SELCHANGE {
			selectProjectByCombo(hHomeProject)
		}
	case idSetProject:
		if code == CBN_SELCHANGE {
			selectProjectByCombo(hSetProject)
		}
	case idWorldProject:
		if code == CBN_SELCHANGE {
			selectProjectByCombo(hWorldProject)
		}
	case idProjCombo:
		if code == CBN_SELCHANGE {
			selectProjectByCombo(hProjCombo)
		}
	case idProjList:
		if code == LBN_SELCHANGE {
			showSelectedChapter()
		}
	case idHomeRecentToContext:
		setText(hHomeContext, RecentContext(current, recentCount()))
		setStatus("状态：最近章节已载入续写上下文")
	case idHomeGenerate:
		beginGenerate()
	case idHomeStop:
		cancelMu.Lock()
		if currentCancel != nil {
			currentCancel()
		}
		cancelMu.Unlock()
		setStatus("状态：正在停止…")
	case idHomeSaveChapter:
		saveGeneratedChapter()
	case idHomeCopy:
		if copyText(getText(hHomeOutput)) {
			setStatus("状态：续写结果已复制")
		} else {
			msgBox("复制失败", "无法打开 Windows 剪贴板，请稍后重试。", MB_OK|MB_ICONERROR)
			setStatus("状态：复制失败")
		}
	case idHomeClear:
		if strings.TrimSpace(getText(hHomeOutput)) != "" && msgBox("确认清空", "只清空当前未入库的续写结果，不会删除历史章节。\n\n确定清空吗？", MB_YESNO|MB_ICONQUESTION) != IDYES {
			return
		}
		setText(hHomeOutput, "")
		current.Draft = ""
		if e := SaveProject(&current); e != nil {
			msgBox("保存失败", e.Error(), MB_OK|MB_ICONERROR)
			setStatus("状态：清空草稿后保存失败")
			return
		}
		setStatus("状态：续写结果已清空")
	case idSetSaveNovel:
		_ = saveNovelFields()
	case idWorldSave:
		_ = saveWorldFields()
	case idWorldGenRealms:
		if !saveNovelFields() || !saveWorldFields() {
			return
		}
		startAsync("realms", func(ctx context.Context) (string, Usage, error) {
			t, e := GenerateRealms(ctx, httpClient, currentAPISettings(), apiKey, current)
			return t, Usage{}, e
		})
	case idWorldGenOutline:
		if !saveNovelFields() || !saveWorldFields() {
			return
		}
		startAsync("outline", func(ctx context.Context) (string, Usage, error) {
			t, e := GenerateOutline(ctx, httpClient, currentAPISettings(), apiKey, current)
			return t, Usage{}, e
		})
	case idProjNew:
		name := strings.TrimSpace(getText(hProjNewName))
		if name == "" {
			msgBox("新建项目", "请填写项目名称。", MB_OK|MB_ICONERROR)
			return
		}
		p := NewProject(name)
		if e := SaveProject(&p); e != nil {
			msgBox("新建失败", e.Error(), MB_OK|MB_ICONERROR)
			return
		}
		current = p
		settings.LastProjectID = current.ID
		_ = SaveSettings(settings)
		setText(hProjNewName, "")
		loadCurrentProjectToUI()
		setStatus("状态：已新建项目 " + name)
	case idProjRenameProject:
		name := strings.TrimSpace(getText(hProjRenameName))
		if name == "" {
			msgBox("重命名项目", "项目名称不能为空。", MB_OK|MB_ICONERROR)
			return
		}
		current.Name = name
		if e := SaveProject(&current); e != nil {
			msgBox("重命名失败", e.Error(), MB_OK|MB_ICONERROR)
			return
		}
		refreshProjects()
		setText(hProjRenameName, current.Name)
		setStatus("状态：项目已重命名为 " + current.Name)
	case idProjRename:
		i := selectedChapterIndex()
		if i < 0 {
			return
		}
		if !RenameChapter(&current, current.Chapters[i].No, getText(hProjTitle)) {
			return
		}
		if e := SaveProject(&current); e != nil {
			msgBox("保存失败", e.Error(), MB_OK|MB_ICONERROR)
			return
		}
		refreshChapterList()
		setStatus("状态：章节标题已修改，全文 TXT 已更新")
	case idProjSaveEdit:
		i := selectedChapterIndex()
		if i < 0 {
			return
		}
		c := current.Chapters[i]
		if e := UpsertChapter(&current, c.No, getText(hProjTitle), getText(hProjBody)); e != nil {
			msgBox("保存失败", e.Error(), MB_OK|MB_ICONERROR)
			return
		}
		if e := SaveProject(&current); e != nil {
			msgBox("保存失败", e.Error(), MB_OK|MB_ICONERROR)
			return
		}
		refreshChapterList()
		setStatus("状态：章节正文已修改，全文 TXT 已更新")
	case idProjDeleteChapter:
		i := selectedChapterIndex()
		if i < 0 {
			return
		}
		c := current.Chapters[i]
		if msgBox("删除章节", "确定删除「"+c.Title+"」吗？\n删除前 project.json 会保留 .bak 备份。", MB_YESNO|MB_ICONQUESTION) != IDYES {
			return
		}
		DeleteChapter(&current, c.No)
		if e := SaveProject(&current); e != nil {
			msgBox("删除失败", e.Error(), MB_OK|MB_ICONERROR)
			return
		}
		refreshChapterList()
		refreshProjects()
		setStatus("状态：章节已删除，全文 TXT 已重建")
	case idProjAsContext:
		i := selectedChapterIndex()
		if i < 0 {
			return
		}
		setText(hHomeContext, current.Chapters[i].Body)
		switchPage(pageHome)
		setStatus("状态：已将所选章节放入续写上下文")
	case idProjExport:
		defaultName := safeWindowsFileName(current.Name) + "_完整小说.txt"
		if p, ok := chooseSave("导出完整小说", "txt", "文本文件 (*.txt)\x00*.txt", defaultName); ok {
			if e := ExportFullNovel(current, p); e != nil {
				msgBox("导出失败", e.Error(), MB_OK|MB_ICONERROR)
				setStatus("状态：完整小说导出失败")
				return
			}
			setStatus("状态：完整小说已导出到 " + p)
		}
	case idProjOpenFolder:
		openFolder(projectDir(current.ID))
	case idProjDeleteProject:
		if len(projectList) <= 1 {
			msgBox("无法删除", "至少保留一个小说项目。", MB_OK|MB_ICONERROR)
			return
		}
		if msgBox("删除项目", "确定删除整个项目「"+current.Name+"」吗？此操作会删除该项目目录。", MB_YESNO|MB_ICONQUESTION) != IDYES {
			return
		}
		if e := DeleteProjectData(current.ID); e != nil {
			msgBox("删除失败", e.Error(), MB_OK|MB_ICONERROR)
			return
		}
		ps, e := ListProjects()
		if e != nil || len(ps) == 0 {
			msgBox("删除后加载失败", fmt.Sprintf("%v", e), MB_OK|MB_ICONERROR)
			return
		}
		current = ps[0]
		settings.LastProjectID = current.ID
		_ = SaveSettings(settings)
		loadCurrentProjectToUI()
		setStatus("状态：项目已删除")
	case idAPIShow:
		if send(hAPIShow, BM_GETCHECK, 0, 0) == BST_CHECKED {
			send(hAPIKey, EM_SETPASSWORDCHAR, 0, 0)
		} else {
			send(hAPIKey, EM_SETPASSWORDCHAR, uintptr('●'), 0)
		}
		pInvalidateRect.Call(uintptr(hAPIKey), 0, 1)
	case idAPISave:
		if e := saveKeyLocal(false); e != nil {
			msgBox("保存失败", e.Error(), MB_OK|MB_ICONERROR)
		} else {
			setStatus("状态：API 设置已保存到本机")
		}
	case idAPITest:
		key := strings.TrimSpace(getText(hAPIKey))
		s := currentAPISettings()
		startAsync("test", func(ctx context.Context) (string, Usage, error) {
			t, e := TestAPI(ctx, httpClient, s, key)
			return t, Usage{}, e
		})
	case idAPIClear:
		if msgBox("清除本机 Key", "确定清除本机保存的 API Key 吗？小说项目不会受影响。", MB_YESNO|MB_ICONQUESTION) != IDYES {
			return
		}
		apiKey = ""
		setText(hAPIKey, "")
		settings.APIKeyCipher = ""
		settings.RememberKey = false
		if e := SaveSettings(settings); e != nil {
			msgBox("清除失败", e.Error(), MB_OK|MB_ICONERROR)
			return
		}
		setStatus("状态：本机 API Key 已清除")
	}
}

func drawButton(dis *DRAWITEMSTRUCT) {
	id := int(dis.CtlID)
	active := false
	isNav := false
	for i, b := range navButtons {
		if b == dis.HwndItem {
			isNav = true
			if i == currentPage {
				active = true
			}
		}
	}
	fill := brushSoft
	textColor := rgb(103, 39, 77)
	if isNav && !active {
		fill = brushSidebar
		textColor = rgb(255, 235, 246)
	}
	if id == idHomeGenerate || active {
		fill = brushPink
		textColor = rgb(255, 255, 255)
	}
	if id == idHomeStop {
		fill = brushSoft
		textColor = rgb(142, 96, 121)
	}
	np, _, _ := pGetStockObject.Call(NULL_PEN)
	pSelectObject.Call(uintptr(dis.HDC), np)
	pSelectObject.Call(uintptr(dis.HDC), uintptr(fill))
	pRoundRect.Call(uintptr(dis.HDC), uintptr(dis.RcItem.Left+1), uintptr(dis.RcItem.Top+1), uintptr(dis.RcItem.Right-1), uintptr(dis.RcItem.Bottom-1), 16, 16)
	pSetBkMode.Call(uintptr(dis.HDC), TRANSPARENT)
	pSetTextColor.Call(uintptr(dis.HDC), textColor)
	txt := getText(dis.HwndItem)
	r := dis.RcItem
	pDrawTextW.Call(uintptr(dis.HDC), uintptr(unsafe.Pointer(ptr(txt))), ^uintptr(0), uintptr(unsafe.Pointer(&r)), DT_CENTER|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
}

func paintBackground(hwnd syscall.Handle) {
	var ps PAINTSTRUCT
	hdc, _, _ := pBeginPaint.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&ps)))
	if hdc == 0 {
		return
	}
	defer pEndPaint.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&ps)))
	var rc RECT
	pGetClientRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&rc)))

	// Clean base background. The v4.1.0 build stretched the complete UI mock-up
	// behind the real Win32 controls, which caused duplicated titles and labels.
	// v4.1.3 reuses the decorative TOP strip and paints the actual work area
	// with solid colours so no reference-image text can sit behind real controls.
	all := RECT{0, 0, rc.Right, rc.Bottom}
	pFillRect.Call(hdc, uintptr(unsafe.Pointer(&all)), uintptr(brushSoft))
	side := RECT{0, 0, 270, rc.Bottom}
	pFillRect.Call(hdc, uintptr(unsafe.Pointer(&side)), uintptr(brushSidebar))

	// Decorative banner: crop only the top-right artwork from the supplied mock-up.
	if bgBitmap != 0 && rc.Right > 270 {
		mem, _, _ := pCreateCompatibleDC.Call(hdc)
		old, _, _ := pSelectObject.Call(mem, uintptr(bgBitmap))
		var bm BITMAP
		pGetObjectW.Call(uintptr(bgBitmap), unsafe.Sizeof(bm), uintptr(unsafe.Pointer(&bm)))
		if bm.Width > 320 && bm.Height > 170 {
			srcX := int32(270)
			srcW := bm.Width - srcX
			bannerH := int32(145)
			pSetStretchBltMode.Call(hdc, HALFTONE)
			pStretchBlt.Call(hdc, 270, 0, uintptr(rc.Right-270), uintptr(bannerH), mem, uintptr(srcX), 0, uintptr(srcW), 170, SRCCOPY)
		}
		pSelectObject.Call(mem, old)
		pDeleteDC.Call(mem)
	}

	// Solid content card. It intentionally covers every form/control from the
	// reference image so switching pages can never leave transparent-text ghosts.
	np, _, _ := pGetStockObject.Call(NULL_PEN)
	pSelectObject.Call(hdc, np)
	pSelectObject.Call(hdc, uintptr(brushWhite))
	pRoundRect.Call(hdc, 270, 145, uintptr(rc.Right-16), uintptr(rc.Bottom-14), 28, 28)
}

func writeStartupLog(where string, recovered any) {
	dir := filepath.Join(appBaseDir(), "Logs")
	_ = os.MkdirAll(dir, 0755)
	path := filepath.Join(dir, "startup.log")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s FATAL at %s: %v\n%s\n", time.Now().Format("2006-01-02 15:04:05.000"), where, recovered, debug.Stack())
}

func safeWndProc(hwnd syscall.Handle, msg uint32, wParam, lParam uintptr) (ret uintptr) {
	defer func() {
		if r := recover(); r != nil {
			writeStartupLog(fmt.Sprintf("WndProc message=0x%X", msg), r)
			text := fmt.Sprintf("心阅小说启动/运行异常，已记录诊断日志。\n\n错误：%v\n\n日志：%s", r, filepath.Join(appBaseDir(), "Logs", "startup.log"))
			pMessageBoxW.Call(uintptr(hwnd), uintptr(unsafe.Pointer(ptr(text))), uintptr(unsafe.Pointer(ptr("心阅小说运行异常"))), MB_OK|MB_ICONERROR)
			ret = 0
		}
	}()
	return wndProc(hwnd, msg, wParam, lParam)
}
func wndProc(hwnd syscall.Handle, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case WM_CREATE:
		mainHwnd = hwnd
		loadAssets()
		if appIcon != 0 {
			send(hwnd, WM_SETICON, ICON_BIG, uintptr(appIcon))
			send(hwnd, WM_SETICON, ICON_SMALL, uintptr(appIcon))
		}
		buildUI()
		return 0
	case WM_PAINT:
		paintBackground(hwnd)
		return 0
	case WM_ERASEBKGND:
		return 1
	case WM_CTLCOLORSTATIC:
		hdc := syscall.Handle(wParam)
		if sidebarStatics[syscall.Handle(lParam)] {
			pSetBkMode.Call(uintptr(hdc), 2) // OPAQUE
			pSetBkColor.Call(uintptr(hdc), rgb(86, 41, 82))
			pSetTextColor.Call(uintptr(hdc), rgb(255, 241, 248))
			return uintptr(brushSidebar)
		}
		pSetBkMode.Call(uintptr(hdc), 2) // OPAQUE: clears old transparent label pixels on page switch
		pSetBkColor.Call(uintptr(hdc), rgb(255, 250, 252))
		pSetTextColor.Call(uintptr(hdc), rgb(103, 45, 82))
		return uintptr(brushWhite)
	case WM_CTLCOLOREDIT, WM_CTLCOLORLISTBOX:
		hdc := syscall.Handle(wParam)
		pSetTextColor.Call(uintptr(hdc), rgb(79, 42, 67))
		pSetBkColor.Call(uintptr(hdc), rgb(255, 252, 253))
		return uintptr(brushEdit)
	case WM_DRAWITEM:
		dis := (*DRAWITEMSTRUCT)(unsafe.Pointer(lParam))
		if dis != nil {
			drawButton(dis)
			return 1
		}
	case WM_COMMAND:
		id := int(wParam & 0xffff)
		code := int((wParam >> 16) & 0xffff)
		handleCommand(id, code)
		return 0
	case WM_ASYNC_DONE:
		finishAsync()
		return 0
	case WM_GETMINMAXINFO:
		mm := (*MINMAXINFO)(unsafe.Pointer(lParam))
		if mm != nil {
			mm.PtMinTrackSize = POINT{1200, 780}
		}
		return 0
	case WM_CLOSE:
		cancelMu.Lock()
		if currentCancel != nil {
			currentCancel()
		}
		cancelMu.Unlock()
		return pDefWindowProcWCall(hwnd, msg, wParam, lParam)
	case WM_DESTROY:
		pPostQuitMessage.Call(0)
		return 0
	}
	return pDefWindowProcWCall(hwnd, msg, wParam, lParam)
}
func pDefWindowProcWCall(h syscall.Handle, m uint32, w, l uintptr) uintptr {
	r, _, _ := pDefWindowProcW.Call(uintptr(h), uintptr(m), w, l)
	return r
}

func initBrushes() {
	brushWhite = syscall.Handle(mustBrush(255, 255, 255))
	brushEdit = syscall.Handle(mustBrush(255, 252, 253))
	brushSidebar = syscall.Handle(mustBrush(86, 41, 82))
	brushPink = syscall.Handle(mustBrush(222, 65, 139))
	brushPinkDark = syscall.Handle(mustBrush(181, 47, 111))
	brushSoft = syscall.Handle(mustBrush(255, 239, 247))
	nb, _, _ := pGetStockObject.Call(NULL_BRUSH)
	brushTransparent = syscall.Handle(nb)
}
func mustBrush(r, g, b byte) uintptr { x, _, _ := pCreateSolidBrush.Call(rgb(r, g, b)); return x }

func main() {
	runtime.LockOSThread()
	pSetProcessDPIAware.Call()
	initFonts()
	initBrushes()
	hInst, _, _ := pGetModuleHandleW.Call(0)
	cls := ptr("XinyueNovelV411")
	cursor, _, _ := pLoadCursorW.Call(0, IDC_ARROW)
	wc := WNDCLASSEX{CbSize: uint32(unsafe.Sizeof(WNDCLASSEX{})), LpfnWndProc: syscall.NewCallback(safeWndProc), HInstance: syscall.Handle(hInst), HCursor: syscall.Handle(cursor), LpszClassName: cls}
	if appIcon != 0 {
		wc.HIcon = appIcon
		wc.HIconSm = appIcon
	}
	r, _, _ := pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	if r == 0 {
		panic("RegisterClassExW failed")
	}
	style := uintptr(WS_OVERLAPPED | WS_CAPTION | WS_SYSMENU | WS_THICKFRAME | WS_MINIMIZEBOX | WS_MAXIMIZEBOX | WS_VISIBLE | WS_CLIPCHILDREN)
	hwnd, _, _ := pCreateWindowExW.Call(0, uintptr(unsafe.Pointer(cls)), uintptr(unsafe.Pointer(ptr(AppName+" v"+AppVersion))), style, 40, 25, 1586, 992, 0, 0, hInst, 0)
	if hwnd == 0 {
		panic("CreateWindowExW failed")
	}
	mainHwnd = syscall.Handle(hwnd)
	pShowWindow.Call(hwnd, SW_SHOW)
	pUpdateWindow.Call(hwnd)
	var msg MSG
	for {
		rv, _, _ := pGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(rv) <= 0 {
			break
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		pDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}
}
