package update

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func serverReturning(t *testing.T, body string, status int) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestCheck_NewerReleaseAvailable(t *testing.T) {
	srv := serverReturning(t, `{"tag_name":"v0.3.0","html_url":"https://example.com/releases/v0.3.0"}`, http.StatusOK)
	c := Checker{URL: srv.URL}
	info, err := c.Check(context.Background(), "0.2.0")
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !info.Available {
		t.Errorf("Available = false, want true (0.2.0 -> v0.3.0)")
	}
	if info.Latest != "v0.3.0" || info.URL != "https://example.com/releases/v0.3.0" {
		t.Errorf("Info = %+v", info)
	}
}

func TestCheck_AlreadyLatest(t *testing.T) {
	srv := serverReturning(t, `{"tag_name":"v0.2.0","html_url":"https://example.com/releases/v0.2.0"}`, http.StatusOK)
	c := Checker{URL: srv.URL}
	info, err := c.Check(context.Background(), "0.2.0")
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if info.Available {
		t.Errorf("Available = true, want false (already on latest)")
	}
}

func TestCheck_DevBuildIsAlwaysBehind(t *testing.T) {
	srv := serverReturning(t, `{"tag_name":"v0.1.0","html_url":"https://example.com/releases/v0.1.0"}`, http.StatusOK)
	c := Checker{URL: srv.URL}
	info, err := c.Check(context.Background(), "0.0.0-dev")
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !info.Available {
		t.Errorf("Available = false, want true (dev build is always behind a real release)")
	}
}

func TestCheck_NewerLocalVersion(t *testing.T) {
	// Not realistic in practice, but must not report an update backwards.
	srv := serverReturning(t, `{"tag_name":"v0.1.0","html_url":"https://example.com/releases/v0.1.0"}`, http.StatusOK)
	c := Checker{URL: srv.URL}
	info, err := c.Check(context.Background(), "0.9.0")
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if info.Available {
		t.Errorf("Available = true, want false (local 0.9.0 is newer than latest release 0.1.0)")
	}
}

func TestCheck_HTTPError(t *testing.T) {
	srv := serverReturning(t, `not found`, http.StatusNotFound)
	c := Checker{URL: srv.URL}
	if _, err := c.Check(context.Background(), "0.1.0"); err == nil {
		t.Fatal("expected an error on a non-200 response")
	}
}

func TestCheck_MalformedJSON(t *testing.T) {
	srv := serverReturning(t, `not json`, http.StatusOK)
	c := Checker{URL: srv.URL}
	if _, err := c.Check(context.Background(), "0.1.0"); err == nil {
		t.Fatal("expected an error on malformed JSON")
	}
}

func TestCheck_ContextTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
	}))
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	c := Checker{URL: srv.URL}
	if _, err := c.Check(ctx, "0.1.0"); err == nil {
		t.Fatal("expected a context-deadline error")
	}
}

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"0.1.0", "0.2.0", -1},
		{"0.2.0", "0.1.0", 1},
		{"1.0.0", "1.0.0", 0},
		{"0.0.0-dev", "0.1.0", -1},
		{"1.2.3", "1.2.10", -1},
		{"2.0.0", "1.9.9", 1},
	}
	for _, tc := range cases {
		if got := compareVersions(tc.a, tc.b); got != tc.want {
			t.Errorf("compareVersions(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}
