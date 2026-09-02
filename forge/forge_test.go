package forge

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"sync"
	"testing"

	"code.gitea.io/sdk/gitea"
	gh "github.com/google/go-github/v84/github"
	graphql "github.com/hasura/go-graphql-client"
	gl "gitlab.com/gitlab-org/api/client-go"

	"ysun.co/miroir/config"
)

func TestDispatch(t *testing.T) {
	tests := []struct {
		forge  config.Forge
		domain string
		want   string
	}{
		{config.Github, "github.com", "*forge.ghForge"},
		{config.Gitlab, "gitlab.com", "*forge.glForge"},
		{config.Codeberg, "codeberg.org", "*forge.cbForge"},
		{config.Sourcehut, "git.sr.ht", "*forge.srhtForge"},
	}
	for _, tt := range tests {
		f, err := Dispatch(tt.forge, "dummy", tt.domain)
		if err != nil {
			t.Fatal(err)
		}
		if got := fmt.Sprintf("%T", f); got != tt.want {
			t.Errorf("Dispatch(%v): got %s, want %s", tt.forge, got, tt.want)
		}
	}
	if _, err := Dispatch(config.Forge(99), "dummy", "example.com"); err == nil {
		t.Error("expected error for unknown forge")
	}
}

func TestNewGithubDomains(t *testing.T) {
	for _, domain := range []string{"github.com", "GitHub.com"} {
		g, err := newGithub("t", domain)
		if err != nil {
			t.Fatal(err)
		}
		if got := g.c.BaseURL.String(); got != "https://api.github.com/" {
			t.Errorf("%s base url: got %q", domain, got)
		}
	}

	e, err := newGithub("t", "github.example.com")
	if err != nil {
		t.Fatal(err)
	}
	if got := e.c.BaseURL.String(); got != "https://github.example.com/api/v3/" {
		t.Errorf("enterprise base url: got %q", got)
	}
}

// requestLog records method+path pairs handled by a fake forge server
type requestLog struct {
	mu   sync.Mutex
	seen []string
}

func (l *requestLog) add(r *http.Request) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.seen = append(l.seen, r.Method+" "+r.URL.Path)
}

func (l *requestLog) get() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.seen...)
}

