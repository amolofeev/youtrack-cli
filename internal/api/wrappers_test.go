package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestListQueryOmitsZeroParams(t *testing.T) {
	if q := listQuery("", 0, 0); len(q) != 0 {
		t.Errorf("listQuery(\"\",0,0) = %v, want empty", q)
	}
	q := listQuery(FieldsIssueList, 30, 5)
	if q.Get("fields") != FieldsIssueList {
		t.Errorf("fields = %q, want %q", q.Get("fields"), FieldsIssueList)
	}
	if q.Get("$top") != "30" || q.Get("$skip") != "5" {
		t.Errorf("$top/$skip = %q/%q, want 30/5", q.Get("$top"), q.Get("$skip"))
	}
}

type requestCapture struct {
	method, path, escapedPath, rawQuery, auth string
}

func captureRequest(t *testing.T, body string) (*httptest.Server, *requestCapture) {
	t.Helper()
	c := &requestCapture{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c.method = r.Method
		c.path = r.URL.Path
		c.escapedPath = r.URL.EscapedPath()
		c.rawQuery = r.URL.RawQuery
		c.auth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(ts.Close)
	return ts, c
}

func TestMe(t *testing.T) {
	ts, cap := captureRequest(t, `{"$type":"User","id":"1-1","login":"alex","fullName":"Alex Amolofeev","email":"alex@example.com","guest":true}`)
	defer ts.Close()

	c, err := newTestClient(ts.URL)
	if err != nil {
		t.Fatalf("newTestClient error: %v", err)
	}
	u, err := c.Me(context.Background(), FieldsAuthStatus)
	if err != nil {
		t.Fatalf("Me() error: %v", err)
	}
	if cap.method != http.MethodGet || cap.path != "/users/me" {
		t.Errorf("request = %s %s, want GET /users/me", cap.method, cap.path)
	}
	if cap.rawQuery != "fields=id,login,fullName,email,guest" {
		t.Errorf("raw query = %q, want fields=%s", cap.rawQuery, FieldsAuthStatus)
	}
	if u.Type != "User" || u.ID != "1-1" || u.Login != "alex" || u.FullName != "Alex Amolofeev" || u.Email != "alex@example.com" {
		t.Errorf("Me() = %+v, want User alex", u)
	}
	if u.Guest == nil || !*u.Guest {
		t.Errorf("Guest = %v, want true", u.Guest)
	}
}

func TestMeEmptyFields(t *testing.T) {
	ts, cap := captureRequest(t, `{"id":"1-1","login":"alex"}`)
	defer ts.Close()

	c, err := newTestClient(ts.URL)
	if err != nil {
		t.Fatalf("newTestClient error: %v", err)
	}
	if _, err := c.Me(context.Background(), ""); err != nil {
		t.Fatalf("Me() error: %v", err)
	}
	if cap.rawQuery != "" {
		t.Errorf("raw query = %q, want empty", cap.rawQuery)
	}
}

func TestListIssues(t *testing.T) {
	ts, cap := captureRequest(t, `[{"$type":"Issue","id":"2-1","idReadable":"PRJ-1","summary":"Fix login","created":1700000000000,"updated":1700000000001,"resolved":null,"project":{"$type":"Project","id":"3-1","shortName":"PRJ"},"reporter":{"$type":"User","id":"1-1","login":"alex","fullName":"Alex"},"customFields":[{"$type":"EnumIssueCustomField","id":"4-1","name":"Priority","value":{"$type":"EnumBundleElement","id":"5-1","name":"Major"}}]}]`)
	defer ts.Close()

	c, err := newTestClient(ts.URL)
	if err != nil {
		t.Fatalf("newTestClient error: %v", err)
	}
	issues, err := c.ListIssues(context.Background(), "has: open", FieldsIssueList, 30, 5)
	if err != nil {
		t.Fatalf("ListIssues() error: %v", err)
	}
	if cap.method != http.MethodGet || cap.path != "/issues" {
		t.Errorf("request = %s %s, want GET /issues", cap.method, cap.path)
	}
	q, err := url.ParseQuery(cap.rawQuery)
	if err != nil {
		t.Fatalf("ParseQuery(%q) error: %v", cap.rawQuery, err)
	}
	if q.Get("query") != "has: open" || q.Get("$top") != "30" || q.Get("$skip") != "5" || q.Get("fields") != FieldsIssueList {
		t.Errorf("query params = %v, want query/top/skip/fields set", q)
	}
	if len(issues) != 1 {
		t.Fatalf("len(issues) = %d, want 1", len(issues))
	}
	it := issues[0]
	if it.Type != "Issue" || it.IDReadable != "PRJ-1" || it.Summary != "Fix login" {
		t.Errorf("issue = %+v, want $type Issue PRJ-1", it)
	}
	if it.Project == nil || it.Project.ShortName != "PRJ" {
		t.Errorf("project = %+v, want shortName PRJ", it.Project)
	}
	if it.Reporter == nil || it.Reporter.Login != "alex" {
		t.Errorf("reporter = %+v, want alex", it.Reporter)
	}
	if len(it.CustomFields) != 1 {
		t.Fatalf("customFields = %d, want 1", len(it.CustomFields))
	}
	cf := it.CustomFields[0]
	if cf.Type != "EnumIssueCustomField" || cf.Name != "Priority" {
		t.Errorf("customField = %+v, want EnumIssueCustomField Priority", cf)
	}
	v, ok := cf.ValueObject()
	if !ok || v.Type != "EnumBundleElement" || v.Name != "Major" {
		t.Errorf("ValueObject = (%+v,%v), want EnumBundleElement Major", v, ok)
	}
}

func TestIssue(t *testing.T) {
	ts, cap := captureRequest(t, `{"$type":"Issue","id":"2-1","idReadable":"PRJ-42","summary":"S","description":"D","tags":[{"$type":"Tag","id":"7-1","name":"backend"}],"commentsCount":3}`)
	defer ts.Close()

	c, err := newTestClient(ts.URL)
	if err != nil {
		t.Fatalf("newTestClient error: %v", err)
	}
	it, err := c.Issue(context.Background(), "PRJ-42", FieldsIssueView)
	if err != nil {
		t.Fatalf("Issue() error: %v", err)
	}
	if cap.path != "/issues/PRJ-42" {
		t.Errorf("path = %q, want /issues/PRJ-42", cap.path)
	}
	if cap.rawQuery != "fields="+FieldsIssueView {
		t.Errorf("raw query = %q, want fields=%s", cap.rawQuery, FieldsIssueView)
	}
	if it.IDReadable != "PRJ-42" || it.Description != "D" || it.CommentsCount == nil || *it.CommentsCount != 3 {
		t.Errorf("issue = %+v, want PRJ-42 with description and 3 comments", it)
	}
	if len(it.Tags) != 1 || it.Tags[0].Name != "backend" {
		t.Errorf("tags = %+v, want [backend]", it.Tags)
	}
}

func TestIssueEscapesPath(t *testing.T) {
	ts, cap := captureRequest(t, `{}`)
	defer ts.Close()

	c, err := newTestClient(ts.URL)
	if err != nil {
		t.Fatalf("newTestClient error: %v", err)
	}
	if _, err := c.Issue(context.Background(), "PRJ 42", FieldsIssueView); err != nil {
		t.Fatalf("Issue() error: %v", err)
	}
	if cap.escapedPath != "/issues/PRJ%2042" {
		t.Errorf("escaped path = %q, want /issues/PRJ%%2042", cap.escapedPath)
	}
}

func TestIssueComments(t *testing.T) {
	ts, cap := captureRequest(t, `[{"$type":"IssueComment","id":"6-1","text":"hello","created":1700000000000,"author":{"$type":"User","id":"1-1","login":"alex","fullName":"Alex"}}]`)
	defer ts.Close()

	c, err := newTestClient(ts.URL)
	if err != nil {
		t.Fatalf("newTestClient error: %v", err)
	}
	comments, err := c.IssueComments(context.Background(), "PRJ-42", FieldsIssueComments, 30, 0)
	if err != nil {
		t.Fatalf("IssueComments() error: %v", err)
	}
	if cap.path != "/issues/PRJ-42/comments" {
		t.Errorf("path = %q, want /issues/PRJ-42/comments", cap.path)
	}
	q, err := url.ParseQuery(cap.rawQuery)
	if err != nil {
		t.Fatalf("ParseQuery(%q) error: %v", cap.rawQuery, err)
	}
	if q.Get("$top") != "30" || q.Get("$skip") != "" || q.Get("fields") != FieldsIssueComments {
		t.Errorf("query params = %v, want top=30, no skip", q)
	}
	if len(comments) != 1 {
		t.Fatalf("len(comments) = %d, want 1", len(comments))
	}
	cm := comments[0]
	if cm.Type != "IssueComment" || cm.Text != "hello" || cm.Author == nil || cm.Author.Login != "alex" {
		t.Errorf("comment = %+v, want IssueComment hello by alex", cm)
	}
}

func TestListProjects(t *testing.T) {
	ts, cap := captureRequest(t, `[{"$type":"Project","id":"3-1","name":"Demo","shortName":"DEMO","archived":false,"leader":{"$type":"User","id":"1-1","login":"alex","fullName":"Alex"}}]`)
	defer ts.Close()

	c, err := newTestClient(ts.URL)
	if err != nil {
		t.Fatalf("newTestClient error: %v", err)
	}
	projects, err := c.ListProjects(context.Background(), FieldsProjectList, 10, 0)
	if err != nil {
		t.Fatalf("ListProjects() error: %v", err)
	}
	if cap.path != "/admin/projects" {
		t.Errorf("path = %q, want /admin/projects", cap.path)
	}
	q, err := url.ParseQuery(cap.rawQuery)
	if err != nil {
		t.Fatalf("ParseQuery(%q) error: %v", cap.rawQuery, err)
	}
	if q.Get("$top") != "10" || q.Get("fields") != FieldsProjectList {
		t.Errorf("query params = %v, want top=10 and fields", q)
	}
	if len(projects) != 1 {
		t.Fatalf("len(projects) = %d, want 1", len(projects))
	}
	p := projects[0]
	if p.Type != "Project" || p.ShortName != "DEMO" || p.Leader == nil || p.Leader.Login != "alex" {
		t.Errorf("project = %+v, want DEMO led by alex", p)
	}
	if p.Archived == nil || *p.Archived {
		t.Errorf("Archived = %v, want false", p.Archived)
	}
}

func TestListTags(t *testing.T) {
	ts, cap := captureRequest(t, `[{"$type":"Tag","id":"7-1","name":"backend","untagOnResolve":true}]`)
	defer ts.Close()

	c, err := newTestClient(ts.URL)
	if err != nil {
		t.Fatalf("newTestClient error: %v", err)
	}
	tags, err := c.ListTags(context.Background(), "back", FieldsTagList, 0, 0)
	if err != nil {
		t.Fatalf("ListTags() error: %v", err)
	}
	if cap.path != "/tags" {
		t.Errorf("path = %q, want /tags", cap.path)
	}
	q, err := url.ParseQuery(cap.rawQuery)
	if err != nil {
		t.Fatalf("ParseQuery(%q) error: %v", cap.rawQuery, err)
	}
	if q.Get("query") != "back" || q.Get("fields") != FieldsTagList {
		t.Errorf("query params = %v, want query and fields", q)
	}
	if len(tags) != 1 {
		t.Fatalf("len(tags) = %d, want 1", len(tags))
	}
	tg := tags[0]
	if tg.Type != "Tag" || tg.Name != "backend" || tg.UntagOnResolve == nil || !*tg.UntagOnResolve {
		t.Errorf("tag = %+v, want backend with untagOnResolve", tg)
	}
}

func TestSearch(t *testing.T) {
	ts, cap := captureRequest(t, `[{"$type":"Issue","id":"2-1","idReadable":"PRJ-1","summary":"S"}]`)
	defer ts.Close()

	c, err := newTestClient(ts.URL)
	if err != nil {
		t.Fatalf("newTestClient error: %v", err)
	}
	issues, err := c.Search(context.Background(), "has: open", FieldsIssueList, 30, 5)
	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}
	if cap.path != "/issues" {
		t.Errorf("path = %q, want /issues", cap.path)
	}
	q, err := url.ParseQuery(cap.rawQuery)
	if err != nil {
		t.Fatalf("ParseQuery(%q) error: %v", cap.rawQuery, err)
	}
	if q.Get("query") != "has: open" {
		t.Errorf("query param = %q, want has: open", q.Get("query"))
	}
	if len(issues) != 1 || issues[0].IDReadable != "PRJ-1" {
		t.Errorf("issues = %+v, want [PRJ-1]", issues)
	}
}

