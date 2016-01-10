package shanhu

import (
	"encoding/json"
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
	users    map[string]bool
}

// NewHandler creates a new http handler.
func NewHandler(c *Config) (*Handler, error) {
	gh := newGitHub(c.GitHubAppID, c.GitHubAppSecret, []byte(c.StateKey))

	users := make(map[string]bool)
	for _, u := range c.Users {
		users[u] = true
	}

	return &Handler{
		c:        c,
		gh:       gh,
		sessions: newSessionStore([]byte(c.SessionKey)),
		users:    users,
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

func (h *Handler) hasUser(u string) bool { return h.users[u] }

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
		if !h.hasUser(user) {
			fmt.Fprintf(c.w, "User %q is not authorized.", user)
			log.Printf("Unauthorized user %q tried to sign in.", user)
			return
		}

		log.Printf("User %q signed in.", user)
		session, expires := h.sessions.New(user)
		c.writeCookie("session", session, expires)
		c.redirect("/")
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

		user := session.User
		if !h.hasUser(user) {
			c.clearCookie("session")
			msg := fmt.Sprintf("user %q not authorized, " +
				"please contact liulonnie@gmail.com.")
			http.Error(c.w, msg, 403)
			return
		}

		h.serveUser(c, user, path)
	}
}

func (h *Handler) serveUser(c *context, user, path string) {
	log.Printf("[%s] %s", user, path)
	if strings.HasPrefix(path, "/data/") {
		h.serveData(c, user, path)
		return
	}

	switch path {
	case "/proj.html", "/":
		serveFile(c.w, "_/proj.html")
	case "/file.html":
		serveFile(c.w, "_/file.html")

	case "/js/proj.js":
		serveFile(c.w, "_/proj.js")
	case "/js/file.js":
		serveFile(c.w, "_/file.js")

	default:
		http.Error(c.w, "File not found.", 404)
	}
}

func serveJsVar(w http.ResponseWriter, v interface{}) {
	fmt.Fprintf(w, "var shanhuData = ")
	bs, err := json.Marshal(v)
	if err != nil {
		log.Fatal(err)
	}
	w.Write(bs)
	fmt.Fprintf(w, ";")
}

func (h *Handler) serveData(c *context, user, path string) {
	obj := make(map[string]interface{})
	obj["session"] = map[string]string{
		"user": user,
	}

	switch path {
	case "/data/proj.js":
		obj["proj"] = nil
	default:
		http.Error(c.w, "Invalid data request.", 404)
		return
	}

	serveJsVar(c.w, obj)
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	c := &context{w: w, req: req}
	path := req.URL.Path
	h.serve(c, path)
}
