package test

import (
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"strings"
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

			cassette := playback.CassetteFromContext(ctx)
			assert.NotNil(t, cassette)
			assert.IsType(t, &playback.Cassette{}, cassette)
		})
		t.Run("playback.NewContextWithCassette", func(t *testing.T) {
			p := &playback.Playback{}

			cassette, _ := p.NewCassette()

			ctx := context.Background()
			ctx = playback.NewContextWithCassette(ctx, cassette)

			cassetteGot := playback.CassetteFromContext(ctx)

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
		defer removeFilename(t, cassette.PathName())

		key := "rand.Intn"

		p.SetMode(playback.ModeRecord)
		numberExpected := cassette.Result(key, rand.Intn(randRange)).(int)

		err = cassette.Finalize()
		if err != nil {
			t.Fatal("can't finalize cassette")
		}

		cassette, err = p.CassetteFromFile(cassette.PathName())
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
			defer removeFilename(t, cassette.PathName())

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
			p := playback.New()
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
			defer removeFilename(t, cassette.PathName())
			assert.NotEqual(t, "", cassette.PathName())
			assert.Equal(t, playback.PathTypeFile, cassette.PathType())
		})
		t.Run("if OFF then creates cassettes without file", func(t *testing.T) {
			p := playback.New()

			cassette, _ := p.NewCassette()
			assert.Equal(t, "", cassette.PathName())
			assert.Equal(t, playback.PathTypeNil, cassette.PathType())
		})
	})

	t.Run("playback.Result: record and playback", func(t *testing.T) {
		rand.Seed(time.Now().Unix())
		randRange := 100

		t.Run("value", func(t *testing.T) {
			t.Run("replaying works", func(t *testing.T) {
				p := playback.New().WithFile()

				cassette, _ := p.NewCassette()
				defer removeFilename(t, cassette.PathName())

				key := "rand.Intn"

				p.SetMode(playback.ModeRecord)
				numberExpected := cassette.Result(key, rand.Intn(randRange)).(int)
				cassette.Finalize()

				p.SetMode(playback.ModePlayback)
				cassette, _ = p.CassetteFromFile(cassette.PathName())
				numberGot := cassette.Result(key, rand.Intn(randRange)).(int)

				assert.Equal(t, numberExpected, numberGot)
				assert.True(t, cassette.IsPlaybackSucceeded())
			})

			t.Run("can't replay if not recorded", func(t *testing.T) {
				key := "rand.Intn"

				p := playback.New().WithFile()
				cassette, _ := p.NewCassette()
				defer removeFilename(t, cassette.PathName())

				p.SetMode(playback.ModePlayback)

				assert.Equal(t, 0, cassette.Result(key, rand.Intn(randRange)))
				assert.False(t, cassette.IsPlaybackSucceeded())
			})

			t.Run("can't replay twice if recorded once", func(t *testing.T) {
				p := playback.New().WithFile()
				cassette, _ := p.NewCassette()
				defer removeFilename(t, cassette.PathName())

				key := "rand.Intn"

				p.SetMode(playback.ModeRecord)
				numberExpected := cassette.Result(key, rand.Intn(randRange)).(int)

				cassette, _ = p.CassetteFromFile(cassette.PathName())

				p.SetMode(playback.ModePlayback)
				assert.Equal(t, numberExpected, cassette.Result(key, rand.Intn(randRange)))

				assert.Equal(t, 0, cassette.Result(key, rand.Intn(randRange)))
				assert.False(t, cassette.IsPlaybackSucceeded())
			})

			t.Run("can replay twice if recorded twice", func(t *testing.T) {
				p := playback.New().WithFile()
				cassette, _ := p.NewCassette()
				defer removeFilename(t, cassette.PathName())

				key := "rand.Intn"

				expectedBody := []int{10, 30}

				p.SetMode(playback.ModeRecord)
				cassette.Result(key, expectedBody[0])
				cassette.Result(key, expectedBody[1])

				cassette, _ = p.CassetteFromFile(cassette.PathName())

				p.SetMode(playback.ModePlayback)
				assert.Equal(t, expectedBody[0], cassette.Result(key, rand.Intn(randRange)))
				assert.Equal(t, expectedBody[1], cassette.Result(key, rand.Intn(randRange)))

				assert.True(t, cassette.IsPlaybackSucceeded())
			})

			t.Run("recorded twice, replayed once: IsPlaybackSucceeded is false", func(t *testing.T) {
				p := playback.New().WithFile()
				cassette, _ := p.NewCassette()
				defer removeFilename(t, cassette.PathName())

				key := "rand.Intn"

				expectedBody := []int{10, 30}

				p.SetMode(playback.ModeRecord)
				cassette.Result(key, expectedBody[0])
				cassette.Result(key, expectedBody[1])

				cassette, _ = p.CassetteFromFile(cassette.PathName())

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
				defer removeFilename(t, cassette.PathName())

				key := "rand.Intn"
				f := func() interface{} { return rand.Intn(randRange) }

				p.SetMode(playback.ModeRecord)
				numberExpected := cassette.Result(key, f).(int)

				cassette, _ = p.CassetteFromFile(cassette.PathName())

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
			defer removeFilename(t, cassette.PathName())

			p.SetMode(playback.ModeRecord)

			key := "rand.Intn"
			numberExpected := cassette.Result(key, rand.Intn(randRange)).(int)

			contentsExpected := "- kind: result\n" +
				"  key: rand.Intn\n" +
				"  id: 1\n" +
				"  request: \"\"\n" +
				"  requestdump: \"\"\n" +
				"  response: |\n" +
				"    type: int\n" +
				"    value: " + strconv.Itoa(numberExpected) + "\n"
			contentsGot, err := ioutil.ReadFile(cassette.PathName())
			if err != nil {
				t.Fatal(err)
			}

			assert.Equal(t, contentsExpected, string(contentsGot))
		})
	})

	t.Run("playback.Http: record and playback", func(t *testing.T) {
		t.Run("caller", func(t *testing.T) {
			counter := 0
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				counter++
				w.Header().Set("Hi", strconv.Itoa(counter))
				fmt.Fprintf(w, "Hello, %d\n", counter)
			}))
			defer ts.Close()

			t.Run("replaying works", func(t *testing.T) {
				p := playback.New().WithFile()

				httpClient := &http.Client{
					Transport: p.HTTPTransport(http.DefaultTransport),
				}

				p.SetMode(playback.ModeRecord)

				req, _ := http.NewRequest("GET", ts.URL, nil)
				ctx := p.NewContext(req.Context())
				req = req.WithContext(ctx)
				expectedResponse, _ := httpClient.Do(req)
				expectedBody, _ := ioutil.ReadAll(expectedResponse.Body)

				cassette := playback.CassetteFromContext(ctx)
				defer removeFilename(t, cassette.PathName())
				cassette, _ = p.CassetteFromFile(cassette.PathName())

				p.SetMode(playback.ModePlayback)

				req, _ = http.NewRequest("GET", ts.URL, nil)
				req = req.WithContext(playback.NewContextWithCassette(req.Context(), cassette))
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
				req = req.WithContext(playback.NewContextWithCassette(req.Context(), cassette))
				gotResponse, err := httpClient.Do(req)
				assert.Equal(t, &url.Error{Op: "Get", URL: ts.URL, Err: playback.ErrPlaybackFailed}, err)
				assert.Nil(t, gotResponse)

				assert.False(t, cassette.IsPlaybackSucceeded())
			})

			t.Run("file contents are correct", func(t *testing.T) {
				p := playback.New().WithFile()
				cassette, _ := p.NewCassette()
				defer removeFilename(t, cassette.PathName())

				httpClient := &http.Client{
					Transport: p.HTTPTransport(http.DefaultTransport),
				}

				p.SetMode(playback.ModeRecord)

				req, _ := http.NewRequest("GET", ts.URL, nil)
				req = req.WithContext(playback.NewContextWithCassette(req.Context(), cassette))
				response, _ := httpClient.Do(req)
				body, _ := ioutil.ReadAll(response.Body)

				key := keyOfRequest(req)

				contentsCommon := "" +
					"- kind: http\n" +
					"  key: " + key + "\n" +
					"  id: 1\n" +
					"  request: curl -X 'GET' '" + ts.URL + "'\n" +
					"  requestdump: " + `"GET / HTTP/1.1\r\nHost: ` + strings.TrimPrefix(ts.URL, "http://") + `\r\n\r\n"` + "\n"
				contentsExpected := "" +
					contentsCommon +
					"  response: \"\"\n" +

					contentsCommon +
					`  response: "HTTP/1.1 200 OK\r\nContent-Length: 9\r\nContent-Type: text/plain; charset=utf-8\r\nDate:` + "\n" +
					`    ` + response.Header.Get("Date") + `\r\nHi: 2\r\n\r\n` + strings.TrimSuffix(string(body), "\n") + `\n"` + "\n"

				contentsGot, err := ioutil.ReadFile(cassette.PathName())
				if err != nil {
					t.Fatal(err)
				}

				assert.Equal(t, contentsExpected, string(contentsGot))
			})

			t.Run("can record two cassettes in parallel", func(t *testing.T) {
				p := playback.New().WithFile()
				cassette, _ := p.NewCassette()
				defer removeFilename(t, cassette.PathName())

				httpClient := &http.Client{
					Transport: p.HTTPTransport(http.DefaultTransport),
				}

				// TODO
				return
				p.SetMode(playback.ModeRecord)

				expectedResponse, _ := httpClient.Get(ts.URL)
				expectedBody, _ := ioutil.ReadAll(expectedResponse.Body)

				cassette, _ = p.CassetteFromFile(cassette.PathName())

				p.SetMode(playback.ModePlayback)

				gotResponse, _ := httpClient.Get(ts.URL)
				gotBody, _ := ioutil.ReadAll(gotResponse.Body)

				assert.Equal(t, expectedBody, gotBody)
				assert.Equal(t, expectedResponse.StatusCode, gotResponse.StatusCode)
				assert.Equal(t, expectedResponse.Header, gotResponse.Header)

				assert.True(t, cassette.IsPlaybackSucceeded())
			})
		})

		tests := []struct {
			title          string
			serverFails    bool
			expectedStatus int
			expectedBody   string
		}{{
			title:          "server returns ok",
			serverFails:    false,
			expectedStatus: http.StatusOK,
			expectedBody:   "served10",
		}, {
			title:          "server returns error",
			serverFails:    true,
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   "",
		}}

		for _, test := range tests {
			t.Run(test.title, func(t *testing.T) {
				serverResponse := "served"
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Failed", serverResponse)
					if test.serverFails {
						w.WriteHeader(http.StatusInternalServerError)
					}
					fmt.Fprintf(w, serverResponse)
				}))
				defer ts.Close()

				t.Run("creates cassette on record", func(t *testing.T) {
					p := playback.New().WithFile()

					resultResponse := "10"

					httpClient := &http.Client{
						Transport: p.HTTPTransport(http.DefaultTransport),
					}
					handler := p.NewHTTPMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						resultStr := playback.CassetteFromContext(r.Context()).Result("test", resultResponse).(string)
						req, _ := http.NewRequest("GET", ts.URL, nil)
						req = req.WithContext(playback.ProxyCassetteContext(r.Context()))
						httpResponse, err := httpClient.Do(req)
						if err != nil || httpResponse.StatusCode != http.StatusOK {
							w.WriteHeader(http.StatusInternalServerError)
							return
						}

						httpBytes, _ := ioutil.ReadAll(httpResponse.Body)
						io.WriteString(w, string(httpBytes)+resultStr)
					}))

					p.SetMode(playback.ModeRecord)

					req := httptest.NewRequest("GET", "http://example.com/foo", nil)
					w := httptest.NewRecorder()
					handler.ServeHTTP(w, req)

					resp := w.Result()

					assert.Equal(t, test.expectedStatus, resp.StatusCode)
					assert.Equal(t, string(playback.PathTypeFile), resp.Header.Get(playback.HeaderCassettePathType))

					body, _ := ioutil.ReadAll(resp.Body)
					assert.Equal(t, test.expectedBody, string(body))
					assert.Equal(t, "", resp.Header.Get(playback.HeaderSuccess))

					p.SetMode(playback.ModePlayback)
					cassettePathName := resp.Header.Get(playback.HeaderCassettePathName)

					cassette, _ := p.CassetteFromFile(cassettePathName)
					defer removeFilename(t, cassette.PathName())

					t.Run("playbacks from cassette in context", func(t *testing.T) {
						cassette, _ := p.CassetteFromFile(cassettePathName)
						req := req.WithContext(playback.NewContextWithCassette(req.Context(), cassette))

						w = httptest.NewRecorder()
						handler.ServeHTTP(w, req)

						resp := w.Result()

						assert.True(t, cassette.IsPlaybackSucceeded())
						assert.True(t, cassette.IsHTTPResponseCorrect(resp))
						assert.Equal(t, "true", resp.Header.Get(playback.HeaderSuccess))

						body, _ = ioutil.ReadAll(resp.Body)
						assert.Equal(t, test.expectedBody, string(body))
					})

					t.Run("playbacks from cassette path in request headers", func(t *testing.T) {
						req := httptest.NewRequest("GET", "http://example.com/foo", nil)
						req.Header.Set(playback.HeaderCassettePathName, cassettePathName)
						req.Header.Set(playback.HeaderCassettePathType, string(playback.PathTypeFile))

						w = httptest.NewRecorder()
						handler.ServeHTTP(w, req)

						resp := w.Result()
						body, _ = ioutil.ReadAll(resp.Body)
						assert.Equal(t, test.expectedBody, string(body))

						assert.Equal(t, playback.ModePlayback, playback.Mode(resp.Header.Get(playback.HeaderMode)))
						assert.Equal(t, cassettePathName, resp.Header.Get(playback.HeaderCassettePathName))
						assert.Equal(t, "true", resp.Header.Get(playback.HeaderSuccess))
					})

					t.Run("request is recorded to the cassette", func(t *testing.T) {
						reqGot, err := cassette.HTTPRequest()
						assert.Nil(t, err)

						reqGot = reqGot.WithContext(context.Background())
						reqExpected := req.WithContext(context.Background())
						reqExpected.RemoteAddr = ""

						assert.Equal(t, reqExpected, reqGot)
					})

					t.Run("response is recorded to the cassette", func(t *testing.T) {
						respGot, err := cassette.HTTPResponse(req)
						assert.Nil(t, err)

						responseDumpGot, _ := httputil.DumpResponse(respGot, false)
						responseDumpExpected, _ := httputil.DumpResponse(resp, false)

						bodyGot, _ := ioutil.ReadAll(respGot.Body)

						assert.Equal(t, responseDumpExpected, responseDumpGot)
						assert.Equal(t, body, bodyGot)
					})
				})
			})
		}
	})

	// TODO record http with error status
	// TODO Can record background cassette and link it with per call cassettes
	// TODO Can record and playback separate cassettes in parallel
	// TODO Can be used as grpc middleware at server
	// TODO Can list created cassettes
	// TODO Can finalize cassette and drop it from active cassettes list
	// TODO Bypass all recordings/playbacks in ModeOff
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

func keyOfRequest(req *http.Request) string {
	requestDump, _ := httputil.DumpRequest(req, true)
	key := req.URL.Path + "?" + calcMD5(requestDump)

	return key
}

func calcMD5(data []byte) string {
	return fmt.Sprintf("%x", md5.Sum(data))
}