func ghTestForge(t *testing.T, h http.Handler) (*ghForge, *requestLog) {
	t.Helper()
	log := &requestLog{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.add(r)
		h.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	c := gh.NewClient(srv.Client())
	u, err := url.Parse(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	c.BaseURL = u
	c.UploadURL = u
	return &ghForge{c: c}, log
}

func TestGithubSyncCreatesWhenMissing(t *testing.T) {
	f, log := ghTestForge(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/repos/alice/x":
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprint(w, `{"message":"Not Found"}`)
		case r.Method == "POST" && r.URL.Path == "/user/repos":
			w.WriteHeader(http.StatusCreated)
			fmt.Fprint(w, `{"name":"x"}`)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	if err := f.Sync(t.Context(), "alice", Meta{Name: "x"}); err != nil {
		t.Fatal(err)
	}
	want := []string{"GET /repos/alice/x", "POST /user/repos"}
	if got := log.get(); !slices.Equal(got, want) {
		t.Fatalf("requests: got %v want %v", got, want)
	}
}

func TestGithubSyncNoopWhenUpToDate(t *testing.T) {
	f, log := ghTestForge(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"name":"x","description":"d","private":true,"archived":false}`)
	}))
	desc := "d"
	if err := f.Sync(t.Context(), "alice", Meta{Name: "x", Desc: &desc, Vis: config.Private}); err != nil {
		t.Fatal(err)
	}
	if got := log.get(); len(got) != 1 {
		t.Fatalf("expected a single GET, got %v", got)
	}
}

func TestGithubSyncUnarchivesBeforeUpdate(t *testing.T) {
	var mu sync.Mutex
	var patches []map[string]any
	f, log := ghTestForge(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			fmt.Fprint(w, `{"name":"x","description":"old","private":true,"archived":true}`)
			return
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode patch body: %v", err)
		}
		mu.Lock()
		patches = append(patches, body)
		mu.Unlock()
		fmt.Fprint(w, `{"name":"x"}`)
	}))
	desc := "new"
	if err := f.Sync(t.Context(), "alice", Meta{Name: "x", Desc: &desc, Vis: config.Private}); err != nil {
		t.Fatal(err)
	}
	want := []string{"GET /repos/alice/x", "PATCH /repos/alice/x", "PATCH /repos/alice/x"}
	if got := log.get(); !slices.Equal(got, want) {
		t.Fatalf("requests: got %v want %v", got, want)
	}
	if len(patches) != 2 {
		t.Fatalf("patches: got %d, want 2", len(patches))
	}
	// the first patch must only unarchive, the second carries the update
	if v, ok := patches[0]["archived"].(bool); !ok || v {
		t.Fatalf("first patch should set archived=false, got %v", patches[0])
	}
	if _, ok := patches[0]["description"]; ok {
		t.Fatalf("first patch should not carry the update, got %v", patches[0])
	}
	if v, _ := patches[1]["description"].(string); v != "new" {
		t.Fatalf("second patch should update description, got %v", patches[1])
	}
}

func TestGithubCreateExistsMapping(t *testing.T) {
	nameTaken := `{"message":"Repository creation failed.","errors":[{"resource":"Repository","field":"name","code":"custom","message":"name already exists on this account"}]}`
	other := `{"message":"Validation Failed","errors":[{"resource":"Repository","field":"description","code":"invalid","message":"description is too long"}]}`

	for _, tt := range []struct {
		body       string
		wantExists bool
	}{
		{nameTaken, true},
		{other, false},
	} {
		f, _ := ghTestForge(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnprocessableEntity)
			fmt.Fprint(w, tt.body)
		}))
		err := f.create(t.Context(), Meta{Name: "x"})
		if err == nil {
			t.Fatal("expected error")
		}
		if got := errors.Is(err, ErrExists); got != tt.wantExists {
			t.Errorf("ErrExists = %v want %v for %q", got, tt.wantExists, tt.body)
		}
	}
}

func glTestForge(t *testing.T, h http.Handler) (*glForge, *requestLog) {
	t.Helper()
	log := &requestLog{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.add(r)
		h.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	c, err := gl.NewClient("t", gl.WithBaseURL(srv.URL), gl.WithHTTPClient(srv.Client()))
	if err != nil {
		t.Fatal(err)
	}
	return &glForge{c: c}, log
}

func TestGitlabSyncCreatesWhenMissing(t *testing.T) {
	f, log := glTestForge(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET":
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprint(w, `{"message":"404 Project Not Found"}`)
		case r.Method == "POST" && r.URL.Path == "/api/v4/projects":
			w.WriteHeader(http.StatusCreated)
			fmt.Fprint(w, `{"id":1}`)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	if err := f.Sync(t.Context(), "alice", Meta{Name: "x"}); err != nil {
		t.Fatal(err)
	}
	if got := log.get(); len(got) != 2 || got[1] != "POST /api/v4/projects" {
		t.Fatalf("requests: got %v", got)
	}
}

func TestGitlabSyncArchivesOnDrift(t *testing.T) {
	f, log := glTestForge(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			fmt.Fprint(w, `{"id":1,"description":"d","visibility":"private","archived":false}`)
			return
		}
		fmt.Fprint(w, `{"id":1}`)
	}))
	desc := "d"
	if err := f.Sync(t.Context(), "alice", Meta{Name: "x", Desc: &desc, Vis: config.Private, Archived: true}); err != nil {
		t.Fatal(err)
	}
	want := []string{"GET /api/v4/projects/alice/x", "POST /api/v4/projects/alice/x/archive"}
	if got := log.get(); !slices.Equal(got, want) {
		t.Fatalf("requests: got %v want %v", got, want)
	}
}

func cbTestForge(t *testing.T, h http.Handler) (*cbForge, *requestLog) {
	t.Helper()
	log := &requestLog{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.add(r)
		h.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	c, err := gitea.NewClient(srv.URL, gitea.SetToken("t"), gitea.SetGiteaVersion(""))
	if err != nil {
		t.Fatal(err)
	}
	return &cbForge{c: c}, log
}

func TestCodebergSyncUpdatesOnDrift(t *testing.T) {
	f, log := cbTestForge(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			fmt.Fprint(w, `{"name":"x","description":"old","private":true,"archived":false}`)
			return
		}
		fmt.Fprint(w, `{"name":"x"}`)
	}))
	desc := "new"
	if err := f.Sync(t.Context(), "alice", Meta{Name: "x", Desc: &desc, Vis: config.Private}); err != nil {
		t.Fatal(err)
	}
	want := []string{"GET /api/v1/repos/alice/x", "PATCH /api/v1/repos/alice/x"}
	if got := log.get(); !slices.Equal(got, want) {
		t.Fatalf("requests: got %v want %v", got, want)
	}
}

func TestCodebergSyncArchivesAfterCreate(t *testing.T) {
	f, log := cbTestForge(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET":
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprint(w, `{"message":"not found"}`)
		case r.Method == "POST" && r.URL.Path == "/api/v1/user/repos":
			w.WriteHeader(http.StatusCreated)
			fmt.Fprint(w, `{"name":"x"}`)
		default:
			fmt.Fprint(w, `{"name":"x"}`)
		}
	}))
	if err := f.Sync(t.Context(), "alice", Meta{Name: "x", Archived: true}); err != nil {
		t.Fatal(err)
	}
	want := []string{"GET /api/v1/repos/alice/x", "POST /api/v1/user/repos", "PATCH /api/v1/repos/alice/x"}
	if got := log.get(); !slices.Equal(got, want) {
		t.Fatalf("requests: got %v want %v", got, want)
	}
}

func TestCodebergSyncUnarchivesBeforeUpdate(t *testing.T) {
	var mu sync.Mutex
	var patches []map[string]any
	f, log := cbTestForge(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			fmt.Fprint(w, `{"name":"x","description":"old","private":true,"archived":true}`)
			return
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode patch body: %v", err)
		}
		mu.Lock()
		patches = append(patches, body)
		mu.Unlock()
		fmt.Fprint(w, `{"name":"x"}`)
	}))
	desc := "new"
	if err := f.Sync(t.Context(), "alice", Meta{Name: "x", Desc: &desc, Vis: config.Private}); err != nil {
		t.Fatal(err)
	}
	want := []string{"GET /api/v1/repos/alice/x", "PATCH /api/v1/repos/alice/x", "PATCH /api/v1/repos/alice/x"}
	if got := log.get(); !slices.Equal(got, want) {
		t.Fatalf("requests: got %v want %v", got, want)
	}
	if len(patches) != 2 {
		t.Fatalf("patches: got %d, want 2", len(patches))
	}
	if v, ok := patches[0]["archived"].(bool); !ok || v {
		t.Fatalf("first patch should set archived=false, got %v", patches[0])
	}
	if _, ok := patches[0]["description"]; ok {
		t.Fatalf("first patch should not carry the update, got %v", patches[0])
	}
	if v, _ := patches[1]["description"].(string); v != "new" {
		t.Fatalf("second patch should update description, got %v", patches[1])
	}
}

func srhtTestForge(t *testing.T, responses []string) (*srhtForge, *requestLog) {
	t.Helper()
	log := &requestLog{}
	var mu sync.Mutex
	i := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.add(r)
		mu.Lock()
		body := responses[min(i, len(responses)-1)]
		i++
		mu.Unlock()
		fmt.Fprint(w, body)
	}))
	t.Cleanup(srv.Close)
	return &srhtForge{c: graphql.NewClient(srv.URL+"/query", srv.Client())}, log
}

func TestSourcehutSyncCreatesWhenMissing(t *testing.T) {
	f, log := srhtTestForge(t, []string{
		`{"data":{"me":{"repository":{"id":0}}}}`,
		`{"data":{"createRepository":{"id":1}}}`,
	})
	if err := f.Sync(t.Context(), "~alice", Meta{Name: "x"}); err != nil {
		t.Fatal(err)
	}
	if got := log.get(); len(got) != 2 {
		t.Fatalf("expected query then create, got %v", got)
	}
}

func TestSourcehutSyncNoopWhenUpToDate(t *testing.T) {
	f, log := srhtTestForge(t, []string{
		`{"data":{"me":{"repository":{"id":1,"description":"d","visibility":"PRIVATE"}}}}`,
	})
	desc := "d"
	if err := f.Sync(t.Context(), "~alice", Meta{Name: "x", Desc: &desc, Vis: config.Private}); err != nil {
		t.Fatal(err)
	}
	if got := log.get(); len(got) != 1 {
		t.Fatalf("expected a single query, got %v", got)
	}
}

func TestDescOrEmpty(t *testing.T) {
	s := "hello"
	if got := descOrEmpty(&s); got != "hello" {
		t.Errorf("got %q, want %q", got, "hello")
	}
	if got := descOrEmpty(nil); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestSrhtIsExists(t *testing.T) {
	tests := []struct {
		msg  string
		want bool
	}{
		{"this name is already in use", true},
		{"repository already exists", true},
		{"Already Exists", true},
		{"invalid name", false},
		{"repository not found", false},
		{"permission denied", false},
		{"internal server error", false},
	}
	for _, tt := range tests {
		got := srhtIsExists(fmt.Errorf("%s", tt.msg))
		if got != tt.want {
			t.Errorf("srhtIsExists(%q) = %v, want %v", tt.msg, got, tt.want)
		}
	}
}
