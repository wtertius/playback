package test

import (
	"context"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/wtertius/playback"
)

type Test struct {
	title string
	f     func(t *testing.T, p *playback.Playback)
}

func TestCassete(t *testing.T) {
	t.Run("playback passing through context", func(t *testing.T) {
		p := &playback.Playback{}

		ctx := context.Background()
		ctx = playback.NewContext(ctx, p)

		assert.Equal(t, p, playback.FromContext(ctx))
	})

	t.Run("playback.Result: record and playback", func(t *testing.T) {
		// init random
		rand.Seed(time.Now().Unix())
		randRange := 100

		runTests(t, []Test{
			{
				title: "replaying works",
				f: func(t *testing.T, p *playback.Playback) {
					key := "rand.Intn"

					p.SetMode(playback.ModeRecord)
					numberExpected := p.Result(key, rand.Intn(randRange)).(int)

					p.Cassette().UnmarshalFromFile()

					p.SetMode(playback.ModePlayback)
					numberGot := p.Result(key, rand.Intn(randRange)).(int)

					assert.Equal(t, numberExpected, numberGot)
					assert.True(t, p.Cassette().CheckPlay())
				},
			},
			{
				title: "can't replay if not recorded",
				f: func(t *testing.T, p *playback.Playback) {
					key := "rand.Intn"

					p.SetMode(playback.ModePlayback)

					assert.Equal(t, 0, p.Result(key, rand.Intn(randRange)))
					assert.False(t, p.Cassette().CheckPlay())
				},
			},
			{
				title: "can't replay twice if recorded once",
				f: func(t *testing.T, p *playback.Playback) {
					key := "rand.Intn"

					p.SetMode(playback.ModeRecord)
					numberExpected := p.Result(key, rand.Intn(randRange)).(int)

					p.Cassette().UnmarshalFromFile()

					p.SetMode(playback.ModePlayback)
					assert.Equal(t, numberExpected, p.Result(key, rand.Intn(randRange)))

					assert.Equal(t, 0, p.Result(key, rand.Intn(randRange)))
					assert.False(t, p.Cassette().CheckPlay())
				},
			},
			{
				title: "can replay twice if recorded twice",
				f: func(t *testing.T, p *playback.Playback) {
					key := "rand.Intn"

					expectedBody := []int{10, 30}

					p.SetMode(playback.ModeRecord)
					p.Result(key, expectedBody[0])
					p.Result(key, expectedBody[1])

					p.Cassette().UnmarshalFromFile()

					p.SetMode(playback.ModePlayback)
					assert.Equal(t, expectedBody[0], p.Result(key, rand.Intn(randRange)))
					assert.Equal(t, expectedBody[1], p.Result(key, rand.Intn(randRange)))

					assert.True(t, p.Cassette().CheckPlay())
				},
			},
			{
				title: "recorded twice, replayed once: CheckPlay is false",
				f: func(t *testing.T, p *playback.Playback) {
					key := "rand.Intn"

					expectedBody := []int{10, 30}

					p.SetMode(playback.ModeRecord)
					p.Result(key, expectedBody[0])
					p.Result(key, expectedBody[1])

					p.Cassette().UnmarshalFromFile()

					p.SetMode(playback.ModePlayback)
					assert.Equal(t, expectedBody[0], p.Result(key, rand.Intn(randRange)))

					assert.False(t, p.Cassette().CheckPlay())
				},
			},
			{
				title: "file contents are correct",
				f: func(t *testing.T, p *playback.Playback) {
					p.SetMode(playback.ModeRecord)

					key := "rand.Intn"
					numberExpected := p.Result(key, rand.Intn(randRange)).(int)

					contentsExpected := "- kind: result\n" +
						"  key: rand.Intn\n" +
						"  id: 1\n" +
						"  request: \"\"\n" +
						"  response: |\n" +
						"    type: int\n" +
						"    value: " + strconv.Itoa(numberExpected) + "\n"
					contentsGot, err := ioutil.ReadFile(p.Cassette().FileName())
					if err != nil {
						t.Fatal(err)
					}

					assert.Equal(t, contentsExpected, string(contentsGot))
				},
			},
		})
	})
	t.Run("playback.Http: record and playback", func(t *testing.T) {
		counter := 0
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			counter++
			w.Header().Set("Hi", strconv.Itoa(counter))
			fmt.Fprintf(w, "Hello, %d\n", counter)
		}))
		defer ts.Close()

		runTests(t, []Test{
			{
				title: "replaying works",
				f: func(t *testing.T, p *playback.Playback) {
					httpClient := &http.Client{
						Transport: p.HTTPTransport(http.DefaultTransport),
					}

					p.SetMode(playback.ModeRecord)

					expectedResponse, _ := httpClient.Get(ts.URL)
					expectedBody, _ := ioutil.ReadAll(expectedResponse.Body)

					p.Cassette().UnmarshalFromFile()

					p.SetMode(playback.ModePlayback)

					gotResponse, _ := httpClient.Get(ts.URL)
					gotBody, _ := ioutil.ReadAll(gotResponse.Body)

					assert.Equal(t, expectedBody, gotBody)
					assert.Equal(t, expectedResponse.StatusCode, gotResponse.StatusCode)
					assert.Equal(t, expectedResponse.Header, gotResponse.Header)

					assert.True(t, p.Cassette().CheckPlay())
				},
			},
			{
				title: "can't replay if not recorded",
				f: func(t *testing.T, p *playback.Playback) {
					httpClient := &http.Client{
						Transport: p.HTTPTransport(http.DefaultTransport),
					}

					p.SetMode(playback.ModePlayback)

					gotResponse, err := httpClient.Get(ts.URL)
					assert.Equal(t, &url.Error{Op: "Get", URL: ts.URL, Err: playback.ErrPlaybackFailed}, err)
					assert.Nil(t, gotResponse)

					assert.False(t, p.Cassette().CheckPlay())
				},
			},
			{
				title: "file contents are correct",
				f: func(t *testing.T, p *playback.Playback) {
					httpClient := &http.Client{
						Transport: p.HTTPTransport(http.DefaultTransport),
					}

					p.SetMode(playback.ModeRecord)

					req, _ := http.NewRequest("GET", ts.URL, nil)
					response, _ := httpClient.Do(req)
					body, _ := ioutil.ReadAll(response.Body)

					key, _ := playback.RequestToCurl(req)

					contentsExpected := "" +
						"- kind: http\n" +
						"  key: " + key + "\n" +
						"  id: 1\n" +
						"  request: curl -X 'GET' '" + ts.URL + "'\n" +
						"  response: \"\"\n" +

						"- kind: http\n" +
						"  key: " + key + "\n" +
						"  id: 1\n" +
						"  request: curl -X 'GET' '" + ts.URL + "'\n" +
						"  response: |\n" +
						"    statuscode: 200\n" +
						"    header:\n" +
						"      Content-Length:\n" +
						"      - \"9\"\n" +
						"      Content-Type:\n" +
						"      - text/plain; charset=utf-8\n" +
						"      Date:\n" +
						"      - " + response.Header.Get("Date") + "\n" +
						"      Hi:\n" +
						"      - \"2\"\n" +
						"    body: |\n" +
						"      " + string(body) + ""
					contentsGot, err := ioutil.ReadFile(p.Cassette().FileName())
					if err != nil {
						t.Fatal(err)
					}

					assert.Equal(t, contentsExpected, string(contentsGot))
				},
			},
		})
	})
}

func runTests(t *testing.T, tests []Test) {
	for _, test := range tests {
		t.Run(test.title, func(t *testing.T) {
			file := tempFile(t, playback.BasenamePrefix+"*.yml")
			defer removeFile(t, file)

			p := playback.New().WithFile(file)

			test.f(t, p)
		})
	}
}

func tempFile(t *testing.T, mask string) *os.File {
	file, err := ioutil.TempFile("", mask)
	if err != nil {
		t.Fatal(err)
	}

	return file
}

func removeFile(t *testing.T, file *os.File) {
	file.Sync()
	file.Close()

	err := os.Remove(file.Name())
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("Can't remove file %s", file.Name())
	}
}
