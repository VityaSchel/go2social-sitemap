package main

import (
	"cmp"
	"context"
	"encoding/json/v2"
	"encoding/xml"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"
	"sync"
	"time"
)

type status struct {
	ID        string `json:"id" xml:"-"`
	URL       string `json:"url" xml:"loc"`
	CreatedAt string `json:"created_at" xml:"lastmod"`
	LocalOnly bool   `json:"local_only" xml:"-"`
	Account   struct {
		Indexable bool `json:"indexable"`
	} `json:"account" xml:"-"`
}

func crawl(client *http.Client, instance, token string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	var sitemap struct {
		XMLName  struct{} `xml:"http://www.sitemaps.org/schemas/sitemap/0.9 urlset"`
		Statuses []status `xml:"url"`
	}
	incomplete := func(s status) bool { return s.ID == "" || s.URL == "" || s.CreatedAt == "" }
	for cursor := ""; ; {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, instance+"/api/v1/timelines/public?local=true&limit=40"+cursor, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		res, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		var page []status
		err = json.UnmarshalRead(res.Body, &page)
		res.Body.Close()
		if err != nil || res.StatusCode != http.StatusOK || page == nil || slices.ContainsFunc(page, incomplete) {
			return nil, fmt.Errorf("%s: want 200 with a JSON array of complete statuses, got %q (%v)", req.URL, res.Status, err)
		}
		if len(page) == 0 {
			body, err := xml.Marshal(sitemap)
			return append([]byte(xml.Header), body...), err
		}
		cursor = "&max_id=" + url.QueryEscape(page[len(page)-1].ID)
		sitemap.Statuses = append(sitemap.Statuses, slices.DeleteFunc(page, func(s status) bool { return s.LocalOnly || !s.Account.Indexable })...)
	}
}

func sitemapHandler(client *http.Client, instance, token string) http.HandlerFunc {
	var mu sync.Mutex
	var sitemap []byte
	var err error
	var crawledAt time.Time
	return func(w http.ResponseWriter, _ *http.Request) {
		arrived := time.Now()
		mu.Lock()
		if crawledAt.Before(arrived) && (err != nil || time.Since(crawledAt) >= time.Hour) {
			if sitemap, err = crawl(client, instance, token); err != nil {
				log.Print(err)
			}
			crawledAt = time.Now()
		}
		body, failed := sitemap, err != nil
		mu.Unlock()
		if failed {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/xml")
		w.Write(body)
	}
}

func main() {
	instance, token := strings.TrimRight(os.Getenv("GOTOSOCIAL_URL"), "/"), os.Getenv("TOKEN")
	if instance == "" || token == "" {
		log.Fatal("GOTOSOCIAL_URL and TOKEN must be set")
	}
	listener, err := net.Listen("tcp", net.JoinHostPort(cmp.Or(os.Getenv("HOST"), "127.0.0.1"), cmp.Or(os.Getenv("PORT"), "3000")))
	if err != nil {
		log.Fatal(err)
	}
	http.Handle("GET /sitemap.xml", sitemapHandler(&http.Client{Timeout: 30 * time.Second}, instance, token))
	fmt.Printf("Server is running on http://%s\n", listener.Addr())
	log.Fatal((&http.Server{ReadTimeout: 10 * time.Second}).Serve(listener))
}
