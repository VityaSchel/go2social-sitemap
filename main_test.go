package main

import (
	"bytes"
	"cmp"
	"encoding/xml"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"
	"testing/synctest"
	"time"
)

const (
	firstPage = `[{"id":"03","url":"https://gts.example/@a/statuses/03?x=1&y=<2>\"'","created_at":"2026-10-02T09:34:51.918Z","local_only":false,"account":{"id":"A","url":"https://gts.example/@a","created_at":"2020-01-01T00:00:00.000Z","indexable":true}},
		{"id":"02","url":"https://gts.example/@a/statuses/02","created_at":"2026-10-01T00:00:00.000Z","account":{"indexable":true}}]`
	hiddenPage = `[{"id":"00","url":"https://gts.example/@a/statuses/00","created_at":"2026-09-30T12:00:00.000Z","local_only":true,"account":{"indexable":true}},{"id":"0+1","url":"https://gts.example/@b/statuses/0+1","created_at":"2026-09-30T11:00:00.000Z","account":{"indexable":false}}]`
	secondPage = `[{"id":"01","url":"https://gts.example/@a/statuses/01","created_at":"2026-09-30T10:00Z","account":{"indexable":true}}]`
)

type fakeTimeline struct {
	status   int
	pages    map[string]string
	requests []string
}

func (f *fakeTimeline) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.requests = append(f.requests, r.Header.Get("Authorization")+" "+r.Method+" http://"+r.Host+r.RequestURI)
	w.WriteHeader(cmp.Or(len(f.requests)/10*http.StatusLoopDetected, f.status, http.StatusOK))
	io.WriteString(w, cmp.Or(f.pages[r.URL.Query().Get("max_id")], "[]"))
}

func sitemapOf(t *testing.T, timeline http.Handler) func() *httptest.ResponseRecorder {
	handler := sitemapHandler(httptest.NewTestServer(t, timeline).Client(), "http://gts.example", "secret")
	return func() *httptest.ResponseRecorder {
		response := httptest.NewRecorder()
		handler(response, httptest.NewRequest(http.MethodGet, "/sitemap.xml", nil))
		return response
	}
}

func expect(t *testing.T, what string, got, want any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s:\n got %v\nwant %v", what, got, want)
	}
}

func TestCrawlPaginatesEscapesAndSkipsUnindexable(t *testing.T) {
	timeline := &fakeTimeline{pages: map[string]string{"": firstPage, "02": hiddenPage, "0+1": secondPage}}
	response := sitemapOf(t, timeline)()
	expect(t, "status", response.Code, http.StatusOK)
	expect(t, "headers", response.Header(), http.Header{"Content-Type": {"application/xml"}})
	expect(t, "body", response.Body.String(), xml.Header+`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">`+
		`<url><loc>https://gts.example/@a/statuses/03?x=1&amp;y=&lt;2&gt;&#34;&#39;</loc><lastmod>2026-10-02T09:34:51.918Z</lastmod></url>`+
		`<url><loc>https://gts.example/@a/statuses/02</loc><lastmod>2026-10-01T00:00:00.000Z</lastmod></url>`+
		`<url><loc>https://gts.example/@a/statuses/01</loc><lastmod>2026-09-30T10:00Z</lastmod></url></urlset>`)
	expect(t, "requests", timeline.requests, []string{
		"Bearer secret GET http://gts.example/api/v1/timelines/public?local=true&limit=40",
		"Bearer secret GET http://gts.example/api/v1/timelines/public?local=true&limit=40&max_id=02",
		"Bearer secret GET http://gts.example/api/v1/timelines/public?local=true&limit=40&max_id=0%2B1",
		"Bearer secret GET http://gts.example/api/v1/timelines/public?local=true&limit=40&max_id=01",
	})
}

