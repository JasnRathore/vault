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
	Name     string `json:"name"`
	Size     string `json:"size"`
	SizeRaw  int64  `json:"sizeRaw"`
	ModTime  string `json:"modTime"`
	IsDir    bool   `json:"isDir"`
	Ext      string `json:"ext"`
	Icon     string `json:"icon"`
	Path     string `json:"path"`
	MimeType string `json:"mimeType"`
}

type Crumb struct {
	Name string
	Path string
}

type StatResponse struct {
	TotalFiles  int     `json:"totalFiles"`
	TotalDirs   int     `json:"totalDirs"`
	DirPath     string  `json:"dirPath"`
	TotalSize   string  `json:"totalSize"`
	DiskTotal   string  `json:"diskTotal"`
	DiskFree    string  `json:"diskFree"`
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
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".svg":
		return "image/svg+xml"
	case ".webp":
		return "image/webp"
	case ".mp4":
		return "video/mp4"
	case ".webm":
		return "video/webm"
	case ".mp3":
		return "audio/mpeg"
	case ".ogg":
		return "audio/ogg"
	case ".wav":
		return "audio/wav"
	case ".pdf":
		return "application/pdf"
	case ".txt", ".md", ".rst", ".log":
		return "text/plain"
	case ".html", ".htm":
		return "text/html"
	case ".css":
		return "text/css"
	case ".js", ".ts":
		return "text/javascript"
	case ".json":
		return "application/json"
	case ".go", ".py", ".rs", ".c", ".cpp", ".java", ".rb", ".sh",
		".bash", ".yaml", ".yml", ".toml", ".xml", ".tsx", ".jsx":
		return "text/plain"
	default:
		return "application/octet-stream"
	}
}

func getIcon(name string, isDir bool) string {
	if isDir {
		return "folder"
	}
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
	if isDir {
		return "DIR"
	}
	ext := strings.ToUpper(strings.TrimPrefix(filepath.Ext(name), "."))
	if ext == "" {
		return "FILE"
	}
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
		if err == nil && !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size
}

func listDirectory(urlPath string) ([]FileEntry, error) {
	cleanPath, err := safePath(urlPath)
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(cleanPath)
	if err != nil {
		return nil, err
	}

	var files []FileEntry
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		entryPath := filepath.Join(urlPath, entry.Name())
		if !strings.HasPrefix(entryPath, "/") {
			entryPath = "/" + entryPath
		}
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
		if files[i].IsDir != files[j].IsDir {
			return files[i].IsDir
		}
		return strings.ToLower(files[i].Name) < strings.ToLower(files[j].Name)
	})
	return files, nil
}

func buildBreadcrumbs(urlPath string) []Crumb {
	if urlPath == "/" || urlPath == "" {
		return nil
	}
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
	if urlPath == "/" || urlPath == "" {
		return ""
	}
	p := filepath.Dir(strings.TrimRight(urlPath, "/"))
	if p == "." {
		return "/"
	}
	return p
}

// ─── Handlers ────────────────────────────────────────────────────────────────

