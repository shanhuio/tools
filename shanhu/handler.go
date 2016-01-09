package shanhu

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

// Handler is the http handler for shanhu.io website
type Handler struct {
	c        *Config
	gh       *gitHub
	sessions *sessionStore
}

// NewHandler creates a new http handler.
func NewHandler(c *Config) (*Handler, error) {
	gh := newGitHub(c.GitHubAppID, c.GitHubAppSecret, []byte(c.StateKey))
	return &Handler{
		c:        c,
		gh:       gh,
		sessions: newSessionStore([]byte(c.SessionKey)),
	}, nil
}

func serveFile(w http.ResponseWriter, file string) {
	f, err := os.Open(file)
	if os.IsNotExist(err) {
		http.Error(w, "File not found.", 404)
		return
	}

	if err != nil {
		log.Println(err)
		return
	}

	defer f.Close()
	if _, err := io.Copy(w, f); err != nil {
		log.Println(err)
	}
}

func (h *Handler) serve(c *context, path string) {
	if strings.HasPrefix(path, "/assets/") {
		serveFile(c.w, "_"+path)
		return
	}

	switch path {
	case "/favicon.ico":
		serveFile(c.w, "_/assets/favicon.ico")
	case "/style.css":
		c.w.Header().Set("Content-Type", "text/css")
		serveFile(c.w, "_/style.css")

	case "/github/signin":
		c.redirect(h.gh.signInURL())
	case "/github/callback":
		user, err := h.gh.callback(c.req)
		if err != nil {
			// print a better error message page
			fmt.Fprintln(c.w, err)
			return
		}
		if user != "h8liu" {
			fmt.Fprintln(c.w, err)
			return
		}
		if user == "h8liu" {
			session, expires := h.sessions.New(user)
			c.writeCookie("session", session, expires)
			c.redirect("/")
		}
	case "/github/signout":
		c.clearCookie("session")
		c.redirect("/")

	default:
		sessionStr := c.readCookie("session")
		ok, session := h.sessions.Check(sessionStr)
		if !ok {
			c.clearCookie("session")
			serveFile(c.w, "_/signin.html")
			return
		}

		h.serveUser(c, session.User, path)
	}
}

func (h *Handler) serveUser(c *context, user, path string) {
	if strings.HasPrefix(path, "/data/") {
		h.serveData(c, user, path)
		return
	}

	switch path {
	case "/proj.html", "/":
		serveFile(c.w, "_/proj.html")
	case "/file.html":
		serveFile(c.w, "_/file.html")
	default:
		http.Error(c.w, "File not found.", 404)
	}
}

func (h *Handler) serveData(c *context, user, path string) {
	http.Error(c.w, "Data not implemented yet.", 404)
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	c := &context{w: w, req: req}
	path := req.URL.Path
	log.Println("serving: ", path)
	h.serve(c, path)
}
