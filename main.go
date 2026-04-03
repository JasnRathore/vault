package main

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const directoryPath = "."

// ─── Data Models ────────────────────────────────────────────────────────────

type FileEntry struct {
	Name    string `json:"name"`
	Size    string `json:"size"`
	SizeRaw int64  `json:"sizeRaw"`
	ModTime string `json:"modTime"`
	IsDir   bool   `json:"isDir"`
	Ext     string `json:"ext"`
	Icon    string `json:"icon"`
	Path    string `json:"path"`
	MimeType string `json:"mimeType"`
}

type Crumb struct {
	Name string
	Path string
}

type StatResponse struct {
	TotalFiles  int    `json:"totalFiles"`
	TotalDirs   int    `json:"totalDirs"`
	DirPath     string `json:"dirPath"`
	TotalSize   string `json:"totalSize"`
	DiskTotal   string `json:"diskTotal"`
	DiskFree    string `json:"diskFree"`
	DiskUsedPct float64 `json:"diskUsedPct"`
}

type APIResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func formatSize(size int64) string {
	switch {
	case size < 1024:
		return fmt.Sprintf("%d B", size)
	case size < 1024*1024:
		return fmt.Sprintf("%.1f KB", float64(size)/1024)
	case size < 1024*1024*1024:
		return fmt.Sprintf("%.1f MB", float64(size)/(1024*1024))
	default:
		return fmt.Sprintf("%.2f GB", float64(size)/(1024*1024*1024))
	}
}

func getMimeType(name string) string {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".jpg", ".jpeg": return "image/jpeg"
	case ".png":          return "image/png"
	case ".gif":          return "image/gif"
	case ".svg":          return "image/svg+xml"
	case ".webp":         return "image/webp"
	case ".mp4":          return "video/mp4"
	case ".webm":         return "video/webm"
	case ".mp3":          return "audio/mpeg"
	case ".ogg":          return "audio/ogg"
	case ".wav":          return "audio/wav"
	case ".pdf":          return "application/pdf"
	case ".txt", ".md", ".rst", ".log": return "text/plain"
	case ".html", ".htm": return "text/html"
	case ".css":          return "text/css"
	case ".js", ".ts":    return "text/javascript"
	case ".json":         return "application/json"
	case ".go", ".py", ".rs", ".c", ".cpp", ".java", ".rb", ".sh",
		 ".bash", ".yaml", ".yml", ".toml", ".xml", ".tsx", ".jsx":
		return "text/plain"
	default:              return "application/octet-stream"
	}
}

func getIcon(name string, isDir bool) string {
	if isDir { return "folder" }
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".go", ".js", ".ts", ".jsx", ".tsx", ".py", ".rs", ".c", ".cpp",
		 ".java", ".rb", ".swift", ".kt", ".php":
		return "code"
	case ".html", ".htm", ".css", ".scss", ".sass":
		return "html"
	case ".json", ".yaml", ".yml", ".toml", ".xml", ".env":
		return "settings"
	case ".md", ".txt", ".rst", ".log":
		return "article"
	case ".png", ".jpg", ".jpeg", ".gif", ".svg", ".webp", ".ico", ".bmp":
		return "image"
	case ".mp4", ".mov", ".avi", ".mkv", ".webm":
		return "movie"
	case ".mp3", ".wav", ".flac", ".ogg", ".aac":
		return "audio_file"
	case ".pdf":
		return "picture_as_pdf"
	case ".zip", ".tar", ".gz", ".rar", ".7z", ".bz2":
		return "folder_zip"
	case ".sh", ".bash", ".zsh", ".fish":
		return "terminal"
	case ".exe", ".bin", ".out", ".dmg", ".app":
		return "apps"
	case ".doc", ".docx", ".odt":
		return "description"
	case ".xls", ".xlsx", ".csv":
		return "table_chart"
	case ".ppt", ".pptx":
		return "slideshow"
	case ".db", ".sqlite", ".sql":
		return "storage"
	case ".ttf", ".otf", ".woff", ".woff2":
		return "font_download"
	default:
		return "draft"
	}
}

func getExt(name string, isDir bool) string {
	if isDir { return "DIR" }
	ext := strings.ToUpper(strings.TrimPrefix(filepath.Ext(name), "."))
	if ext == "" { return "FILE" }
	return ext
}

func isPreviewable(name string) bool {
	mime := getMimeType(name)
	return strings.HasPrefix(mime, "image/") ||
		strings.HasPrefix(mime, "video/") ||
		strings.HasPrefix(mime, "audio/") ||
		strings.HasPrefix(mime, "text/") ||
		mime == "application/json" ||
		mime == "application/pdf"
}

func safePath(urlPath string) (string, error) {
	cleanPath := filepath.Clean(filepath.Join(directoryPath, urlPath))
	if !strings.HasPrefix(filepath.Clean(cleanPath), filepath.Clean(directoryPath)) {
		return "", fmt.Errorf("access denied")
	}
	return cleanPath, nil
}

func dirSize(path string) int64 {
	var size int64
	filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() { size += info.Size() }
		return nil
	})
	return size
}

func listDirectory(urlPath string) ([]FileEntry, error) {
	cleanPath, err := safePath(urlPath)
	if err != nil { return nil, err }

	entries, err := os.ReadDir(cleanPath)
	if err != nil { return nil, err }

	var files []FileEntry
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil { continue }
		entryPath := filepath.Join(urlPath, entry.Name())
		if !strings.HasPrefix(entryPath, "/") { entryPath = "/" + entryPath }
		files = append(files, FileEntry{
			Name:     entry.Name(),
			Size:     formatSize(info.Size()),
			SizeRaw:  info.Size(),
			ModTime:  info.ModTime().Format("Jan 02, 2006  15:04"),
			IsDir:    entry.IsDir(),
			Ext:      getExt(entry.Name(), entry.IsDir()),
			Icon:     getIcon(entry.Name(), entry.IsDir()),
			Path:     entryPath,
			MimeType: getMimeType(entry.Name()),
		})
	}

	sort.Slice(files, func(i, j int) bool {
		if files[i].IsDir != files[j].IsDir { return files[i].IsDir }
		return strings.ToLower(files[i].Name) < strings.ToLower(files[j].Name)
	})
	return files, nil
}

func buildBreadcrumbs(urlPath string) []Crumb {
	if urlPath == "/" || urlPath == "" { return nil }
	parts := strings.Split(strings.Trim(urlPath, "/"), "/")
	var crumbs []Crumb
	for i, p := range parts {
		crumbs = append(crumbs, Crumb{
			Name: p,
			Path: "/" + strings.Join(parts[:i+1], "/"),
		})
	}
	return crumbs
}

func parentPath(urlPath string) string {
	if urlPath == "/" || urlPath == "" { return "" }
	p := filepath.Dir(strings.TrimRight(urlPath, "/"))
	if p == "." { return "/" }
	return p
}

// ─── Handlers ────────────────────────────────────────────────────────────────

func browseHandler(tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		urlPath := strings.TrimPrefix(r.URL.Path, "/browse")
		if urlPath == "" { urlPath = "/" }

		entries, err := listDirectory(urlPath)
		if err != nil {
			http.Error(w, "Cannot read directory: "+err.Error(), http.StatusInternalServerError)
			return
		}

		totalFiles, totalDirs := 0, 0
		for _, e := range entries {
			if e.IsDir { totalDirs++ } else { totalFiles++ }
		}

		absPath, _ := filepath.Abs(directoryPath)
		entriesJSON, _ := json.Marshal(entries)

		data := struct {
			CurrentPath  string
			ParentPath   string
			Breadcrumbs  []Crumb
			Entries      []FileEntry
			EntriesJSON  template.JS
			TotalFiles   int
			TotalDirs    int
			ServerTime   string
			DirPath      string
		}{
			CurrentPath:  urlPath,
			ParentPath:   parentPath(urlPath),
			Breadcrumbs:  buildBreadcrumbs(urlPath),
			Entries:      entries,
			EntriesJSON:  template.JS(entriesJSON),
			TotalFiles:   totalFiles,
			TotalDirs:    totalDirs,
			ServerTime:   time.Now().Format("15:04:05"),
			DirPath:      absPath,
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := tmpl.Execute(w, data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	urlPath := r.FormValue("path")
	destDir, err := safePath(urlPath)
	if err != nil {
		jsonError(w, "Access denied")
		return
	}

	r.ParseMultipartForm(512 << 20) // 512 MB max
	files := r.MultipartForm.File["files"]
	for _, fh := range files {
		src, err := fh.Open()
		if err != nil { continue }
		defer src.Close()

		destPath := filepath.Join(destDir, filepath.Base(fh.Filename))
		dst, err := os.Create(destPath)
		if err != nil { continue }
		defer dst.Close()
		io.Copy(dst, src)
	}

	json.NewEncoder(w).Encode(APIResponse{Success: true, Message: fmt.Sprintf("Uploaded %d file(s)", len(files))})
}

func deleteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct{ Paths []string `json:"paths"` }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid request")
		return
	}

	count := 0
	for _, p := range req.Paths {
		cleanPath, err := safePath(p)
		if err != nil { continue }
		if err := os.RemoveAll(cleanPath); err == nil { count++ }
	}
	json.NewEncoder(w).Encode(APIResponse{Success: true, Message: fmt.Sprintf("Deleted %d item(s)", count)})
}

func renameHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		OldPath string `json:"oldPath"`
		NewName string `json:"newName"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid request")
		return
	}

	oldClean, err := safePath(req.OldPath)
	if err != nil { jsonError(w, "Access denied"); return }

	newPath := filepath.Join(filepath.Dir(oldClean), filepath.Base(req.NewName))
	if err := os.Rename(oldClean, newPath); err != nil {
		jsonError(w, err.Error())
		return
	}
	json.NewEncoder(w).Encode(APIResponse{Success: true, Message: "Renamed successfully"})
}

func mkdirHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Path    string `json:"path"`
		DirName string `json:"dirName"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid request")
		return
	}

	parentClean, err := safePath(req.Path)
	if err != nil { jsonError(w, "Access denied"); return }

	newDir := filepath.Join(parentClean, filepath.Base(req.DirName))
	if err := os.MkdirAll(newDir, 0755); err != nil {
		jsonError(w, err.Error())
		return
	}
	json.NewEncoder(w).Encode(APIResponse{Success: true, Message: "Folder created"})
}

func moveHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Sources []string `json:"sources"`
		Dest    string   `json:"dest"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid request")
		return
	}

	destClean, err := safePath(req.Dest)
	if err != nil { jsonError(w, "Access denied"); return }

	count := 0
	for _, src := range req.Sources {
		srcClean, err := safePath(src)
		if err != nil { continue }
		destFull := filepath.Join(destClean, filepath.Base(srcClean))
		if err := os.Rename(srcClean, destFull); err == nil { count++ }
	}
	json.NewEncoder(w).Encode(APIResponse{Success: true, Message: fmt.Sprintf("Moved %d item(s)", count)})
}

func zipDownloadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct{ Paths []string `json:"paths"` }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename=\"download.zip\"")

	zw := zip.NewWriter(w)
	defer zw.Close()

	for _, p := range req.Paths {
		cleanPath, err := safePath(p)
		if err != nil { continue }
		addToZip(zw, cleanPath, filepath.Base(cleanPath))
	}
}

func addToZip(zw *zip.Writer, path, nameInZip string) {
	info, err := os.Stat(path)
	if err != nil { return }

	if info.IsDir() {
		entries, _ := os.ReadDir(path)
		for _, e := range entries {
			addToZip(zw, filepath.Join(path, e.Name()), filepath.Join(nameInZip, e.Name()))
		}
		return
	}

	f, err := os.Open(path)
	if err != nil { return }
	defer f.Close()

	w, err := zw.Create(nameInZip)
	if err != nil { return }
	io.Copy(w, f)
}

func statsHandler(w http.ResponseWriter, r *http.Request) {
	absPath, _ := filepath.Abs(directoryPath)
	entries, _ := os.ReadDir(directoryPath)
	files, dirs := 0, 0
	for _, e := range entries {
		if e.IsDir() { dirs++ } else { files++ }
	}

	total, free := diskUsage(directoryPath)
	used := total - free
	usedPct := 0.0
	if total > 0 { usedPct = float64(used) / float64(total) * 100 }
	totalSize := dirSize(directoryPath)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(StatResponse{
		TotalFiles:  files,
		TotalDirs:   dirs,
		DirPath:     absPath,
		TotalSize:   formatSize(totalSize),
		DiskTotal:   formatSize(int64(total)),
		DiskFree:    formatSize(int64(free)),
		DiskUsedPct: usedPct,
	})
}

func listAPIHandler(w http.ResponseWriter, r *http.Request) {
	urlPath := r.URL.Query().Get("path")
	if urlPath == "" { urlPath = "/" }
	entries, err := listDirectory(urlPath)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(APIResponse{Success: false, Message: err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entries)
}

func newFileHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Path     string `json:"path"`
		FileName string `json:"fileName"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid request"); return
	}
	parentClean, err := safePath(req.Path)
	if err != nil { jsonError(w, "Access denied"); return }

	newFile := filepath.Join(parentClean, filepath.Base(req.FileName))
	f, err := os.Create(newFile)
	if err != nil { jsonError(w, err.Error()); return }
	f.Close()
	json.NewEncoder(w).Encode(APIResponse{Success: true, Message: "File created"})
}

func fileInfoHandler(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("path")
	cleanPath, err := safePath(filePath)
	if err != nil { jsonError(w, "Access denied"); return }

	info, err := os.Stat(cleanPath)
	if err != nil { jsonError(w, err.Error()); return }

	type FileInfo struct {
		Name      string `json:"name"`
		Path      string `json:"path"`
		Size      string `json:"size"`
		SizeRaw   int64  `json:"sizeRaw"`
		ModTime   string `json:"modTime"`
		IsDir     bool   `json:"isDir"`
		MimeType  string `json:"mimeType"`
		Ext       string `json:"ext"`
		DirSize   string `json:"dirSize,omitempty"`
		DirCount  int    `json:"dirCount,omitempty"`
	}

	fi := FileInfo{
		Name:     info.Name(),
		Path:     filePath,
		Size:     formatSize(info.Size()),
		SizeRaw:  info.Size(),
		ModTime:  info.ModTime().Format("Jan 02, 2006 15:04:05"),
		IsDir:    info.IsDir(),
		MimeType: getMimeType(info.Name()),
		Ext:      getExt(info.Name(), info.IsDir()),
	}
	if info.IsDir() {
		var count int
		filepath.Walk(cleanPath, func(_ string, i os.FileInfo, _ error) error {
			if !i.IsDir() { count++ }; return nil
		})
		fi.DirSize  = formatSize(dirSize(cleanPath))
		fi.DirCount = count
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(fi)
}

func textReadHandler(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("path")
	cleanPath, err := safePath(filePath)
	if err != nil { http.Error(w, "Access denied", http.StatusForbidden); return }

	info, err := os.Stat(cleanPath)
	if err != nil || info.IsDir() { http.Error(w, "Not found", http.StatusNotFound); return }

	// limit to 1MB for preview
	limitBytes := int64(1 << 20)
	sizeToRead := info.Size()
	if sizeToRead > limitBytes { sizeToRead = limitBytes }

	f, err := os.Open(cleanPath)
	if err != nil { http.Error(w, "Cannot open file", http.StatusInternalServerError); return }
	defer f.Close()

	truncated := info.Size() > limitBytes
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if truncated {
		w.Header().Set("X-Truncated", strconv.FormatBool(truncated))
	}
	io.CopyN(w, f, sizeToRead)
}

func jsonError(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	json.NewEncoder(w).Encode(APIResponse{Success: false, Message: msg})
}

// ─── Main ─────────────────────────────────────────────────────────────────────

func main() {
	if _, err := os.Stat(directoryPath); os.IsNotExist(err) {
		fmt.Printf("Directory '%s' not found.\n", directoryPath)
		return
	}

	funcMap := template.FuncMap{
		"not": func(b bool) bool { return !b },
	}
	tmpl, err := template.New("dashboard").Funcs(funcMap).Parse(dashboardHTML)
	if err != nil {
		fmt.Printf("Template error: %s\n", err)
		return
	}

	// Static files
	http.Handle("/files/", http.StripPrefix("/files", http.FileServer(http.Dir(directoryPath))))

	// Root redirect
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/browse/", http.StatusFound)
			return
		}
		http.NotFound(w, r)
	})

	// API routes
	http.HandleFunc("/api/stats",    statsHandler)
	http.HandleFunc("/api/list",     listAPIHandler)
	http.HandleFunc("/api/upload",   uploadHandler)
	http.HandleFunc("/api/delete",   deleteHandler)
	http.HandleFunc("/api/rename",   renameHandler)
	http.HandleFunc("/api/mkdir",    mkdirHandler)
	http.HandleFunc("/api/move",     moveHandler)
	http.HandleFunc("/api/zip",      zipDownloadHandler)
	http.HandleFunc("/api/newfile",  newFileHandler)
	http.HandleFunc("/api/fileinfo", fileInfoHandler)
	http.HandleFunc("/api/text",     textReadHandler)

	// Browser
	http.HandleFunc("/browse",  browseHandler(tmpl))
	http.HandleFunc("/browse/", browseHandler(tmpl))

	port := 8080
	fmt.Printf("FileNav running at http://localhost:%d\n", port)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil); err != nil {
		fmt.Printf("Error: %s\n", err)
	}
}

// ─── HTML Template ───────────────────────────────────────────────────────────

const dashboardHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Vault · {{.CurrentPath}}</title>
<link rel="preconnect" href="https://fonts.googleapis.com">
<link href="https://fonts.googleapis.com/css2?family=IBM+Plex+Mono:wght@400;500;600&family=IBM+Plex+Sans:wght@300;400;500;600&display=swap" rel="stylesheet">
<link href="https://fonts.googleapis.com/icon?family=Material+Icons+Round" rel="stylesheet">
<style>
*, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }

:root {
  --bg:       #070a0f;
  --s0:       #0c1018;
  --s1:       #111620;
  --s2:       #161c28;
  --s3:       #1e2535;
  --b0:       #1f2840;
  --b1:       #263048;
  --b2:       #2d3a56;
  --cx:       #4d9fff;
  --cx2:      #2e7de0;
  --cx3:      #1a5cb8;
  --cg:       #34d399;
  --ca:       #fb923c;
  --cr:       #f87171;
  --cp:       #a78bfa;
  --cy:       #fbbf24;
  --t0:       #f0f4ff;
  --t1:       #b0bcd8;
  --t2:       #697a9e;
  --t3:       #3d4f70;
  --mono:     'IBM Plex Mono', monospace;
  --sans:     'IBM Plex Sans', sans-serif;
  --r:        6px;
  --shadow:   0 4px 24px rgba(0,0,0,.6);
}

html, body { height: 100%; background: var(--bg); color: var(--t0); font-family: var(--sans); font-size: 14px; overflow: hidden; }