func browseHandler(tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		urlPath := strings.TrimPrefix(r.URL.Path, "/browse")
		if urlPath == "" {
			urlPath = "/"
		}

		entries, err := listDirectory(urlPath)
		if err != nil {
			http.Error(w, "Cannot read directory: "+err.Error(), http.StatusInternalServerError)
			return
		}

		totalFiles, totalDirs := 0, 0
		for _, e := range entries {
			if e.IsDir {
				totalDirs++
			} else {
				totalFiles++
			}
		}

		absPath, _ := filepath.Abs(directoryPath)
		entriesJSON, _ := json.Marshal(entries)

		data := struct {
			CurrentPath string
			ParentPath  string
			Breadcrumbs []Crumb
			Entries     []FileEntry
			EntriesJSON template.JS
			TotalFiles  int
			TotalDirs   int
			ServerTime  string
			DirPath     string
		}{
			CurrentPath: urlPath,
			ParentPath:  parentPath(urlPath),
			Breadcrumbs: buildBreadcrumbs(urlPath),
			Entries:     entries,
			EntriesJSON: template.JS(entriesJSON),
			TotalFiles:  totalFiles,
			TotalDirs:   totalDirs,
			ServerTime:  time.Now().Format("15:04:05"),
			DirPath:     absPath,
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

	r.ParseMultipartForm(512 << 20)
	files := r.MultipartForm.File["files"]
	for _, fh := range files {
		src, err := fh.Open()
		if err != nil {
			continue
		}
		defer src.Close()

		destPath := filepath.Join(destDir, filepath.Base(fh.Filename))
		dst, err := os.Create(destPath)
		if err != nil {
			continue
		}
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

	var req struct {
		Paths []string `json:"paths"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid request")
		return
	}

	count := 0
	for _, p := range req.Paths {
		cleanPath, err := safePath(p)
		if err != nil {
			continue
		}
		if err := os.RemoveAll(cleanPath); err == nil {
			count++
		}
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
	if err != nil {
		jsonError(w, "Access denied")
		return
	}

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
	if err != nil {
		jsonError(w, "Access denied")
		return
	}

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
	if err != nil {
		jsonError(w, "Access denied")
		return
	}

	count := 0
	for _, src := range req.Sources {
		srcClean, err := safePath(src)
		if err != nil {
			continue
		}
		destFull := filepath.Join(destClean, filepath.Base(srcClean))
		if err := os.Rename(srcClean, destFull); err == nil {
			count++
		}
	}
	json.NewEncoder(w).Encode(APIResponse{Success: true, Message: fmt.Sprintf("Moved %d item(s)", count)})
}

func zipDownloadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Paths []string `json:"paths"`
	}
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
		if err != nil {
			continue
		}
		addToZip(zw, cleanPath, filepath.Base(cleanPath))
	}
}

func addToZip(zw *zip.Writer, path, nameInZip string) {
	info, err := os.Stat(path)
	if err != nil {
		return
	}

	if info.IsDir() {
		entries, _ := os.ReadDir(path)
		for _, e := range entries {
			addToZip(zw, filepath.Join(path, e.Name()), filepath.Join(nameInZip, e.Name()))
		}
		return
	}

	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	w, err := zw.Create(nameInZip)
	if err != nil {
		return
	}
	io.Copy(w, f)
}

func statsHandler(w http.ResponseWriter, r *http.Request) {
	absPath, _ := filepath.Abs(directoryPath)
	entries, _ := os.ReadDir(directoryPath)
	files, dirs := 0, 0
	for _, e := range entries {
		if e.IsDir() {
			dirs++
		} else {
			files++
		}
	}

	total, free := diskUsage(directoryPath)
	used := total - free
	usedPct := 0.0
	if total > 0 {
		usedPct = float64(used) / float64(total) * 100
	}
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
	if urlPath == "" {
		urlPath = "/"
	}
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
		jsonError(w, "Invalid request")
		return
	}
	parentClean, err := safePath(req.Path)
	if err != nil {
		jsonError(w, "Access denied")
		return
	}

	newFile := filepath.Join(parentClean, filepath.Base(req.FileName))
	f, err := os.Create(newFile)
	if err != nil {
		jsonError(w, err.Error())
		return
	}
	f.Close()
	json.NewEncoder(w).Encode(APIResponse{Success: true, Message: "File created"})
}

func fileInfoHandler(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("path")
	cleanPath, err := safePath(filePath)
	if err != nil {
		jsonError(w, "Access denied")
		return
	}

	info, err := os.Stat(cleanPath)
	if err != nil {
		jsonError(w, err.Error())
		return
	}

	type FileInfo struct {
		Name     string `json:"name"`
		Path     string `json:"path"`
		Size     string `json:"size"`
		SizeRaw  int64  `json:"sizeRaw"`
		ModTime  string `json:"modTime"`
		IsDir    bool   `json:"isDir"`
		MimeType string `json:"mimeType"`
		Ext      string `json:"ext"`
		DirSize  string `json:"dirSize,omitempty"`
		DirCount int    `json:"dirCount,omitempty"`
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
			if !i.IsDir() {
				count++
			}
			return nil
		})
		fi.DirSize = formatSize(dirSize(cleanPath))
		fi.DirCount = count
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(fi)
}

func textReadHandler(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("path")
	cleanPath, err := safePath(filePath)
	if err != nil {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	info, err := os.Stat(cleanPath)
	if err != nil || info.IsDir() {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	limitBytes := int64(1 << 20)
	sizeToRead := info.Size()
	if sizeToRead > limitBytes {
		sizeToRead = limitBytes
	}

	f, err := os.Open(cleanPath)
	if err != nil {
		http.Error(w, "Cannot open file", http.StatusInternalServerError)
		return
	}
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

	http.Handle("/files/", http.StripPrefix("/files", http.FileServer(http.Dir(directoryPath))))

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/browse/", http.StatusFound)
			return
		}
		http.NotFound(w, r)
	})

	http.HandleFunc("/api/stats", statsHandler)
	http.HandleFunc("/api/list", listAPIHandler)
	http.HandleFunc("/api/upload", uploadHandler)
	http.HandleFunc("/api/delete", deleteHandler)
	http.HandleFunc("/api/rename", renameHandler)
	http.HandleFunc("/api/mkdir", mkdirHandler)
	http.HandleFunc("/api/move", moveHandler)
	http.HandleFunc("/api/zip", zipDownloadHandler)
	http.HandleFunc("/api/newfile", newFileHandler)
	http.HandleFunc("/api/fileinfo", fileInfoHandler)
	http.HandleFunc("/api/text", textReadHandler)

	http.HandleFunc("/browse", browseHandler(tmpl))
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
<title>VAULT / {{.CurrentPath}}</title>
<link rel="preconnect" href="https://fonts.googleapis.com">
<link href="https://fonts.googleapis.com/css2?family=Editorial+New:ital@0;1&family=Neue+Haas+Grotesk+Display+Pro:wght@400;500;700&display=swap" rel="stylesheet">
<link href="https://fonts.googleapis.com/css2?family=DM+Mono:ital,wght@0,300;0,400;0,500;1,300&family=Unbounded:wght@400;700;900&family=DM+Sans:ital,opsz,wght@0,9..40,300;0,9..40,400;0,9..40,500;0,9..40,700;1,9..40,300&display=swap" rel="stylesheet">
<link href="https://fonts.googleapis.com/icon?family=Material+Icons+Round" rel="stylesheet">
<style>
*,*::before,*::after{box-sizing:border-box;margin:0;padding:0}

:root {
  --ink:     #0d0f0f;
  --ink2:    #1c1f1f;
  --ink3:    #2a2e2e;
  --paper:   #f4f2ec;
  --paper2:  #edeae2;
  --paper3:  #e4e1d8;
  --rule:    rgba(13,15,15,0.10);
  --rule2:   rgba(13,15,15,0.18);
  --rule3:   rgba(13,15,15,0.05);
  --muted:   #6b6f6e;
  --faint:   #9ea2a1;
  --acc:     #c8ff00;
  --acc-ink: #1a2200;
  --red:     #c0392b;
  --cyan:    #005f73;
  --amber:   #b5620a;
  --grn:     #256b3a;
  --purp:    #4a3f8f;
  --mono:    'DM Mono', monospace;
  --sans:    'DM Sans', sans-serif;
  --disp:    'Unbounded', sans-serif;
  --serif:   'Georgia', serif;
  --panel-w: 320px;
}

html,body{height:100%;background:var(--paper);color:var(--ink);font-family:var(--sans);font-size:14px;overflow:hidden}

::-webkit-scrollbar{width:3px;height:3px}
::-webkit-scrollbar-track{background:transparent}
::-webkit-scrollbar-thumb{background:var(--rule2)}

/* ── SHELL ── */
.app{display:grid;grid-template-rows:56px 1fr;grid-template-columns:240px 1fr;height:100vh;background:var(--paper)}
.app.panel-open{grid-template-columns:240px 1fr var(--panel-w)}

/* ── TOPBAR ── */
.topbar{
  grid-column:1/-1;
  display:flex;align-items:stretch;
  background:var(--ink);
  z-index:100;
}
.tb-logo{
  width:240px;flex-shrink:0;
  display:flex;align-items:center;gap:0;
  border-right:1px solid rgba(255,255,255,0.08);
  padding:0;
}
.tb-logo-mark{
  width:56px;height:56px;
  background:var(--acc);
  display:flex;align-items:center;justify-content:center;
  flex-shrink:0;
}
.tb-logo-mark span{
  font-family:var(--disp);font-size:15px;font-weight:700;
  color:var(--acc-ink);letter-spacing:0;line-height:1;
}
.tb-logo-name{
  padding:0 16px;
  font-family:var(--disp);font-size:13px;font-weight:400;letter-spacing:2px;
  color:rgba(255,255,255,0.5);text-transform:uppercase;
}

.tb-path{
  flex:1;display:flex;align-items:center;
  padding:0 24px;font-family:var(--mono);font-size:11px;color:rgba(255,255,255,0.3);
  overflow:hidden;gap:0;
  border-right:1px solid rgba(255,255,255,0.08);
}
.tb-path a{color:rgba(255,255,255,0.45);text-decoration:none;transition:color .1s;letter-spacing:.02em}
.tb-path a:hover{color:var(--acc)}
.tb-path .sep{color:rgba(255,255,255,0.15);margin:0 6px}
.tb-path .cur{color:rgba(255,255,255,0.7)}

.tb-right{display:flex;align-items:stretch;margin-left:auto}
.tb-icon-btn{
  display:flex;align-items:center;justify-content:center;gap:7px;
  padding:0 18px;border:none;background:none;
  font-family:var(--sans);font-size:12px;font-weight:500;letter-spacing:.03em;
  color:rgba(255,255,255,0.4);cursor:pointer;transition:color .12s,background .12s;
  border-left:1px solid rgba(255,255,255,0.08);white-space:nowrap;
}
.tb-icon-btn .material-icons-round{font-size:15px}
.tb-icon-btn:hover{color:var(--acc);background:rgba(200,255,0,0.07)}
.tb-icon-btn.primary{background:var(--acc);color:var(--acc-ink);font-weight:600}
.tb-icon-btn.primary:hover{background:#d4ff1a;color:var(--acc-ink)}
.tb-clock{
  font-family:var(--mono);font-size:11px;color:rgba(255,255,255,0.25);
  padding:0 20px;border-left:1px solid rgba(255,255,255,0.08);
  display:flex;align-items:center;letter-spacing:.05em;
}

/* ── SIDEBAR ── */
.sidebar{
  background:var(--paper);
  border-right:1px solid var(--rule2);
  display:flex;flex-direction:column;overflow:hidden;
}

/* Masthead stats */
.sb-masthead{
  padding:0;border-bottom:1px solid var(--rule2);
  display:grid;grid-template-columns:1fr 1fr;
}
.sb-stat{
  padding:20px 20px 16px;
  position:relative;
}
.sb-stat+.sb-stat{border-left:1px solid var(--rule2)}
.sb-stat-num{
  font-family:var(--disp);font-size:54px;line-height:.9;
  color:var(--ink);letter-spacing:-1px;
  display:block;
}
.sb-stat.accent .sb-stat-num{color:var(--ink)}
.sb-stat-rule{
  width:24px;height:2px;background:var(--ink);
  margin:8px 0 6px;display:block;
}
.sb-stat.accent .sb-stat-rule{background:var(--acc);box-shadow:none}
.sb-stat-lbl{
  font-family:var(--sans);font-size:10px;font-weight:700;letter-spacing:.12em;
  text-transform:uppercase;color:var(--muted);
}

/* Disk block */
.sb-disk{padding:14px 20px;border-bottom:1px solid var(--rule2)}
.sb-disk-head{display:flex;justify-content:space-between;align-items:baseline;margin-bottom:10px}
.sb-disk-label{font-family:var(--sans);font-size:10px;font-weight:700;letter-spacing:.12em;text-transform:uppercase;color:var(--muted)}
.sb-disk-pct{font-family:var(--mono);font-size:11px;color:var(--ink3)}
.sb-disk-bar{height:3px;background:var(--rule2)}
.sb-disk-fill{height:100%;background:var(--ink);transition:width .6s ease}
.sb-disk-fill.warn{background:var(--red)}
.sb-disk-sub{display:flex;justify-content:space-between;margin-top:7px}
.sb-disk-sub span{font-family:var(--mono);font-size:10px;color:var(--faint)}

/* Nav */
.sb-nav{padding:16px 0 0;flex:1;overflow-y:auto}
.sb-nav-cap{
  font-family:var(--sans);font-size:9px;font-weight:700;letter-spacing:.15em;
  text-transform:uppercase;color:var(--faint);
  padding:0 20px;margin-bottom:4px;
}
.sb-link{
  display:flex;align-items:center;gap:10px;
  padding:9px 20px;
  font-family:var(--sans);font-size:13px;font-weight:400;letter-spacing:.01em;
  color:var(--muted);text-decoration:none;
  transition:background .1s,color .1s;
  border-left:2px solid transparent;
  position:relative;
}
.sb-link:hover{background:var(--paper2);color:var(--ink)}
.sb-link.active{background:var(--paper2);color:var(--ink);border-left-color:var(--ink);font-weight:500}
.sb-link .material-icons-round{font-size:15px;opacity:.5}
.sb-link.active .material-icons-round{opacity:.8}

.sb-div{height:1px;background:var(--rule);margin:12px 0}

/* Recent */
.sb-recent-cap{
  font-family:var(--sans);font-size:9px;font-weight:700;letter-spacing:.15em;
  text-transform:uppercase;color:var(--faint);
  padding:0 20px;margin-bottom:4px;
}
.sb-scroll{overflow-y:auto}
.recent-item{
  display:flex;align-items:center;gap:9px;
  padding:7px 20px;cursor:pointer;transition:background .1s;
}
.recent-item:hover{background:var(--paper2)}
.recent-item .ri-icon{font-size:13px !important;color:var(--faint)}
.recent-item .ri-name{
  flex:1;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;
  font-family:var(--mono);font-size:10px;color:var(--muted);
}
.recent-item .ri-time{font-family:var(--mono);font-size:9px;color:var(--faint)}

/* ── MAIN ── */
.main-wrap{display:flex;flex-direction:column;overflow:hidden;position:relative;background:var(--paper)}

/* ── TOOLBAR ── */
.toolbar{
  display:flex;align-items:stretch;
  height:44px;flex-shrink:0;
  background:var(--paper);
  border-bottom:1px solid var(--rule2);
}
.tb-search-wrap{
  position:relative;display:flex;align-items:center;
  border-right:1px solid var(--rule);
  flex-shrink:0;
}
.tb-search-icon{position:absolute;left:14px;font-size:14px !important;color:var(--faint);pointer-events:none}
.tb-search{
  background:none;border:none;outline:none;
  padding:0 14px 0 38px;height:44px;width:200px;
  font-family:var(--mono);font-size:11px;color:var(--ink);
  transition:width .2s;
}
.tb-search::placeholder{color:var(--faint)}
.tb-search:focus{width:260px}

.tool-sep{width:1px;background:var(--rule);flex-shrink:0;align-self:stretch;margin:8px 0}

.tool-btn{
  display:flex;align-items:center;gap:6px;
  padding:0 14px;height:44px;
  font-family:var(--sans);font-size:12px;font-weight:500;letter-spacing:.03em;
  background:none;border:none;border-right:1px solid var(--rule);
  color:var(--muted);cursor:pointer;transition:color .1s,background .1s;white-space:nowrap;
}
.tool-btn .material-icons-round{font-size:14px}
.tool-btn:hover{color:var(--ink);background:var(--paper2)}
.tool-btn.active{color:var(--ink);background:var(--paper2)}
.tb-spacer{flex:1}
.tb-view-group{display:flex;border-left:1px solid var(--rule)}

/* Bulk bar */
.bulk-bar{
  display:none;align-items:stretch;
  height:38px;flex-shrink:0;
  background:var(--ink);
  border-bottom:1px solid var(--ink2);
}
.bulk-bar.visible{display:flex}
.bulk-count{
  font-family:var(--disp);font-size:11px;font-weight:700;letter-spacing:.05em;
  color:var(--acc);padding:0 18px;
  border-right:1px solid rgba(255,255,255,0.1);
  display:flex;align-items:center;
}
.bulk-act{
  display:flex;align-items:center;gap:6px;
  padding:0 14px;
  font-family:var(--sans);font-size:11px;font-weight:500;letter-spacing:.04em;
  background:none;border:none;border-right:1px solid rgba(255,255,255,0.08);
  color:rgba(255,255,255,0.5);cursor:pointer;transition:color .1s,background .1s;
}
.bulk-act:hover{color:#fff;background:rgba(255,255,255,0.06)}
.bulk-act.danger:hover{color:#f87171}
.bulk-act .material-icons-round{font-size:13px}
.bulk-spacer{flex:1}

/* ── FILE AREA ── */
.file-area{flex:1;overflow-y:auto;padding:0}

/* Table */
.ftable{width:100%;border-collapse:collapse}
.fth{
  font-family:var(--sans);font-size:9px;font-weight:700;letter-spacing:.14em;text-transform:uppercase;
  color:var(--faint);text-align:left;padding:10px 16px;
  background:var(--paper2);
  border-bottom:1px solid var(--rule2);
  cursor:pointer;user-select:none;white-space:nowrap;
  transition:color .1s;position:sticky;top:0;z-index:2;
}
.fth:hover{color:var(--ink)}
.fth.sort-asc::after{content:' ↑';color:var(--ink)}
.fth.sort-desc::after{content:' ↓';color:var(--ink)}
.fth-check{width:44px;cursor:default !important}
.fth-name{width:55%}
.fth:last-child{text-align:right}

.frow{
  border-bottom:1px solid var(--rule3);
  border-left:3px solid transparent;
  transition:border-left-color .08s,background .08s;
  animation:rowIn .18s ease both;
}
.frow:hover{background:var(--paper2);border-left-color:var(--ink)}
.frow.selected{background:var(--paper2);border-left-color:var(--ink)}
.frow.hidden{display:none}

@keyframes rowIn{from{opacity:0;transform:translateY(3px)}to{opacity:1;transform:none}}

.frow td{padding:0 16px;height:46px;vertical-align:middle}
.frow td:first-child{padding-left:12px}

/* Name cell */
.cn{display:flex;align-items:center;gap:10px;min-width:0}

/* Type badge */
.cn-type{
  flex-shrink:0;
  font-family:var(--mono);font-size:8px;font-weight:500;letter-spacing:.04em;
  padding:2px 5px;border:1px solid;
  text-transform:uppercase;white-space:nowrap;
}
.t-dir  {color:var(--acc-ink);border-color:var(--acc);background:var(--acc)}
.t-code {color:var(--cyan);  border-color:rgba(0,95,115,.35); background:rgba(0,95,115,.07)}
.t-img  {color:var(--purp);  border-color:rgba(74,63,143,.35);background:rgba(74,63,143,.07)}
.t-media{color:var(--grn);   border-color:rgba(37,107,58,.35); background:rgba(37,107,58,.07)}
.t-arch {color:var(--red);   border-color:rgba(192,57,43,.35); background:rgba(192,57,43,.07)}
.t-data {color:var(--amber); border-color:rgba(181,98,10,.35); background:rgba(181,98,10,.07)}
.t-doc  {color:var(--muted); border-color:var(--rule2);        background:var(--paper2)}
.t-file {color:var(--faint); border-color:var(--rule);         background:none}

.cn a{
  font-family:var(--sans);font-size:13px;font-weight:400;color:var(--ink);
  text-decoration:none;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;
  transition:color .1s;
}
.cn a:hover{color:var(--ink);text-decoration:underline}

.cell-size{font-family:var(--mono);font-size:11px;color:var(--faint);white-space:nowrap;width:80px}
.cell-time{font-family:var(--mono);font-size:11px;color:var(--faint);white-space:nowrap;width:140px}
.cell-mime{font-family:var(--mono);font-size:10px;color:var(--faint);white-space:nowrap;max-width:110px;overflow:hidden;text-overflow:ellipsis}

.cell-acts{text-align:right;white-space:nowrap;width:100px}
.act{
  display:inline-flex;align-items:center;justify-content:center;
  width:28px;height:28px;background:none;border:1px solid transparent;
  color:var(--faint);cursor:pointer;text-decoration:none;
  transition:all .1s;opacity:0;
}
.frow:hover .act{opacity:1}
.act .material-icons-round{font-size:13px}
.act:hover{border-color:var(--rule2);color:var(--ink);background:var(--paper3)}
.act.del:hover{color:var(--red);border-color:rgba(192,57,43,.35)}

/* ── GRID VIEW ── */
.fgrid{display:none;padding:20px;gap:10px}
.fgrid.active{display:grid;grid-template-columns:repeat(auto-fill,minmax(130px,1fr))}
.gc{
  background:var(--paper);border:1px solid var(--rule2);
  padding:16px 12px 12px;
  display:flex;flex-direction:column;align-items:center;gap:6px;
  cursor:pointer;transition:border-color .1s,background .1s;
  text-decoration:none;position:relative;animation:rowIn .18s ease both;
}
.gc:hover{border-color:var(--ink);background:var(--paper2)}
.gc.selected{border-color:var(--ink);background:var(--paper2)}
.gc.hidden{display:none}
.gc-check{position:absolute;top:7px;left:7px}
.gc-icon{font-size:26px !important;color:var(--muted)}
.gc-name{font-family:var(--sans);font-size:11px;color:var(--ink);text-align:center;word-break:break-all;line-height:1.4}
.gc-meta{font-family:var(--mono);font-size:9px;color:var(--faint)}

/* Drop overlay */
.drop-overlay{
  position:absolute;inset:0;background:rgba(13,15,15,0.04);
  border:2px dashed var(--ink);
  display:none;align-items:center;justify-content:center;z-index:200;
  flex-direction:column;gap:12px;pointer-events:none;
}
.drop-overlay.active{display:flex}
.drop-overlay .material-icons-round{font-size:40px;color:var(--ink)}
.drop-overlay p{font-family:var(--disp);font-size:16px;font-weight:700;letter-spacing:.1em;text-transform:uppercase;color:var(--ink)}

/* ── DETAILS PANEL ── */
.details-panel{
  background:var(--paper);border-left:1px solid var(--rule2);
  display:flex;flex-direction:column;overflow:hidden;
}
.dp-hd{
  display:flex;align-items:center;justify-content:space-between;
  height:44px;padding:0 16px;
  background:var(--paper2);
  border-bottom:1px solid var(--rule2);flex-shrink:0;
}
.dp-hd-label{font-family:var(--sans);font-size:9px;font-weight:700;letter-spacing:.15em;text-transform:uppercase;color:var(--muted)}
.dp-close{background:none;border:none;color:var(--faint);cursor:pointer;padding:4px;transition:color .1s}
.dp-close:hover{color:var(--ink)}
.dp-close .material-icons-round{font-size:16px}

.dp-preview{
  display:flex;align-items:center;justify-content:center;
  height:160px;background:var(--paper2);border-bottom:1px solid var(--rule2);
  overflow:hidden;flex-shrink:0;
}
.dp-preview img{max-width:100%;max-height:100%;object-fit:contain}
.dp-preview video{max-width:100%;max-height:100%}
.dp-preview audio{width:260px}
.dp-preview .dp-big-icon{font-size:48px !important;color:var(--rule2)}

.dp-body{flex:1;overflow-y:auto;padding:16px}
.dp-filename{
  font-family:var(--sans);font-size:14px;font-weight:500;color:var(--ink);
  word-break:break-all;line-height:1.5;margin-bottom:16px;
  padding-bottom:14px;border-bottom:1px solid var(--rule);
}
.dp-row{
  display:flex;justify-content:space-between;align-items:flex-start;
  padding:8px 0;border-bottom:1px solid var(--rule3);
}
.dp-row:last-child{border-bottom:none}
.dp-k{font-family:var(--sans);font-size:10px;font-weight:700;letter-spacing:.08em;text-transform:uppercase;color:var(--faint)}
.dp-v{font-family:var(--mono);font-size:10px;color:var(--muted);text-align:right;max-width:55%;word-break:break-all;line-height:1.5}

.dp-code{
  font-family:var(--mono);font-size:10px;color:var(--muted);
  background:var(--paper2);padding:12px;
  overflow:auto;max-height:260px;
  border-top:1px solid var(--rule);
  border-left:3px solid var(--ink);
  white-space:pre;tab-size:2;margin-top:14px;
  line-height:1.6;
}
.dp-trunc{font-family:var(--mono);font-size:9px;color:var(--faint);margin-top:6px}

.dp-acts{
  padding:12px 16px;border-top:1px solid var(--rule2);
  display:flex;flex-wrap:wrap;gap:6px;flex-shrink:0;
  background:var(--paper2);
}
.dp-act{
  display:flex;align-items:center;gap:5px;
  padding:6px 12px;
  font-family:var(--sans);font-size:11px;font-weight:500;letter-spacing:.03em;
  background:var(--paper);border:1px solid var(--rule2);color:var(--muted);
  cursor:pointer;text-decoration:none;transition:all .1s;
}
.dp-act:hover{border-color:var(--ink);color:var(--ink)}
.dp-act.danger:hover{border-color:var(--red);color:var(--red)}
.dp-act .material-icons-round{font-size:12px}

/* ── CTX MENU ── */
.ctx{
  position:fixed;background:var(--ink);
  padding:4px;min-width:190px;
  box-shadow:0 12px 40px rgba(0,0,0,0.3);
  z-index:999;display:none;animation:ctxIn .08s ease;
}
.ctx.visible{display:block}
@keyframes ctxIn{from{opacity:0;transform:translateY(-4px)}to{opacity:1;transform:none}}
.ctx-item{
  display:flex;align-items:center;gap:10px;padding:9px 14px;
  font-family:var(--sans);font-size:12px;font-weight:400;
  color:rgba(255,255,255,0.6);cursor:pointer;transition:background .06s,color .06s;
}
.ctx-item:hover{background:rgba(255,255,255,0.07);color:#fff}
.ctx-item .material-icons-round{font-size:14px;opacity:.6}
.ctx-item.danger{color:rgba(239,68,68,0.8)}
.ctx-item.danger:hover{background:rgba(239,68,68,0.08);color:#ef4444}
.ctx-sep{height:1px;background:rgba(255,255,255,0.08);margin:3px 0}

/* ── MODALS ── */
.modal-ov{
  position:fixed;inset:0;background:rgba(13,15,15,0.6);
  display:none;align-items:center;justify-content:center;z-index:500;
}
.modal-ov.open{display:flex}
.modal{
  background:var(--paper);border:1px solid var(--rule2);
  width:440px;max-width:92vw;padding:32px;
  animation:modalIn .15s ease;
  box-shadow:0 20px 60px rgba(0,0,0,0.15);
}
@keyframes modalIn{from{opacity:0;transform:translateY(-10px)}to{opacity:1;transform:none}}
.modal-title{
  font-family:var(--disp);font-size:24px;letter-spacing:.05em;
  color:var(--ink);margin-bottom:24px;line-height:1;
}
.modal-label{
  font-family:var(--sans);font-size:10px;font-weight:700;letter-spacing:.12em;text-transform:uppercase;
  color:var(--muted);margin-bottom:7px;
}
.modal-inp{
  width:100%;background:var(--paper2);border:1px solid var(--rule2);
  padding:11px 14px;font-family:var(--mono);font-size:13px;color:var(--ink);
  outline:none;transition:border-color .12s;margin-bottom:24px;
}
.modal-inp:focus{border-color:var(--ink)}
.modal-btns{display:flex;gap:8px;justify-content:flex-end}
.mbtn{
  padding:9px 22px;
  font-family:var(--sans);font-size:13px;font-weight:600;letter-spacing:.04em;
  cursor:pointer;border:1px solid;transition:all .12s;
}
.mbtn.cancel{background:none;border-color:var(--rule2);color:var(--muted)}
.mbtn.cancel:hover{border-color:var(--ink);color:var(--ink)}
.mbtn.confirm{background:var(--ink);border-color:var(--ink);color:var(--paper)}
.mbtn.confirm:hover{background:var(--ink2)}
.mbtn.danger{background:none;border-color:var(--red);color:var(--red)}
.mbtn.danger:hover{background:rgba(192,57,43,.07)}

/* Upload */
.upload-zone{
  border:1px dashed var(--rule2);padding:28px 20px;text-align:center;
  cursor:pointer;transition:border-color .12s,background .12s;margin-bottom:16px;
}
.upload-zone:hover,.upload-zone.drag{border-color:var(--ink);background:var(--paper2)}
.upload-zone .material-icons-round{font-size:32px;color:var(--faint);display:block;margin-bottom:8px}
.upload-zone p{font-family:var(--sans);font-size:13px;font-weight:600;letter-spacing:.04em;color:var(--muted)}
.upload-zone small{font-family:var(--mono);font-size:10px;color:var(--faint)}
.upload-list{max-height:110px;overflow-y:auto;margin-bottom:12px}
.ufi{
  display:flex;align-items:center;gap:8px;padding:5px 0;
  font-family:var(--mono);font-size:10px;color:var(--muted);
  border-bottom:1px solid var(--rule3);
}
.ufi .material-icons-round{font-size:12px;color:var(--faint)}
.uprog{height:2px;background:var(--rule2);margin-bottom:12px}
.uprog-fill{height:100%;background:var(--ink);transition:width .3s}

/* ── TOAST ── */
.toast-wrap{position:fixed;bottom:20px;right:20px;display:flex;flex-direction:column;gap:6px;z-index:9999}
.toast{
  display:flex;align-items:center;gap:10px;
  padding:10px 16px;
  font-family:var(--sans);font-size:13px;font-weight:500;
  border:1px solid;
  box-shadow:0 4px 20px rgba(0,0,0,0.12);
  animation:toastIn .12s ease;
}
.toast.ok{background:var(--ink);border-color:var(--ink);color:var(--acc)}
.toast.err{background:var(--paper);border-color:rgba(192,57,43,.4);color:var(--red)}
.toast .material-icons-round{font-size:15px}
@keyframes toastIn{from{opacity:0;transform:translateX(12px)}to{opacity:1;transform:none}}

/* Checkbox */
.ck{
  width:14px;height:14px;appearance:none;
  border:1px solid var(--rule2);background:none;cursor:pointer;transition:all .1s;
  flex-shrink:0;
}
.ck:checked{background:var(--ink);border-color:var(--ink)}
.ck:checked::after{content:'✓';display:block;text-align:center;font-size:9px;color:var(--paper);line-height:1.5}

/* Empty */
.empty-state{text-align:center;padding:80px 0;color:var(--faint)}
.empty-state .material-icons-round{font-size:44px;display:block;margin-bottom:12px;color:var(--rule2)}
.empty-state p{font-family:var(--sans);font-size:14px;font-weight:500;letter-spacing:.08em;text-transform:uppercase;color:var(--faint)}

/* Stagger */
.frow:nth-child(1){animation-delay:0ms}
.frow:nth-child(2){animation-delay:15ms}
.frow:nth-child(3){animation-delay:30ms}
.frow:nth-child(4){animation-delay:45ms}
.frow:nth-child(5){animation-delay:60ms}
.frow:nth-child(6){animation-delay:75ms}
.frow:nth-child(7){animation-delay:90ms}
.frow:nth-child(8){animation-delay:105ms}
.frow:nth-child(9){animation-delay:120ms}
.frow:nth-child(10){animation-delay:135ms}

/* Dir count badge for grid */
.gc-dir{
  background:var(--acc);color:var(--acc-ink);
  font-family:var(--mono);font-size:9px;font-weight:700;
  padding:1px 5px;
}
</style>
</head>
<body>
<div class="app" id="app">

<!-- ── TOPBAR ── -->
<header class="topbar">
  <div class="tb-logo">
    <div class="tb-logo-mark"><span>VLT</span></div>
    <span class="tb-logo-name">Vault</span>
  </div>
  <div class="tb-path">
    <a href="/browse/">root</a>
    {{range .Breadcrumbs}}<span class="sep">/</span><a href="/browse{{.Path}}">{{.Name}}</a>{{end}}
  </div>
  <div class="tb-right">
    <button class="tb-icon-btn" onclick="openMkdir()">
      <span class="material-icons-round">create_new_folder</span> New Folder
    </button>
    <button class="tb-icon-btn" onclick="openNewFile()">
      <span class="material-icons-round">note_add</span> New File
    </button>
    <button class="tb-icon-btn primary" onclick="openUpload()">
      <span class="material-icons-round">upload</span> Upload
    </button>
    <div class="tb-clock" id="clock">{{.ServerTime}}</div>
  </div>
</header>

<!-- ── SIDEBAR ── -->
<aside class="sidebar">
  <div class="sb-masthead">
    <div class="sb-stat accent">
      <span class="sb-stat-num" id="sb-dirs">{{.TotalDirs}}</span>
      <span class="sb-stat-rule"></span>
      <span class="sb-stat-lbl">Folders</span>
    </div>
    <div class="sb-stat">
      <span class="sb-stat-num" id="sb-files">{{.TotalFiles}}</span>
      <span class="sb-stat-rule"></span>
      <span class="sb-stat-lbl">Files</span>
    </div>
  </div>

  <div class="sb-disk" id="diskBlock">
    <div class="sb-disk-head">
      <span class="sb-disk-label">Storage</span>
      <span class="sb-disk-pct" id="diskPct">–</span>
    </div>
    <div class="sb-disk-bar"><div class="sb-disk-fill" id="diskFill" style="width:0%"></div></div>
    <div class="sb-disk-sub">
      <span id="diskFree">–</span>
      <span id="diskTotal">–</span>
    </div>
  </div>

  <div class="sb-nav">
    <div class="sb-nav-cap">Navigation</div>
    <a href="/browse/" class="sb-link {{if eq .CurrentPath "/"}}active{{end}}">
      <span class="material-icons-round">home</span> Root
    </a>
    {{if .ParentPath}}
    <a href="/browse{{.ParentPath}}" class="sb-link">
      <span class="material-icons-round">arrow_upward</span> Parent folder
    </a>
    {{end}}

    <div class="sb-div"></div>
    <div class="sb-nav-cap">Recent</div>
    <div class="sb-scroll" id="recentList"></div>
  </div>
</aside>

<!-- ── MAIN ── -->
<div class="main-wrap">
  <div class="drop-overlay" id="dropOverlay">
    <span class="material-icons-round">upload_file</span>
    <p>Drop files to upload</p>
  </div>

  <!-- Toolbar -->
  <div class="toolbar">
    <div class="tb-search-wrap">
      <span class="material-icons-round tb-search-icon">search</span>
      <input class="tb-search" type="text" placeholder="Filter…" id="searchInput" oninput="filterFiles()">
    </div>
    <div class="tb-spacer"></div>
    <button class="tool-btn" id="btnSort" onclick="cycleSortField()" title="Sort">
      <span class="material-icons-round">sort</span> Sort
    </button>
    <div class="tb-view-group">
      <button class="tool-btn active" id="btnList" onclick="setView('list')" title="List view">
        <span class="material-icons-round">view_list</span>
      </button>
      <button class="tool-btn" id="btnGrid" onclick="setView('grid')" title="Grid view">
        <span class="material-icons-round">grid_view</span>
      </button>
    </div>
  </div>

  <!-- Bulk bar -->
  <div class="bulk-bar" id="bulkBar">
    <span class="bulk-count" id="bulkCount">0 SELECTED</span>
    <button class="bulk-act" onclick="bulkDownloadZip()">
      <span class="material-icons-round">archive</span> ZIP download
    </button>
    <button class="bulk-act danger" onclick="bulkDelete()">
      <span class="material-icons-round">delete</span> Delete
    </button>
    <div class="bulk-spacer"></div>
    <button class="bulk-act" onclick="clearSelection()">
      <span class="material-icons-round">close</span> Clear
    </button>
  </div>

  <!-- File area -->
  <div class="file-area" id="fileArea"
    ondragover="handleDragOver(event)"
    ondragleave="handleDragLeave(event)"
    ondrop="handleDrop(event)">

    {{if .Entries}}

    <!-- LIST -->
    <table class="ftable" id="listView">
      <thead>
        <tr>
          <th class="fth fth-check"><input type="checkbox" class="ck" id="selectAll" onchange="toggleSelectAll(this)"></th>
          <th class="fth fth-name" onclick="sortBy('name')">Name</th>
          <th class="fth" onclick="sortBy('size')">Size</th>
          <th class="fth" onclick="sortBy('time')">Modified</th>
          <th class="fth" style="width:100px;text-align:right">Actions</th>
        </tr>
      </thead>
      <tbody id="listBody">
        {{range .Entries}}
        <tr class="frow"
            data-name="{{.Name}}"
            data-path="{{.Path}}"
            data-isdir="{{.IsDir}}"
            data-size="{{.SizeRaw}}"
            data-time="{{.ModTime}}"
            data-ext="{{.Ext}}"
            data-mime="{{.MimeType}}"
            oncontextmenu="showCtx(event,'{{.Path}}',{{.IsDir}},'{{.Name}}')">
          <td><input type="checkbox" class="ck file-check" onchange="onCheckChange()"></td>
          <td>
            <div class="cn">
              <span class="cn-type {{if .IsDir}}t-dir{{else if eq .Icon "code"}}t-code{{else if eq .Icon "html"}}t-code{{else if eq .Icon "image"}}t-img{{else if eq .Icon "movie"}}t-media{{else if eq .Icon "audio_file"}}t-media{{else if eq .Icon "folder_zip"}}t-arch{{else if eq .Icon "table_chart"}}t-data{{else if eq .Icon "storage"}}t-data{{else if eq .Icon "article"}}t-doc{{else}}t-file{{end}}">{{.Ext}}</span>
              {{if .IsDir}}
                <a href="/browse{{.Path}}">{{.Name}}</a>
              {{else}}
                <a href="#" onclick="openPreview('{{.Path}}','{{.MimeType}}','{{.Name}}');return false">{{.Name}}</a>
              {{end}}
            </div>
          </td>
          <td class="cell-size">{{if not .IsDir}}{{.Size}}{{else}}—{{end}}</td>
          <td class="cell-time">{{.ModTime}}</td>
          <td class="cell-acts">
            {{if not .IsDir}}
            <a class="act" href="#" onclick="openPreview('{{.Path}}','{{.MimeType}}','{{.Name}}');return false" title="Preview">
              <span class="material-icons-round">visibility</span>
            </a>
            <a class="act" href="/files{{.Path}}" download title="Download">
              <span class="material-icons-round">download</span>
            </a>
            {{else}}
            <a class="act" href="/browse{{.Path}}" title="Open">
              <span class="material-icons-round">folder_open</span>
            </a>
            {{end}}
            <button class="act" onclick="openRename('{{.Path}}','{{.Name}}')" title="Rename">
              <span class="material-icons-round">drive_file_rename_outline</span>
            </button>
            <button class="act del" onclick="confirmDelete(['{{.Path}}'])" title="Delete">
              <span class="material-icons-round">delete</span>
            </button>
          </td>
        </tr>
        {{end}}
      </tbody>
    </table>

    <!-- GRID -->
    <div class="fgrid" id="gridView">
      {{range .Entries}}
      <a class="gc"
         data-name="{{.Name}}" data-path="{{.Path}}" data-isdir="{{.IsDir}}"
         href="{{if .IsDir}}/browse{{.Path}}{{else}}#{{end}}"
         onclick="{{if not .IsDir}}openPreview('{{.Path}}','{{.MimeType}}','{{.Name}}');return false{{end}}"
         oncontextmenu="showCtx(event,'{{.Path}}',{{.IsDir}},'{{.Name}}')">
        <input type="checkbox" class="ck gc-check file-check" onchange="onCheckChange()" onclick="event.stopPropagation()">
        <span class="material-icons-round gc-icon"
          style="{{if .IsDir}}color:var(--ink){{else if eq .Icon "code"}}color:var(--cyan){{else if eq .Icon "image"}}color:var(--purp){{else if eq .Icon "movie"}}color:var(--grn){{else if eq .Icon "audio_file"}}color:var(--grn){{else if eq .Icon "folder_zip"}}color:var(--red){{else}}color:var(--muted){{end}}">{{.Icon}}</span>
        <span class="gc-name">{{.Name}}</span>
        {{if .IsDir}}<span class="gc-dir">DIR</span>{{else}}<span class="gc-meta">{{.Size}}</span>{{end}}
      </a>
      {{end}}
    </div>

    {{else}}
    <div class="empty-state">
      <span class="material-icons-round">folder_open</span>
      <p>Empty directory</p>
    </div>
    {{end}}
  </div>

  <!-- Details Panel -->
  <aside class="details-panel" id="detailsPanel" style="display:none">
    <div class="dp-hd">
      <span class="dp-hd-label">File details</span>
      <button class="dp-close" onclick="closePanel()"><span class="material-icons-round">close</span></button>
    </div>
    <div class="dp-preview" id="dpPreview">
      <span class="material-icons-round dp-big-icon">insert_drive_file</span>
    </div>
    <div class="dp-body">
      <div class="dp-filename" id="dpName">–</div>
      <div id="dpMeta"></div>
      <div id="dpCode"></div>
    </div>
    <div class="dp-acts" id="dpActs"></div>
  </aside>
</div>
</div>

<!-- Context Menu -->
<div class="ctx" id="ctxMenu">
  <div class="ctx-item" id="ctxOpen"><span class="material-icons-round">folder_open</span> Open</div>
  <div class="ctx-item" id="ctxPreview"><span class="material-icons-round">visibility</span> Preview</div>
  <div class="ctx-sep"></div>
  <div class="ctx-item" id="ctxDl"><span class="material-icons-round">download</span> Download</div>
  <div class="ctx-item" onclick="ctxAct('zip')"><span class="material-icons-round">archive</span> Download as ZIP</div>
  <div class="ctx-sep"></div>
  <div class="ctx-item" onclick="ctxAct('rename')"><span class="material-icons-round">drive_file_rename_outline</span> Rename</div>
  <div class="ctx-item" onclick="ctxAct('copy')"><span class="material-icons-round">content_copy</span> Copy path</div>
  <div class="ctx-sep"></div>
  <div class="ctx-item danger" onclick="ctxAct('delete')"><span class="material-icons-round">delete</span> Delete</div>
</div>

<div class="toast-wrap" id="toastWrap"></div>

<!-- UPLOAD MODAL -->
<div class="modal-ov" id="uploadModal">
  <div class="modal">
    <div class="modal-title">Upload Files</div>
    <div class="upload-zone" id="uploadZone" onclick="document.getElementById('fileInput').click()">
      <span class="material-icons-round">cloud_upload</span>
      <p>Click to select, or drag files here</p>
      <small>Multiple files supported — no size limit</small>
    </div>
    <input type="file" id="fileInput" multiple style="display:none" onchange="handleFileSelect(this.files)">
    <div class="upload-list" id="uploadList"></div>
    <div class="uprog" id="uprog" style="display:none"><div class="uprog-fill" id="uprogFill" style="width:0%"></div></div>
    <div class="modal-btns">
      <button class="mbtn cancel" onclick="closeModal('uploadModal')">Cancel</button>
      <button class="mbtn confirm" onclick="doUpload()">Upload</button>
    </div>
  </div>
</div>

<!-- MKDIR MODAL -->
<div class="modal-ov" id="mkdirModal">
  <div class="modal">
    <div class="modal-title">New Folder</div>
    <div class="modal-label">Folder Name</div>
    <input class="modal-inp" type="text" id="mkdirInput" placeholder="untitled-folder" onkeydown="if(event.key==='Enter')doMkdir()">
    <div class="modal-btns">
      <button class="mbtn cancel" onclick="closeModal('mkdirModal')">Cancel</button>
      <button class="mbtn confirm" onclick="doMkdir()">Create</button>
    </div>
  </div>
</div>

<!-- NEW FILE MODAL -->
<div class="modal-ov" id="newFileModal">
  <div class="modal">
    <div class="modal-title">New File</div>
    <div class="modal-label">File Name</div>
    <input class="modal-inp" type="text" id="newFileInput" placeholder="untitled.txt" onkeydown="if(event.key==='Enter')doNewFile()">
    <div class="modal-btns">
      <button class="mbtn cancel" onclick="closeModal('newFileModal')">Cancel</button>
      <button class="mbtn confirm" onclick="doNewFile()">Create</button>
    </div>
  </div>
</div>

<!-- RENAME MODAL -->
<div class="modal-ov" id="renameModal">
  <div class="modal">
    <div class="modal-title">Rename</div>
    <div class="modal-label">New Name</div>
    <input class="modal-inp" type="text" id="renameInput" onkeydown="if(event.key==='Enter')doRename()">
    <div class="modal-btns">
      <button class="mbtn cancel" onclick="closeModal('renameModal')">Cancel</button>
      <button class="mbtn confirm" onclick="doRename()">Rename</button>
    </div>
  </div>
</div>

<!-- DELETE MODAL -->
<div class="modal-ov" id="deleteModal">
  <div class="modal">
    <div class="modal-title">Delete</div>
    <p id="deleteMsg" style="font-family:var(--sans);font-size:14px;color:var(--muted);margin-bottom:24px;line-height:1.7"></p>
    <div class="modal-btns">
      <button class="mbtn cancel" onclick="closeModal('deleteModal')">Cancel</button>
      <button class="mbtn danger" onclick="doDelete()">Delete permanently</button>
    </div>
  </div>
</div>

<script>
const CURRENT_PATH = {{printf "%q" .CurrentPath}};
let entries = {{.EntriesJSON}};
let ctxTarget=null, renameTarget=null, deleteTargets=[];
let uploadFiles=[], sortField='name', sortDir='asc';

// Clock
function tick(){const e=document.getElementById('clock');if(e)e.textContent=new Date().toLocaleTimeString('en-US',{hour12:false})}
tick();setInterval(tick,1000);

// View
let currentView=localStorage.getItem('vault-view')||'list';
function setView(v){
  currentView=v;localStorage.setItem('vault-view',v);
  const L=document.getElementById('listView'),G=document.getElementById('gridView');
  const bL=document.getElementById('btnList'),bG=document.getElementById('btnGrid');
  if(!L)return;
  if(v==='grid'){L.style.display='none';G.classList.add('active');bG.classList.add('active');bL.classList.remove('active')}
  else{L.style.display='';G.classList.remove('active');bL.classList.add('active');bG.classList.remove('active')}
}
setView(currentView);

// Filter
function filterFiles(){
  const q=document.getElementById('searchInput').value.toLowerCase();
  document.querySelectorAll('[data-name]').forEach(el=>el.classList.toggle('hidden',!el.dataset.name.toLowerCase().includes(q)));
}

// Sort
const sortFields=['name','size','time'];
function cycleSortField(){
  const idx=sortFields.indexOf(sortField);
  sortBy(sortFields[(idx+1)%sortFields.length]);
}
function sortBy(field){
  if(sortField===field)sortDir=sortDir==='asc'?'desc':'asc';
  else{sortField=field;sortDir='asc';}
  document.querySelectorAll('.fth').forEach(th=>th.classList.remove('sort-asc','sort-desc'));
  const thMap={name:1,size:2,time:3};
  const ths=document.querySelectorAll('.fth');
  if(thMap[field])ths[thMap[field]].classList.add(sortDir==='asc'?'sort-asc':'sort-desc');
  const tbody=document.getElementById('listBody');
  if(!tbody)return;
  const rows=Array.from(tbody.querySelectorAll('.frow'));
  rows.sort((a,b)=>{
    const aD=a.dataset.isdir==='true',bD=b.dataset.isdir==='true';
    if(aD!==bD)return aD?-1:1;
    if(field==='size')return sortDir==='asc'?parseInt(a.dataset.size)-parseInt(b.dataset.size):parseInt(b.dataset.size)-parseInt(a.dataset.size);
    const av=(field==='name'?a.dataset.name:a.dataset.time).toLowerCase();
    const bv=(field==='name'?b.dataset.name:b.dataset.time).toLowerCase();
    const c=av<bv?-1:av>bv?1:0;return sortDir==='asc'?c:-c;
  });
  rows.forEach(r=>tbody.appendChild(r));
}

// Selection
function getSelected(){
  const s=[];
  document.querySelectorAll('.frow .file-check:checked').forEach(c=>s.push(c.closest('.frow').dataset.path));
  document.querySelectorAll('.gc .file-check:checked').forEach(c=>s.push(c.closest('.gc').dataset.path));
  return[...new Set(s)];
}
function onCheckChange(){
  const sel=getSelected();
  document.getElementById('bulkCount').textContent=sel.length+' SELECTED';
  document.getElementById('bulkBar').classList.toggle('visible',sel.length>0);
  document.querySelectorAll('.frow').forEach(r=>r.classList.toggle('selected',!!r.querySelector('.file-check:checked')));
  document.querySelectorAll('.gc').forEach(c=>c.classList.toggle('selected',!!c.querySelector('.file-check:checked')));
}
function toggleSelectAll(cb){document.querySelectorAll('.file-check').forEach(c=>c.checked=cb.checked);onCheckChange()}
function clearSelection(){document.querySelectorAll('.file-check,#selectAll').forEach(c=>c.checked=false);onCheckChange()}

// Context menu
let ctxData=null;
function showCtx(e,path,isDir,name){
  e.preventDefault();ctxData={path,isDir,name};
  const m=document.getElementById('ctxMenu');
  const op=document.getElementById('ctxOpen'),pv=document.getElementById('ctxPreview'),dl=document.getElementById('ctxDl');
  op.style.display=isDir?'':'none';pv.style.display=isDir?'none':'';dl.style.display=isDir?'none':'';
  if(!isDir){
    pv.onclick=()=>{const en=entries.find(x=>x.path===path);openPreview(path,en?en.mimeType:'',name);closeCtx()};
    dl.onclick=()=>{window.location='/files'+path;closeCtx()};
  }else{
    op.onclick=()=>{window.location='/browse'+path;closeCtx()};
  }
  m.style.left=Math.min(e.clientX,window.innerWidth-200)+'px';
  m.style.top=Math.min(e.clientY,window.innerHeight-260)+'px';
  m.classList.add('visible');
}
function ctxAct(a){
  if(!ctxData)return;closeCtx();
  if(a==='rename')openRename(ctxData.path,ctxData.name);
  if(a==='delete')confirmDelete([ctxData.path]);
  if(a==='copy'){navigator.clipboard.writeText(ctxData.path);toast('Path copied','ok')}
  if(a==='zip')dlZip([ctxData.path]);
}
function closeCtx(){document.getElementById('ctxMenu').classList.remove('visible')}
document.addEventListener('click',closeCtx);

// Preview panel
async function openPreview(path,mime,name){
  addRecent(name,path,mime);
  const panel=document.getElementById('detailsPanel');
  document.getElementById('app').classList.add('panel-open');
  panel.style.display='flex';
  document.getElementById('dpName').textContent=name;
  document.getElementById('dpPreview').innerHTML='<span class="material-icons-round dp-big-icon" style="color:var(--rule2)">hourglass_empty</span>';
  document.getElementById('dpMeta').innerHTML='';document.getElementById('dpCode').innerHTML='';

  const info=await fetch('/api/fileinfo?path='+encodeURIComponent(path)).then(r=>r.json());
  const rows=info.isDir
    ?[['Size',info.dirSize],['Items',info.dirCount+' files'],['Modified',info.modTime],['Path',info.path]]
    :[['Size',info.size],['Type',info.mimeType],['Modified',info.modTime],['Path',info.path]];
  document.getElementById('dpMeta').innerHTML=rows.map(([k,v])=>'<div class="dp-row"><span class="dp-k">'+k+'</span><span class="dp-v">'+v+'</span></div>').join('');

  let acts='';
  if(!info.isDir){
    acts+='<a class="dp-act" href="/files'+path+'" download><span class="material-icons-round">download</span>Download</a>';
    acts+='<a class="dp-act" href="/files'+path+'" target="_blank"><span class="material-icons-round">open_in_new</span>Open</a>';
  }
  acts+='<button class="dp-act" onclick="openRename(\''+path+'\',\''+name+'\')"><span class="material-icons-round">edit</span>Rename</button>';
  acts+='<button class="dp-act danger" onclick="confirmDelete([\''+path+'\'])"><span class="material-icons-round">delete</span>Delete</button>';
  document.getElementById('dpActs').innerHTML=acts;

  const prev=document.getElementById('dpPreview');
  if(mime.startsWith('image/')){prev.innerHTML='<img src="/files'+path+'" alt="" loading="lazy">'}
  else if(mime.startsWith('video/')){prev.innerHTML='<video controls src="/files'+path+'"></video>'}
  else if(mime.startsWith('audio/')){prev.innerHTML='<div style="text-align:center;padding:20px"><span class="material-icons-round" style="font-size:36px;color:var(--grn);display:block;margin-bottom:12px">audio_file</span><audio controls src="/files'+path+'"></audio></div>'}
  else if(mime==='application/pdf'){prev.innerHTML='<span class="material-icons-round dp-big-icon" style="color:var(--red)">picture_as_pdf</span>'}
  else{const icon=entries.find(x=>x.path===path);prev.innerHTML='<span class="material-icons-round dp-big-icon" style="color:var(--muted)">'+((icon&&icon.icon)||'insert_drive_file')+'</span>'}

  if(mime.startsWith('text/')||mime==='application/json'){
    try{
      const tr=await fetch('/api/text?path='+encodeURIComponent(path));
      const truncated=tr.headers.get('x-truncated')==='true';
      const txt=await tr.text();
      const esc=txt.replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;');
      document.getElementById('dpCode').innerHTML='<div class="dp-code">'+esc+'</div>'+(truncated?'<p class="dp-trunc">Preview truncated at 1 MB</p>':'');
    }catch(e){}
  }
}
function closePanel(){document.getElementById('detailsPanel').style.display='none';document.getElementById('app').classList.remove('panel-open')}

// Upload
function openUpload(){
  uploadFiles=[];
  document.getElementById('uploadList').innerHTML='';
  document.getElementById('uprog').style.display='none';
  document.getElementById('uprogFill').style.width='0%';
  document.getElementById('fileInput').value='';
  openModal('uploadModal');
}
function handleFileSelect(files){
  uploadFiles=Array.from(files);
  document.getElementById('uploadList').innerHTML=uploadFiles.map(f=>'<div class="ufi"><span class="material-icons-round">insert_drive_file</span><span style="flex:1;overflow:hidden;text-overflow:ellipsis">'+f.name+'</span><span>'+fmtSize(f.size)+'</span></div>').join('');
}
async function doUpload(){
  if(!uploadFiles.length){toast('No files selected','err');return}
  const fd=new FormData();fd.append('path',CURRENT_PATH);
  uploadFiles.forEach(f=>fd.append('files',f));
  document.getElementById('uprog').style.display='block';
  const xhr=new XMLHttpRequest();
  xhr.upload.onprogress=e=>{if(e.lengthComputable)document.getElementById('uprogFill').style.width=(e.loaded/e.total*100)+'%'};
  xhr.onload=()=>{const r=JSON.parse(xhr.responseText);if(r.success){toast(r.message,'ok');closeModal('uploadModal');location.reload()}else toast(r.message,'err')};
  xhr.open('POST','/api/upload');xhr.send(fd);
}

// Drag & drop
function handleDragOver(e){e.preventDefault();document.getElementById('dropOverlay').classList.add('active')}
function handleDragLeave(e){if(!e.relatedTarget||!document.getElementById('fileArea').contains(e.relatedTarget))document.getElementById('dropOverlay').classList.remove('active')}
function handleDrop(e){e.preventDefault();document.getElementById('dropOverlay').classList.remove('active');const files=Array.from(e.dataTransfer.files);if(!files.length)return;uploadFiles=files;openUpload();handleFileSelect(files)}
const uz=document.getElementById('uploadZone');
uz.addEventListener('dragover',e=>{e.preventDefault();uz.classList.add('drag')});
uz.addEventListener('dragleave',()=>uz.classList.remove('drag'));
uz.addEventListener('drop',e=>{e.preventDefault();uz.classList.remove('drag');handleFileSelect(e.dataTransfer.files)});

// Mkdir
function openMkdir(){document.getElementById('mkdirInput').value='';openModal('mkdirModal');setTimeout(()=>document.getElementById('mkdirInput').focus(),80)}
async function doMkdir(){const n=document.getElementById('mkdirInput').value.trim();if(!n)return;const r=await api('/api/mkdir',{path:CURRENT_PATH,dirName:n});if(r.success){toast(r.message,'ok');closeModal('mkdirModal');location.reload()}else toast(r.message,'err')}

// New file
function openNewFile(){document.getElementById('newFileInput').value='';openModal('newFileModal');setTimeout(()=>document.getElementById('newFileInput').focus(),80)}
async function doNewFile(){const n=document.getElementById('newFileInput').value.trim();if(!n)return;const r=await api('/api/newfile',{path:CURRENT_PATH,fileName:n});if(r.success){toast(r.message,'ok');closeModal('newFileModal');location.reload()}else toast(r.message,'err')}

// Rename
function openRename(path,name){renameTarget=path;document.getElementById('renameInput').value=name;openModal('renameModal');setTimeout(()=>{const i=document.getElementById('renameInput');i.focus();i.select()},80)}
async function doRename(){const n=document.getElementById('renameInput').value.trim();if(!n||!renameTarget)return;const r=await api('/api/rename',{oldPath:renameTarget,newName:n});if(r.success){toast(r.message,'ok');closeModal('renameModal');location.reload()}else toast(r.message,'err')}

// Delete
function confirmDelete(paths){
  deleteTargets=paths;
  document.getElementById('deleteMsg').innerHTML='Permanently delete <strong style="color:var(--red)">'+(paths.length===1?paths[0]:paths.length+' items')+'</strong>? This cannot be undone.';
  openModal('deleteModal');
}
async function doDelete(){const r=await api('/api/delete',{paths:deleteTargets});if(r.success){toast(r.message,'ok');closeModal('deleteModal');location.reload()}else toast(r.message,'err')}
function bulkDelete(){const s=getSelected();if(s.length)confirmDelete(s)}

// ZIP
async function bulkDownloadZip(){dlZip(getSelected())}
async function dlZip(paths){
  if(!paths.length)return;toast('Building ZIP…','ok');
  const res=await fetch('/api/zip',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({paths})});
  const blob=await res.blob();const url=URL.createObjectURL(blob);
  const a=document.createElement('a');a.href=url;a.download='download.zip';a.click();URL.revokeObjectURL(url);
}

// Disk stats
async function loadDisk(){
  try{
    const d=await fetch('/api/stats').then(r=>r.json());
    document.getElementById('diskFree').textContent=d.diskFree+' free';
    document.getElementById('diskTotal').textContent=d.diskTotal+' total';
    document.getElementById('diskPct').textContent=d.diskUsedPct.toFixed(1)+'%';
    const f=document.getElementById('diskFill');
    f.style.width=d.diskUsedPct+'%';
    if(d.diskUsedPct>85)f.classList.add('warn');
  }catch(e){}
}
loadDisk();

// Recent
function addRecent(name,path,mime){
  let r=JSON.parse(localStorage.getItem('vault-recent')||'[]');
  r=r.filter(x=>x.path!==path);r.unshift({name,path,mime,t:Date.now()});r=r.slice(0,10);
  localStorage.setItem('vault-recent',JSON.stringify(r));renderRecent();
}
function renderRecent(){
  const r=JSON.parse(localStorage.getItem('vault-recent')||'[]');
  const el=document.getElementById('recentList');
  if(!r.length){el.innerHTML='<div style="padding:8px 20px;font-family:var(--mono);font-size:10px;color:var(--faint)">No recent files</div>';return}
  el.innerHTML=r.map(x=>'<div class="recent-item" onclick="openPreview(\''+x.path+'\',\''+x.mime+'\',\''+x.name+'\')">'
    +'<span class="material-icons-round ri-icon">'+mimeIcon(x.mime)+'</span>'
    +'<span class="ri-name">'+x.name+'</span>'
    +'<span class="ri-time">'+ago(x.t)+'</span></div>').join('');
}
function ago(ts){const s=Math.floor((Date.now()-ts)/1000);if(s<60)return 'now';if(s<3600)return Math.floor(s/60)+'m';if(s<86400)return Math.floor(s/3600)+'h';return Math.floor(s/86400)+'d'}
function mimeIcon(m){if(!m)return 'draft';if(m.startsWith('image/'))return 'image';if(m.startsWith('video/'))return 'movie';if(m.startsWith('audio/'))return 'audio_file';if(m.startsWith('text/'))return 'article';if(m==='application/pdf')return 'picture_as_pdf';return 'draft'}
renderRecent();

// Utils
async function api(url,body){const r=await fetch(url,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(body)});return r.json()}
function openModal(id){document.getElementById(id).classList.add('open')}
function closeModal(id){document.getElementById(id).classList.remove('open')}
document.querySelectorAll('.modal-ov').forEach(m=>m.addEventListener('click',e=>{if(e.target===m)m.classList.remove('open')}));
function toast(msg,type){
  const w=document.getElementById('toastWrap');
  const t=document.createElement('div');t.className='toast '+type;
  t.innerHTML='<span class="material-icons-round">'+(type==='ok'?'check_circle':'error')+'</span>'+msg;
  w.appendChild(t);setTimeout(()=>{t.style.transition='opacity .2s';t.style.opacity='0';setTimeout(()=>t.remove(),220)},3000);
}
function fmtSize(b){if(b<1024)return b+' B';if(b<1048576)return(b/1024).toFixed(1)+' KB';if(b<1073741824)return(b/1048576).toFixed(1)+' MB';return(b/1073741824).toFixed(2)+' GB'}

// Keyboard
document.addEventListener('keydown',e=>{
  if(e.key==='Escape'){closeCtx();closePanel();document.querySelectorAll('.modal-ov.open').forEach(m=>m.classList.remove('open'))}
  if((e.ctrlKey||e.metaKey)&&e.key==='a'){e.preventDefault();document.querySelectorAll('.file-check').forEach(c=>c.checked=true);onCheckChange()}
  if((e.ctrlKey||e.metaKey)&&e.key==='f'){e.preventDefault();document.getElementById('searchInput').focus()}
  if((e.ctrlKey||e.metaKey)&&e.key==='u'){e.preventDefault();openUpload()}
});
</script>
</body>
</html>`
