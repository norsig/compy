package main

import (
	. "gopkg.in/check.v1"

	"bytes"
	gzipp "compress/gzip"
	gifp "image/gif"
	jpegp "image/jpeg"
	pngp "image/png"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/ahmetalpbalkan/go-httpbin"
	"github.com/barnacs/compy/proxy"
	tc "github.com/barnacs/compy/transcoder"
	"github.com/chai2010/webp"
	brotlidec "gopkg.in/kothar/brotli-go.v0/dec"
)

func Test(t *testing.T) {
	TestingT(t)
}

type CompyTest struct {
	client *http.Client
	server *httptest.Server
	proxy  *proxy.Proxy
}

var _ = Suite(&CompyTest{})

func (s *CompyTest) SetUpSuite(c *C) {
	s.server = httptest.NewServer(httpbin.GetMux())

	s.proxy = proxy.New()
	s.proxy.AddTranscoder("image/gif", &tc.Gif{})
	s.proxy.AddTranscoder("image/jpeg", tc.NewJpeg(50))
	s.proxy.AddTranscoder("image/png", &tc.Png{})
	s.proxy.AddTranscoder("text/html", &tc.Zip{&tc.Identity{}, *brotli, *gzip, true})
	go func() {
		err := s.proxy.Start(*host)
		if err != nil {
			c.Fatal(err)
		}
	}()

	proxyUrl := &url.URL{Scheme: "http", Host: "localhost" + *host}
	tr := &http.Transport{
		DisableCompression: true,
		Proxy:              http.ProxyURL(proxyUrl),
	}
	s.client = &http.Client{Transport: tr}
}

func (s *CompyTest) TearDownSuite(c *C) {
	s.server.Close()

	// TODO: Go 1.8 will provide http.Server.Shutdown for proxy.Proxy
}

func (s *CompyTest) TestHttpBin(c *C) {
	resp, err := s.client.Get(s.server.URL + "/status/200")
	c.Assert(err, IsNil)
	c.Assert(resp.StatusCode, Equals, 200)
}

func (s *CompyTest) TestNoGzip(c *C) {
	resp, err := http.Get(s.server.URL + "/html")
	c.Assert(err, IsNil)
	defer resp.Body.Close()
	c.Assert(resp.StatusCode, Equals, 200)
	c.Assert(resp.Header.Get("Content-Encoding"), Equals, "")

	_, err = gzipp.NewReader(resp.Body)
	c.Assert(err, NotNil)
}

func (s *CompyTest) TestGzip(c *C) {
	req, err := http.NewRequest("GET", s.server.URL+"/html", nil)
	c.Assert(err, IsNil)
	req.Header.Add("Accept-Encoding", "gzip")

	resp, err := s.client.Do(req)
	c.Assert(err, IsNil)
	defer resp.Body.Close()
	c.Assert(resp.StatusCode, Equals, 200)
	c.Assert(resp.Header.Get("Content-Encoding"), Equals, "gzip")

	gzr, err := gzipp.NewReader(resp.Body)
	c.Assert(err, IsNil)
	defer gzr.Close()
	_, err = ioutil.ReadAll(gzr)
	c.Assert(err, IsNil)
}

func (s *CompyTest) TestBrotli(c *C) {
	req, err := http.NewRequest("GET", s.server.URL+"/html", nil)
	c.Assert(err, IsNil)
	req.Header.Add("Accept-Encoding", "br, gzip")

	resp, err := s.client.Do(req)
	c.Assert(err, IsNil)
	defer resp.Body.Close()
	c.Assert(resp.StatusCode, Equals, 200)
	c.Assert(resp.Header.Get("Content-Encoding"), Equals, "br")

	brr := brotlidec.NewBrotliReader(resp.Body)
	defer brr.Close()
	_, err = ioutil.ReadAll(brr)
	c.Assert(err, IsNil)
}

func (s *CompyTest) TestGif(c *C) {
	resp, err := http.Get(s.server.URL + "/image/gif")
	c.Assert(err, IsNil)
	uncompressedLength, err := new(bytes.Buffer).ReadFrom(resp.Body)
	c.Assert(err, IsNil)
	resp.Body.Close()

	resp, err = s.client.Get(s.server.URL + "/image/gif")
	c.Assert(err, IsNil)
	defer resp.Body.Close()
	c.Assert(resp.StatusCode, Equals, 200)
	c.Assert(resp.Header.Get("Content-Type"), Equals, "image/gif")

	_, err = gifp.Decode(resp.Body)
	c.Assert(err, IsNil)
	compressedLength := resp.ContentLength
	c.Assert(uncompressedLength > compressedLength, Equals, true)
}

func (s *CompyTest) TestJpeg(c *C) {
	resp, err := s.client.Get(s.server.URL + "/image/jpeg")
	c.Assert(err, IsNil)
	defer resp.Body.Close()
	c.Assert(resp.StatusCode, Equals, 200)
	c.Assert(resp.Header.Get("Content-Type"), Equals, "image/jpeg")

	_, err = jpegp.Decode(resp.Body)
	c.Assert(err, IsNil)
}

func (s *CompyTest) TestJpegToWebP(c *C) {
	req, err := http.NewRequest("GET", s.server.URL+"/image/jpeg", nil)
	c.Assert(err, IsNil)
	req.Header.Add("Accept", "image/webp,image/jpeg")

	resp, err := s.client.Do(req)
	c.Assert(err, IsNil)
	defer resp.Body.Close()
	c.Assert(resp.StatusCode, Equals, 200)
	c.Assert(resp.Header.Get("Content-Type"), Equals, "image/webp")

	_, err = webp.Decode(resp.Body)
	c.Assert(err, IsNil)
}

func (s *CompyTest) TestPng(c *C) {
	resp, err := s.client.Get(s.server.URL + "/image/png")
	c.Assert(err, IsNil)
	defer resp.Body.Close()
	c.Assert(resp.StatusCode, Equals, 200)
	c.Assert(resp.Header.Get("Content-Type"), Equals, "image/png")

	_, err = pngp.Decode(resp.Body)
	c.Assert(err, IsNil)
}
func (s *CompyTest) TestPngToWebP(c *C) {
	req, err := http.NewRequest("GET", s.server.URL+"/image/png", nil)
	c.Assert(err, IsNil)
	req.Header.Add("Accept", "image/webp")

	resp, err := s.client.Do(req)
	c.Assert(err, IsNil)
	defer resp.Body.Close()
	c.Assert(resp.StatusCode, Equals, 200)
	c.Assert(resp.Header.Get("Content-Type"), Equals, "image/webp")

	_, err = webp.Decode(resp.Body)
	c.Assert(err, IsNil)
}
