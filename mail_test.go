package main

import (
	"bufio"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"strings"
	"testing"
	"time"
)

func TestSMTP(t *testing.T) {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tpl := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "localhost"}, DNSNames: []string{"localhost"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, _ := x509.CreateCertificate(rand.Reader, tpl, tpl, &key.PublicKey, key)
	cert := tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
	ln, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{cert}})
	if err != nil {
		t.Fatal(err)
	}
	got := make(chan string, 1)
	go func() {
		c, _ := ln.Accept()
		defer c.Close()
		rd := bufio.NewReader(c)
		var data strings.Builder
		w := func(s string) { c.Write([]byte(s + "\r\n")) }
		w("220 hi")
		inData := false
		for {
			line, err := rd.ReadString('\n')
			if err != nil {
				return
			}
			line = strings.TrimRight(line, "\r\n")
			if inData {
				if line == "." {
					inData = false
					got <- data.String()
					w("250 ok")
				} else {
					data.WriteString(line + "\n")
				}
				continue
			}
			switch {
			case strings.HasPrefix(line, "EHLO"):
				w("250-x")
				w("250 AUTH LOGIN")
			case line == "AUTH LOGIN":
				w("334 VXNlcm5hbWU6")
			case strings.HasPrefix(line, "MAIL"), strings.HasPrefix(line, "RCPT"):
				w("250 ok")
			case line == "DATA":
				inData = true
				w("354 go")
			case line == "QUIT":
				w("221 bye")
				return
			default: // логин и пароль
				if strings.Contains(line, "@") || len(line) > 3 {
					if strings.HasPrefix(line, "Ym9i") { // base64 пароля "bob"
					}
				}
				if line == "cGFzcw==" { // "pass"
					w("235 ok")
				} else {
					w("334 UGFzc3dvcmQ6")
				}
			}
		}
	}()
	pool := x509.NewCertPool()
	pc, _ := x509.ParseCertificate(der)
	pool.AddCert(pc)
	smtpTLS = &tls.Config{RootCAs: pool, ServerName: "localhost"}
	_, port, _ := net.SplitHostPort(ln.Addr().String())
	smtpURL = "smtps://user%40x.com:pass@localhost:" + port
	mailFrom = "Чат-Чат <user@x.com>"
	if err := smtpSend("papa@mail.ru", "Код входа", "Ваш код: 123456"); err != nil {
		t.Fatal(err)
	}
	select {
	case d := <-got:
		if !strings.Contains(d, "To: papa@mail.ru") || !strings.Contains(d, "Subject: =?UTF-8?B?") {
			t.Fatal("плохое письмо:", d)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("письмо не дошло")
	}
}
