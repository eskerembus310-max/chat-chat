package main

import (
	"bytes"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	codeTTL       = 10 * 60 * 1000
	maxImageBytes = 600 * 1024
)

var (
	devCodes             bool
	smtpURL, mailFrom    string
	brevoKey             string
	twSID, twTok, twFrom string
)

func canEmail() bool { return smtpURL != "" || brevoKey != "" }
func canSms() bool   { return twSID != "" && twTok != "" && twFrom != "" }

/* ───── ошибки ───── */
type HErr struct {
	Status    int
	Code, Msg string
}

func fail(status int, code, msg string) { panic(&HErr{status, code, msg}) }
func bad(code, msg string)              { fail(400, code, msg) }

/* ───── ограничитель частоты ───── */
var rate = map[string][]int64{}

func limit(key string, max int, windowMs int64) {
	t := now()
	arr := rate[key][:0:0]
	for _, x := range rate[key] {
		if t-x < windowMs {
			arr = append(arr, x)
		}
	}
	if len(arr) >= max {
		rate[key] = arr
		fail(429, "too_many", "Слишком много попыток. Подождите немного.")
	}
	rate[key] = append(arr, t)
}
func gcLoop() {
	for range time.Tick(10 * time.Minute) {
		mu.Lock()
		t := now()
		for k, v := range rate {
			f := v[:0:0]
			for _, x := range v {
				if t-x < 3600e3 {
					f = append(f, x)
				}
			}
			if len(f) == 0 {
				delete(rate, k)
			} else {
				rate[k] = f
			}
		}
		for k, c := range db.Codes {
			if c.Expires < t {
				delete(db.Codes, k)
			}
		}
		mu.Unlock()
	}
}

