// Чат-Чат: один файл. Внутри сервер, база данных и само приложение. Ничего устанавливать не нужно.
package main

import (
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
)

//go:embed *.html *.js *.css *.webmanifest *.png *.svg *.woff
var assets embed.FS

var lastHit int64

// Адрес общего сервера. Можно вшить при сборке (-X main.defaultServer=...), задать SERVER_URL
// в settings.env или написать в файле адрес.txt рядом с программой.
var defaultServer = ""

func serverURL() string {
	ok := func(u string) bool { return strings.HasPrefix(u, "https://") || strings.HasPrefix(u, "http://") }
	if u := strings.TrimSpace(os.Getenv("SERVER_URL")); ok(u) {
		return u
	}
	if exe, err := os.Executable(); err == nil {
		for _, n := range []string{"адрес.txt", "address.txt"} {
			if b, err := os.ReadFile(filepath.Join(filepath.Dir(exe), n)); err == nil {
				if u := strings.TrimSpace(strings.TrimPrefix(string(b), "\uFEFF")); ok(u) {
					return u
				}
			}
		}
	}
	return defaultServer
}

var mimes = map[string]string{".html": "text/html; charset=utf-8", ".js": "text/javascript; charset=utf-8", ".css": "text/css; charset=utf-8",
	".json": "application/json", ".webmanifest": "application/manifest+json", ".svg": "image/svg+xml", ".png": "image/png", ".jpg": "image/jpeg",
	".webp": "image/webp", ".gif": "image/gif", ".ico": "image/x-icon", ".otf": "font/otf", ".ttf": "font/ttf", ".woff": "font/woff", ".woff2": "font/woff2", ".txt": "text/plain; charset=utf-8", ".webm": "audio/webm", ".ogg": "audio/ogg", ".m4a": "audio/mp4", ".mp3": "audio/mpeg", ".wav": "audio/wav"}

func loadEnvFile(p string) {
	b, err := os.ReadFile(p)
	if err != nil {
		return
	}
	for _, line := range strings.Split(strings.TrimPrefix(string(b), "\uFEFF"), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if i := strings.Index(line, "="); i > 0 {
			k, v := strings.TrimSpace(line[:i]), strings.TrimSpace(line[i+1:])
			if os.Getenv(k) == "" {
				_ = os.Setenv(k, v)
			}
		}
	}
}

func router(w http.ResponseWriter, r *http.Request) {
	atomic.StoreInt64(&lastHit, time.Now().Unix())
	p := r.URL.Path
	switch {
	case p == "/ws":
		handleWS(w, r)
		return
	case strings.HasPrefix(p, "/api/") || p == "/healthz":
		handleAPI(w, r)
		return
	}
	if r.Method != "GET" && r.Method != "HEAD" {
		w.WriteHeader(405)
		return
	}
	h := w.Header()
	secHeaders(h)
	if strings.HasPrefix(p, "/files/") {
		name := p[7:]
		if !reFile.MatchString(name) {
			http.NotFound(w, r)
			return
		}
		f, err := os.Open(filepath.Join(uploadDir, name))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer f.Close()
		st, _ := f.Stat()
		ct := mimes[strings.ToLower(path.Ext(name))]
		h.Set("Content-Type", ct)
		h.Set("Cache-Control", "public, max-age=31536000, immutable")
		http.ServeContent(w, r, name, st.ModTime(), f)
		return
	}
	rel := path.Clean("/" + p)
	if rel == "/" {
		rel = "/index.html"
	}
	b, err := fs.ReadFile(assets, path.Base(rel))
	if err != nil {
		rel = "/index.html"
		b, err = fs.ReadFile(assets, "index.html")
		if err != nil {
			http.NotFound(w, r)
			return
		}
	}
	h.Set("Content-Type", mimes[strings.ToLower(path.Ext(rel))])
	if rel == "/sw.js" || rel == "/index.html" {
		h.Set("Cache-Control", "no-cache")
	} else {
		h.Set("Cache-Control", "public, max-age=3600")
	}
	if r.Method != "HEAD" {
		_, _ = w.Write(b)
	}
}

func isOurs(port int) bool {
	c := http.Client{Timeout: 1500 * time.Millisecond}
	resp, err := c.Get(fmt.Sprintf("http://127.0.0.1:%d/healthz", port))
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	b := make([]byte, 200)
	n, _ := resp.Body.Read(b)
	return strings.Contains(string(b[:n]), `"chatchat"`)
}

