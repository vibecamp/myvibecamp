package main

import (
	"context"
	"encoding/gob"
	"net/http"
	"strings"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/kurrik/oauth1a"
	log "github.com/sirupsen/logrus"
)

type Session struct {
	UserName    string
	TwitterName string
	TwitterID   string
	Oauth       *oauth1a.UserConfig
}

func init() {
	gob.Register(Session{})
}

const sessionKey = "s"

func GetSession(c *gin.Context) *Session {
	defaultSession := sessions.Default(c)
	s := defaultSession.Get(sessionKey)
	if s == nil {
		return new(Session)
	}

	ss := s.(Session)
	return &ss
}

func SaveSession(c *gin.Context, s *Session) {
	defaultSession := sessions.Default(c)
	defaultSession.Set(sessionKey, &s)
	defaultSession.Save()
}

func ClearSession(c *gin.Context) {
	session := sessions.Default(c)
	session.Clear()
	session.Save()
}

func SuccessFlash(c *gin.Context, value string) {
	defaultSession := sessions.Default(c)
	defaultSession.AddFlash(value, "success")
	defaultSession.Save()
}

func WarningFlash(c *gin.Context, value string) {
	defaultSession := sessions.Default(c)
	defaultSession.AddFlash(value, "warning")
	defaultSession.Save()
}

func ErrorFlash(c *gin.Context, value string) {
	defaultSession := sessions.Default(c)
	defaultSession.AddFlash(value, "error")
	defaultSession.Save()
}

func GetFlashes(c *gin.Context) map[string][]string {
	defaultSession := sessions.Default(c)
	f := map[string][]string{
		"success": flashes(defaultSession.Flashes("success")),
		"error":   flashes(defaultSession.Flashes("error")),
		"warning": flashes(defaultSession.Flashes("warning")),
		"info":    flashes(defaultSession.Flashes("info")),
	}
	defaultSession.Save() // saves the fact that we got flashes
	return f
}

func flashes(f []interface{}) []string {
	converted := make([]string, len(f))
	for i := 0; i < len(f); i++ {
		converted[i] = f[i].(string)
	}
	return converted
}

func (s *Session) SignedIn() bool {
	return s != nil && (s.TwitterName != "" || s.UserName != "")
}

func SignInHandler(c *gin.Context) {
	session := GetSession(c)

	session.Oauth = &oauth1a.UserConfig{}
	err := session.Oauth.GetRequestToken(context.Background(), service, http.DefaultClient)
	if err != nil {
		log.Debugf("Could not get request token: %v", err)
		c.String(http.StatusInternalServerError, "Problem getting the request token")
		c.Abort()
		return
	}

	url, err := session.Oauth.GetAuthorizeURL(service)
	if err != nil {
		log.Debugf("Could not get authorization URL: %v", err)
		c.String(http.StatusInternalServerError, "Problem getting the authorization URL")
		c.Abort()
		return
	}

	SaveSession(c, session)

	log.Debugf("Redirecting user to %v\n", url)
	c.Redirect(http.StatusFound, url)
}

func SignOutHandler(c *gin.Context) {
	ClearSession(c)
	c.Redirect(http.StatusFound, "/")
}

func CallbackHandler(c *gin.Context) {
	log.Debugf("Callback hit") //. %v current sessions.\n", len(sessions))

	session := GetSession(c)
	if session.Oauth == nil || session.Oauth.RequestTokenKey == "" {
		log.Tracef("No user config in session")
		c.String(http.StatusBadRequest, "error: no session found")
		c.Abort()
		return
	}

	token, verifier, err := session.Oauth.ParseAuthorize(c.Request, service)
	if err != nil {
		log.Tracef("Could not parse authorization: %v", err)
		c.String(http.StatusInternalServerError, "error: could not parse authorization")
		c.Abort()
		return
	}

	err = session.Oauth.GetAccessToken(context.Background(), token, verifier, service, http.DefaultClient)
	if err != nil {
		log.Tracef("Error getting access token: %v", err)
		c.String(http.StatusInternalServerError, "error: could not get access token")
		c.Abort()
		return
	}

	session.TwitterName = session.Oauth.AccessValues.Get("screen_name")
	session.TwitterID = session.Oauth.AccessValues.Get("user_id")
	session.UserName = strings.ToLower(session.TwitterName)
	session.Oauth = nil

	if localDevMode {
		// session.TwitterName = "GRINTESTING" // login as this user, for dev
		session.UserName = strings.ToLower(session.TwitterName)
	}

	SaveSession(c, session)
	c.Redirect(http.StatusFound, "/signin-redirect")
}
