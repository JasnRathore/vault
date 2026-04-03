package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const directoryPath = "."

type FileEntry struct {
	Name    string
	Size    string
	ModTime string
	IsDir   bool
	Ext     string
	Icon    string
	Path    string
}

type PageData struct {
	CurrentPath string
	ParentPath  string
	Entries     []FileEntry
	TotalFiles  int
	TotalDirs   int
	ServerTime  string
	DirPath     string
}

type StatResponse struct {
	TotalFiles int    `json:"totalFiles"`
	TotalDirs  int    `json:"totalDirs"`
	DirPath    string `json:"dirPath"`
}

func formatSize(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%d B", size)
	} else if size < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(size)/1024)
	} else if size < 1024*1024*1024 {
		return fmt.Sprintf("%.1f MB", float64(size)/(1024*1024))
	}
	return fmt.Sprintf("%.1f GB", float64(size)/(1024*1024*1024))
}

func getIcon(name string, isDir bool) string {
	if isDir {
		return "folder"
	}
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".go":
		return "code"
	case ".js", ".ts", ".jsx", ".tsx":
		return "code"
	case ".py":
		return "code"
	case ".html", ".htm", ".css", ".scss":
		return "code"
	case ".json", ".yaml", ".yml", ".toml", ".xml":
		return "settings"
	case ".md", ".txt", ".rst":
		return "description"
	case ".png", ".jpg", ".jpeg", ".gif", ".svg", ".webp", ".ico":
		return "image"
	case ".mp4", ".mov", ".avi", ".mkv":
		return "movie"
	case ".mp3", ".wav", ".flac", ".ogg":
		return "audio_file"
	case ".pdf":
		return "picture_as_pdf"
	case ".zip", ".tar", ".gz", ".rar", ".7z":
		return "folder_zip"
	case ".sh", ".bash", ".zsh", ".fish":
		return "terminal"
	case ".exe", ".bin", ".out":
		return "apps"
	default:
		return "insert_drive_file"
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

func listDirectory(urlPath string) ([]FileEntry, error) {
	cleanPath := filepath.Clean(filepath.Join(directoryPath, urlPath))
	if !strings.HasPrefix(cleanPath, filepath.Clean(directoryPath)) {
		return nil, fmt.Errorf("access denied")
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
		files = append(files, FileEntry{
			Name:    entry.Name(),
			Size:    formatSize(info.Size()),
			ModTime: info.ModTime().Format("Jan 02, 2006  15:04"),
			IsDir:   entry.IsDir(),
			Ext:     getExt(entry.Name(), entry.IsDir()),
			Icon:    getIcon(entry.Name(), entry.IsDir()),
			Path:    entryPath,
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

const dashboardHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>FileNav / {{.CurrentPath}}</title>
<link href="https://fonts.googleapis.com/css2?family=Space+Mono:ital,wght@0,400;0,700;1,400&family=DM+Sans:wght@300;400;500;600&display=swap" rel="stylesheet">
<link href="https://fonts.googleapis.com/icon?family=Material+Icons" rel="stylesheet">
<style>
  *, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }

  :root {
    --bg:        #0d0f14;
    --surface:   #13161e;
    --surface2:  #1a1e2a;
    --border:    #252a38;
    --border2:   #2e3447;
    --accent:    #5b8af0;
    --accent2:   #3d6bdb;
    --green:     #3ecf8e;
    --amber:     #f5a623;
    --red:       #f25e5e;
    --purple:    #9b72f5;
    --text:      #e2e6f3;
    --text2:     #8b92ab;
    --text3:     #555f7a;
    --mono:      'Space Mono', monospace;
    --sans:      'DM Sans', sans-serif;
    --radius:    8px;
    --row-h:     44px;
  }

  html, body { height: 100%; background: var(--bg); color: var(--text); font-family: var(--sans); }

  /* ── Layout ── */
  .app { display: grid; grid-template-rows: 56px 1fr; grid-template-columns: 220px 1fr; height: 100vh; }

  /* ── Topbar ── */
  .topbar {
    grid-column: 1 / -1;
    display: flex; align-items: center; gap: 16px;
    padding: 0 24px;
    background: var(--surface);
    border-bottom: 1px solid var(--border);
    z-index: 10;
  }
  .logo {
    font-family: var(--mono); font-size: 15px; font-weight: 700;
    color: var(--accent); letter-spacing: -0.5px; white-space: nowrap;
  }
  .logo span { color: var(--text3); }
  .divider { width: 1px; height: 24px; background: var(--border2); margin: 0 4px; }
  .breadcrumb {
    display: flex; align-items: center; gap: 4px;
    font-family: var(--mono); font-size: 12px; color: var(--text2); flex: 1; min-width: 0;
    overflow: hidden;
  }
  .breadcrumb a { color: var(--accent); text-decoration: none; white-space: nowrap; }
  .breadcrumb a:hover { text-decoration: underline; }
  .breadcrumb .sep { color: var(--text3); }
  .topbar-right { display: flex; align-items: center; gap: 12px; margin-left: auto; }
  .clock {
    font-family: var(--mono); font-size: 11px; color: var(--text3);
    background: var(--surface2); border: 1px solid var(--border); border-radius: 4px;
    padding: 4px 10px; white-space: nowrap;
  }
  .badge {
    font-family: var(--mono); font-size: 10px; letter-spacing: 0.5px; text-transform: uppercase;
    padding: 3px 8px; border-radius: 4px; border: 1px solid;
  }
  .badge-green  { color: var(--green);  border-color: #1e4a36; background: #0d2b20; }

  /* ── Sidebar ── */
  .sidebar {
    background: var(--surface);
    border-right: 1px solid var(--border);
    padding: 20px 0;
    overflow-y: auto;
  }
  .sidebar-section { padding: 0 16px; margin-bottom: 24px; }
  .sidebar-label {
    font-family: var(--mono); font-size: 9px; letter-spacing: 1.5px; text-transform: uppercase;
    color: var(--text3); margin-bottom: 10px;
  }
  .stat-card {
    background: var(--surface2); border: 1px solid var(--border);
    border-radius: var(--radius); padding: 12px 14px; margin-bottom: 8px;
  }
  .stat-card .stat-val {
    font-family: var(--mono); font-size: 22px; font-weight: 700; color: var(--text); line-height: 1;
  }
  .stat-card .stat-lbl {
    font-size: 11px; color: var(--text2); margin-top: 4px;
  }
  .stat-card.accent-card { border-color: #1e3060; background: #0d1830; }
  .stat-card.accent-card .stat-val { color: var(--accent); }
  .stat-card.green-card  { border-color: #1e4a36; background: #0d2b20; }
  .stat-card.green-card  .stat-val { color: var(--green); }

  .nav-item {
    display: flex; align-items: center; gap: 10px;
    padding: 8px 12px; border-radius: var(--radius);
    font-size: 13px; color: var(--text2); cursor: pointer;
    text-decoration: none; transition: background 0.15s, color 0.15s;
  }
  .nav-item:hover { background: var(--surface2); color: var(--text); }
  .nav-item.active { background: #151c30; color: var(--accent); }
  .nav-item .material-icons { font-size: 17px; }

  /* ── Main ── */
  .main { display: flex; flex-direction: column; overflow: hidden; }

  .toolbar {
    display: flex; align-items: center; gap: 10px;
    padding: 12px 24px;
    border-bottom: 1px solid var(--border);
    background: var(--surface);
    flex-shrink: 0;
  }
  .toolbar-title { font-size: 13px; font-weight: 600; color: var(--text2); margin-right: auto; }
  .search-wrap {
    position: relative; display: flex; align-items: center;
  }
  .search-icon {
    position: absolute; left: 10px;
    font-size: 16px; color: var(--text3);
    pointer-events: none;
  }
  .search-input {
    background: var(--surface2); border: 1px solid var(--border2);
    border-radius: var(--radius); padding: 7px 12px 7px 34px;
    font-family: var(--sans); font-size: 13px; color: var(--text);
    width: 220px; outline: none; transition: border-color 0.15s;
  }
  .search-input::placeholder { color: var(--text3); }
  .search-input:focus { border-color: var(--accent); }

  .btn {
    display: flex; align-items: center; gap: 6px;
    padding: 7px 12px; border-radius: var(--radius);
    font-family: var(--sans); font-size: 13px; font-weight: 500;
    cursor: pointer; border: 1px solid var(--border2); background: var(--surface2);
    color: var(--text2); transition: all 0.15s; white-space: nowrap;
  }
  .btn .material-icons { font-size: 15px; }
  .btn:hover { background: var(--border); color: var(--text); }
  .btn.active { background: #151c30; border-color: var(--accent2); color: var(--accent); }

  /* ── File Table ── */
  .file-area { flex: 1; overflow-y: auto; padding: 20px 24px; }

  .file-table { width: 100%; border-collapse: collapse; }
  .file-table thead th {
    font-family: var(--mono); font-size: 10px; letter-spacing: 1px; text-transform: uppercase;
    color: var(--text3); font-weight: 400; text-align: left;
    padding: 0 12px 10px; border-bottom: 1px solid var(--border);
  }
  .file-table thead th:last-child { text-align: right; }

  .file-row {
    border-bottom: 1px solid var(--border);
    transition: background 0.1s;
    animation: fadeRow 0.3s ease both;
  }
  .file-row:last-child { border-bottom: none; }
  .file-row:hover { background: var(--surface2); }
  .file-row.hidden { display: none; }

  @keyframes fadeRow {
    from { opacity: 0; transform: translateY(6px); }
    to   { opacity: 1; transform: translateY(0); }
  }

  .file-row td { padding: 0 12px; height: var(--row-h); vertical-align: middle; }

  .cell-name {
    display: flex; align-items: center; gap: 10px;
    font-size: 13px; font-weight: 500; color: var(--text);
    white-space: nowrap; overflow: hidden; text-overflow: ellipsis; max-width: 380px;
  }
  .cell-name a { color: inherit; text-decoration: none; }
  .cell-name a:hover { color: var(--accent); }

  .file-icon { font-size: 18px; flex-shrink: 0; }
  .icon-folder { color: var(--amber); }
  .icon-code   { color: var(--accent); }
  .icon-image  { color: var(--purple); }
  .icon-doc    { color: var(--text2); }
  .icon-zip    { color: var(--red); }
  .icon-media  { color: var(--green); }

  .ext-badge {
    font-family: var(--mono); font-size: 9px; letter-spacing: 0.5px;
    padding: 2px 5px; border-radius: 3px; flex-shrink: 0;
    background: var(--surface2); border: 1px solid var(--border2); color: var(--text3);
  }
  .ext-badge.dir { background: #1a1400; border-color: #3a2e00; color: var(--amber); }

  .cell-size  { font-family: var(--mono); font-size: 11px; color: var(--text3); white-space: nowrap; }
  .cell-time  { font-family: var(--mono); font-size: 11px; color: var(--text3); white-space: nowrap; }
  .cell-action { text-align: right; }

  .action-btn {
    display: inline-flex; align-items: center;
    background: none; border: 1px solid transparent; border-radius: 4px;
    color: var(--text3); cursor: pointer; padding: 4px; transition: all 0.15s;
    text-decoration: none;
  }
  .action-btn .material-icons { font-size: 15px; }
  .action-btn:hover { border-color: var(--border2); color: var(--accent); background: var(--surface2); }

  /* Grid view */
  .file-grid { display: none; }
  .file-grid.active { display: grid; grid-template-columns: repeat(auto-fill, minmax(150px, 1fr)); gap: 12px; }

  .grid-card {
    background: var(--surface2); border: 1px solid var(--border);
    border-radius: var(--radius); padding: 16px 12px 12px;
    display: flex; flex-direction: column; align-items: center; gap: 8px;
    cursor: pointer; transition: border-color 0.15s, transform 0.15s;
    animation: fadeRow 0.3s ease both;
    text-decoration: none;
  }
  .grid-card:hover { border-color: var(--accent2); transform: translateY(-2px); }
  .grid-card.hidden { display: none; }
  .grid-icon { font-size: 32px; }
  .grid-name {
    font-size: 12px; font-weight: 500; color: var(--text);
    text-align: center; word-break: break-all; line-height: 1.4;
  }
  .grid-meta { font-family: var(--mono); font-size: 10px; color: var(--text3); }

  /* Empty state */
  .empty { text-align: center; padding: 80px 0; color: var(--text3); }
  .empty .material-icons { font-size: 48px; display: block; margin-bottom: 12px; }
  .empty p { font-size: 14px; }

  /* Scrollbar */
  ::-webkit-scrollbar { width: 6px; }
  ::-webkit-scrollbar-track { background: transparent; }
  ::-webkit-scrollbar-thumb { background: var(--border2); border-radius: 3px; }
</style>
</head>
<body>
<div class="app">

  <!-- Topbar -->
  <header class="topbar">
    <div class="logo">FILE<span>/</span>NAV</div>
    <div class="divider"></div>
    <nav class="breadcrumb">
      <a href="/">~</a>
      {{range $i, $crumb := .Breadcrumbs}}<span class="sep">/</span><a href="{{$crumb.Path}}">{{$crumb.Name}}</a>{{end}}
    </nav>
    <div class="topbar-right">
      <span class="badge badge-green">● LIVE</span>
      <span class="clock" id="clock">{{.ServerTime}}</span>
    </div>
  </header>

  <!-- Sidebar -->
  <aside class="sidebar">
    <div class="sidebar-section">
      <div class="sidebar-label">Overview</div>
      <div class="stat-card green-card">
        <div class="stat-val">{{.TotalDirs}}</div>
        <div class="stat-lbl">Directories</div>
      </div>
      <div class="stat-card accent-card">
        <div class="stat-val">{{.TotalFiles}}</div>
        <div class="stat-lbl">Files</div>
      </div>
    </div>
    <div class="sidebar-section">
      <div class="sidebar-label">Navigation</div>
      <a href="/" class="nav-item {{if eq .CurrentPath "/"}}active{{end}}">
        <span class="material-icons">home</span> Root
      </a>
      {{if .ParentPath}}
      <a href="{{.ParentPath}}" class="nav-item">
        <span class="material-icons">arrow_upward</span> Parent Dir
      </a>
      {{end}}
    </div>
    <div class="sidebar-section">
      <div class="sidebar-label">Serving</div>
      <div style="font-family: var(--mono); font-size: 11px; color: var(--text3); line-height: 1.8; word-break: break-all;">
        {{.DirPath}}
      </div>
    </div>
  </aside>

  <!-- Main -->
  <main class="main">
    <div class="toolbar">
      <span class="toolbar-title">{{if eq .CurrentPath "/"}}Root{{else}}{{.CurrentPath}}{{end}}</span>
      <div class="search-wrap">
        <span class="material-icons search-icon">search</span>
        <input class="search-input" type="text" placeholder="Filter files…" id="searchInput" oninput="filterFiles()">
      </div>
      <button class="btn" id="btnList" onclick="setView('list')" title="List view">
        <span class="material-icons">view_list</span>
      </button>
      <button class="btn" id="btnGrid" onclick="setView('grid')" title="Grid view">
        <span class="material-icons">grid_view</span>
      </button>
    </div>

    <div class="file-area">
      {{if .Entries}}

      <!-- List View -->
      <table class="file-table" id="listView">
        <thead>
          <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Size</th>
            <th>Modified</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          {{range .Entries}}
          <tr class="file-row" data-name="{{.Name}}">
            <td>
              <div class="cell-name">
                <span class="material-icons file-icon {{if .IsDir}}icon-folder{{else if eq .Icon "code"}}icon-code{{else if eq .Icon "image"}}icon-image{{else if eq .Icon "movie"}}icon-media{{else if eq .Icon "audio_file"}}icon-media{{else if eq .Icon "folder_zip"}}icon-zip{{else}}icon-doc{{end}}">{{.Icon}}</span>
                {{if .IsDir}}
                  <a href="/browse{{.Path}}">{{.Name}}</a>
                {{else}}
                  <a href="/files{{.Path}}" target="_blank">{{.Name}}</a>
                {{end}}
              </div>
            </td>
            <td><span class="ext-badge {{if .IsDir}}dir{{end}}">{{.Ext}}</span></td>
            <td class="cell-size">{{if not .IsDir}}{{.Size}}{{else}}—{{end}}</td>
            <td class="cell-time">{{.ModTime}}</td>
            <td class="cell-action">
              {{if not .IsDir}}
              <a class="action-btn" href="/files{{.Path}}" download title="Download">
                <span class="material-icons">download</span>
              </a>
              {{else}}
              <a class="action-btn" href="/browse{{.Path}}" title="Open">
                <span class="material-icons">chevron_right</span>
              </a>
              {{end}}
            </td>
          </tr>
          {{end}}
        </tbody>
      </table>

      <!-- Grid View -->
      <div class="file-grid" id="gridView">
        {{range .Entries}}
        <a class="grid-card" data-name="{{.Name}}"
           href="{{if .IsDir}}/browse{{.Path}}{{else}}/files{{.Path}}{{end}}"
           {{if not .IsDir}}target="_blank"{{end}}>
          <span class="material-icons grid-icon {{if .IsDir}}icon-folder{{else if eq .Icon "code"}}icon-code{{else if eq .Icon "image"}}icon-image{{else if eq .Icon "movie"}}icon-media{{else if eq .Icon "audio_file"}}icon-media{{else if eq .Icon "folder_zip"}}icon-zip{{else}}icon-doc{{end}}">{{.Icon}}</span>
          <span class="grid-name">{{.Name}}</span>
          <span class="grid-meta">{{if not .IsDir}}{{.Size}}{{else}}DIR{{end}}</span>
        </a>
        {{end}}
      </div>

      {{else}}
      <div class="empty">
        <span class="material-icons">folder_open</span>
        <p>This directory is empty.</p>
      </div>
      {{end}}
    </div>
  </main>

</div>

<script>
  // Clock
  function tick() {
    const el = document.getElementById('clock');
    if (el) el.textContent = new Date().toLocaleTimeString('en-US', {hour12: false});
  }
  tick(); setInterval(tick, 1000);

  // View toggle
  let currentView = localStorage.getItem('fileNavView') || 'list';
  function setView(v) {
    currentView = v;
    localStorage.setItem('fileNavView', v);
    const list = document.getElementById('listView');
    const grid = document.getElementById('gridView');
    const btnList = document.getElementById('btnList');
    const btnGrid = document.getElementById('btnGrid');
    if (!list || !grid) return;
    if (v === 'grid') {
      list.style.display = 'none'; grid.classList.add('active');
      btnGrid.classList.add('active'); btnList.classList.remove('active');
    } else {
      list.style.display = ''; grid.classList.remove('active');
      btnList.classList.add('active'); btnGrid.classList.remove('active');
    }
  }
  setView(currentView);

  // Filter
  function filterFiles() {
    const q = document.getElementById('searchInput').value.toLowerCase();
    document.querySelectorAll('[data-name]').forEach(el => {
      const match = el.dataset.name.toLowerCase().includes(q);
      el.classList.toggle('hidden', !match);
    });
  }

  // Stagger row animation
  document.querySelectorAll('.file-row, .grid-card').forEach((el, i) => {
    el.style.animationDelay = (i * 25) + 'ms';
  });
</script>
</body>
</html>`

type Crumb struct {
	Name string
	Path string
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

		data := struct {
			CurrentPath string
			ParentPath  string
			Breadcrumbs []Crumb
			Entries     []FileEntry
			TotalFiles  int
			TotalDirs   int
			ServerTime  string
			DirPath     string
		}{
			CurrentPath: urlPath,
			ParentPath:  parentPath(urlPath),
			Breadcrumbs: buildBreadcrumbs(urlPath),
			Entries:     entries,
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

func main() {
	_, err := os.Stat(directoryPath)
	if os.IsNotExist(err) {
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

	// Serve raw files
	http.Handle("/files/", http.StripPrefix("/files", http.FileServer(http.Dir(directoryPath))))

	// Redirect root to browser
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/browse/", http.StatusFound)
			return
		}
		http.NotFound(w, r)
	})

	// Stats API
	http.HandleFunc("/api/stats", func(w http.ResponseWriter, r *http.Request) {
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
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(StatResponse{TotalFiles: files, TotalDirs: dirs, DirPath: absPath})
	})

	// Directory browser
	http.HandleFunc("/browse", browseHandler(tmpl))
	http.HandleFunc("/browse/", browseHandler(tmpl))

	port := 8080
	fmt.Printf("FileNav dashboard running at http://localhost:%d\n", port)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil); err != nil {
		fmt.Printf("Error: %s\n", err)
	}
}