func TestCacheLastsOneHourAndNeverServesStale(t *testing.T) {
	log.SetOutput(io.Discard)
	synctest.Test(t, func(t *testing.T) {
		timeline := &fakeTimeline{pages: map[string]string{"": secondPage}}
		get := sitemapOf(t, timeline)
		fresh := get().Body.String()
		time.Sleep(time.Hour - time.Nanosecond)
		expect(t, "cached body", get().Body.String(), fresh)
		expect(t, "requests within the hour", len(timeline.requests), 2)
		time.Sleep(time.Nanosecond)
		timeline.status = http.StatusServiceUnavailable
		stale := get()
		expect(t, "status after failed rebuild", stale.Code, http.StatusInternalServerError)
		expect(t, "body after failed rebuild", stale.Body.Len(), 0)
		expect(t, "requests after expiry", len(timeline.requests), 3)
		time.Sleep(time.Nanosecond)
		timeline.status = http.StatusOK
		expect(t, "body after recovery", get().Body.String(), fresh)
		expect(t, "failure is not cached", len(timeline.requests), 5)
		get()
		expect(t, "recovery is cached", len(timeline.requests), 5)
	})
}

func TestCrawlFailures(t *testing.T) {
	var logged bytes.Buffer
	log.SetOutput(&logged)
	for name, broken := range map[string]fakeTimeline{
		"bad token":        {status: http.StatusUnauthorized, pages: map[string]string{"": `{"error":"Unauthorized"}`}},
		"non-200":          {status: http.StatusAccepted, pages: map[string]string{"": secondPage}},
		"null":             {pages: map[string]string{"": `null`}},
		"trailing garbage": {pages: map[string]string{"": `[] []`}},
		"missing id":       {pages: map[string]string{"": `[{"url":"https://gts.example/2","created_at":"2026-10-02T09:34:51.918Z"},{"id":"01","url":"https://gts.example/1","created_at":"2026-10-02T09:34:51.918Z"}]`}},
		"null url":         {pages: map[string]string{"": `[{"id":"03","url":"https://gts.example/3","created_at":"2026-10-02T09:34:51.918Z"},{"id":"02","url":null,"created_at":"2026-10-02T09:34:51.918Z"},{"id":"01","url":"https://gts.example/1","created_at":"2026-10-02T09:34:51.918Z"}]`}},
		"empty created_at": {pages: map[string]string{"": `[{"id":"01","url":"https://gts.example/1","created_at":""}]`}},
		"numeric id":       {pages: map[string]string{"": `[{"id":1,"url":"https://gts.example/1","created_at":"2026-10-02T09:34:51.918Z"}]`}},
		"bad second page":  {pages: map[string]string{"": firstPage, "02": `[{"id":"","url":"https://gts.example/1","created_at":"2026-10-02T09:34:51.918Z"}]`}},
	} {
		t.Run(name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				logged.Reset()
				get := sitemapOf(t, &broken)
				response := get()
				expect(t, "status", response.Code, http.StatusInternalServerError)
				expect(t, "body", response.Body.String(), "")
				expect(t, "requests", len(broken.requests), len(broken.pages))
				expect(t, "logged once", bytes.Count(logged.Bytes(), []byte("\n")), 1)
				expect(t, "token stays out of the log", bytes.Contains(logged.Bytes(), []byte("secret")), false)
				time.Sleep(time.Nanosecond)
				broken.status, broken.pages = http.StatusOK, nil
				expect(t, "body after recovery", get().Body.String(), xml.Header+`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"></urlset>`)
			})
		})
	}
}

func TestHungUpstreamTimesOut(t *testing.T) {
	log.SetOutput(io.Discard)
	synctest.Test(t, func(t *testing.T) {
		get := sitemapOf(t, http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			<-r.Context().Done()
		}))
		start := time.Now()
		expect(t, "status", get().Code, http.StatusInternalServerError)
		expect(t, "waited", time.Since(start), 5*time.Minute)
	})
}

func TestConcurrentRequestsShareOneCrawl(t *testing.T) {
	log.SetOutput(io.Discard)
	for _, status := range []int{http.StatusOK, http.StatusServiceUnavailable} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				timeline := &fakeTimeline{status: status}
				get := sitemapOf(t, timeline)
				var requests sync.WaitGroup
				for range 20 {
					requests.Go(func() { get() })
				}
				requests.Wait()
				expect(t, "crawls", len(timeline.requests), 1)
			})
		})
	}
}