func openWindow(url string) {
	if runtime.GOOS != "windows" {
		log.Println("откройте:", url)
		return
	}
	profile := filepath.Join(os.Getenv("LOCALAPPDATA"), "ChatChat", "profile")
	var cands []string
	for _, base := range []string{os.Getenv("ProgramFiles(x86)"), os.Getenv("ProgramFiles"), os.Getenv("LOCALAPPDATA")} {
		if base != "" {
			cands = append(cands, filepath.Join(base, "Microsoft", "Edge", "Application", "msedge.exe"), filepath.Join(base, "Google", "Chrome", "Application", "chrome.exe"))
		}
	}
	for _, c := range cands {
		if _, err := os.Stat(c); err == nil {
			if exec.Command(c, "--app="+url, "--user-data-dir="+profile, "--no-first-run", "--window-size=1100,760").Start() == nil {
				return
			}
		}
	}
	_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
}

func main() {
	exe, _ := os.Executable()
	loadEnvFile(filepath.Join(filepath.Dir(exe), "settings.env"))
	if u := serverURL(); u != "" && runtime.GOOS == "windows" && os.Getenv("NO_WINDOW") == "" {
		openWindow(u) // клиентский режим: сервер где-то в интернете, здесь только окно
		return
	}
	dataDir = os.Getenv("DATA_DIR")
	if dataDir == "" {
		base := os.Getenv("LOCALAPPDATA")
		if base == "" {
			base, _ = os.UserConfigDir()
		}
		if base == "" {
			base = "."
		}
		dataDir = filepath.Join(base, "ChatChat", "data")
	}
	uploadDir = filepath.Join(dataDir, "uploads")
	if err := os.MkdirAll(uploadDir, 0o700); err != nil {
		msgBox("Чат-Чат", "Не удалось создать папку для данных:\n"+err.Error())
		os.Exit(1)
	}
	if lf, err := os.OpenFile(filepath.Join(dataDir, "log.txt"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600); err == nil {
		if st, _ := lf.Stat(); st != nil && st.Size() > 1<<20 {
			lf.Close()
			lf, _ = os.OpenFile(filepath.Join(dataDir, "log.txt"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
		}
		if runtime.GOOS == "windows" {
			log.SetOutput(lf)
		}
	}
	smtpURL, mailFrom = os.Getenv("SMTP_URL"), os.Getenv("MAIL_FROM")
	brevoKey = os.Getenv("BREVO_API_KEY")
	twSID, twTok, twFrom = os.Getenv("TWILIO_ACCOUNT_SID"), os.Getenv("TWILIO_AUTH_TOKEN"), os.Getenv("TWILIO_FROM")
	// Тестовый режим (код на экране) включается только явно или у одиночной копии на Windows без почты и SMS.
	standalone := runtime.GOOS == "windows" && !canEmail() && !canSms()
	switch os.Getenv("DEV_CODES") {
	case "1":
		devCodes = true
	case "0":
		devCodes = false
	default:
		devCodes = standalone
	}
	local := runtime.GOOS == "windows" && os.Getenv("NO_WINDOW") == ""
	if b, err := os.ReadFile(filepath.Join(dataDir, "secret.key")); err == nil {
		secret = strings.TrimSpace(string(b))
	}
	if secret == "" {
		secret = randHex(32)
		_ = os.WriteFile(filepath.Join(dataDir, "secret.key"), []byte(secret), 0o600)
	}
	openDB()
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() { <-sig; flush(); os.Exit(0) }()
	go gcLoop()

	host := "127.0.0.1" // в тестовом режиме код виден на экране, поэтому наружу сервер не открываем
	if !devCodes {
		host = "0.0.0.0"
	}
	base := 3000
	if v := os.Getenv("PORT"); v != "" {
		if n, ok := toInt(v); ok {
			base = n
		}
	}
	if local && isOurs(base) { // уже запущен: просто показываем окно
		openWindow(fmt.Sprintf("http://localhost:%d", base))
		return
	}
	var ln net.Listener
	var port int
	var err error
	for p := base; p < base+10; p++ {
		if ln, err = net.Listen("tcp", fmt.Sprintf("%s:%d", host, p)); err == nil {
			port = p
			break
		}
	}
	if ln == nil {
		msgBox("Чат-Чат", "Не удалось запустить: все порты заняты.\n"+err.Error())
		os.Exit(1)
	}
	log.Printf("Чат-Чат запущен: http://localhost:%d (данные: %s)", port, dataDir)
	atomic.StoreInt64(&lastHit, time.Now().Unix())
	srv := &http.Server{Handler: http.HandlerFunc(router), ReadHeaderTimeout: 15 * time.Second}
	go func() {
		if err := srv.Serve(ln); err != nil {
			log.Println(err)
			os.Exit(1)
		}
	}()
	if local {
		openWindow(fmt.Sprintf("http://localhost:%d", port))
		// Окно закрыто и никто не подключён — выключаемся.
		for {
			time.Sleep(10 * time.Second)
			if atomic.LoadInt64(&wsCount) == 0 && time.Now().Unix()-atomic.LoadInt64(&lastHit) > 120 {
				flush()
				return
			}
		}
	}
	select {}
}
