package main

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var smtpTLS *tls.Config // для тестов

func smtpSend(to, subject, text string) error {
	u, err := url.Parse(smtpURL)
	if err != nil {
		return err
	}
	port := u.Port()
	if port == "" {
		port = "465"
	}
	user := u.User.Username()
	pass, _ := u.User.Password()
	from := mailFrom
	if from == "" {
		from = user
	}
	fromAddr := from
	if m := regexp.MustCompile(`<([^>]+)>`).FindStringSubmatch(from); m != nil {
		fromAddr = m[1]
	}
	conn, err := tls.DialWithDialer(&net.Dialer{Timeout: 15 * time.Second}, "tcp", net.JoinHostPort(u.Hostname(), port), tlsCfg(u.Hostname()))
	if err != nil {
		return err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(30 * time.Second))
	rd := bufio.NewReader(conn)
	read := func(want string) error {
		for {
			line, err := rd.ReadString('\n')
			if err != nil {
				return err
			}
			if len(line) < 4 {
				return fmt.Errorf("smtp: %q", line)
			}
			if line[3] == '-' {
				continue
			}
			if !strings.HasPrefix(line, want) {
				return fmt.Errorf("smtp %s", strings.TrimSpace(line))
			}
			return nil
		}
	}
	send := func(s, want string) error {
		if _, err := conn.Write([]byte(s + "\r\n")); err != nil {
			return err
		}
		return read(want)
	}
	b64 := func(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }
	body := strings.Join([]string{
		"From: " + strings.NewReplacer("\r", "", "\n", "").Replace(from), "To: " + to,
		"Subject: =?UTF-8?B?" + b64(subject) + "?=", "MIME-Version: 1.0", "Content-Type: text/plain; charset=utf-8",
		"Content-Transfer-Encoding: base64", "", b64(text),
	}, "\r\n")
	if err := read("220"); err != nil {
		return err
	}
	steps := [][2]string{{"EHLO chatchat.local", "250"}, {"AUTH LOGIN", "334"}, {b64(user), "334"}, {b64(pass), "235"},
		{"MAIL FROM:<" + fromAddr + ">", "250"}, {"RCPT TO:<" + to + ">", "250"}, {"DATA", "354"}, {body + "\r\n.", "250"}}
	for _, s := range steps {
		if err := send(s[0], s[1]); err != nil {
			return err
		}
	}
	_, _ = conn.Write([]byte("QUIT\r\n"))
	return nil
}

func twilioSend(to, text string) error {
	form := url.Values{"To": {to}, "From": {twFrom}, "Body": {text}}
	req, _ := http.NewRequest("POST", "https://api.twilio.com/2010-04-01/Accounts/"+twSID+"/Messages.json", strings.NewReader(form.Encode()))
	req.SetBasicAuth(twSID, twTok)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("twilio %d", resp.StatusCode)
	}
	return nil
}

func tlsCfg(host string) *tls.Config {
	if smtpTLS != nil {
		return smtpTLS
	}
	return &tls.Config{ServerName: host}
}

// brevoSend отправляет письмо через HTTP-API Brevo (бесплатно до 300 писем в день).
// MAIL_FROM должен быть адресом, подтверждённым в Brevo.
func brevoSend(to, subject, text string) error {
	name, addr := "Чат-Чат", mailFrom
	if m := regexp.MustCompile(`^\s*(.*?)\s*<([^>]+)>\s*$`).FindStringSubmatch(mailFrom); m != nil {
		if m[1] != "" {
			name = m[1]
		}
		addr = m[2]
	}
	payload, _ := json.Marshal(map[string]any{
		"sender":      map[string]string{"name": name, "email": addr},
		"to":          []map[string]string{{"email": to}},
		"subject":     subject,
		"textContent": text,
	})
	req, _ := http.NewRequest("POST", "https://api.brevo.com/v3/smtp/email", bytes.NewReader(payload))
	req.Header.Set("api-key", brevoKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("brevo %d", resp.StatusCode)
	}
	return nil
}
