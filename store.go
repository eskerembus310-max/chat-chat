package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type User struct {
	ID        int            `json:"id"`
	Email     string         `json:"email"`
	Phone     string         `json:"phone"`
	Name      string         `json:"name"`
	Username  string         `json:"username"`
	Bio       string         `json:"bio"`
	Avatar    string         `json:"avatar"`
	CreatedAt int64          `json:"createdAt"`
	LastSeen  int64          `json:"lastSeen"`
	Settings  map[string]any `json:"settings"`
}
type Code struct {
	Hash     string `json:"hash"`
	Expires  int64  `json:"expires"`
	Attempts int    `json:"attempts"`
	SentAt   int64  `json:"sentAt"`
}
type Chat struct {
	ID        int    `json:"id"`
	Type      string `json:"type"`
	Title     string `json:"title"`
	DmKey     string `json:"dmKey"`
	CreatedBy int    `json:"createdBy"`
	CreatedAt int64  `json:"createdAt"`
}
type Member struct {
	ChatID   int `json:"chatId"`
	UserID   int `json:"userId"`
	LastRead int `json:"lastRead"`
}
type Emoji struct {
	ID        int    `json:"id"`
	OwnerID   int    `json:"ownerId"`
	Name      string `json:"name"`
	File      string `json:"file"`
	CreatedAt int64  `json:"createdAt"`
}
type Message struct {
	ID        int    `json:"id"`
	ChatID    int    `json:"chatId"`
	UserID    int    `json:"userId"`
	Kind      string `json:"kind"`
	Text      string `json:"text"`
	EmojiID   int    `json:"emojiId"`
	CreatedAt int64  `json:"createdAt"`
}
type Contact struct {
	ID        int    `json:"id"`
	OwnerID   int    `json:"ownerId"`
	Name      string `json:"name"`
	Target    string `json:"target"`
	Kind      string `json:"kind"`
	UserID    int    `json:"userId"`
	CreatedAt int64  `json:"createdAt"`
}
type DB struct {
	Users    []*User          `json:"users"`
	Codes    map[string]*Code `json:"codes"`
	Sessions map[string]int   `json:"sessions"`
	Chats    []*Chat          `json:"chats"`
	Members  []*Member        `json:"members"`
	Emojis   []*Emoji         `json:"emojis"`
	Messages []*Message       `json:"messages"`
	Contacts []*Contact       `json:"contacts"`
	Seq      map[string]int   `json:"seq"`
}

var (
	mu        sync.Mutex
	db        *DB
	msgsBy    = map[int][]*Message{}
	dataDir   string
	uploadDir string
	secret    string
	saveTimer *time.Timer
)

func now() int64 { return time.Now().UnixMilli() }
func sha(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}
func randHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
func nextID(kind string) int { db.Seq[kind]++; return db.Seq[kind] }

func openDB() {
	db = &DB{Codes: map[string]*Code{}, Sessions: map[string]int{}, Seq: map[string]int{}}
	b, err := os.ReadFile(filepath.Join(dataDir, "chatchat.json"))
	if err == nil {
		if err := json.Unmarshal(b, db); err != nil {
			bak := filepath.Join(dataDir, "chatchat.broken-"+time.Now().Format("20060102-150405")+".json")
			_ = os.WriteFile(bak, b, 0o600)
			log.Println("файл данных повреждён, копия:", bak)
			db = &DB{Codes: map[string]*Code{}, Sessions: map[string]int{}, Seq: map[string]int{}}
		}
	}
	if db.Codes == nil {
		db.Codes = map[string]*Code{}
	}
	if db.Sessions == nil {
		db.Sessions = map[string]int{}
	}
	if db.Seq == nil {
		db.Seq = map[string]int{}
	}
	for _, m := range db.Messages {
		msgsBy[m.ChatID] = append(msgsBy[m.ChatID], m)
	}
}

// save: откладываем запись на диск на долю секунды (вызывать под mu).
func save() {
	if saveTimer != nil {
		return
	}
	saveTimer = time.AfterFunc(400*time.Millisecond, flush)
}
func flush() {
	mu.Lock()
	saveTimer = nil
	b, err := json.Marshal(db)
	mu.Unlock()
	if err != nil {
		log.Println("save:", err)
		return
	}
	tmp := filepath.Join(dataDir, "chatchat.json.tmp")
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		log.Println("save:", err)
		return
	}
	if err := os.Rename(tmp, filepath.Join(dataDir, "chatchat.json")); err != nil {
		log.Println("save:", err)
	}
}

func userByID(id int) *User {
	for _, u := range db.Users {
		if u.ID == id {
			return u
		}
	}
	return nil
}
func userByEmail(e string) *User {
	for _, u := range db.Users {
		if u.Email != "" && u.Email == e {
			return u
		}
	}
	return nil
}
func userByPhone(p string) *User {
	for _, u := range db.Users {
		if u.Phone != "" && u.Phone == p {
			return u
		}
	}
	return nil
}
func userByName(n string) *User {
	for _, u := range db.Users {
		if u.Username != "" && u.Username == n {
			return u
		}
	}
	return nil
}
func chatByID(id int) *Chat {
	for _, c := range db.Chats {
		if c.ID == id {
			return c
		}
	}
	return nil
}
func memberIDs(chatID int) []int {
	out := []int{}
	for _, m := range db.Members {
		if m.ChatID == chatID {
			out = append(out, m.UserID)
		}
	}
	return out
}
func memberRow(chatID, userID int) *Member {
	for _, m := range db.Members {
		if m.ChatID == chatID && m.UserID == userID {
			return m
		}
	}
	return nil
}
func isMember(chatID, userID int) bool { return memberRow(chatID, userID) != nil }
func emojiByID(id int) *Emoji {
	for _, e := range db.Emojis {
		if e.ID == id {
			return e
		}
	}
	return nil
}
func messageByID(id int) *Message {
	for _, list := range msgsBy {
		for _, m := range list {
			if m.ID == id {
				return m
			}
		}
	}
	return nil
}
func peersOf(userID int, includeSelf bool) []int {
	seen := map[int]bool{}
	out := []int{}
	for _, m1 := range db.Members {
		if m1.UserID != userID {
			continue
		}
		for _, m2 := range db.Members {
			if m2.ChatID == m1.ChatID && (includeSelf || m2.UserID != userID) && !seen[m2.UserID] {
				seen[m2.UserID] = true
				out = append(out, m2.UserID)
			}
		}
	}
	return out
}
func sortedContacts(owner int) []*Contact {
	out := []*Contact{}
	for _, c := range db.Contacts {
		if c.OwnerID == owner {
			out = append(out, c)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	return out
}