/* ── Scrollbars ── */
::-webkit-scrollbar { width: 5px; height: 5px; }
::-webkit-scrollbar-track { background: transparent; }
::-webkit-scrollbar-thumb { background: var(--b1); border-radius: 3px; }
::-webkit-scrollbar-thumb:hover { background: var(--b2); }

/* ── App Shell ── */
.app { display: grid; grid-template-rows: 48px 1fr; grid-template-columns: 240px 1fr; height: 100vh; }
.app.panel-open { grid-template-columns: 240px 1fr 320px; }

/* ── Topbar ── */
.topbar {
  grid-column: 1 / -1;
  display: flex; align-items: center; gap: 0;
  background: var(--s0); border-bottom: 1px solid var(--b0);
  padding: 0 16px; z-index: 100;
}
.logo {
  display: flex; align-items: center; gap: 8px;
  font-family: var(--mono); font-size: 13px; font-weight: 600;
  color: var(--cx); letter-spacing: 1px; white-space: nowrap;
  padding-right: 16px; border-right: 1px solid var(--b0); margin-right: 0;
  width: 240px; flex-shrink: 0;
}
.logo-icon { font-size: 18px !important; }
.topbar-path {
  display: flex; align-items: center; gap: 0;
  font-family: var(--mono); font-size: 11px; color: var(--t2);
  flex: 1; min-width: 0; padding: 0 16px; overflow: hidden;
}
.topbar-path a { color: var(--cx); text-decoration: none; white-space: nowrap; }
.topbar-path a:hover { color: var(--t0); }
.topbar-path .sep { color: var(--t3); margin: 0 4px; }
.topbar-right { display: flex; align-items: center; gap: 8px; margin-left: auto; }
.tbar-btn {
  display: flex; align-items: center; justify-content: center;
  width: 32px; height: 32px; border-radius: var(--r);
  background: none; border: 1px solid transparent; color: var(--t2);
  cursor: pointer; transition: all .15s;
}
.tbar-btn:hover { background: var(--s2); border-color: var(--b1); color: var(--t0); }
.tbar-btn .material-icons-round { font-size: 18px; }
.clock {
  font-family: var(--mono); font-size: 11px; color: var(--t3);
  padding: 4px 10px; background: var(--s1); border: 1px solid var(--b0); border-radius: var(--r);
}

/* ── Sidebar ── */
.sidebar {
  background: var(--s0); border-right: 1px solid var(--b0);
  display: flex; flex-direction: column; overflow: hidden;
}
.sb-section { padding: 16px 12px 8px; }
.sb-label {
  font-family: var(--mono); font-size: 9px; letter-spacing: 2px; text-transform: uppercase;
  color: var(--t3); padding: 0 4px; margin-bottom: 6px;
}
.sb-nav-btn {
  display: flex; align-items: center; gap: 8px;
  padding: 7px 10px; border-radius: var(--r);
  font-size: 12.5px; color: var(--t1); cursor: pointer;
  text-decoration: none; transition: background .12s, color .12s;
  border: 1px solid transparent;
}
.sb-nav-btn:hover { background: var(--s2); color: var(--t0); }
.sb-nav-btn.active { background: var(--cx3); border-color: var(--cx2); color: var(--t0); }
.sb-nav-btn .material-icons-round { font-size: 16px; color: var(--t2); }
.sb-nav-btn.active .material-icons-round { color: var(--cx); }

.sb-divider { height: 1px; background: var(--b0); margin: 8px 12px; }

/* Disk usage widget */
.disk-widget { margin: 0 12px 8px; padding: 12px; background: var(--s1); border: 1px solid var(--b0); border-radius: var(--r); }
.disk-row { display: flex; justify-content: space-between; align-items: center; margin-bottom: 8px; }
.disk-label { font-size: 11px; color: var(--t2); }
.disk-val { font-family: var(--mono); font-size: 11px; color: var(--t1); }
.disk-bar { height: 4px; background: var(--b1); border-radius: 2px; overflow: hidden; }
.disk-fill { height: 100%; background: linear-gradient(90deg, var(--cx), var(--cp)); border-radius: 2px; transition: width .6s ease; }
.disk-fill.warn { background: linear-gradient(90deg, var(--ca), var(--cr)); }

/* Mini stats */
.stats-row { display: flex; gap: 8px; padding: 0 12px 12px; }
.mini-stat { flex: 1; background: var(--s1); border: 1px solid var(--b0); border-radius: var(--r); padding: 10px 10px 8px; }
.mini-stat .v { font-family: var(--mono); font-size: 18px; font-weight: 600; line-height: 1; }
.mini-stat .l { font-size: 10px; color: var(--t2); margin-top: 3px; }
.mini-stat.c-blue .v { color: var(--cx); }
.mini-stat.c-green .v { color: var(--cg); }

/* Recent files */
.sb-scroll { flex: 1; overflow-y: auto; }
.recent-item {
  display: flex; align-items: center; gap: 8px;
  padding: 6px 16px; font-size: 12px; color: var(--t1);
  cursor: pointer; transition: background .1s;
}
.recent-item:hover { background: var(--s2); }
.recent-item .material-icons-round { font-size: 15px; color: var(--t3); flex-shrink: 0; }
.recent-item .name { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; flex: 1; }
.recent-item .rtime { font-family: var(--mono); font-size: 10px; color: var(--t3); white-space: nowrap; }

/* ── Main ── */
.main { display: flex; flex-direction: column; overflow: hidden; background: var(--bg); }

/* Toolbar */
.toolbar {
  display: flex; align-items: center; gap: 6px;
  padding: 8px 16px;
  border-bottom: 1px solid var(--b0);
  background: var(--s0); flex-shrink: 0;
  min-height: 52px;
}
.search-wrap { position: relative; display: flex; align-items: center; }
.search-icon { position: absolute; left: 9px; font-size: 15px !important; color: var(--t3); pointer-events: none; }
.search-input {
  background: var(--s1); border: 1px solid var(--b1);
  border-radius: var(--r); padding: 6px 12px 6px 32px;
  font-family: var(--sans); font-size: 12.5px; color: var(--t0);
  width: 200px; outline: none; transition: border-color .15s, width .2s;
}
.search-input::placeholder { color: var(--t3); }
.search-input:focus { border-color: var(--cx); width: 260px; }

.tb-btn {
  display: flex; align-items: center; gap: 5px;
  padding: 6px 11px; border-radius: var(--r);
  font-family: var(--sans); font-size: 12px; font-weight: 500;
  cursor: pointer; border: 1px solid var(--b1); background: var(--s2);
  color: var(--t1); transition: all .15s; white-space: nowrap;
}
.tb-btn .material-icons-round { font-size: 14px; }
.tb-btn:hover { background: var(--b1); color: var(--t0); }
.tb-btn.active { background: rgba(77,159,255,.12); border-color: var(--cx2); color: var(--cx); }
.tb-btn.danger { color: var(--cr); }
.tb-btn.danger:hover { background: rgba(248,113,113,.12); border-color: var(--cr); }
.tb-btn.primary { background: var(--cx3); border-color: var(--cx2); color: var(--t0); }
.tb-btn.primary:hover { background: var(--cx2); }

.tb-sep { width: 1px; height: 22px; background: var(--b1); margin: 0 2px; flex-shrink: 0; }
.tb-spacer { flex: 1; }

/* Bulk actions bar */
.bulk-bar {
  display: none; align-items: center; gap: 8px;
  padding: 8px 16px; background: rgba(77,159,255,.06);
  border-bottom: 1px solid var(--cx3); flex-shrink: 0;
}
.bulk-bar.visible { display: flex; }
.bulk-count { font-family: var(--mono); font-size: 12px; color: var(--cx); font-weight: 600; }
.bulk-spacer { flex: 1; }

/* File area */
.file-area { flex: 1; overflow-y: auto; padding: 16px; }

/* Drop overlay */
.drop-overlay {
  position: absolute; inset: 0; background: rgba(77,159,255,.08);
  border: 2px dashed var(--cx); border-radius: var(--r);
  display: none; align-items: center; justify-content: center; z-index: 200;
  font-size: 18px; font-weight: 600; color: var(--cx); flex-direction: column; gap: 12px;
  pointer-events: none;
}
.drop-overlay.active { display: flex; }
.drop-overlay .material-icons-round { font-size: 56px; }
.main-wrap { position: relative; display: flex; flex-direction: column; flex: 1; overflow: hidden; }

/* ── Table ── */
.file-table { width: 100%; border-collapse: collapse; }
.ft-head th {
  font-family: var(--mono); font-size: 9.5px; letter-spacing: 1.5px; text-transform: uppercase;
  color: var(--t3); font-weight: 500; text-align: left;
  padding: 0 10px 10px; border-bottom: 1px solid var(--b0);
  white-space: nowrap; cursor: pointer; user-select: none;
}
.ft-head th:hover { color: var(--t1); }
.ft-head th.sort-asc::after  { content: ' ↑'; color: var(--cx); }
.ft-head th.sort-desc::after { content: ' ↓'; color: var(--cx); }
.ft-head th:first-child { padding-left: 6px; }
.ft-head th.th-check { width: 32px; cursor: default; }
.ft-head th:last-child { text-align: right; }

.file-row {
  border-bottom: 1px solid var(--b0); transition: background .1s;
  animation: rowIn .25s ease both;
}
.file-row:last-child { border-bottom: none; }
.file-row:hover { background: var(--s1); }
.file-row.selected { background: rgba(77,159,255,.08) !important; }
.file-row.hidden { display: none; }

@keyframes rowIn { from { opacity:0; transform:translateY(4px) } to { opacity:1; transform:none } }

