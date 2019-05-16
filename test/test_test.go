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
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/wtertius/playback"
)

func TestCassete(t *testing.T) {
	t.Run("passing cassette through context", func(t *testing.T) {
		t.Run("NewContext", func(t *testing.T) {
			p := &playback.Playback{}

			ctx := context.Background()
			ctx = p.NewContext(ctx)

			cassette := playback.FromContext(ctx)
			assert.NotNil(t, cassette)
			assert.IsType(t, &playback.Cassette{}, cassette)
		})
		t.Run("NewContextWithCassette", func(t *testing.T) {
			p := &playback.Playback{}

			cassette, _ := p.NewCassette()

			ctx := context.Background()
			ctx = p.NewContextWithCassette(ctx, cassette)

			cassetteGot := playback.FromContext(ctx)

			assert.Equal(t, cassette, cassetteGot)
		})
	})

	t.Run("playback can record and playback to/from file", func(t *testing.T) {
		rand.Seed(time.Now().Unix())
		randRange := 100

		p := playback.New().WithFile()

		cassette, err := p.NewCassette()
		if err != nil {
			t.Fatal("Can't create file for cassette")
		}
		defer removeFilename(t, cassette.Filename())

		key := "rand.Intn"

		p.SetMode(playback.ModeRecord)
		numberExpected := cassette.Result(key, rand.Intn(randRange)).(int)

		err = cassette.Finalize()
		if err != nil {
			t.Fatal("can't finalize cassette")
		}

		cassette, err = p.CassetteFromFile(cassette.Filename())
		if err != nil {
			t.Fatal("Can't create cassette from file")
		}

		p.SetMode(playback.ModePlayback)
		numberGot := cassette.Result(key, rand.Intn(randRange)).(int)

		assert.Equal(t, numberExpected, numberGot, "Got the same result")
		assert.True(t, cassette.IsPlaybackSucceeded(), "Playback is succeeded")
	})

	t.Run("lock", func(t *testing.T) {
		t.Run("Can lock cassette for record", func(t *testing.T) {
			p := playback.New().WithFile()
			cassette, _ := p.NewCassette()

			key := "rand.Intn"

			expectedBody := []int{10, 30}

			p.SetMode(playback.ModeRecord)
			cassette.Result(key, expectedBody[0])
			assert.Nil(t, cassette.Error())

			cassette.Lock()
			cassette.Result(key, expectedBody[1])

			assert.Error(t, cassette.Error())
		})
		t.Run("Can unlock cassette for record", func(t *testing.T) {
			p := playback.New().WithFile()
			cassette, _ := p.NewCassette()

			key := "rand.Intn"

			expectedBody := []int{10, 30}

			p.SetMode(playback.ModeRecord)
			cassette.Result(key, expectedBody[0])
			assert.Nil(t, cassette.Error())

			cassette.Lock()
			cassette.Unlock()
			cassette.Result(key, expectedBody[1])

			assert.Nil(t, cassette.Error())
		})
	})

	t.Run("playback.WithFile", func(t *testing.T) {
		t.Run("if ON then creates cassettes with file", func(t *testing.T) {
			p := playback.New().WithFile()

			cassette, _ := p.NewCassette()
			assert.NotEqual(t, "", cassette.Filename())
		})
		t.Run("if OFF then doesn't create cassettes with file", func(t *testing.T) {
			p := playback.New()

			cassette, _ := p.NewCassette()
			assert.Equal(t, "", cassette.Filename())
		})
	})

	t.Run("playback.Result: record and playback", func(t *testing.T) {
		rand.Seed(time.Now().Unix())
		randRange := 100

		t.Run("value", func(t *testing.T) {
			t.Run("replaying works", func(t *testing.T) {
				p := playback.New().WithFile()

				cassette, _ := p.NewCassette()
				defer removeFilename(t, cassette.Filename())

				key := "rand.Intn"

				p.SetMode(playback.ModeRecord)
				numberExpected := cassette.Result(key, rand.Intn(randRange)).(int)
				cassette.Finalize()

				p.SetMode(playback.ModePlayback)
				cassette, _ = p.CassetteFromFile(cassette.Filename())
				numberGot := cassette.Result(key, rand.Intn(randRange)).(int)

				assert.Equal(t, numberExpected, numberGot)
				assert.True(t, cassette.IsPlaybackSucceeded())
			})

			t.Run("can't replay if not recorded", func(t *testing.T) {
				key := "rand.Intn"

				p := playback.New().WithFile()
				cassette, _ := p.NewCassette()
				p.SetMode(playback.ModePlayback)

				assert.Equal(t, 0, cassette.Result(key, rand.Intn(randRange)))
				assert.False(t, cassette.IsPlaybackSucceeded())
			})

			t.Run("can't replay twice if recorded once", func(t *testing.T) {
				p := playback.New().WithFile()
				cassette, _ := p.NewCassette()

				key := "rand.Intn"

				p.SetMode(playback.ModeRecord)
				numberExpected := cassette.Result(key, rand.Intn(randRange)).(int)

				cassette, _ = p.CassetteFromFile(cassette.Filename())

				p.SetMode(playback.ModePlayback)
				assert.Equal(t, numberExpected, cassette.Result(key, rand.Intn(randRange)))

				assert.Equal(t, 0, cassette.Result(key, rand.Intn(randRange)))
				assert.False(t, cassette.IsPlaybackSucceeded())
			})

			t.Run("can replay twice if recorded twice", func(t *testing.T) {
				p := playback.New().WithFile()
				cassette, _ := p.NewCassette()

				key := "rand.Intn"

				expectedBody := []int{10, 30}

				p.SetMode(playback.ModeRecord)
				cassette.Result(key, expectedBody[0])
				cassette.Result(key, expectedBody[1])

				cassette, _ = p.CassetteFromFile(cassette.Filename())

				p.SetMode(playback.ModePlayback)
				assert.Equal(t, expectedBody[0], cassette.Result(key, rand.Intn(randRange)))
				assert.Equal(t, expectedBody[1], cassette.Result(key, rand.Intn(randRange)))

				assert.True(t, cassette.IsPlaybackSucceeded())
			})

			t.Run("recorded twice, replayed once: IsPlaybackSucceeded is false", func(t *testing.T) {
				p := playback.New().WithFile()
				cassette, _ := p.NewCassette()

				key := "rand.Intn"

				expectedBody := []int{10, 30}

				p.SetMode(playback.ModeRecord)
				cassette.Result(key, expectedBody[0])
				cassette.Result(key, expectedBody[1])

				cassette, _ = p.CassetteFromFile(cassette.Filename())

				p.SetMode(playback.ModePlayback)
				assert.Equal(t, expectedBody[0], cassette.Result(key, rand.Intn(randRange)))

				assert.False(t, cassette.IsPlaybackSucceeded())
			})

			t.Run("can record two cassettes in parallel", func(t *testing.T) {
				p := playback.New()

				key := "rand.Intn"

				expectedBody := []int{10, 30}

				cassettes := make([]*playback.Cassette, 2)
				cassettes[0], _ = p.NewCassette()
				cassettes[1], _ = p.NewCassette()

				p.SetMode(playback.ModeRecord)

				var wg sync.WaitGroup
				wg.Add(2)
				go func() {
					defer wg.Done()
					cassettes[0].Result(key, expectedBody[0])
				}()
				go func() {
					defer wg.Done()
					cassettes[1].Result(key, expectedBody[1])
				}()
				wg.Wait()

				p.SetMode(playback.ModePlayback)
				assert.Equal(t, expectedBody[0], cassettes[0].Result(key, rand.Intn(randRange)))
				assert.Equal(t, expectedBody[1], cassettes[1].Result(key, rand.Intn(randRange)))

				assert.True(t, cassettes[0].IsPlaybackSucceeded())
				assert.True(t, cassettes[1].IsPlaybackSucceeded())
			})
		})

		t.Run("func", func(t *testing.T) {
			t.Run("replaying works", func(t *testing.T) {
				p := playback.New().WithFile()
				cassette, _ := p.NewCassette()

				key := "rand.Intn"
				f := func() interface{} { return rand.Intn(randRange) }

				p.SetMode(playback.ModeRecord)
				numberExpected := cassette.Result(key, f).(int)

				cassette, _ = p.CassetteFromFile(cassette.Filename())

				p.SetMode(playback.ModePlayback)
				numberGot := cassette.Result(key, f).(int)

				assert.Equal(t, numberExpected, numberGot)
				assert.True(t, cassette.IsPlaybackSucceeded())
			})
			// TODO? result.Func can return error.
		})

		t.Run("file contents are correct", func(t *testing.T) {
			p := playback.New().WithFile()
			cassette, _ := p.NewCassette()

			p.SetMode(playback.ModeRecord)

			key := "rand.Intn"
			numberExpected := cassette.Result(key, rand.Intn(randRange)).(int)

			contentsExpected := "- kind: result\n" +
				"  key: rand.Intn\n" +
				"  id: 1\n" +
				"  request: \"\"\n" +
				"  response: |\n" +
				"    type: int\n" +
				"    value: " + strconv.Itoa(numberExpected) + "\n"
			contentsGot, err := ioutil.ReadFile(cassette.Filename())
			if err != nil {
				t.Fatal(err)
			}

			assert.Equal(t, contentsExpected, string(contentsGot))
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

		t.Run("replaying works", func(t *testing.T) {
			p := playback.New()
			p.WithFile()

			httpClient := &http.Client{
				Transport: p.HTTPTransport(http.DefaultTransport),
			}

			p.SetMode(playback.ModeRecord)

			req, _ := http.NewRequest("GET", ts.URL, nil)
			ctx := p.NewContext(req.Context())
			req = req.WithContext(ctx)
			expectedResponse, _ := httpClient.Do(req)
			expectedBody, _ := ioutil.ReadAll(expectedResponse.Body)

			cassette := playback.FromContext(ctx)
			cassette.Rewind()

			p.SetMode(playback.ModePlayback)

			req, _ = http.NewRequest("GET", ts.URL, nil)
			req = req.WithContext(p.NewContextWithCassette(req.Context(), cassette))
			gotResponse, _ := httpClient.Do(req)
			gotBody, _ := ioutil.ReadAll(gotResponse.Body)

			assert.Equal(t, expectedBody, gotBody)
			assert.Equal(t, expectedResponse.StatusCode, gotResponse.StatusCode)
			assert.Equal(t, expectedResponse.Header, gotResponse.Header)

			assert.True(t, cassette.IsPlaybackSucceeded())
		})

		t.Run("can't replay if not recorded", func(t *testing.T) {
			p := playback.New()
			cassette, _ := p.NewCassette()

			httpClient := &http.Client{
				Transport: p.HTTPTransport(http.DefaultTransport),
			}

			p.SetMode(playback.ModePlayback)

			req, _ := http.NewRequest("GET", ts.URL, nil)
			req = req.WithContext(p.NewContextWithCassette(req.Context(), cassette))
			gotResponse, err := httpClient.Do(req)
			assert.Equal(t, &url.Error{Op: "Get", URL: ts.URL, Err: playback.ErrPlaybackFailed}, err)
			assert.Nil(t, gotResponse)

			assert.False(t, cassette.IsPlaybackSucceeded())
		})

		t.Run("file contents are correct", func(t *testing.T) {
			p := playback.New().WithFile()
			cassette, _ := p.NewCassette()

			httpClient := &http.Client{
				Transport: p.HTTPTransport(http.DefaultTransport),
			}

			p.SetMode(playback.ModeRecord)

			req, _ := http.NewRequest("GET", ts.URL, nil)
			req = req.WithContext(p.NewContextWithCassette(req.Context(), cassette))
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
			contentsGot, err := ioutil.ReadFile(cassette.Filename())
			if err != nil {
				t.Fatal(err)
			}

			assert.Equal(t, contentsExpected, string(contentsGot))
		})

		t.Run("can record two cassettes in parallel", func(t *testing.T) {
			p := playback.New().WithFile()
			cassette, _ := p.NewCassette()

			httpClient := &http.Client{
				Transport: p.HTTPTransport(http.DefaultTransport),
			}

			// TODO
			return
			p.SetMode(playback.ModeRecord)

			expectedResponse, _ := httpClient.Get(ts.URL)
			expectedBody, _ := ioutil.ReadAll(expectedResponse.Body)

			cassette, _ = p.CassetteFromFile(cassette.Filename())

			p.SetMode(playback.ModePlayback)

			gotResponse, _ := httpClient.Get(ts.URL)
			gotBody, _ := ioutil.ReadAll(gotResponse.Body)

			assert.Equal(t, expectedBody, gotBody)
			assert.Equal(t, expectedResponse.StatusCode, gotResponse.StatusCode)
			assert.Equal(t, expectedResponse.Header, gotResponse.Header)

			assert.True(t, cassette.IsPlaybackSucceeded())
		})
	})

	// TODO Cassette is distinguished from file with interface
	// TODO Can be used as http middleware at server
	// TODO Can be used as grpc middleware at server
	// TODO Can list created cassettes
	// TODO Can finalize cassette and drop it from created cassettes list
}

func tempFile(t *testing.T, mask string) *os.File {
	file, err := ioutil.TempFile("", mask)
	if err != nil {
		t.Fatal(err)
	}

	return file
}

func removeFilename(t *testing.T, filename string) {
	err := os.Remove(filename)
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("Can't remove file %s", filename)
	}
}
