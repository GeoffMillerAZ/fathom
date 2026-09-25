package gallery

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Unlike the public HTTP journey tests, these tests seed the private store to
// exercise fixed expiry without sleeps or a production clock-injection API.
func shellDemoStoreTestBrowser(store *shellDemoStore, id string) *shellDemoTestBrowser {
	browser := shellDemoTestNewBrowser(http.HandlerFunc(store.serveHTTP))
	if id != "" {
		browser.cookies[shellDemoCookieName] = &http.Cookie{Name: shellDemoCookieName, Value: id}
	}
	return browser
}

func TestShellDemoStoreExpiredSession(t *testing.T) {
	for _, method := range []string{http.MethodPost, http.MethodGet} {
		t.Run(method, func(t *testing.T) {
			id := strings.Repeat("a", 64)
			old := &shellDemoSession{State: defaultShellDemoState(9), CSRF: "expired-form-token", Expires: time.Now().Add(-time.Hour)}
			old.State.DisplayName = "Expired synthetic values"
			old.State.TimeZone, old.State.StartPage = "UTC", "activity"
			old.State.Digest, old.State.SignedIn = false, false
			old.State.Access, old.State.Availability = "viewer", "error"
			store := &shellDemoStore{sessions: map[string]*shellDemoSession{id: old}}
			browser := shellDemoStoreTestBrowser(store, id)
			if method == http.MethodPost {
				form := url.Values{"csrf": {old.CSRF}, "action": {"save-profile"}, "return_to": {"profile"}, "version": {"9"}, "display_name": {"Must not save"}}
				response := browser.post(t, form, http.StatusForbidden)
				if len(response.Result().Cookies()) != 0 || len(store.sessions) != 0 {
					t.Fatal("expired POST must delete the expired session without issuing or creating a replacement")
				}
				if old.State.DisplayName != "Expired synthetic values" || old.State.Version != 9 {
					t.Fatal("expired POST changed the discarded saved state")
				}
				return
			}

			response := browser.get(t, "/shell/profile", http.StatusOK)
			cookie := browser.cookies[shellDemoCookieName]
			if cookie == nil || cookie.Value == id || cookie.Value == "" || len(response.Result().Cookies()) != 1 {
				t.Fatal("expired GET must issue a fresh session identity")
			}
			fresh := store.sessions[cookie.Value]
			if fresh == nil || len(store.sessions) != 1 || store.sessions[id] != nil {
				t.Fatal("expired GET retained the old identity or failed to create its replacement")
			}
			if !reflect.DeepEqual(fresh.State, defaultShellDemoState(1)) || fresh.CSRF == old.CSRF || fresh.CSRF == "" {
				t.Fatal("expired GET resurrected old state or reused its form token")
			}
			form := shellDemoTestForm(t, response.Body.String(), "save-profile")
			if form.Get("display_name") != "Alex Morgan" || form.Get("version") != "1" || form.Get("csrf") != fresh.CSRF {
				t.Fatal("replacement page does not render the new default session")
			}
		})
	}
}

func TestShellDemoStoreExpiryReleasesOnlyExpiredCapacity(t *testing.T) {
	store := &shellDemoStore{sessions: make(map[string]*shellDemoSession)}
	liveBefore := make(map[string]shellDemoSession)
	expires := time.Now().Add(time.Hour)
	expiredID := ""
	for index := 0; index < shellDemoMaxSessions; index++ {
		id := fmt.Sprintf("%064x", index+1)
		state := defaultShellDemoState(uint64(index + 1))
		state.DisplayName = fmt.Sprintf("Synthetic survivor %d", index)
		session := &shellDemoSession{State: state, CSRF: "token-" + id, Expires: expires}
		store.sessions[id] = session
		if index == 0 {
			expiredID = id
			session.Expires = time.Now().Add(-time.Hour)
		} else {
			snapshot := *session
			snapshot.State = copyShellDemoState(session.State)
			liveBefore[id] = snapshot
		}
	}
	browser := shellDemoStoreTestBrowser(store, "")
	browser.get(t, "/shell/about", http.StatusOK)
	if len(store.sessions) != shellDemoMaxSessions || store.sessions[expiredID] != nil {
		t.Fatal("expired slot was not reclaimed before checking capacity")
	}
	if cookie := browser.cookies[shellDemoCookieName]; cookie == nil || cookie.Value == expiredID || store.sessions[cookie.Value] == nil {
		t.Fatal("reclaimed slot did not receive a fresh session identity")
	}
	for id, before := range liveBefore {
		after := store.sessions[id]
		if after == nil || !reflect.DeepEqual(*after, before) {
			t.Fatalf("reclaiming an expired slot changed a live session %s", id)
		}
	}
	overflow := shellDemoStoreTestBrowser(store, "")
	response := overflow.get(t, "/shell/about", http.StatusServiceUnavailable)
	if len(response.Result().Cookies()) != 0 || len(store.sessions) != shellDemoMaxSessions {
		t.Fatal("reclaiming one expired slot incorrectly made room for two new sessions")
	}
	liveID := fmt.Sprintf("%064x", 2)
	live := shellDemoStoreTestBrowser(store, liveID)
	form := shellDemoTestForm(t, live.get(t, "/shell/profile", http.StatusOK).Body.String(), "save-profile")
	if form.Get("display_name") != liveBefore[liveID].State.DisplayName || form.Get("version") != "2" {
		t.Fatal("live browser could not recover its exact saved state after capacity reclamation")
	}
}