.file-row td { padding: 0 10px; height: 42px; vertical-align: middle; }
.file-row td:first-child { padding-left: 6px; }

/* Checkbox */
.row-check {
  width: 16px; height: 16px; border-radius: 4px;
  border: 1.5px solid var(--b2); background: none; cursor: pointer;
  appearance: none; transition: all .15s; flex-shrink: 0;
}
.row-check:checked { background: var(--cx); border-color: var(--cx); }
.row-check:checked::after { content: '✓'; display: block; text-align: center; font-size: 10px; color: #fff; line-height: 1.4; }

/* Name cell */
.cell-name {
  display: flex; align-items: center; gap: 8px;
  max-width: 360px; overflow: hidden;
}
.cell-name a { color: var(--t0); text-decoration: none; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; font-size: 13px; }
.cell-name a:hover { color: var(--cx); }
.file-icon { font-size: 17px !important; flex-shrink: 0; }
.icon-folder { color: var(--cy); }
.icon-code   { color: var(--cx); }
.icon-image  { color: var(--cp); }
.icon-doc    { color: var(--t2); }
.icon-zip    { color: var(--cr); }
.icon-media  { color: var(--cg); }
.icon-data   { color: var(--ca); }

.ext-pill {
  font-family: var(--mono); font-size: 8.5px; letter-spacing: .5px; padding: 2px 5px;
  border-radius: 3px; background: var(--s2); border: 1px solid var(--b1); color: var(--t3);
  flex-shrink: 0; text-transform: uppercase;
}
.ext-pill.dir { background: rgba(251,191,36,.06); border-color: rgba(251,191,36,.2); color: var(--cy); }

.cell-size { font-family: var(--mono); font-size: 11px; color: var(--t3); white-space: nowrap; }
.cell-time { font-family: var(--mono); font-size: 11px; color: var(--t3); white-space: nowrap; }
.cell-mime { font-size: 11px; color: var(--t3); white-space: nowrap; max-width: 120px; overflow: hidden; text-overflow: ellipsis; }
.cell-action { text-align: right; white-space: nowrap; }

.act-btn {
  display: inline-flex; align-items: center; justify-content: center;
  width: 26px; height: 26px; border-radius: 4px;
  background: none; border: 1px solid transparent; color: var(--t3);
  cursor: pointer; transition: all .15s; text-decoration: none;
  opacity: 0;
}
.file-row:hover .act-btn { opacity: 1; }
.act-btn .material-icons-round { font-size: 14px; }
.act-btn:hover { border-color: var(--b2); color: var(--cx); background: var(--s2); }
.act-btn.del:hover { color: var(--cr); border-color: rgba(248,113,113,.3); }

/* Grid view */
.file-grid { display: none; }
.file-grid.active { display: grid; grid-template-columns: repeat(auto-fill, minmax(130px, 1fr)); gap: 10px; }
.grid-card {
  background: var(--s1); border: 1px solid var(--b0);
  border-radius: var(--r); padding: 16px 10px 12px;
  display: flex; flex-direction: column; align-items: center; gap: 6px;
  cursor: pointer; transition: border-color .15s, transform .15s, background .15s;
  text-decoration: none; animation: rowIn .25s ease both; position: relative;
}
.grid-card:hover { border-color: var(--b2); background: var(--s2); transform: translateY(-2px); }
.grid-card.selected { border-color: var(--cx2); background: rgba(77,159,255,.08); }
.grid-card.hidden { display: none; }
.grid-check { position: absolute; top: 6px; left: 6px; }
.grid-icon { font-size: 30px !important; }
.grid-name { font-size: 11px; font-weight: 500; color: var(--t1); text-align: center; word-break: break-all; line-height: 1.4; }
.grid-meta { font-family: var(--mono); font-size: 9.5px; color: var(--t3); }

/* Context menu */
.ctx-menu {
  position: fixed; background: var(--s1); border: 1px solid var(--b1);
  border-radius: var(--r); padding: 4px; min-width: 170px;
  box-shadow: var(--shadow); z-index: 999; display: none;
  animation: menuIn .12s ease both;
}
.ctx-menu.visible { display: block; }
@keyframes menuIn { from { opacity:0; transform:scale(.95) } to { opacity:1; transform:none } }
.ctx-item {
  display: flex; align-items: center; gap: 9px;
  padding: 7px 12px; border-radius: 4px;
  font-size: 12.5px; color: var(--t1); cursor: pointer; transition: background .1s;
}
.ctx-item:hover { background: var(--s3); color: var(--t0); }
.ctx-item .material-icons-round { font-size: 15px; color: var(--t2); }
.ctx-item.danger { color: var(--cr); }
.ctx-item.danger .material-icons-round { color: var(--cr); }
.ctx-sep { height: 1px; background: var(--b0); margin: 4px 0; }

/* ── Details Panel ── */
.details-panel {
  background: var(--s0); border-left: 1px solid var(--b0);
  display: flex; flex-direction: column; overflow: hidden;
}
.dp-header {
  display: flex; align-items: center; justify-content: space-between;
  padding: 14px 16px; border-bottom: 1px solid var(--b0); flex-shrink: 0;
}
.dp-title { font-family: var(--mono); font-size: 11px; letter-spacing: 1px; text-transform: uppercase; color: var(--t2); }
.dp-close { background: none; border: none; color: var(--t3); cursor: pointer; padding: 2px; border-radius: 3px; }
.dp-close:hover { color: var(--t0); }
.dp-close .material-icons-round { font-size: 16px; }

.dp-preview {
  display: flex; align-items: center; justify-content: center;
  height: 160px; background: var(--s1); border-bottom: 1px solid var(--b0);
  overflow: hidden; flex-shrink: 0; position: relative;
}
.dp-preview img { max-width: 100%; max-height: 100%; object-fit: contain; }
.dp-preview video { max-width: 100%; max-height: 100%; }
.dp-preview .dp-icon { font-size: 56px !important; color: var(--t3); }
.dp-preview .dp-audio-wrap { text-align: center; }
.dp-preview audio { width: 220px; margin-top: 8px; }

.dp-body { flex: 1; overflow-y: auto; padding: 16px; }
.dp-name { font-size: 13px; font-weight: 600; color: var(--t0); margin-bottom: 12px; word-break: break-all; }
.dp-row { display: flex; justify-content: space-between; align-items: flex-start; padding: 7px 0; border-bottom: 1px solid var(--b0); }
.dp-row:last-child { border-bottom: none; }
.dp-key { font-family: var(--mono); font-size: 10.5px; color: var(--t3); white-space: nowrap; }
.dp-val { font-family: var(--mono); font-size: 10.5px; color: var(--t1); text-align: right; word-break: break-all; max-width: 60%; }

.dp-actions { padding: 12px 16px; border-top: 1px solid var(--b0); display: flex; gap: 6px; flex-wrap: wrap; }
.dp-action-btn {
  display: flex; align-items: center; gap: 5px;
  padding: 6px 10px; border-radius: var(--r);
  font-size: 12px; cursor: pointer; border: 1px solid var(--b1);
  background: var(--s2); color: var(--t1); text-decoration: none; transition: all .15s;
}
.dp-action-btn:hover { background: var(--b1); color: var(--t0); }
.dp-action-btn .material-icons-round { font-size: 13px; }
.dp-action-btn.danger { color: var(--cr); }
.dp-action-btn.danger:hover { background: rgba(248,113,113,.1); border-color: rgba(248,113,113,.3); }

/* Code preview */
.code-preview {
  font-family: var(--mono); font-size: 11px; color: var(--t1);
  background: var(--s1); border-radius: var(--r); padding: 12px;
  overflow: auto; max-height: 300px; margin-top: 12px;
  border: 1px solid var(--b0); white-space: pre; tab-size: 2;
}
.truncate-note { font-size: 11px; color: var(--t3); margin-top: 6px; font-style: italic; }

/* ── Modals ── */
.modal-overlay {
  position: fixed; inset: 0; background: rgba(0,0,0,.6);
  display: none; align-items: center; justify-content: center; z-index: 500;
  backdrop-filter: blur(2px);
}
.modal-overlay.open { display: flex; }
.modal {
  background: var(--s1); border: 1px solid var(--b1); border-radius: 10px;
  width: 400px; max-width: 90vw; padding: 24px;
  animation: modalIn .2s ease;
}
@keyframes modalIn { from { opacity:0; transform:scale(.94) translateY(-10px) } to { opacity:1; transform:none } }
.modal-title { font-family: var(--mono); font-size: 13px; font-weight: 600; color: var(--t0); margin-bottom: 16px; }
.modal-label { font-size: 12px; color: var(--t2); margin-bottom: 6px; }
.modal-input {
  width: 100%; background: var(--s0); border: 1px solid var(--b1); border-radius: var(--r);
  padding: 9px 12px; font-family: var(--mono); font-size: 13px; color: var(--t0);
  outline: none; transition: border-color .15s; margin-bottom: 16px;
}
.modal-input:focus { border-color: var(--cx); }
.modal-btns { display: flex; gap: 8px; justify-content: flex-end; }
.modal-btn {
  padding: 7px 16px; border-radius: var(--r); font-size: 13px; font-weight: 500;
  cursor: pointer; border: 1px solid var(--b1); transition: all .15s;
}
.modal-btn.cancel { background: var(--s2); color: var(--t1); }
.modal-btn.cancel:hover { background: var(--b1); color: var(--t0); }
.modal-btn.confirm { background: var(--cx3); border-color: var(--cx2); color: var(--t0); }
.modal-btn.confirm:hover { background: var(--cx2); }
.modal-btn.danger { background: rgba(248,113,113,.1); border-color: rgba(248,113,113,.3); color: var(--cr); }
.modal-btn.danger:hover { background: rgba(248,113,113,.2); }

/* Upload modal */
.upload-zone {
  border: 2px dashed var(--b2); border-radius: var(--r);
  padding: 32px 20px; text-align: center; cursor: pointer;
  transition: border-color .15s, background .15s; margin-bottom: 16px;
}
.upload-zone:hover, .upload-zone.drag { border-color: var(--cx); background: rgba(77,159,255,.04); }
.upload-zone .material-icons-round { font-size: 40px; color: var(--t3); display: block; margin-bottom: 8px; }
.upload-zone p { font-size: 13px; color: var(--t2); }
.upload-zone small { font-size: 11px; color: var(--t3); }
.upload-list { max-height: 120px; overflow-y: auto; margin-bottom: 12px; }
.upload-file-item { display: flex; align-items: center; gap: 8px; padding: 5px 0; font-size: 12px; color: var(--t1); border-bottom: 1px solid var(--b0); }
.upload-file-item .material-icons-round { font-size: 14px; color: var(--t3); }
.upload-progress { height: 3px; background: var(--b1); border-radius: 2px; margin-top: 4px; overflow: hidden; }
.upload-progress-fill { height: 100%; background: linear-gradient(90deg, var(--cx), var(--cg)); border-radius: 2px; transition: width .3s; }

/* Toast */
.toast-container { position: fixed; bottom: 20px; right: 20px; display: flex; flex-direction: column; gap: 8px; z-index: 9999; }
.toast {
  display: flex; align-items: center; gap: 10px;
  padding: 10px 16px; border-radius: var(--r);
  background: var(--s2); border: 1px solid var(--b1);
  color: var(--t0); font-size: 13px; box-shadow: var(--shadow);
  animation: toastIn .2s ease; min-width: 220px;
}
.toast.success { border-color: rgba(52,211,153,.3); background: rgba(52,211,153,.08); }
.toast.error   { border-color: rgba(248,113,113,.3); background: rgba(248,113,113,.08); }
.toast .material-icons-round { font-size: 16px; }
.toast.success .material-icons-round { color: var(--cg); }
.toast.error   .material-icons-round { color: var(--cr); }
@keyframes toastIn { from { opacity:0; transform:translateX(20px) } to { opacity:1; transform:none } }

/* Empty state */
.empty { text-align: center; padding: 80px 0; color: var(--t3); }
.empty .material-icons-round { font-size: 48px; display: block; margin-bottom: 12px; color: var(--b2); }
.empty p { font-size: 14px; }

/* Stagger animation */
.file-row:nth-child(1)  { animation-delay: 0ms }
.file-row:nth-child(2)  { animation-delay: 20ms }
.file-row:nth-child(3)  { animation-delay: 40ms }
.file-row:nth-child(4)  { animation-delay: 60ms }
.file-row:nth-child(5)  { animation-delay: 80ms }
.file-row:nth-child(6)  { animation-delay: 100ms }
.file-row:nth-child(7)  { animation-delay: 120ms }
.file-row:nth-child(8)  { animation-delay: 140ms }
.file-row:nth-child(9)  { animation-delay: 160ms }
.file-row:nth-child(10) { animation-delay: 180ms }
</style>
</head>
<body>
<div class="app" id="app">

  <!-- ── Topbar ─────────────────────────── -->
  <header class="topbar">
    <div class="logo">
      <span class="material-icons-round logo-icon">dns</span>
      VAULT
    </div>
    <nav class="topbar-path">
      <a href="/browse/">~</a>
      {{range .Breadcrumbs}}<span class="sep">/</span><a href="/browse{{.Path}}">{{.Name}}</a>{{end}}
    </nav>
    <div class="topbar-right">
      <button class="tbar-btn" onclick="toggleTheme()" title="Toggle theme">
        <span class="material-icons-round">contrast</span>
      </button>
      <button class="tbar-btn" onclick="openUpload()" title="Upload files">
        <span class="material-icons-round">upload</span>
      </button>
      <span class="clock" id="clock">{{.ServerTime}}</span>
    </div>
  </header>

  <!-- ── Sidebar ────────────────────────── -->
  <aside class="sidebar">
    <div class="sb-section">
      <div class="sb-label">Browse</div>
      <a href="/browse/" class="sb-nav-btn {{if eq .CurrentPath "/"}}active{{end}}">
        <span class="material-icons-round">home</span> Home
      </a>
      {{if .ParentPath}}
      <a href="/browse{{.ParentPath}}" class="sb-nav-btn">
        <span class="material-icons-round">arrow_upward</span> Parent
      </a>
      {{end}}
    </div>

    <div class="sb-divider"></div>
    <div class="stats-row">
      <div class="mini-stat c-blue">
        <div class="v" id="sb-files">{{.TotalFiles}}</div>
        <div class="l">Files</div>
      </div>
      <div class="mini-stat c-green">
        <div class="v" id="sb-dirs">{{.TotalDirs}}</div>
        <div class="l">Folders</div>
      </div>
    </div>

    <div class="disk-widget" id="diskWidget">
      <div class="disk-row">
        <span class="disk-label">Disk Usage</span>
        <span class="disk-val" id="diskPct">–</span>
      </div>
      <div class="disk-bar"><div class="disk-fill" id="diskFill" style="width:0%"></div></div>
      <div class="disk-row" style="margin-top:6px;margin-bottom:0">
        <span class="disk-label" id="diskFree">–</span>
        <span class="disk-val" id="diskTotal">–</span>
      </div>
    </div>

    <div class="sb-divider"></div>
    <div class="sb-section" style="padding-bottom:4px">
      <div class="sb-label">Recent</div>
    </div>
    <div class="sb-scroll" id="recentList"></div>
  </aside>

  <!-- ── Main ───────────────────────────── -->
  <div class="main-wrap">
    <main class="main" id="mainPane"
      ondragover="handleDragOver(event)"
      ondragleave="handleDragLeave(event)"
      ondrop="handleDrop(event)">

      <div class="drop-overlay" id="dropOverlay">
        <span class="material-icons-round">cloud_upload</span>
        Drop files to upload
      </div>

      <!-- Toolbar -->
      <div class="toolbar">
        <div class="search-wrap">
          <span class="material-icons-round search-icon">search</span>
          <input class="search-input" type="text" placeholder="Filter…" id="searchInput" oninput="filterFiles()">
        </div>
        <div class="tb-sep"></div>
        <button class="tb-btn primary" onclick="openUpload()">
          <span class="material-icons-round">upload</span> Upload
        </button>
        <button class="tb-btn" onclick="openMkdir()">
          <span class="material-icons-round">create_new_folder</span> New Folder
        </button>
        <button class="tb-btn" onclick="openNewFile()">
          <span class="material-icons-round">note_add</span> New File
        </button>
        <div class="tb-sep"></div>
        <div class="tb-spacer"></div>
        <button class="tb-btn" id="btnSort" onclick="cycleSortMenu()" title="Sort">
          <span class="material-icons-round">sort</span>
        </button>
        <button class="tb-btn" id="btnList" onclick="setView('list')" title="List view">
          <span class="material-icons-round">view_list</span>
        </button>
        <button class="tb-btn" id="btnGrid" onclick="setView('grid')" title="Grid view">
          <span class="material-icons-round">grid_view</span>
        </button>
      </div>

      <!-- Bulk actions bar -->
      <div class="bulk-bar" id="bulkBar">
        <span class="bulk-count" id="bulkCount">0 selected</span>
        <button class="tb-btn" onclick="bulkDownloadZip()">
          <span class="material-icons-round">archive</span> Download ZIP
        </button>
        <button class="tb-btn danger" onclick="bulkDelete()">
          <span class="material-icons-round">delete</span> Delete
        </button>
        <div class="bulk-spacer"></div>
        <button class="tb-btn" onclick="clearSelection()">
          <span class="material-icons-round">close</span> Clear
        </button>
      </div>

      <!-- Files -->
      <div class="file-area" id="fileArea">
        {{if .Entries}}

        <!-- List View -->
        <table class="file-table" id="listView">
          <thead class="ft-head">
            <tr>
              <th class="th-check"><input type="checkbox" class="row-check" id="selectAll" onchange="toggleSelectAll(this)"></th>
              <th onclick="sortBy('name')">Name</th>
              <th onclick="sortBy('ext')">Type</th>
              <th onclick="sortBy('size')">Size</th>
              <th onclick="sortBy('time')">Modified</th>
              <th>Actions</th>
            </tr>
          </thead>
          <tbody id="listBody">
            {{range .Entries}}
            <tr class="file-row"
                data-name="{{.Name}}"
                data-path="{{.Path}}"
                data-isdir="{{.IsDir}}"
                data-size="{{.SizeRaw}}"
                data-time="{{.ModTime}}"
                data-ext="{{.Ext}}"
                data-mime="{{.MimeType}}"
                oncontextmenu="showCtxMenu(event, '{{.Path}}', {{.IsDir}}, '{{.Name}}')">
              <td><input type="checkbox" class="row-check file-check" onchange="onCheckChange()"></td>
              <td>
                <div class="cell-name">
                  <span class="material-icons-round file-icon {{if .IsDir}}icon-folder{{else if eq .Icon "code"}}icon-code{{else if eq .Icon "html"}}icon-code{{else if eq .Icon "image"}}icon-image{{else if eq .Icon "movie"}}icon-media{{else if eq .Icon "audio_file"}}icon-media{{else if eq .Icon "folder_zip"}}icon-zip{{else if eq .Icon "table_chart"}}icon-data{{else if eq .Icon "storage"}}icon-data{{end}}">{{.Icon}}</span>
                  {{if .IsDir}}
                    <a href="/browse{{.Path}}">{{.Name}}</a>
                  {{else}}
                    <a href="#" onclick="openPreview('{{.Path}}', '{{.MimeType}}', '{{.Name}}'); return false;">{{.Name}}</a>
                  {{end}}
                  <span class="ext-pill {{if .IsDir}}dir{{end}}">{{.Ext}}</span>
                </div>
              </td>
              <td class="cell-mime">{{.MimeType}}</td>
              <td class="cell-size">{{if not .IsDir}}{{.Size}}{{else}}—{{end}}</td>
              <td class="cell-time">{{.ModTime}}</td>
              <td class="cell-action">
                {{if not .IsDir}}
                <a class="act-btn" href="#" onclick="openPreview('{{.Path}}', '{{.MimeType}}', '{{.Name}}'); return false;" title="Preview">
                  <span class="material-icons-round">visibility</span>
                </a>
                <a class="act-btn" href="/files{{.Path}}" download title="Download">
                  <span class="material-icons-round">download</span>
                </a>
                {{else}}
                <a class="act-btn" href="/browse{{.Path}}" title="Open">
                  <span class="material-icons-round">folder_open</span>
                </a>
                {{end}}
                <button class="act-btn" onclick="openRename('{{.Path}}', '{{.Name}}')" title="Rename">
                  <span class="material-icons-round">drive_file_rename_outline</span>
                </button>
                <button class="act-btn del" onclick="confirmDelete(['{{.Path}}'])" title="Delete">
                  <span class="material-icons-round">delete</span>
                </button>
              </td>
            </tr>
            {{end}}
          </tbody>
        </table>

        <!-- Grid View -->
        <div class="file-grid" id="gridView">
          {{range .Entries}}
          <a class="grid-card"
             data-name="{{.Name}}" data-path="{{.Path}}" data-isdir="{{.IsDir}}"
             href="{{if .IsDir}}/browse{{.Path}}{{else}}#{{end}}"
             onclick="{{if not .IsDir}}openPreview('{{.Path}}', '{{.MimeType}}', '{{.Name}}'); return false;{{end}}"
             oncontextmenu="showCtxMenu(event, '{{.Path}}', {{.IsDir}}, '{{.Name}}')">
            <input type="checkbox" class="row-check grid-check file-check" onchange="onCheckChange()" onclick="event.stopPropagation()">
            <span class="material-icons-round grid-icon {{if .IsDir}}icon-folder{{else if eq .Icon "code"}}icon-code{{else if eq .Icon "html"}}icon-code{{else if eq .Icon "image"}}icon-image{{else if eq .Icon "movie"}}icon-media{{else if eq .Icon "audio_file"}}icon-media{{else if eq .Icon "folder_zip"}}icon-zip{{else if eq .Icon "table_chart"}}icon-data{{else if eq .Icon "storage"}}icon-data{{end}}">{{.Icon}}</span>
            <span class="grid-name">{{.Name}}</span>
            <span class="grid-meta">{{if not .IsDir}}{{.Size}}{{else}}DIR{{end}}</span>
          </a>
          {{end}}
        </div>

        {{else}}
        <div class="empty">
          <span class="material-icons-round">folder_open</span>
          <p>This directory is empty.</p>
        </div>
        {{end}}
      </div>
    </main>

    <!-- Details Panel -->
    <aside class="details-panel" id="detailsPanel" style="display:none">
      <div class="dp-header">
        <span class="dp-title">Details</span>
        <button class="dp-close" onclick="closePanel()">
          <span class="material-icons-round">close</span>
        </button>
      </div>
      <div class="dp-preview" id="dpPreview">
        <span class="material-icons-round dp-icon">insert_drive_file</span>
      </div>
      <div class="dp-body">
        <div class="dp-name" id="dpName">–</div>
        <div id="dpMeta"></div>
        <div id="dpCodePreview"></div>
      </div>
      <div class="dp-actions" id="dpActions"></div>
    </aside>
  </div>
</div>

<!-- Context Menu -->
<div class="ctx-menu" id="ctxMenu">
  <div class="ctx-item" id="ctxOpen"><span class="material-icons-round">open_in_new</span> Open</div>
  <div class="ctx-item" id="ctxPreview"><span class="material-icons-round">visibility</span> Preview</div>
  <div class="ctx-sep"></div>
  <div class="ctx-item" id="ctxDownload"><span class="material-icons-round">download</span> Download</div>
  <div class="ctx-item" onclick="ctxAction('zip')"><span class="material-icons-round">archive</span> Download as ZIP</div>
  <div class="ctx-sep"></div>
  <div class="ctx-item" onclick="ctxAction('rename')"><span class="material-icons-round">drive_file_rename_outline</span> Rename</div>
  <div class="ctx-item" onclick="ctxAction('copy-path')"><span class="material-icons-round">content_copy</span> Copy Path</div>
  <div class="ctx-sep"></div>
  <div class="ctx-item danger" onclick="ctxAction('delete')"><span class="material-icons-round">delete</span> Delete</div>
</div>

<!-- Toast container -->
<div class="toast-container" id="toastContainer"></div>

<!-- ── Modals ── -->

<!-- Upload Modal -->
<div class="modal-overlay" id="uploadModal">
  <div class="modal">
    <div class="modal-title">Upload Files</div>
    <div class="upload-zone" id="uploadZone" onclick="document.getElementById('fileInput').click()">
      <span class="material-icons-round">cloud_upload</span>
      <p>Click to select files or drag & drop here</p>
      <small>Multiple files supported</small>
    </div>
    <input type="file" id="fileInput" multiple style="display:none" onchange="handleFileSelect(this.files)">
    <div class="upload-list" id="uploadList"></div>
    <div class="upload-progress" id="uploadProgress" style="display:none">
      <div class="upload-progress-fill" id="uploadProgressFill" style="width:0%"></div>
    </div>
    <div class="modal-btns">
      <button class="modal-btn cancel" onclick="closeModal('uploadModal')">Cancel</button>
      <button class="modal-btn confirm" onclick="doUpload()">Upload</button>
    </div>
  </div>
</div>

<!-- Mkdir Modal -->
<div class="modal-overlay" id="mkdirModal">
  <div class="modal">
    <div class="modal-title">New Folder</div>
    <div class="modal-label">Folder name</div>
    <input class="modal-input" type="text" id="mkdirInput" placeholder="untitled-folder" onkeydown="if(event.key==='Enter')doMkdir()">
    <div class="modal-btns">
      <button class="modal-btn cancel" onclick="closeModal('mkdirModal')">Cancel</button>
      <button class="modal-btn confirm" onclick="doMkdir()">Create</button>
    </div>
  </div>
</div>

<!-- New File Modal -->
<div class="modal-overlay" id="newFileModal">
  <div class="modal">
    <div class="modal-title">New File</div>
    <div class="modal-label">File name</div>
    <input class="modal-input" type="text" id="newFileInput" placeholder="untitled.txt" onkeydown="if(event.key==='Enter')doNewFile()">
    <div class="modal-btns">
      <button class="modal-btn cancel" onclick="closeModal('newFileModal')">Cancel</button>
      <button class="modal-btn confirm" onclick="doNewFile()">Create</button>
    </div>
  </div>
</div>

<!-- Rename Modal -->
<div class="modal-overlay" id="renameModal">
  <div class="modal">
    <div class="modal-title">Rename</div>
    <div class="modal-label">New name</div>
    <input class="modal-input" type="text" id="renameInput" onkeydown="if(event.key==='Enter')doRename()">
    <div class="modal-btns">
      <button class="modal-btn cancel" onclick="closeModal('renameModal')">Cancel</button>
      <button class="modal-btn confirm" onclick="doRename()">Rename</button>
    </div>
  </div>
</div>

<!-- Delete Modal -->
<div class="modal-overlay" id="deleteModal">
  <div class="modal">
    <div class="modal-title">Confirm Delete</div>
    <p id="deleteMsg" style="font-size:13px;color:var(--t1);margin-bottom:20px;line-height:1.6"></p>
    <div class="modal-btns">
      <button class="modal-btn cancel" onclick="closeModal('deleteModal')">Cancel</button>
      <button class="modal-btn danger" onclick="doDelete()">Delete</button>
    </div>
  </div>
</div>

<script>
const CURRENT_PATH = {{printf "%q" .CurrentPath}};
let entries = {{.EntriesJSON}};
let ctxTarget = null;
let renameTarget = null;
let deleteTargets = [];
let pendingUploadFiles = [];
let sortField = 'name', sortDir = 'asc';

// ── Clock ──────────────────────────────────────────────────────────────────
function tick() {
  const el = document.getElementById('clock');
  if (el) el.textContent = new Date().toLocaleTimeString('en-US', {hour12:false});
}
tick(); setInterval(tick, 1000);

// ── View toggle ────────────────────────────────────────────────────────────
let currentView = localStorage.getItem('vaultView') || 'list';
function setView(v) {
  currentView = v;
  localStorage.setItem('vaultView', v);
  const list = document.getElementById('listView');
  const grid = document.getElementById('gridView');
  const btnL = document.getElementById('btnList');
  const btnG = document.getElementById('btnGrid');
  if (!list) return;
  if (v === 'grid') {
    list.style.display = 'none'; grid.classList.add('active');
    btnG.classList.add('active'); btnL.classList.remove('active');
  } else {
    list.style.display = ''; grid.classList.remove('active');
    btnL.classList.add('active'); btnG.classList.remove('active');
  }
}
setView(currentView);

// ── Filter ────────────────────────────────────────────────────────────────
function filterFiles() {
  const q = document.getElementById('searchInput').value.toLowerCase();
  document.querySelectorAll('[data-name]').forEach(el => {
    el.classList.toggle('hidden', !el.dataset.name.toLowerCase().includes(q));
  });
}

// ── Sort ──────────────────────────────────────────────────────────────────
function sortBy(field) {
  if (sortField === field) sortDir = sortDir === 'asc' ? 'desc' : 'asc';
  else { sortField = field; sortDir = 'asc'; }
  document.querySelectorAll('.ft-head th').forEach(th => {
    th.classList.remove('sort-asc','sort-desc');
  });
  const thMap = {name:1,ext:2,size:3,time:4};
  const ths = document.querySelectorAll('.ft-head th');
  if (thMap[field]) ths[thMap[field]].classList.add(sortDir==='asc'?'sort-asc':'sort-desc');

  const tbody = document.getElementById('listBody');
  const rows = Array.from(tbody.querySelectorAll('.file-row'));
  rows.sort((a,b) => {
    let av, bv;
    if (field==='name') { av=a.dataset.name.toLowerCase(); bv=b.dataset.name.toLowerCase(); }
    else if (field==='ext') { av=a.dataset.ext; bv=b.dataset.ext; }
    else if (field==='size') { av=parseInt(a.dataset.size)||0; bv=parseInt(b.dataset.size)||0; return sortDir==='asc'?av-bv:bv-av; }
    else if (field==='time') { av=a.dataset.time; bv=b.dataset.time; }
    // dirs always first
    const aDir = a.dataset.isdir==='true', bDir = b.dataset.isdir==='true';
    if (aDir!==bDir) return aDir?-1:1;
    const cmp = av<bv?-1:av>bv?1:0;
    return sortDir==='asc'?cmp:-cmp;
  });
  rows.forEach(r => tbody.appendChild(r));
}
function cycleSortMenu() { sortBy(sortField); }

// ── Selection ─────────────────────────────────────────────────────────────
function getSelected() {
  const rows = document.querySelectorAll('.file-row');
  const cards = document.querySelectorAll('.grid-card');
  const selected = [];
  rows.forEach(r => {
    if (r.querySelector('.file-check')?.checked) selected.push(r.dataset.path);
  });
  if (selected.length === 0) {
    cards.forEach(c => {
      if (c.querySelector('.file-check')?.checked) selected.push(c.dataset.path);
    });
  }
  return [...new Set(selected)];
}
function onCheckChange() {
  const sel = getSelected();
  const bar = document.getElementById('bulkBar');
  document.getElementById('bulkCount').textContent = sel.length + ' selected';
  bar.classList.toggle('visible', sel.length > 0);
  document.querySelectorAll('.file-row').forEach(r => {
    r.classList.toggle('selected', r.querySelector('.file-check')?.checked);
  });
  document.querySelectorAll('.grid-card').forEach(c => {
    c.classList.toggle('selected', c.querySelector('.file-check')?.checked);
  });
}
function toggleSelectAll(cb) {
  document.querySelectorAll('.file-check').forEach(c => c.checked = cb.checked);
  onCheckChange();
}
function clearSelection() {
  document.querySelectorAll('.file-check, #selectAll').forEach(c => c.checked = false);
  onCheckChange();
}

// ── Context Menu ──────────────────────────────────────────────────────────
function showCtxMenu(e, path, isDir, name) {
  e.preventDefault();
  ctxTarget = {path, isDir, name};
  const m = document.getElementById('ctxMenu');
  const open = document.getElementById('ctxOpen');
  const prev = document.getElementById('ctxPreview');
  const dl   = document.getElementById('ctxDownload');
  open.style.display = isDir ? '' : 'none';
  prev.style.display = isDir ? 'none' : '';
  dl.style.display   = isDir ? 'none' : '';
  if (!isDir) {
    prev.onclick = () => { const entry=entries.find(x=>x.path===path); openPreview(path, entry?.mimeType||'', name); closeCtx(); };
    dl.onclick   = () => { window.location='/files'+path; closeCtx(); };
  } else {
    open.onclick = () => { window.location='/browse'+path; closeCtx(); };
  }
  m.style.left = Math.min(e.clientX, window.innerWidth-180) + 'px';
  m.style.top  = Math.min(e.clientY, window.innerHeight-260) + 'px';
  m.classList.add('visible');
}
function ctxAction(action) {
  if (!ctxTarget) return;
  closeCtx();
  if (action==='rename')    openRename(ctxTarget.path, ctxTarget.name);
  if (action==='delete')    confirmDelete([ctxTarget.path]);
  if (action==='copy-path') { navigator.clipboard.writeText(ctxTarget.path); toast('Path copied', 'success'); }
  if (action==='zip')       bulkDownloadZipPaths([ctxTarget.path]);
}
function closeCtx() { document.getElementById('ctxMenu').classList.remove('visible'); }
document.addEventListener('click', closeCtx);

// ── Preview / Details Panel ────────────────────────────────────────────────
async function openPreview(path, mime, name) {
  addRecent(name, path, mime);
  const panel = document.getElementById('detailsPanel');
  const app   = document.getElementById('app');
  panel.style.display = 'flex';
  app.classList.add('panel-open');

  document.getElementById('dpName').textContent = name;
  document.getElementById('dpPreview').innerHTML = '<span class="material-icons-round dp-icon">hourglass_empty</span>';
  document.getElementById('dpMeta').innerHTML = '';
  document.getElementById('dpCodePreview').innerHTML = '';

  // Fetch info
  const infoRes = await fetch('/api/fileinfo?path='+encodeURIComponent(path));
  const info = await infoRes.json();

  let metaHtml = '';
  const rows = [
    ['Size',     info.isDir ? info.dirSize : info.size],
    ['Type',     info.isDir ? 'Directory ('+info.dirCount+' files)' : info.mimeType],
    ['Modified', info.modTime],
    ['Path',     info.path],
  ];
  rows.forEach(([k,v]) => { metaHtml += '<div class="dp-row"><span class="dp-key">'+k+'</span><span class="dp-val">'+v+'</span></div>'; });
  document.getElementById('dpMeta').innerHTML = metaHtml;

  // Actions
  let actHtml = '';
  if (!info.isDir) {
    actHtml += '<a class="dp-action-btn" href="/files'+path+'" download><span class="material-icons-round">download</span> Download</a>';
    actHtml += '<a class="dp-action-btn" href="/files'+path+'" target="_blank"><span class="material-icons-round">open_in_new</span> Open</a>';
  }
  actHtml += '<button class="dp-action-btn" onclick="openRename(\''+path+'\',\''+name+'\')"><span class="material-icons-round">edit</span> Rename</button>';
  actHtml += '<button class="dp-action-btn danger" onclick="confirmDelete([\''+path+'\'])"><span class="material-icons-round">delete</span> Delete</button>';
  document.getElementById('dpActions').innerHTML = actHtml;

  // Preview
  const prev = document.getElementById('dpPreview');
  if (mime.startsWith('image/')) {
    prev.innerHTML = '<img src="/files'+path+'" alt="'+name+'" loading="lazy">';
  } else if (mime.startsWith('video/')) {
    prev.innerHTML = '<video controls src="/files'+path+'"></video>';
  } else if (mime.startsWith('audio/')) {
    prev.innerHTML = '<div class="dp-audio-wrap"><span class="material-icons-round" style="font-size:48px;color:var(--cg)">audio_file</span><br><audio controls src="/files'+path+'"></audio></div>';
  } else if (mime === 'application/pdf') {
    prev.innerHTML = '<span class="material-icons-round" style="font-size:48px;color:var(--cr)">picture_as_pdf</span>';
    document.getElementById('dpCodePreview').innerHTML = '<a class="dp-action-btn" href="/files'+path+'" target="_blank" style="display:inline-flex;margin-top:8px"><span class="material-icons-round">open_in_new</span> Open PDF</a>';
  } else if (mime.startsWith('text/') || mime === 'application/json') {
    prev.innerHTML = '<span class="material-icons-round dp-icon">article</span>';
    // load text
    const textRes = await fetch('/api/text?path='+encodeURIComponent(path));
    const truncated = textRes.headers.get('x-truncated') === 'true';
    const text = await textRes.text();
    const escaped = text.replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;');
    let codeHtml = '<div class="code-preview">'+escaped+'</div>';
    if (truncated) codeHtml += '<p class="truncate-note">Preview truncated (showing first 1 MB)</p>';
    document.getElementById('dpCodePreview').innerHTML = codeHtml;
  } else {
    const icon = entries.find(x=>x.path===path)?.icon || 'insert_drive_file';
    prev.innerHTML = '<span class="material-icons-round dp-icon">'+icon+'</span>';
  }
}
function closePanel() {
  document.getElementById('detailsPanel').style.display = 'none';
  document.getElementById('app').classList.remove('panel-open');
}

// ── Upload ────────────────────────────────────────────────────────────────
let uploadFiles = [];
function openUpload() {
  uploadFiles = [];
  document.getElementById('uploadList').innerHTML = '';
  document.getElementById('uploadProgress').style.display = 'none';
  document.getElementById('uploadProgressFill').style.width = '0%';
  document.getElementById('fileInput').value = '';
  openModal('uploadModal');
}
function handleFileSelect(files) {
  uploadFiles = Array.from(files);
  renderUploadList();
}
function renderUploadList() {
  const list = document.getElementById('uploadList');
  list.innerHTML = uploadFiles.map(f =>
    '<div class="upload-file-item"><span class="material-icons-round">insert_drive_file</span>'+
    '<span style="flex:1;overflow:hidden;text-overflow:ellipsis;white-space:nowrap">'+f.name+'</span>'+
    '<span style="font-family:var(--mono);font-size:11px;color:var(--t3)">'+formatSizeJS(f.size)+'</span></div>'
  ).join('');
}
async function doUpload() {
  if (!uploadFiles.length) { toast('No files selected','error'); return; }
  const fd = new FormData();
  fd.append('path', CURRENT_PATH);
  uploadFiles.forEach(f => fd.append('files', f));
  document.getElementById('uploadProgress').style.display = 'block';
  try {
    const xhr = new XMLHttpRequest();
    xhr.upload.onprogress = e => {
      if (e.lengthComputable) document.getElementById('uploadProgressFill').style.width = (e.loaded/e.total*100)+'%';
    };
    xhr.onload = () => {
      const r = JSON.parse(xhr.responseText);
      if (r.success) { toast(r.message,'success'); closeModal('uploadModal'); location.reload(); }
      else toast(r.message,'error');
    };
    xhr.open('POST','/api/upload'); xhr.send(fd);
  } catch(err) { toast('Upload failed','error'); }
}
// Drag & drop upload
function handleDragOver(e) {
  e.preventDefault(); document.getElementById('dropOverlay').classList.add('active');
}
function handleDragLeave(e) {
  if (!e.relatedTarget || !document.getElementById('mainPane').contains(e.relatedTarget))
    document.getElementById('dropOverlay').classList.remove('active');
}
function handleDrop(e) {
  e.preventDefault(); document.getElementById('dropOverlay').classList.remove('active');
  const files = Array.from(e.dataTransfer.files);
  if (!files.length) return;
  uploadFiles = files;
  openUpload();
  renderUploadList();
}
// Upload zone drag
const uz = document.getElementById('uploadZone');
uz.addEventListener('dragover', e => { e.preventDefault(); uz.classList.add('drag'); });
uz.addEventListener('dragleave', () => uz.classList.remove('drag'));
uz.addEventListener('drop', e => { e.preventDefault(); uz.classList.remove('drag'); handleFileSelect(e.dataTransfer.files); });

// ── Mkdir ─────────────────────────────────────────────────────────────────
function openMkdir() { document.getElementById('mkdirInput').value=''; openModal('mkdirModal'); setTimeout(()=>document.getElementById('mkdirInput').focus(),100); }
async function doMkdir() {
  const name = document.getElementById('mkdirInput').value.trim();
  if (!name) return;
  const r = await apiFetch('/api/mkdir', {path:CURRENT_PATH, dirName:name});
  if (r.success) { toast(r.message,'success'); closeModal('mkdirModal'); location.reload(); }
  else toast(r.message,'error');
}

// ── New File ──────────────────────────────────────────────────────────────
function openNewFile() { document.getElementById('newFileInput').value=''; openModal('newFileModal'); setTimeout(()=>document.getElementById('newFileInput').focus(),100); }
async function doNewFile() {
  const name = document.getElementById('newFileInput').value.trim();
  if (!name) return;
  const r = await apiFetch('/api/newfile', {path:CURRENT_PATH, fileName:name});
  if (r.success) { toast(r.message,'success'); closeModal('newFileModal'); location.reload(); }
  else toast(r.message,'error');
}

// ── Rename ────────────────────────────────────────────────────────────────
function openRename(path, name) {
  renameTarget = path;
  document.getElementById('renameInput').value = name;
  openModal('renameModal');
  setTimeout(()=>{ const i=document.getElementById('renameInput'); i.focus(); i.select(); }, 100);
}
async function doRename() {
  const newName = document.getElementById('renameInput').value.trim();
  if (!newName || !renameTarget) return;
  const r = await apiFetch('/api/rename', {oldPath:renameTarget, newName});
  if (r.success) { toast(r.message,'success'); closeModal('renameModal'); location.reload(); }
  else toast(r.message,'error');
}

// ── Delete ────────────────────────────────────────────────────────────────
function confirmDelete(paths) {
  deleteTargets = paths;
  document.getElementById('deleteMsg').innerHTML =
    'Are you sure you want to delete <strong style="color:var(--cr)">'+(paths.length===1?paths[0]:paths.length+' items')+'</strong>?<br>This action cannot be undone.';
  openModal('deleteModal');
}
async function doDelete() {
  const r = await apiFetch('/api/delete', {paths:deleteTargets});
  if (r.success) { toast(r.message,'success'); closeModal('deleteModal'); location.reload(); }
  else toast(r.message,'error');
}
function bulkDelete() {
  const sel = getSelected(); if (!sel.length) return;
  confirmDelete(sel);
}

// ── Bulk ZIP ───────────────────────────────────────────────────────────────
async function bulkDownloadZip() { bulkDownloadZipPaths(getSelected()); }
async function bulkDownloadZipPaths(paths) {
  if (!paths.length) return;
  toast('Preparing ZIP…','success');
  const res = await fetch('/api/zip', { method:'POST', headers:{'Content-Type':'application/json'}, body:JSON.stringify({paths}) });
  const blob = await res.blob();
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a'); a.href=url; a.download='download.zip'; a.click();
  URL.revokeObjectURL(url);
}

// ── Disk stats ────────────────────────────────────────────────────────────
async function loadDiskStats() {
  try {
    const r = await fetch('/api/stats');
    const d = await r.json();
    document.getElementById('diskFree').textContent  = d.diskFree + ' free';
    document.getElementById('diskTotal').textContent = d.diskTotal;
    document.getElementById('diskPct').textContent   = d.diskUsedPct.toFixed(1)+'%';
    const fill = document.getElementById('diskFill');
    fill.style.width = d.diskUsedPct+'%';
    if (d.diskUsedPct > 85) fill.classList.add('warn');
  } catch(e) {}
}
loadDiskStats();

// ── Recent files ──────────────────────────────────────────────────────────
function addRecent(name, path, mime) {
  let recents = JSON.parse(localStorage.getItem('vaultRecent')||'[]');
  recents = recents.filter(r => r.path !== path);
  recents.unshift({name, path, mime, time: Date.now()});
  recents = recents.slice(0, 10);
  localStorage.setItem('vaultRecent', JSON.stringify(recents));
  renderRecent();
}
function renderRecent() {
  const recents = JSON.parse(localStorage.getItem('vaultRecent')||'[]');
  const list = document.getElementById('recentList');
  if (!recents.length) { list.innerHTML='<div style="padding:8px 16px;font-size:11px;color:var(--t3)">No recent files</div>'; return; }
  list.innerHTML = recents.map(r => {
    const ago = timeAgo(r.time);
    const icon = getIconForMime(r.mime);
    return '<div class="recent-item" onclick="openPreview(\''+r.path+'\',\''+r.mime+'\',\''+r.name+'\')">'
      +'<span class="material-icons-round">'+icon+'</span>'
      +'<span class="name">'+r.name+'</span>'
      +'<span class="rtime">'+ago+'</span></div>';
  }).join('');
}
function timeAgo(ts) {
  const s = Math.floor((Date.now()-ts)/1000);
  if (s<60) return 'now'; if (s<3600) return Math.floor(s/60)+'m'; if (s<86400) return Math.floor(s/3600)+'h';
  return Math.floor(s/86400)+'d';
}
function getIconForMime(mime) {
  if (!mime) return 'insert_drive_file';
  if (mime.startsWith('image/')) return 'image';
  if (mime.startsWith('video/')) return 'movie';
  if (mime.startsWith('audio/')) return 'audio_file';
  if (mime.startsWith('text/')) return 'article';
  if (mime==='application/pdf') return 'picture_as_pdf';
  return 'insert_drive_file';
}
renderRecent();

// ── Helpers ───────────────────────────────────────────────────────────────
async function apiFetch(url, body) {
  const r = await fetch(url, { method:'POST', headers:{'Content-Type':'application/json'}, body:JSON.stringify(body) });
  return r.json();
}
function openModal(id) { document.getElementById(id).classList.add('open'); }
function closeModal(id) { document.getElementById(id).classList.remove('open'); }
document.querySelectorAll('.modal-overlay').forEach(m => {
  m.addEventListener('click', e => { if (e.target===m) m.classList.remove('open'); });
});
function toast(msg, type='success') {
  const c = document.getElementById('toastContainer');
  const t = document.createElement('div');
  t.className = 'toast '+type;
  t.innerHTML = '<span class="material-icons-round">'+(type==='success'?'check_circle':'error')+'</span> '+msg;
  c.appendChild(t);
  setTimeout(() => { t.style.transition='opacity .3s'; t.style.opacity='0'; setTimeout(()=>t.remove(),300); }, 3000);
}
function formatSizeJS(bytes) {
  if (bytes<1024) return bytes+' B';
  if (bytes<1048576) return (bytes/1024).toFixed(1)+' KB';
  if (bytes<1073741824) return (bytes/1048576).toFixed(1)+' MB';
  return (bytes/1073741824).toFixed(2)+' GB';
}

// ── Keyboard shortcuts ────────────────────────────────────────────────────
document.addEventListener('keydown', e => {
  if (e.key==='Escape') { closeCtx(); closePanel(); document.querySelectorAll('.modal-overlay.open').forEach(m=>m.classList.remove('open')); }
  if ((e.ctrlKey||e.metaKey) && e.key==='a') { e.preventDefault(); document.querySelectorAll('.file-check').forEach(c=>c.checked=true); onCheckChange(); }
  if ((e.ctrlKey||e.metaKey) && e.key==='f') { e.preventDefault(); document.getElementById('searchInput').focus(); }
});
</script>
</body>
</html>`