/* ───── разбор ввода ───── */
var (
	reEmail    = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]{2,}$`)
	rePlus     = regexp.MustCompile(`^\+\d{8,15}$`)
	reNonDigit = regexp.MustCompile(`\D`)
	reBY9      = regexp.MustCompile(`^(25|29|33|44)`)
	reUser     = regexp.MustCompile(`^[a-z][a-z0-9_]{4,31}$`)
	reEmojiNm  = regexp.MustCompile(`^[a-z0-9_]{2,24}$`)
	reDataURL  = regexp.MustCompile(`^data:image/(png|jpeg|webp|gif);base64,([A-Za-z0-9+/=]+)$`)
	reFile     = regexp.MustCompile(`^[a-f0-9]{32}\.(png|jpg|webp|gif|webm|ogg|m4a|mp3|wav)$`)
	reAudio    = regexp.MustCompile(`^data:audio/(webm|ogg|mp4|mpeg|wav|x-m4a|aac)(?:;codecs=[A-Za-z0-9.,-]+)?;base64,([A-Za-z0-9+/=]+)$`)
	reWord     = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	reHex      = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
	reCode     = regexp.MustCompile(`^\d{6}$`)
)

func normPhone(raw string) string {
	t := strings.TrimSpace(raw)
	d := reNonDigit.ReplaceAllString(t, "")
	switch {
	case strings.HasPrefix(t, "+"):
		return "+" + d
	case strings.HasPrefix(d, "00"):
		return "+" + d[2:]
	case len(d) == 11 && strings.HasPrefix(d, "80"):
		return "+375" + d[2:]
	case len(d) == 11 && strings.HasPrefix(d, "8"):
		return "+7" + d[1:]
	case len(d) == 12 && strings.HasPrefix(d, "375"):
		return "+" + d
	case len(d) == 11 && strings.HasPrefix(d, "7"):
		return "+" + d
	case len(d) == 9 && reBY9.MatchString(d):
		return "+375" + d
	case len(d) == 10 && strings.HasPrefix(d, "9"):
		return "+7" + d
	}
	return "+" + d
}
func normTarget(kind, raw string) string {
	s := strings.TrimSpace(raw)
	switch kind {
	case "email":
		e := strings.ToLower(s)
		if len(e) > 120 || !reEmail.MatchString(e) {
			bad("bad_email", "Проверьте адрес почты.")
		}
		return e
	case "phone":
		p := normPhone(s)
		if !rePlus.MatchString(p) {
			bad("bad_phone", "Не получилось разобрать номер. Напишите его с кодом страны, например +375291234567.")
		}
		return p
	}
	bad("bad_kind", "bad_kind")
	return ""
}
func clean(v any, max int) string {
	s := ""
	switch x := v.(type) {
	case nil:
	case string:
		s = x
	default:
		s = fmt.Sprint(x)
	}
	var b strings.Builder
	for _, r := range s {
		if r < 9 || r == 11 || r == 12 || (r >= 14 && r <= 31) || r == 127 || (r >= 0x200b && r <= 0x200f) || (r >= 0x202a && r <= 0x202e) || (r >= 0x2066 && r <= 0x2069) {
			continue
		}
		b.WriteRune(r)
	}
	out := strings.TrimSpace(b.String())
	if r := []rune(out); len(r) > max {
		out = string(r[:max])
	}
	return out
}
func toInt(v any) (int, bool) {
	switch x := v.(type) {
	case float64:
		if x == math.Trunc(x) && math.Abs(x) < 1e12 {
			return int(x), true
		}
	case string:
		if n, err := strconv.Atoi(strings.TrimSpace(x)); err == nil {
			return n, true
		}
	}
	return 0, false
}
func intParam(v any) int {
	n, ok := toInt(v)
	if !ok || n < 1 {
		bad("bad_id", "bad_id")
	}
	return n
}

/* ───── картинки ───── */
func saveImage(data any) string {
	s, _ := data.(string)
	m := reDataURL.FindStringSubmatch(s)
	if m == nil {
		bad("bad_image", "Нужна картинка PNG, JPEG, WebP или GIF.")
	}
	buf, err := base64.StdEncoding.DecodeString(m[2])
	if err != nil || len(buf) == 0 || len(buf) > maxImageBytes {
		bad("image_too_big", "Картинка слишком большая. Максимум 600 КБ.")
	}
	ok := false
	switch m[1] {
	case "png":
		ok = bytes.HasPrefix(buf, []byte{0x89, 0x50, 0x4e, 0x47})
	case "jpeg":
		ok = bytes.HasPrefix(buf, []byte{0xff, 0xd8, 0xff})
	case "gif":
		ok = bytes.HasPrefix(buf, []byte("GIF"))
	case "webp":
		ok = len(buf) >= 12 && string(buf[:4]) == "RIFF" && string(buf[8:12]) == "WEBP"
	}
	if !ok {
		bad("bad_image", "Файл не похож на картинку.")
	}
	ext := m[1]
	if ext == "jpeg" {
		ext = "jpg"
	}
	name := randHex(16) + "." + ext
	if err := os.WriteFile(filepath.Join(uploadDir, name), buf, 0o600); err != nil {
		fail(500, "server_error", "Ошибка сервера.")
	}
	return name
}
func saveAudio(data any) string {
	s, _ := data.(string)
	m := reAudio.FindStringSubmatch(s)
	if m == nil {
		bad("bad_audio", "Не удалось разобрать голосовое сообщение.")
	}
	buf, err := base64.StdEncoding.DecodeString(m[2])
	if err != nil || len(buf) < 100 || len(buf) > 900*1024 {
		bad("audio_too_big", "Голосовое слишком длинное. Запишите покороче.")
	}
	ext, ok := "", false
	switch m[1] {
	case "webm":
		ext, ok = "webm", bytes.HasPrefix(buf, []byte{0x1a, 0x45, 0xdf, 0xa3})
	case "ogg":
		ext, ok = "ogg", bytes.HasPrefix(buf, []byte("OggS"))
	case "mp4", "x-m4a", "aac":
		ext, ok = "m4a", len(buf) > 12 && string(buf[4:8]) == "ftyp"
	case "mpeg":
		ext, ok = "mp3", bytes.HasPrefix(buf, []byte("ID3")) || (buf[0] == 0xff && buf[1]&0xe0 == 0xe0)
	case "wav":
		ext, ok = "wav", string(buf[:4]) == "RIFF" && string(buf[8:12]) == "WAVE"
	}
	if !ok {
		bad("bad_audio", "Файл не похож на аудио.")
	}
	name := randHex(16) + "." + ext
	if err := os.WriteFile(filepath.Join(uploadDir, name), buf, 0o600); err != nil {
		fail(500, "server_error", "Ошибка сервера.")
	}
	return name
}
func copyUpload(name string) string {
	if !reFile.MatchString(name) {
		return ""
	}
	b, err := os.ReadFile(filepath.Join(uploadDir, name))
	if err != nil {
		return ""
	}
	nn := randHex(16) + name[strings.LastIndex(name, "."):]
	if os.WriteFile(filepath.Join(uploadDir, nn), b, 0o600) != nil {
		return ""
	}
	return nn
}
func rmFile(name string) {
	if reFile.MatchString(name) {
		_ = os.Remove(filepath.Join(uploadDir, name))
	}
}
func fileURL(name string) *string {
	if name == "" {
		return nil
	}
	s := "/files/" + name
	return &s
}
func nstr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

/* ───── представления ───── */
type PubUser struct {
	ID       int     `json:"id"`
	Name     string  `json:"name"`
	Username *string `json:"username"`
	Bio      string  `json:"bio"`
	Avatar   *string `json:"avatar"`
	Online   bool    `json:"online"`
	LastSeen int64   `json:"lastSeen"`
}
type SelfUser struct {
	PubUser
	Email    *string        `json:"email"`
	Phone    *string        `json:"phone"`
	Settings map[string]any `json:"settings"`
}
type ReplyView struct {
	ID     int    `json:"id"`
	UserID int    `json:"userId"`
	Kind   string `json:"kind"`
	Text   string `json:"text"`
}
type MsgView struct {
	ID        int              `json:"id"`
	ChatID    int              `json:"chatId"`
	UserID    int              `json:"userId"`
	Kind      string           `json:"kind"`
	Text      string           `json:"text"`
	EmojiID   *int             `json:"emojiId"`
	CreatedAt int64            `json:"createdAt"`
	ReplyTo   *ReplyView       `json:"replyTo"`
	EditedAt  int64            `json:"editedAt"`
	Image     *string          `json:"image"`
	Reactions map[string][]int `json:"reactions"`
	Audio     *string          `json:"audio"`
	AudioMs   int              `json:"audioMs"`
	FwdFrom   *int             `json:"fwdFrom"`
}
type ChatView struct {
	ID        int            `json:"id"`
	Type      string         `json:"type"`
	Title     *string        `json:"title"`
	CreatedBy int            `json:"createdBy"`
	Members   []int          `json:"members"`
	Last      *MsgView       `json:"last"`
	Unread    int            `json:"unread"`
	CreatedAt int64          `json:"createdAt"`
	Reads     map[string]int `json:"reads"`
	Avatar    *string        `json:"avatar"`
	Pinned    bool           `json:"pinned"`
	Muted     bool           `json:"muted"`
}

func publicUser(u *User) PubUser {
	return PubUser{u.ID, u.Name, nstr(u.Username), u.Bio, fileURL(u.Avatar), isOnline(u.ID), u.LastSeen}
}
func selfUser(u *User) SelfUser {
	st := u.Settings
	if st == nil {
		st = map[string]any{}
	}
	return SelfUser{publicUser(u), nstr(u.Email), nstr(u.Phone), st}
}
func msgView(m *Message) *MsgView {
	v := &MsgView{ID: m.ID, ChatID: m.ChatID, UserID: m.UserID, Kind: m.Kind, Text: m.Text, CreatedAt: m.CreatedAt, EditedAt: m.EditedAt, Image: fileURL(m.Image), Reactions: m.Reactions, Audio: fileURL(m.Audio), AudioMs: m.AudioMs}
	if m.FwdFrom != 0 {
		f := m.FwdFrom
		v.FwdFrom = &f
	}
	if v.Reactions == nil {
		v.Reactions = map[string][]int{}
	}
	if m.EmojiID != 0 {
		e := m.EmojiID
		v.EmojiID = &e
	}
	if m.ReplyTo != 0 {
		for _, x := range msgsBy[m.ChatID] {
			if x.ID == m.ReplyTo {
				t := []rune(x.Text)
				if len(t) > 80 {
					t = append(t[:80], '…')
				}
				v.ReplyTo = &ReplyView{x.ID, x.UserID, x.Kind, string(t)}
				break
			}
		}
	}
	return v
}
func chatView(chatID, forUser int) *ChatView {
	c := chatByID(chatID)
	if c == nil {
		return nil
	}
	v := &ChatView{ID: c.ID, Type: c.Type, CreatedBy: c.CreatedBy, Members: memberIDs(chatID), CreatedAt: c.CreatedAt, Reads: map[string]int{}}
	for _, m := range db.Members {
		if m.ChatID == chatID {
			v.Reads[strconv.Itoa(m.UserID)] = m.LastRead
		}
	}
	if c.Type == "group" {
		t := c.Title
		v.Title = &t
	}
	list := msgsBy[chatID]
	if len(list) > 0 {
		v.Last = msgView(list[len(list)-1])
	}
	lr := 0
	v.Avatar = fileURL(c.Avatar)
	if m := memberRow(chatID, forUser); m != nil {
		lr = m.LastRead
		v.Pinned, v.Muted = m.Pinned, m.Muted
	}
	for i := len(list) - 1; i >= 0 && list[i].ID > lr; i-- {
		if list[i].UserID != forUser {
			v.Unread++
		}
	}
	return v
}
func usersFor(ids []int) map[string]PubUser {
	out := map[string]PubUser{}
	for _, id := range ids {
		if u := userByID(id); u != nil {
			out[strconv.Itoa(id)] = publicUser(u)
		}
	}
	return out
}
func cleanSettings(raw any) map[string]any {
	o := map[string]any{}
	s, _ := raw.(map[string]any)
	str := func(k string, max int, re interface{ MatchString(string) bool }) {
		if v, ok := s[k].(string); ok && len(v) <= max && re.MatchString(v) {
			o[k] = v
		}
	}
	num := func(k string, min, max int) {
		v, ok := s[k]
		if !ok || v == nil || v == "" {
			return
		}
		n, ok := toInt(v)
		if f, isF := v.(float64); isF {
			n, ok = int(math.Round(f)), true
		}
		if ok {
			if n < min {
				n = min
			}
			if n > max {
				n = max
			}
			o[k] = n
		}
	}
	str("theme", 24, reWord)
	str("wallpaper", 24, reWord)
	str("font", 24, reWord)
	str("lang", 12, reWord)
	str("accent", 7, reHex)
	num("fontSize", 12, 22)
	num("radius", 0, 28)
	num("dim", 0, 80)
	return o
}

/* ───── запрос ───── */
type Ctx struct {
	ip     string
	query  url.Values
	params map[string]string
	body   map[string]any
	token  string
	user   *User
}

func (c *Ctx) has(k string) bool { _, ok := c.body[k]; return ok }
func (c *Ctx) str(k string) string {
	switch x := c.body[k].(type) {
	case nil:
		return ""
	case string:
		return x
	default:
		return fmt.Sprint(x)
	}
}

type Handler func(*Ctx) any
type Route struct {
	method string
	parts  []string
	fn     Handler
	auth   bool
	nolock bool
}

var routes []*Route

func route(method, pattern string, fn Handler, opts ...string) {
	r := &Route{method: method, parts: strings.Split(strings.Trim(pattern, "/"), "/"), fn: fn, auth: true}
	for _, o := range opts {
		switch o {
		case "noauth":
			r.auth = false
		case "nolock":
			r.nolock = true
		}
	}
	routes = append(routes, r)
}
func matchRoute(method, path string) (*Route, map[string]string) {
	segs := strings.Split(strings.Trim(path, "/"), "/")
	for _, r := range routes {
		if r.method != method || len(r.parts) != len(segs) {
			continue
		}
		params := map[string]string{}
		ok := true
		for i, p := range r.parts {
			if strings.HasPrefix(p, ":") {
				params[p[1:]] = segs[i]
			} else if p != segs[i] {
				ok = false
				break
			}
		}
		if ok {
			return r, params
		}
	}
	return nil, nil
}
func writeJSON(w http.ResponseWriter, status int, v any) {
	b, err := json.Marshal(v)
	if err != nil {
		status, b = 500, []byte(`{"error":"server_error","message":"Ошибка сервера."}`)
	}
	h := w.Header()
	h.Set("Content-Type", "application/json; charset=utf-8")
	h.Set("Cache-Control", "no-store")
	secHeaders(h)
	w.WriteHeader(status)
	_, _ = w.Write(b)
}
func secHeaders(h http.Header) {
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Referrer-Policy", "no-referrer")
	h.Set("X-Frame-Options", "DENY")
	h.Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data: blob:; connect-src 'self' ws: wss:; style-src 'self' 'unsafe-inline'; script-src 'self'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'")
}
func clientIP(r *http.Request) string {
	if x := r.Header.Get("X-Forwarded-For"); x != "" {
		return strings.TrimSpace(strings.Split(x, ",")[0])
	}
	h, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return h
}

func handleAPI(w http.ResponseWriter, r *http.Request) {
	var result any
	status := 200
	func() {
		locked := false
		defer func() {
			if locked {
				mu.Unlock()
			}
			if e := recover(); e != nil {
				he, ok := e.(*HErr)
				if !ok {
					log.Printf("panic: %v", e)
					he = &HErr{500, "server_error", "Ошибка сервера."}
				}
				if he.Status >= 400 && he.Status != 401 {
					log.Printf("%s %s -> %d %s", r.Method, r.URL.Path, he.Status, he.Code)
				}
				status, result = he.Status, map[string]string{"error": he.Code, "message": he.Msg}
			}
		}()
		rt, params := matchRoute(r.Method, r.URL.Path)
		if rt == nil {
			fail(404, "not_found", "Не найдено.")
		}
		ctx := &Ctx{ip: clientIP(r), query: r.URL.Query(), params: params, body: map[string]any{}}
		if r.Method != "GET" && r.Method != "DELETE" {
			r.Body = http.MaxBytesReader(w, r.Body, 1200*1024)
			raw, err := io.ReadAll(r.Body)
			if err != nil {
				fail(413, "too_big", "Слишком большой запрос.")
			}
			if len(bytes.TrimSpace(raw)) > 0 {
				var any0 any
				if json.Unmarshal(raw, &any0) != nil {
					bad("bad_json", "bad_json")
				}
				m, ok := any0.(map[string]any)
				if !ok {
					bad("bad_json", "bad_json")
				}
				ctx.body = m
			}
		}
		if !rt.nolock {
			mu.Lock()
			locked = true
		}
		if rt.auth {
			ctx.token = strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			if !rt.nolock && !locked {
				mu.Lock()
				locked = true
			}
			uid, ok := db.Sessions[sha(ctx.token)]
			if ctx.token == "" || !ok {
				fail(401, "unauthorized", "Нужно войти.")
			}
			ctx.user = userByID(uid)
			if ctx.user == nil {
				fail(401, "unauthorized", "Нужно войти.")
			}
		}
		result = rt.fn(ctx)
		if locked {
			save()
		}
	}()
	writeJSON(w, status, result)
}

/* ───── маршруты ───── */
func ok() any { return map[string]any{"ok": true} }

func init() {
	route("GET", "/healthz", func(c *Ctx) any { return map[string]any{"ok": true, "app": "chatchat"} }, "noauth")
	route("GET", "/api/config", func(c *Ctx) any {
		return map[string]any{"email": canEmail() || devCodes, "phone": canSms() || devCodes, "dev": devCodes}
	}, "noauth")

	route("POST", "/api/auth/request", authRequest, "noauth", "nolock")
	route("POST", "/api/auth/verify", authVerify, "noauth")
	route("POST", "/api/auth/logout", func(c *Ctx) any { delete(db.Sessions, sha(c.token)); return ok() })

	route("GET", "/api/me", func(c *Ctx) any { return map[string]any{"user": selfUser(c.user)} })
	route("PATCH", "/api/me", func(c *Ctx) any {
		u := c.user
		name, bio, username := u.Name, u.Bio, u.Username
		if c.has("name") {
			name = clean(c.body["name"], 50)
		}
		if name == "" {
			bad("name_required", "Введите имя.")
		}
		if c.has("bio") {
			bio = clean(c.body["bio"], 200)
		}
		if c.has("username") {
			v := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(c.str("username")), "@"))
			if v == "" {
				username = ""
			} else {
				if !reUser.MatchString(v) {
					bad("bad_username", "Имя пользователя: от 5 до 32 символов, латиница, цифры и _, начинается с буквы.")
				}
				if o := userByName(v); o != nil && o.ID != u.ID {
					fail(409, "username_taken", "Это имя пользователя уже занято.")
				}
				username = v
			}
		}
		u.Name, u.Bio, u.Username = name, bio, username
		pushUser(u)
		return map[string]any{"user": selfUser(u)}
	})
	route("PUT", "/api/me/avatar", func(c *Ctx) any {
		file := saveImage(c.body["data"])
		rmFile(c.user.Avatar)
		c.user.Avatar = file
		pushUser(c.user)
		return map[string]any{"user": selfUser(c.user)}
	})
	route("DELETE", "/api/me/avatar", func(c *Ctx) any {
		rmFile(c.user.Avatar)
		c.user.Avatar = ""
		pushUser(c.user)
		return map[string]any{"user": selfUser(c.user)}
	})
	route("PUT", "/api/me/settings", func(c *Ctx) any {
		st := cleanSettings(c.body["settings"])
		c.user.Settings = st
		return map[string]any{"settings": st}
	})

	/* контакты */
	route("GET", "/api/contacts", func(c *Ctx) any {
		out := []any{}
		for _, ct := range sortedContacts(c.user.ID) {
			out = append(out, contactView(ct))
		}
		return map[string]any{"contacts": out}
	})
	route("POST", "/api/contacts", func(c *Ctx) any {
		me := c.user.ID
		name := clean(c.body["name"], 50)
		if name == "" {
			bad("name_required", "Введите имя контакта.")
		}
		limit("c:"+strconv.Itoa(me), 120, 3600e3)
		target, kind, userID := "", "", 0
		if v, has := c.body["userId"]; has && v != nil {
			userID = intParam(v)
			u := userByID(userID)
			if u == nil || u.Name == "" {
				fail(404, "not_found", "Пользователь не найден.")
			}
			if u.ID == me {
				bad("self_contact", "Это ваш собственный аккаунт.")
			}
		} else {
			raw := strings.TrimSpace(c.str("target"))
			if raw == "" {
				bad("target_required", "Введите номер телефона или почту.")
			}
			kind = "phone"
			if strings.Contains(raw, "@") {
				kind = "email"
			}
			target = normTarget(kind, raw)
			if target == c.user.Phone || target == c.user.Email {
				bad("self_contact", "Это ваш собственный номер.")
			}
			var u *User
			if kind == "email" {
				u = userByEmail(target)
			} else {
				u = userByPhone(target)
			}
			if u != nil && u.Name != "" {
				userID = u.ID
			}
		}
		var ex *Contact
		for _, ct := range db.Contacts {
			if ct.OwnerID != me {
				continue
			}
			if (userID != 0 && ct.UserID == userID) || (target != "" && ct.Target == target) {
				ex = ct
				break
			}
		}
		if ex == nil && len(sortedContacts(me)) >= 500 {
			bad("too_many", "Можно сохранить не больше 500 контактов.")
		}
		if ex != nil {
			ex.Name = name
			if ex.UserID == 0 {
				ex.UserID = userID
			}
		} else {
			ex = &Contact{ID: nextID("contact"), OwnerID: me, Name: name, Target: target, Kind: kind, UserID: userID, CreatedAt: now()}
			db.Contacts = append(db.Contacts, ex)
		}
		return map[string]any{"contact": contactView(ex)}
	})
	route("PATCH", "/api/contacts/:id", func(c *Ctx) any {
		id := intParam(c.params["id"])
		for _, ct := range db.Contacts {
			if ct.ID == id && ct.OwnerID == c.user.ID {
				name := clean(c.body["name"], 50)
				if name == "" {
					bad("name_required", "Введите имя контакта.")
				}
				ct.Name = name
				return map[string]any{"contact": contactView(ct)}
			}
		}
		fail(404, "not_found", "Контакт не найден.")
		return nil
	})
	route("DELETE", "/api/contacts/:id", func(c *Ctx) any {
		id := intParam(c.params["id"])
		for i, ct := range db.Contacts {
			if ct.ID == id && ct.OwnerID == c.user.ID {
				db.Contacts = append(db.Contacts[:i], db.Contacts[i+1:]...)
				return ok()
			}
		}
		fail(404, "not_found", "Контакт не найден.")
		return nil
	})

	/* поиск людей */
	route("GET", "/api/users/search", func(c *Ctx) any {
		s := clean(c.query.Get("q"), 60)
		if len([]rune(s)) < 2 {
			return map[string]any{"users": []any{}}
		}
		ls := strings.ToLower(s)
		pure := strings.ToLower(strings.TrimPrefix(s, "@"))
		digits := strings.NewReplacer(" ", "", "-", "", "(", "", ")", "").Replace(s)
		out := []PubUser{}
		for _, u := range db.Users {
			if u.ID == c.user.ID || u.Name == "" {
				continue
			}
			var m bool
			if strings.HasPrefix(s, "@") {
				m = strings.HasPrefix(u.Username, pure)
			} else {
				m = strings.Contains(strings.ToLower(u.Name), ls) || (u.Username != "" && strings.HasPrefix(u.Username, pure)) ||
					(u.Email != "" && u.Email == ls) || (u.Phone != "" && u.Phone == digits)
			}
			if m {
				out = append(out, publicUser(u))
				if len(out) >= 20 {
					break
				}
			}
		}
		return map[string]any{"users": out}
	})
	route("GET", "/api/users/:id", func(c *Ctx) any {
		u := userByID(intParam(c.params["id"]))
		if u == nil || u.Name == "" {
			fail(404, "not_found", "Пользователь не найден.")
		}
		return map[string]any{"user": publicUser(u)}
	})

	/* чаты */
	route("GET", "/api/chats", func(c *Ctx) any {
		chats := []*ChatView{}
		all := []int{}
		for _, m := range db.Members {
			if m.UserID == c.user.ID {
				if v := chatView(m.ChatID, c.user.ID); v != nil {
					chats = append(chats, v)
					all = append(all, v.Members...)
				}
			}
		}
		ts := func(v *ChatView) int64 {
			if v.Last != nil {
				return v.Last.CreatedAt
			}
			return v.CreatedAt
		}
		sort.SliceStable(chats, func(i, j int) bool {
			if chats[i].Pinned != chats[j].Pinned {
				return chats[i].Pinned
			}
			return ts(chats[i]) > ts(chats[j])
		})
		return map[string]any{"chats": chats, "users": usersFor(all)}
	})
	route("POST", "/api/chats", createChat)
	route("POST", "/api/chats/:id/members", func(c *Ctx) any {
		id := intParam(c.params["id"])
		ch := chatByID(id)
		if ch == nil || ch.Type != "group" || ch.CreatedBy != c.user.ID {
			fail(403, "forbidden", "Добавлять участников может создатель группы.")
		}
		cur := memberIDs(id)
		has := map[int]bool{}
		for _, x := range cur {
			has[x] = true
		}
		add := []int{}
		if arr, okk := c.body["members"].([]any); okk {
			for _, v := range arr {
				if n, k := toInt(v); k && n > 0 && !has[n] {
					if u := userByID(n); u != nil && u.Name != "" {
						has[n] = true
						add = append(add, n)
					}
				}
			}
		}
		if len(cur)+len(add) > 50 {
			bad("too_many_members", "В группе может быть не больше 50 участников.")
		}
		for _, uid := range add {
			db.Members = append(db.Members, &Member{ChatID: id, UserID: uid})
		}
		all := memberIDs(id)
		for _, uid := range all {
			sendTo(uid, map[string]any{"t": "chat", "chat": chatView(id, uid), "users": usersFor(all)})
		}
		return ok()
	})
	route("POST", "/api/chats/:id/leave", func(c *Ctx) any {
		id := intParam(c.params["id"])
		ch := chatByID(id)
		if ch == nil || ch.Type != "group" || !isMember(id, c.user.ID) {
			fail(404, "not_found", "not_found")
		}
		for i, m := range db.Members {
			if m.ChatID == id && m.UserID == c.user.ID {
				db.Members = append(db.Members[:i], db.Members[i+1:]...)
				break
			}
		}
		rest := memberIDs(id)
		if len(rest) == 0 {
			for i, x := range db.Chats {
				if x.ID == id {
					db.Chats = append(db.Chats[:i], db.Chats[i+1:]...)
					break
				}
			}
			delete(msgsBy, id)
			kept := db.Messages[:0]
			for _, m := range db.Messages {
				if m.ChatID != id {
					kept = append(kept, m)
				}
			}
			db.Messages = kept
		} else {
			for _, uid := range rest {
				sendTo(uid, map[string]any{"t": "chat", "chat": chatView(id, uid), "users": usersFor(rest)})
			}
		}
		return ok()
	})
	route("GET", "/api/chats/:id/messages", func(c *Ctx) any {
		id := intParam(c.params["id"])
		if !isMember(id, c.user.ID) {
			fail(404, "not_found", "Чат не найден.")
		}
		before := math.MaxInt32
		if n, k := toInt(c.query.Get("before")); k && n > 0 {
			before = n
		}
		lim := 50
		if n, k := toInt(c.query.Get("limit")); k && n > 0 {
			lim = n
		}
		if lim > 100 {
			lim = 100
		}
		list := msgsBy[id]
		out := []*MsgView{}
		for i := len(list) - 1; i >= 0 && len(out) < lim; i-- {
			if list[i].ID < before {
				out = append(out, msgView(list[i]))
			}
		}
		more := len(out) == lim
		for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
			out[i], out[j] = out[j], out[i]
		}
		return map[string]any{"messages": out, "more": more}
	})
	route("POST", "/api/chats/:id/messages", func(c *Ctx) any {
		id, me := intParam(c.params["id"]), c.user.ID
		if !isMember(id, me) {
			fail(404, "not_found", "Чат не найден.")
		}
		limit("m:"+strconv.Itoa(me), 60, 60e3)
		m := &Message{ChatID: id, UserID: me, Kind: "text", CreatedAt: now()}
		if v, has := c.body["replyTo"]; has && v != nil {
			rid := intParam(v)
			found := false
			for _, x := range msgsBy[id] {
				if x.ID == rid {
					found = true
				}
			}
			if !found {
				bad("bad_reply", "Сообщение, на которое вы отвечаете, не найдено.")
			}
			m.ReplyTo = rid
		}
		cleanText := func() string {
			t := strings.TrimSpace(strings.ReplaceAll(c.str("text"), "\x00", ""))
			if r := []rune(t); len(r) > 2000 {
				t = strings.TrimSpace(string(r[:2000]))
			}
			return t
		}
		if v, has := c.body["forward"]; has && v != nil {
			src := messageByID(intParam(v))
			if src == nil || !isMember(src.ChatID, me) {
				fail(404, "not_found", "Сообщение не найдено.")
			}
			m.Kind, m.Text, m.EmojiID, m.AudioMs, m.ReplyTo = src.Kind, src.Text, src.EmojiID, src.AudioMs, 0
			if src.Image != "" {
				m.Image = copyUpload(src.Image)
			}
			if src.Audio != "" {
				m.Audio = copyUpload(src.Audio)
			}
			m.FwdFrom = src.UserID
			if src.FwdFrom != 0 {
				m.FwdFrom = src.FwdFrom
			}
		} else if v, has := c.body["emojiId"]; has && v != nil {
			m.EmojiID = intParam(v)
			if emojiByID(m.EmojiID) == nil {
				bad("no_emoji", "Такого смайлика нет.")
			}
			m.Kind = "sticker"
		} else if v, has := c.body["audio"]; has && v != nil {
			m.Audio = saveAudio(v)
			m.Kind = "voice"
			ms, _ := toInt(c.body["duration"])
			if ms < 0 {
				ms = 0
			}
			if ms > 600000 {
				ms = 600000
			}
			m.AudioMs = ms
		} else if v, has := c.body["image"]; has && v != nil {
			m.Image = saveImage(v)
			m.Kind = "image"
			m.Text = cleanText()
		} else {
			m.Text = cleanText()
			if m.Text == "" {
				bad("empty", "Пустое сообщение.")
			}
		}
		m.ID = nextID("message")
		db.Messages = append(db.Messages, m)
		msgsBy[id] = append(msgsBy[id], m)
		if row := memberRow(id, me); row != nil {
			row.LastRead = m.ID
		}
		mv := msgView(m)
		broadcastChat(id, map[string]any{"t": "msg", "message": mv}, 0)
		return map[string]any{"message": mv}
	})
	route("POST", "/api/chats/:id/read", func(c *Ctx) any {
		id := intParam(c.params["id"])
		row := memberRow(id, c.user.ID)
		if row == nil {
			fail(404, "not_found", "not_found")
		}
		if n, k := toInt(c.body["upTo"]); k && n > row.LastRead {
			row.LastRead = n
			broadcastChat(id, map[string]any{"t": "read", "chatId": id, "userId": c.user.ID, "upTo": n}, c.user.ID)
		}
		return ok()
	})
	route("DELETE", "/api/messages/:id", func(c *Ctx) any {
		id := intParam(c.params["id"])
		m := messageByID(id)
		if m == nil || m.UserID != c.user.ID {
			fail(404, "not_found", "Сообщение не найдено.")
		}
		rmFile(m.Image)
		rmFile(m.Audio)
		list := msgsBy[m.ChatID]
		for i, x := range list {
			if x.ID == id {
				msgsBy[m.ChatID] = append(list[:i:i], list[i+1:]...)
				break
			}
		}
		for i, x := range db.Messages {
			if x.ID == id {
				db.Messages = append(db.Messages[:i], db.Messages[i+1:]...)
				break
			}
		}
		broadcastChat(m.ChatID, map[string]any{"t": "del", "chatId": m.ChatID, "id": id}, 0)
		return ok()
	})

	route("GET", "/api/chats/:id/search", func(c *Ctx) any {
		id := intParam(c.params["id"])
		if !isMember(id, c.user.ID) {
			fail(404, "not_found", "Чат не найден.")
		}
		q := strings.ToLower(clean(c.query.Get("q"), 60))
		out := []*MsgView{}
		if len([]rune(q)) >= 2 {
			list := msgsBy[id]
			for i := len(list) - 1; i >= 0 && len(out) < 50; i-- {
				if list[i].Text != "" && strings.Contains(strings.ToLower(list[i].Text), q) {
					out = append(out, msgView(list[i]))
				}
			}
		}
		return map[string]any{"messages": out}
	})
	route("PUT", "/api/chats/:id/prefs", func(c *Ctx) any {
		id := intParam(c.params["id"])
		row := memberRow(id, c.user.ID)
		if row == nil {
			fail(404, "not_found", "Чат не найден.")
		}
		if v, ok := c.body["pinned"].(bool); ok {
			if v && !row.Pinned {
				n := 0
				for _, m := range db.Members {
					if m.UserID == c.user.ID && m.Pinned {
						n++
					}
				}
				if n >= 5 {
					bad("too_many_pins", "Можно закрепить не больше 5 чатов.")
				}
			}
			row.Pinned = v
		}
		if v, ok := c.body["muted"].(bool); ok {
			row.Muted = v
		}
		return map[string]any{"chat": chatView(id, c.user.ID)}
	})
	route("PATCH", "/api/chats/:id", func(c *Ctx) any {
		id := intParam(c.params["id"])
		ch := chatByID(id)
		if ch == nil || ch.Type != "group" || ch.CreatedBy != c.user.ID {
			fail(403, "forbidden", "Менять группу может её создатель.")
		}
		t := clean(c.body["title"], 60)
		if t == "" {
			bad("title_required", "Введите название группы.")
		}
		ch.Title = t
		pushChat(id)
		return map[string]any{"chat": chatView(id, c.user.ID)}
	})
	route("PUT", "/api/chats/:id/avatar", func(c *Ctx) any {
		id := intParam(c.params["id"])
		ch := chatByID(id)
		if ch == nil || ch.Type != "group" || ch.CreatedBy != c.user.ID {
			fail(403, "forbidden", "Менять группу может её создатель.")
		}
		f := saveImage(c.body["data"])
		rmFile(ch.Avatar)
		ch.Avatar = f
		pushChat(id)
		return map[string]any{"chat": chatView(id, c.user.ID)}
	})
	route("DELETE", "/api/chats/:id/avatar", func(c *Ctx) any {
		id := intParam(c.params["id"])
		ch := chatByID(id)
		if ch == nil || ch.Type != "group" || ch.CreatedBy != c.user.ID {
			fail(403, "forbidden", "Менять группу может её создатель.")
		}
		rmFile(ch.Avatar)
		ch.Avatar = ""
		pushChat(id)
		return map[string]any{"chat": chatView(id, c.user.ID)}
	})
	route("PATCH", "/api/messages/:id", func(c *Ctx) any {
		m := messageByID(intParam(c.params["id"]))
		if m == nil || m.UserID != c.user.ID {
			fail(404, "not_found", "Сообщение не найдено.")
		}
		if m.Kind != "text" && m.Kind != "image" {
			bad("not_editable", "Это сообщение нельзя изменить.")
		}
		t := clean(c.body["text"], 2000)
		if t == "" && m.Kind == "text" {
			bad("empty", "Пустое сообщение.")
		}
		m.Text, m.EditedAt = t, now()
		mv := msgView(m)
		broadcastChat(m.ChatID, map[string]any{"t": "edit", "message": mv}, 0)
		return map[string]any{"message": mv}
	})
	route("PUT", "/api/messages/:id/reaction", func(c *Ctx) any {
		m := messageByID(intParam(c.params["id"]))
		if m == nil || !isMember(m.ChatID, c.user.ID) {
			fail(404, "not_found", "Сообщение не найдено.")
		}
		e := c.str("emoji")
		allowed := false
		for _, x := range []string{"👍", "❤️", "😂", "😮", "😢", "🔥", "👎", "🎉"} {
			if x == e {
				allowed = true
			}
		}
		if !allowed {
			bad("bad_reaction", "Такой реакции нет.")
		}
		limit("r:"+strconv.Itoa(c.user.ID), 120, 60e3)
		if m.Reactions == nil {
			m.Reactions = map[string][]int{}
		}
		// у человека только одна реакция на сообщение; повторное нажатие снимает её
		had := false
		for k, ids := range m.Reactions {
			kept := ids[:0:0]
			for _, id := range ids {
				if id == c.user.ID {
					if k == e {
						had = true
					}
				} else {
					kept = append(kept, id)
				}
			}
			if len(kept) == 0 {
				delete(m.Reactions, k)
			} else {
				m.Reactions[k] = kept
			}
		}
		if !had {
			m.Reactions[e] = append(m.Reactions[e], c.user.ID)
		}
		broadcastChat(m.ChatID, map[string]any{"t": "react", "chatId": m.ChatID, "id": m.ID, "reactions": m.Reactions}, 0)
		return map[string]any{"reactions": m.Reactions}
	})

	/* смайлики */
	route("GET", "/api/emojis", func(c *Ctx) any {
		out := []any{}
		for _, e := range db.Emojis {
			out = append(out, emojiView(e))
		}
		return map[string]any{"emojis": out}
	})
	route("POST", "/api/emojis", func(c *Ctx) any {
		name := strings.ToLower(strings.TrimSpace(c.str("name")))
		if !reEmojiNm.MatchString(name) {
			bad("bad_name", "Название смайлика: 2–24 символа, латиница, цифры и _.")
		}
		cnt := 0
		for _, e := range db.Emojis {
			if e.OwnerID == c.user.ID {
				cnt++
			}
			if e.Name == name {
				fail(409, "name_taken", "Смайлик с таким названием уже есть.")
			}
		}
		if cnt >= 100 {
			bad("too_many", "Можно загрузить не больше 100 своих смайликов.")
		}
		file := saveImage(c.body["data"])
		e := &Emoji{ID: nextID("emoji"), OwnerID: c.user.ID, Name: name, File: file, CreatedAt: now()}
		db.Emojis = append(db.Emojis, e)
		ev := emojiView(e)
		broadcastAll(map[string]any{"t": "emoji", "emoji": ev})
		return map[string]any{"emoji": ev}
	})
	route("DELETE", "/api/emojis/:id", func(c *Ctx) any {
		id := intParam(c.params["id"])
		for i, e := range db.Emojis {
			if e.ID == id && e.OwnerID == c.user.ID {
				db.Emojis = append(db.Emojis[:i], db.Emojis[i+1:]...)
				rmFile(e.File)
				for _, m := range db.Messages {
					if m.EmojiID == id {
						m.EmojiID = 0
					}
				}
				broadcastAll(map[string]any{"t": "emoji_del", "id": id})
				return ok()
			}
		}
		fail(404, "not_found", "Смайлик не найден.")
		return nil
	})
}

func emojiView(e *Emoji) map[string]any {
	return map[string]any{"id": e.ID, "name": e.Name, "url": fileURL(e.File), "ownerId": e.OwnerID}
}
func contactView(c *Contact) map[string]any {
	var u *User
	if c.UserID != 0 {
		u = userByID(c.UserID)
	}
	if u == nil && c.Target != "" {
		if c.Kind == "email" {
			u = userByEmail(c.Target)
		} else {
			u = userByPhone(c.Target)
		}
	}
	v := map[string]any{"id": c.ID, "name": c.Name, "target": nstr(c.Target), "kind": nstr(c.Kind), "userId": nil, "user": nil, "createdAt": c.CreatedAt}
	if u != nil && u.Name != "" {
		v["userId"] = u.ID
		v["user"] = publicUser(u)
	}
	return v
}
func pushChat(id int) {
	all := memberIDs(id)
	for _, uid := range all {
		sendTo(uid, map[string]any{"t": "chat", "chat": chatView(id, uid), "users": usersFor(all)})
	}
}
func pushUser(u *User) {
	for _, id := range peersOf(u.ID, true) {
		sendTo(id, map[string]any{"t": "user", "user": publicUser(u)})
	}
}

func createChat(c *Ctx) any {
	me := c.user.ID
	var chatID int
	created := false
	switch c.str("type") {
	case "dm":
		other := intParam(c.body["userId"])
		if other == me {
			bad("self_chat", "Нельзя создать чат с самим собой.")
		}
		ou := userByID(other)
		if ou == nil || ou.Name == "" {
			fail(404, "not_found", "Пользователь не найден.")
		}
		a, b := me, other
		if a > b {
			a, b = b, a
		}
		key := fmt.Sprintf("%d_%d", a, b)
		for _, ch := range db.Chats {
			if ch.DmKey == key {
				chatID = ch.ID
			}
		}
		if chatID == 0 {
			ch := &Chat{ID: nextID("chat"), Type: "dm", DmKey: key, CreatedBy: me, CreatedAt: now()}
			db.Chats = append(db.Chats, ch)
			db.Members = append(db.Members, &Member{ChatID: ch.ID, UserID: me}, &Member{ChatID: ch.ID, UserID: other})
			chatID, created = ch.ID, true
		}
	case "group":
		title := clean(c.body["title"], 60)
		if title == "" {
			bad("title_required", "Введите название группы.")
		}
		seen := map[int]bool{me: true}
		valid := []int{}
		if arr, okk := c.body["members"].([]any); okk {
			for _, v := range arr {
				if n, k := toInt(v); k && n > 0 && !seen[n] && len(valid) < 49 {
					seen[n] = true
					if u := userByID(n); u != nil && u.Name != "" {
						valid = append(valid, n)
					}
				}
			}
		}
		if len(valid) == 0 {
			bad("members_required", "Добавьте хотя бы одного участника.")
		}
		ch := &Chat{ID: nextID("chat"), Type: "group", Title: title, CreatedBy: me, CreatedAt: now()}
		db.Chats = append(db.Chats, ch)
		for _, uid := range append([]int{me}, valid...) {
			db.Members = append(db.Members, &Member{ChatID: ch.ID, UserID: uid})
		}
		chatID, created = ch.ID, true
	default:
		bad("bad_type", "bad_type")
	}
	if created {
		all := memberIDs(chatID)
		for _, uid := range all {
			if uid != me {
				sendTo(uid, map[string]any{"t": "chat", "chat": chatView(chatID, uid), "users": usersFor(all)})
			}
		}
	}
	cv := chatView(chatID, me)
	return map[string]any{"chat": cv, "users": usersFor(cv.Members)}
}

/* ───── вход по коду ───── */
func authRequest(c *Ctx) any {
	kind := c.str("kind")
	target := normTarget(kind, c.str("target"))
	viaReal := (kind == "email" && canEmail()) || (kind == "phone" && canSms())
	var code string
	func() {
		mu.Lock()
		defer mu.Unlock()
		lim := func(n, dev int) int {
			if devCodes {
				return dev
			}
			return n
		}
		limit("ip:"+c.ip, lim(30, 500), 3600e3)
		if old, okk := db.Codes[target]; okk && !devCodes && now()-old.SentAt < 45000 {
			fail(429, "wait", "Код уже отправлен. Подождите минуту и запросите снова.")
		}
		limit("t:"+target, lim(6, 500), 3600e3)
		if !viaReal && !devCodes {
			msg := "Отправка SMS на сервере ещё не настроена."
			if kind == "email" {
				msg = "Отправка писем на сервере ещё не настроена."
			}
			fail(503, "no_provider", msg)
		}
		n, _ := rand.Int(rand.Reader, big.NewInt(900000))
		code = strconv.Itoa(int(n.Int64()) + 100000)
		db.Codes[target] = &Code{Hash: sha(secret + target + code), Expires: now() + codeTTL, SentAt: now()}
		save()
	}()
	text := "Ваш код для входа в Чат-Чат: " + code + "\nОн действует 10 минут. Никому его не сообщайте."
	if viaReal {
		var err error
		if kind == "email" {
			if brevoKey != "" {
				err = brevoSend(target, "Код входа в Чат-Чат", text)
			} else {
				err = smtpSend(target, "Код входа в Чат-Чат", text)
			}
		} else {
			err = twilioSend(target, text)
		}
		if err != nil {
			log.Println("send failed:", err)
			mu.Lock()
			delete(db.Codes, target)
			mu.Unlock()
			fail(502, "send_failed", "Не удалось отправить код. Проверьте адрес или номер и попробуйте позже.")
		}
		return map[string]any{"ok": true, "ttl": codeTTL / 1000}
	}
	log.Printf("[DEV] код для %s: %s", target, code)
	return map[string]any{"ok": true, "ttl": codeTTL / 1000, "devCode": code}
}

func authVerify(c *Ctx) any {
	kind := c.str("kind")
	target := normTarget(kind, c.str("target"))
	limit("v:"+c.ip, map[bool]int{true: 1000, false: 60}[devCodes], 3600e3)
	code := strings.TrimSpace(c.str("code"))
	row := db.Codes[target]
	if row == nil || row.Expires < now() {
		fail(400, "bad_code", "Неверный или просроченный код.")
	}
	if row.Attempts >= 5 {
		delete(db.Codes, target)
		fail(400, "bad_code", "Неверный или просроченный код.")
	}
	if !reCode.MatchString(code) || subtle.ConstantTimeCompare([]byte(sha(secret+target+code)), []byte(row.Hash)) != 1 {
		row.Attempts++
		fail(400, "bad_code", "Неверный или просроченный код.")
	}
	delete(db.Codes, target)
	var u *User
	if kind == "email" {
		u = userByEmail(target)
	} else {
		u = userByPhone(target)
	}
	isNew := false
	if u == nil {
		u = &User{ID: nextID("user"), CreatedAt: now(), Settings: map[string]any{}}
		if kind == "email" {
			u.Email = target
		} else {
			u.Phone = target
		}
		db.Users = append(db.Users, u)
		isNew = true
	}
	token := randHex(32)
	db.Sessions[sha(token)] = u.ID
	return map[string]any{"token": token, "user": selfUser(u), "isNew": isNew || u.Name == ""}
}
