package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"strings"
	"time"
)

const RUSTUP_BASE_URL = "https://static.rust-lang.org"

var client *http.Client
var fileCache *cache

type cache struct {
	Path string
}

func (c *cache) hash(s string) string {
	sha := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sha[:])
}

func (c *cache) Get(s string) (io.Reader, error) {
	fmt.Printf("Fetching %s\n", s)
	h := c.hash(s)
	return os.Open(path.Join(c.Path, h))
}

func (c *cache) Put(s string, r io.Reader) error {
	fmt.Printf("Caching %s\n", s)
	h := c.hash(s)
	p := path.Join(c.Path, h)
	if _, err := os.Stat(p); os.IsNotExist(err) {
		// file doesnt exist yet
		f, err := os.OpenFile(p, os.O_CREATE|os.O_RDWR, 0664)
		if err != nil {
			return err
		}
		io.Copy(f, r)
	}
	return nil
}

func main() {
	port := "8080"
	if os.Getenv("PORT") != "" {
		port = os.Getenv("PORT")
	}

	server := &http.Server{
		Addr:         ":" + port,
		WriteTimeout: 30 * time.Second,
		ReadTimeout:  30 * time.Second,
		Handler:      http.HandlerFunc(handler),
	}

	client = &http.Client{
		Timeout: time.Second * 30,
	}

	fileCache = &cache{
		Path: os.Getenv("CACHE_PATH"),
	}

	err := server.ListenAndServe()
	if err != nil {
		log.Fatal("Could not start http server")
	}
}

func handleManifest(w http.ResponseWriter, r *http.Request) {
	q := r.URL.RequestURI()
	// we have a sha url, fetch the base manifest, rewrite, and compute sha on the fly
	unSha := strings.ReplaceAll(q, ".sha256", "")
	remoteUrl := RUSTUP_BASE_URL + unSha

	fmt.Printf("Getting %s\n", remoteUrl)
	response, err := client.Get(remoteUrl)
	if err != nil {
		log.Printf("Could not get %s\n", remoteUrl)
		log.Printf("%+v\n", err)
	}

	body, err := ioutil.ReadAll(response.Body)
	defer response.Body.Close()
	if err != nil {
		log.Fatalf("Could not read body %s\n", remoteUrl)
	}

	rewrittenBody := bytes.ReplaceAll(body, []byte(RUSTUP_BASE_URL), []byte(os.Getenv("HOST")))
	sha := sha256.Sum256(rewrittenBody)
	fmt.Fprintf(w, "%s  %s\n", hex.EncodeToString(sha[:]), path.Base(remoteUrl))
	// Cache new manifest
	fileCache.Put(unSha, bytes.NewReader(rewrittenBody))
}

func handler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.RequestURI()
	if strings.Contains(q, "/dist/channel") && strings.Contains(q, ".sha256") {
		handleManifest(w, r)
		return
	}

	fmt.Printf("Getting %s\n", q)

	if f, err := fileCache.Get(q); err != nil {
		// file not in cache, or something else bad happened.
		remoteUrl := RUSTUP_BASE_URL + q
		fmt.Printf("Getting %s\n", remoteUrl)
		response, err := client.Get(remoteUrl)
		if err != nil {
			log.Printf("Could not get %s\n", remoteUrl)
			log.Printf("%+v\n", err)
		}

		body, err := ioutil.ReadAll(response.Body)
		defer response.Body.Close()
		if err != nil {
			log.Fatalf("Could not read body %s\n", remoteUrl)
		}

		fileCache.Put(q, bytes.NewReader(body))
		w.Write(body)
	} else {
		fmt.Printf("Cache hit for %s\n", q)
		out, err := ioutil.ReadAll(f)
		if err != nil {
			log.Fatalf("Could not read from cache%s\n", q)
		}
		w.Write(out)
	}
}
