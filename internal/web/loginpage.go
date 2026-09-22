package web

import (
	"crypto/sha256"
	"encoding/base64"
	"html/template"
	"net/http"

	"github.com/ZachCurry13/isoshelf/internal/auth"
)

// The login page is written here rather than in static/, and is a plain HTML
// form rather than script. Somebody who is not logged in should be handed as
// little of isoshelf as possible, and a form that works with no JavaScript at
// all works in every browser, on a phone, and in whatever a NAS puts in front
// of it.

// loginStyle is the page's whole stylesheet. It is inline, which the page's
// own content policy forbids - so the policy for this one page names the
// hash of exactly these bytes instead. Derived from the same constant, so it
// cannot drift: change the style and the hash changes with it.
const loginStyle = `:root { color-scheme: light dark; --bg: #f6f7f9; --card: #fff; --text: #14181f;
  --muted: #5b6472; --line: #d8dce3; --accent: #2f6fed; }
@media (prefers-color-scheme: dark) { :root { --bg: #11151c; --card: #181d26;
  --text: #e8ebf0; --muted: #98a2b3; --line: #2a313c; --accent: #6f9dff; } }
* { box-sizing: border-box; }
body { margin: 0; min-height: 100vh; display: grid; place-items: center;
  padding: 16px; background: var(--bg); color: var(--text);
  font: 16px/1.5 system-ui, -apple-system, "Segoe UI", Roboto, sans-serif; }
main { width: min(380px, 100%); background: var(--card); padding: 24px;
  border: 1px solid var(--line); border-radius: 14px; }
h1 { margin: 0 0 4px; font-size: 1.35rem; }
p { margin: 0 0 18px; color: var(--muted); font-size: 0.9rem; }
label { display: block; margin: 0 0 4px; font-size: 0.9rem; }
input { width: 100%; font: inherit; padding: 9px 11px; margin: 0 0 14px;
  color: var(--text); background: var(--bg);
  border: 1px solid var(--line); border-radius: 8px; }
button { width: 100%; font: inherit; font-weight: 600; padding: 10px;
  color: #fff; background: var(--accent); border: 0; border-radius: 8px;
  cursor: pointer; }
.bad { color: #b3261e; background: #fdecea; border: 1px solid #f5c6c2;
  padding: 9px 11px; border-radius: 8px; margin: 0 0 16px; font-size: 0.9rem; }
@media (prefers-color-scheme: dark) { .bad { color: #ffb4ab; background: #3b1a17;
  border-color: #6b2b25; } }
.note { margin: 16px 0 0; font-size: 0.82rem; }
`

// loginStyleHash is what the content-security-policy header has to say for a
// browser to apply the style above.
var loginStyleHash = func() string {
	sum := sha256.Sum256([]byte(loginStyle))
	return "'sha256-" + base64.StdEncoding.EncodeToString(sum[:]) + "'"
}()

var loginPage = template.Must(template.New("login").Parse(`<!doctype html>
<html lang="en"><head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{if .First}}Set up isoshelf{{else}}isoshelf{{end}}</title>
<style>{{.Style}}</style>
</head><body>
<main>
{{if .First}}
  <h1>Set up isoshelf</h1>
  <p>Choose a username and password. You'll use them to open isoshelf from
  any device on your network.</p>
{{else}}
  <h1>isoshelf</h1>
  <p>Sign in to look after your images.</p>
{{end}}
{{if .Problem}}<div class="bad">{{.Problem}}</div>{{end}}
<form method="post" action="/login">
  <input type="hidden" name="form_token" value="{{.FormToken}}">
  <label for="user">Username</label>
  <input id="user" name="user" autocomplete="username" autocapitalize="none"
         autocorrect="off" spellcheck="false" required autofocus>
  <label for="password">Password</label>
  <input id="password" name="password" type="password" required
         autocomplete="{{if .First}}new-password{{else}}current-password{{end}}">
{{if .First}}
  <label for="again">Password again</label>
  <input id="again" name="again" type="password" required autocomplete="new-password">
  <button type="submit">Set it up</button>
  <p class="note">At least {{.MinPassword}} characters. isoshelf stores a
  scrambled form of it, never the password itself.</p>
{{else}}
  <button type="submit">Sign in</button>
  <p class="note">Forgotten it? The link isoshelf prints in its log still
  works, and Settings can change the password once you're in.</p>
{{end}}
</main>
</body></html>
`))

type loginData struct {
	Style       template.CSS
	First       bool
	Problem     string
	MinPassword int
	FormToken   string
}

// writeLoginPage shows the login form, or the setup form when nobody has a
// login yet.
func (s *Server) writeLoginPage(w http.ResponseWriter, have *auth.Account, problem string, code int) {
	// The cookie has to go on before anything is written, and it pairs with
	// the value in the form below.
	token := s.formToken(w)
	// The guard set a policy that forbids inline styles, which would leave
	// this page as unstyled HTML. Naming the hash of its one stylesheet lets
	// that one through and nothing else.
	w.Header().Set("Content-Security-Policy",
		"default-src 'self'; style-src "+loginStyleHash+"; img-src 'self' data:; frame-ancestors 'none'")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	loginPage.Execute(w, loginData{
		Style:       template.CSS(loginStyle),
		First:       have == nil,
		Problem:     problem,
		MinPassword: auth.MinPassword,
		FormToken:   token,
	})
}
