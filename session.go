package main

import (
	"crypto/rand"
	"encoding/base64"
	"io"
	"net/http"

	"github.com/kurrik/oauth1a"
)

type Session struct {
	key         string
	secret      string
	twitterName string
	twitterID   string
	oauth       *oauth1a.UserConfig
}

var sessions map[string]*Session

func init() {
	sessions = make(map[string]*Session)
}

func NewSessionID() string {
	c := 128
	b := make([]byte, c)
	n, err := io.ReadFull(rand.Reader, b)
	if n != len(b) || err != nil {
		panic("Could not generate random number")
	}
	return base64.URLEncoding.EncodeToString(b)
}

func GetSessionID(r *http.Request) (id string, err error) {
	var c *http.Cookie
	if c, err = r.Cookie("session_id"); err == nil {
		id = c.Value
	}
	return
}

func SessionStartCookie(id string) *http.Cookie {
	return &http.Cookie{
		Name:     "session_id",
		Value:    id,
		MaxAge:   60,
		Secure:   !localDevMode,
		Path:     "/",
		HttpOnly: true,
	}
}

func SessionEndCookie() *http.Cookie {
	return &http.Cookie{
		Name:     "session_id",
		Value:    "",
		MaxAge:   0,
		Secure:   !localDevMode,
		Path:     "/",
		HttpOnly: true,
	}
}