func TestValueObject(t *testing.T) {
	cases := []struct {
		name     string
		value    string
		wantOK   bool
		wantType string
		wantName string
	}{
		{"object", `{"$type":"EnumBundleElement","id":"5-1","name":"Major"}`, true, "EnumBundleElement", "Major"},
		{"null", `null`, false, "", ""},
		{"array", `[1,2]`, false, "", ""},
		{"empty", ``, false, "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := &IssueCustomField{Value: json.RawMessage(tc.value)}
			v, ok := f.ValueObject()
			if ok != tc.wantOK || v.Type != tc.wantType || v.Name != tc.wantName {
				t.Errorf("ValueObject() = (%+v,%v), want (%v,%v,%v)", v, ok, tc.wantOK, tc.wantType, tc.wantName)
			}
		})
	}
}

func TestClientTimeout(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond)
	}))
	defer ts.Close()

	c, err := newTestClient(ts.URL, WithTimeout(50*time.Millisecond), WithMaxRetries(1))
	if err != nil {
		t.Fatalf("newTestClient error: %v", err)
	}
	err = c.Get(context.Background(), "/users/me", nil, &map[string]any{})
	var ae *Error
	if !errors.As(err, &ae) || ae.Type != ErrorNetwork {
		t.Fatalf("error = %v, want *api.Error with Type ErrorNetwork", err)
	}
}