func TestShellDemoStoreReadDoesNotExtendFixedExpiry(t *testing.T) {
	id := strings.Repeat("b", 64)
	expires := time.Now().Add(17 * time.Minute)
	state := defaultShellDemoState(7)
	state.DisplayName = "Fixed lifetime fixture"
	session := &shellDemoSession{State: copyShellDemoState(state), CSRF: "fixed-lifetime-token", Expires: expires}
	store := &shellDemoStore{sessions: map[string]*shellDemoSession{id: session}}
	browser := shellDemoStoreTestBrowser(store, id)
	for _, page := range []string{"/shell/profile", "/shell/help", "/shell/settings"} {
		response := browser.get(t, page, http.StatusOK)
		if !session.Expires.Equal(expires) || len(response.Result().Cookies()) != 0 || store.sessions[id] != session {
			t.Fatal("reading a live session refreshed its fixed expiry, cookie, or identity")
		}
		if !reflect.DeepEqual(session.State, state) || session.CSRF != "fixed-lifetime-token" {
			t.Fatal("reading a live session changed saved values or its form token")
		}
	}
}

func TestShellDemoStoreConcurrentSameVersionSave(t *testing.T) {
	store := &shellDemoStore{sessions: make(map[string]*shellDemoSession)}
	browser := shellDemoStoreTestBrowser(store, "")
	page := browser.get(t, "/shell/profile", http.StatusOK)
	initialForm := shellDemoTestForm(t, page.Body.String(), "save-profile")
	id := browser.cookies[shellDemoCookieName].Value
	before := copyShellDemoState(store.sessions[id].State)
	type result struct {
		name     string
		response *httptest.ResponseRecorder
	}
	start := make(chan struct{})
	ready := make(chan struct{}, 2)
	results := make(chan result, 2)
	for _, name := range []string{"Concurrent synthetic Avery", "Concurrent synthetic Casey"} {
		// Separate browser maps prevent the test harness itself from sharing
		// mutable cookie state across the simultaneous HTTP requests.
		worker := shellDemoStoreTestBrowser(store, id)
		form := url.Values{"csrf": {initialForm.Get("csrf")}, "action": {"save-profile"}, "return_to": {"profile"}, "version": {initialForm.Get("version")}, "display_name": {name}}
		go func() {
			ready <- struct{}{}
			<-start
			results <- result{name: name, response: worker.request(http.MethodPost, "/shell/action", form.Encode(), nil)}
		}()
	}
	<-ready
	<-ready
	close(start)
	first, second := <-results, <-results
	if first.response.Code == http.StatusConflict {
		first, second = second, first
	}
	shellDemoTestStatus(t, first.response, http.StatusSeeOther)
	shellDemoTestStatus(t, second.response, http.StatusConflict)
	shellDemoTestRedirect(t, first.response, "/shell/profile")
	store.mu.Lock()
	after := copyShellDemoState(store.sessions[id].State)
	store.mu.Unlock()
	if after.Version != before.Version+1 || after.DisplayName != first.name || len(after.Activity) != len(before.Activity)+1 {
		t.Fatal("same-version concurrent requests did not produce exactly one saved mutation")
	}
	want := copyShellDemoState(before)
	want.Version++
	want.DisplayName = first.name
	want.Activity = append(want.Activity, "Profile saved in this synthetic session only.")
	if !reflect.DeepEqual(after, want) {
		t.Fatal("the winning profile save changed unrelated saved fields")
	}
	conflictForm := shellDemoTestForm(t, second.response.Body.String(), "save-profile")
	if conflictForm.Get("display_name") != second.name || conflictForm.Get("version") != strconv.FormatUint(after.Version, 10) || !strings.Contains(second.response.Body.String(), "Apply reviewed draft") {
		t.Fatal("losing concurrent request did not retain its draft for explicit current-version review")
	}
}
