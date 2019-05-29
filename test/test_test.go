package test

import (
	"bytes"
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
	"github.com/wtertius/playback/httphelper"
)

func TestCassete(t *testing.T) {
	t.Run("passing cassette through context", func(t *testing.T) {
		t.Run("NewContext", func(t *testing.T) {
			p := playback.New()

			ctx := context.Background()
			ctx = p.NewContext(ctx)

			cassette := playback.CassetteFromContext(ctx)
			assert.NotNil(t, cassette)
			assert.IsType(t, &playback.Cassette{}, cassette)
		})
		t.Run("playback.NewContextWithCassette", func(t *testing.T) {
			p := playback.New()

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

		p := playback.New().WithFile().SetDefaultMode(playback.ModeRecord)

		cassette, err := p.NewCassette()
		if err != nil {
			t.Fatal("Can't create file for cassette")
		}
		defer removeFilename(t, cassette.PathName())

		key := "rand.Intn"
		numberExpected := cassette.Result(key, rand.Intn(randRange)).(int)

		err = cassette.Finalize()
		if err != nil {
			t.Fatal("can't finalize cassette")
		}

		cassette, err = p.CassetteFromFile(cassette.PathName())
		if err != nil {
			t.Fatal("Can't create cassette from file")
		}

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

			cassette.SetMode(playback.ModeRecord)
			cassette.Result(key, expectedBody[0])
			assert.Nil(t, cassette.Error())

			cassette.Lock()
			cassette.Result(key, expectedBody[1])

			assert.Error(t, cassette.Error())
		})
		t.Run("Can unlock cassette for record", func(t *testing.T) {
			p := playback.New().SetDefaultMode(playback.ModeRecord)
			cassette, _ := p.NewCassette()

			key := "rand.Intn"

			expectedBody := []int{10, 30}

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
			p := playback.New().WithFile().SetDefaultMode(playback.ModeRecord)

			cassette, _ := p.NewCassette()
			defer removeFilename(t, cassette.PathName())
			assert.NotEqual(t, "", cassette.PathName())
			assert.Equal(t, playback.PathTypeFile, cassette.PathType())
		})
		t.Run("if OFF then creates cassettes without file", func(t *testing.T) {
			p := playback.New().SetDefaultMode(playback.ModeRecord)

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
				p := playback.New().WithFile().SetDefaultMode(playback.ModeRecord)

				cassette, _ := p.NewCassette()
				defer removeFilename(t, cassette.PathName())

				key := "rand.Intn"

				numberExpected := cassette.Result(key, rand.Intn(randRange)).(int)
				cassette.Finalize()

				cassette, _ = p.CassetteFromFile(cassette.PathName())
				numberGot := cassette.Result(key, rand.Intn(randRange)).(int)

				assert.Equal(t, numberExpected, numberGot)
				assert.True(t, cassette.IsPlaybackSucceeded())
			})

			t.Run("can't replay if not recorded", func(t *testing.T) {
				key := "rand.Intn"

				p := playback.New().WithFile().SetDefaultMode(playback.ModeRecord)
				cassette, _ := p.NewCassette()
				defer removeFilename(t, cassette.PathName())

				cassette.SetMode(playback.ModePlayback)

				assert.Equal(t, 0, cassette.Result(key, rand.Intn(randRange)))
				assert.False(t, cassette.IsPlaybackSucceeded())
			})

			t.Run("can't replay twice if recorded once", func(t *testing.T) {
				p := playback.New().WithFile().SetDefaultMode(playback.ModeRecord)
				cassette, _ := p.NewCassette()
				defer removeFilename(t, cassette.PathName())

				key := "rand.Intn"

				numberExpected := cassette.Result(key, rand.Intn(randRange)).(int)

				cassette, _ = p.CassetteFromFile(cassette.PathName())

				assert.Equal(t, numberExpected, cassette.Result(key, rand.Intn(randRange)))

				assert.Equal(t, 0, cassette.Result(key, rand.Intn(randRange)))
				assert.False(t, cassette.IsPlaybackSucceeded())
			})

			t.Run("can replay twice if recorded twice", func(t *testing.T) {
				p := playback.New().WithFile().SetDefaultMode(playback.ModeRecord)
				cassette, _ := p.NewCassette()
				defer removeFilename(t, cassette.PathName())

				key := "rand.Intn"

				expectedBody := []int{10, 30}

				cassette.Result(key, expectedBody[0])
				cassette.Result(key, expectedBody[1])

				cassette, _ = p.CassetteFromFile(cassette.PathName())

				assert.Equal(t, expectedBody[0], cassette.Result(key, rand.Intn(randRange)))
				assert.Equal(t, expectedBody[1], cassette.Result(key, rand.Intn(randRange)))

				assert.True(t, cassette.IsPlaybackSucceeded())
			})

			t.Run("recorded twice, replayed once: IsPlaybackSucceeded is false", func(t *testing.T) {
				p := playback.New().WithFile().SetDefaultMode(playback.ModeRecord)
				cassette, _ := p.NewCassette()
				defer removeFilename(t, cassette.PathName())

				key := "rand.Intn"

				expectedBody := []int{10, 30}

				cassette.Result(key, expectedBody[0])
				cassette.Result(key, expectedBody[1])

				cassette, _ = p.CassetteFromFile(cassette.PathName())

				assert.Equal(t, expectedBody[0], cassette.Result(key, rand.Intn(randRange)))

				assert.False(t, cassette.IsPlaybackSucceeded())
			})

			t.Run("can record two cassettes in parallel", func(t *testing.T) {
				p := playback.New().SetDefaultMode(playback.ModeRecord)

				key := "rand.Intn"

				expectedBody := []int{10, 30}

				cassettes := make([]*playback.Cassette, 2)
				cassettes[0], _ = p.NewCassette()
				cassettes[1], _ = p.NewCassette()

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

				cassettes[0].SetMode(playback.ModePlayback)
				cassettes[1].SetMode(playback.ModePlayback)

				assert.Equal(t, expectedBody[0], cassettes[0].Result(key, rand.Intn(randRange)))
				assert.Equal(t, expectedBody[1], cassettes[1].Result(key, rand.Intn(randRange)))

				assert.True(t, cassettes[0].IsPlaybackSucceeded())
				assert.True(t, cassettes[1].IsPlaybackSucceeded())
			})

			t.Run("can record and playback separate cassettes in parallel", func(t *testing.T) {
				p := playback.New().SetDefaultMode(playback.ModeRecord)

				key := "rand.Intn"

				expectedBody := []int{10, 30}
				gotBody := make([]int, 2)

				cassettes := make([]*playback.Cassette, 2)
				cassettes[0], _ = p.NewCassette()
				cassettes[1], _ = p.NewCassette()

				cassettes[0].Result(key, expectedBody[0])
				cassettes[0].Finalize()
				cassettes[0].Rewind()
				cassettes[0].SetMode(playback.ModePlayback)

				var wg sync.WaitGroup
				wg.Add(2)
				go func() {
					defer wg.Done()
					gotBody[0] = cassettes[0].Result(key, rand.Intn(randRange)).(int)
				}()
				go func() {
					defer wg.Done()
					cassettes[1].Result(key, expectedBody[1])
				}()
				wg.Wait()

				cassettes[0].SetMode(playback.ModePlayback)
				cassettes[1].SetMode(playback.ModePlayback)

				gotBody[1] = cassettes[1].Result(key, rand.Intn(randRange)).(int)

				assert.Equal(t, expectedBody[0], gotBody[0])
				assert.Equal(t, expectedBody[1], gotBody[1])

				assert.True(t, cassettes[0].IsPlaybackSucceeded())
				assert.True(t, cassettes[1].IsPlaybackSucceeded())
			})
		})

		t.Run("func", func(t *testing.T) {
			t.Run("replaying works", func(t *testing.T) {
				p := playback.New().WithFile().SetDefaultMode(playback.ModeRecord)
				cassette, _ := p.NewCassette()
				defer removeFilename(t, cassette.PathName())

				key := "rand.Intn"
				f := func() interface{} { return rand.Intn(randRange) }

				numberExpected := cassette.Result(key, f).(int)

				cassette, _ = p.CassetteFromFile(cassette.PathName())

				numberGot := cassette.Result(key, f).(int)

				assert.Equal(t, numberExpected, numberGot)
				assert.True(t, cassette.IsPlaybackSucceeded())
			})
			t.Run("panic is recorded and can be replayed", func(t *testing.T) {
				p := playback.New().WithFile().SetDefaultMode(playback.ModeRecord)
				cassette, _ := p.NewCassette()
				defer removeFilename(t, cassette.PathName())

				type Panic struct{ ErrDetails string }
				key := "rand.Intn"
				f := func() int {
					panic("PANIC")
					return rand.Intn(randRange)
				}

				func() {
					defer func() {
						r := recover()
						assert.Equal(t, "PANIC", r)
					}()

					number := cassette.Result(key, f).(int)
					assert.IsType(t, int(1), number)
				}()

				cassette, _ = p.CassetteFromFile(cassette.PathName())

				func() {
					defer func() {
						r := recover()
						assert.Equal(t, "PANIC", r)
					}()

					number := cassette.Result(key, f).(int)
					assert.IsType(t, int(1), number)
				}()
			})
		})

		t.Run("file contents are correct", func(t *testing.T) {
			p := playback.New().WithFile().SetDefaultMode(playback.ModeRecord)
			cassette, _ := p.NewCassette()
			defer removeFilename(t, cassette.PathName())

			key := "rand.Intn"
			numberExpected := cassette.Result(key, rand.Intn(randRange)).(int)

			contentsExpected := "- kind: result\n" +
				"  key: rand.Intn\n" +
				"  id: 1\n" +
				"  request: \"\"\n" +
				"  requestdump: \"\"\n" +
				"  response: |\n" +
				"    type: int\n" +
				"    value: " + strconv.Itoa(numberExpected) + "\n" +
				"  err: null\n" +
				"  panic: null\n"
			contentsGot, err := ioutil.ReadFile(cassette.PathName())
			if err != nil {
				t.Fatal(err)
			}

			assert.Equal(t, contentsExpected, string(contentsGot))
		})
	})

	t.Run("cassette can be marshaled to yaml string", func(t *testing.T) {
		p := playback.New().WithFile().SetDefaultMode(playback.ModeRecord)
		cassette, _ := p.NewCassette()
		defer removeFilename(t, cassette.PathName())

		key := "rand.Intn"
		cassette.Result(key, 10)

		contentsFile, err := ioutil.ReadFile(cassette.PathName())
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, string(contentsFile), string(cassette.MarshalToYAML()))
	})
	t.Run("cassette can be unmarshaled from yaml string", func(t *testing.T) {
		p := playback.New().WithFile().SetDefaultMode(playback.ModeRecord)
		cassette, _ := p.NewCassette()
		defer removeFilename(t, cassette.PathName())

		key := "rand.Intn"
		cassette.Result(key, 10)

		contentsFile, err := ioutil.ReadFile(cassette.PathName())
		if err != nil {
			t.Fatal(err)
		}

		cassette, _ = p.CassetteFromYAML(contentsFile)
		assert.Equal(t, string(contentsFile), string(cassette.MarshalToYAML()))
	})

	t.Run("cassette can be unmarshaled from yaml string", func(t *testing.T) {
		cassettes := make(map[string]*playback.Cassette, 3)
		p := playback.New().WithFile().SetDefaultMode(playback.ModeRecord)
		cassette, _ := p.NewCassette()
		cassettes[cassette.ID] = cassette
		defer removeFilename(t, cassette.PathName())

		key := "rand.Intn"
		cassette.Result(key, 10)

		pathName := cassette.PathName()
		contentsFile, err := ioutil.ReadFile(pathName)
		if err != nil {
			t.Fatal(err)
		}

		cassette, _ = p.CassetteFromYAML(contentsFile)
		cassettes[cassette.ID] = cassette
		cassette, _ = p.CassetteFromFile(pathName)
		cassettes[cassette.ID] = cassette

		list := p.List()
		assert.Equal(t, 3, len(cassettes))
		assert.Equal(t, len(cassettes), len(list))
		assert.Equal(t, cassettes, list)
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
				p := playback.New().WithFile().SetDefaultMode(playback.ModeRecord)

				httpClient := &http.Client{
					Transport: p.HTTPTransport(http.DefaultTransport),
				}

				req, _ := http.NewRequest("GET", ts.URL, nil)
				ctx := p.NewContext(req.Context())
				req = req.WithContext(ctx)
				expectedResponse, _ := httpClient.Do(req)
				expectedBody, _ := ioutil.ReadAll(expectedResponse.Body)

				cassette := playback.CassetteFromContext(ctx)
				defer removeFilename(t, cassette.PathName())
				cassette, _ = p.CassetteFromFile(cassette.PathName())

				req, _ = http.NewRequest("GET", ts.URL, nil)
				req = req.WithContext(playback.NewContextWithCassette(req.Context(), cassette))
				gotResponse, _ := httpClient.Do(req)
				gotBody, _ := ioutil.ReadAll(gotResponse.Body)

				assert.Equal(t, expectedResponse.StatusCode, gotResponse.StatusCode)
				assert.Equal(t, expectedResponse.Header, gotResponse.Header)
				assert.Equal(t, expectedBody, gotBody)

				assert.True(t, cassette.IsPlaybackSucceeded())
			})

			t.Run("can't replay if not recorded", func(t *testing.T) {
				p := playback.New().SetDefaultMode(playback.ModeRecord)
				cassette, _ := p.NewCassette()

				httpClient := &http.Client{
					Transport: p.HTTPTransport(http.DefaultTransport),
				}

				cassette.SetMode(playback.ModePlayback)

				req, _ := http.NewRequest("GET", ts.URL, nil)
				req = req.WithContext(playback.NewContextWithCassette(req.Context(), cassette))
				gotResponse, err := httpClient.Do(req)
				assert.Equal(t, &url.Error{Op: "Get", URL: ts.URL, Err: playback.ErrPlaybackFailed}, err)
				assert.Nil(t, gotResponse)

				assert.False(t, cassette.IsPlaybackSucceeded())
			})

			t.Run("file contents are correct", func(t *testing.T) {
				p := playback.New().WithFile().SetDefaultMode(playback.ModeRecord)
				cassette, _ := p.NewCassette()
				defer removeFilename(t, cassette.PathName())

				cassette.SetSyncMode(playback.SyncModeEveryChange)

				httpClient := &http.Client{
					Transport: p.HTTPTransport(http.DefaultTransport),
				}

				cassette.SetMode(playback.ModeRecord)

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
					"  requestdump: " + `"GET / HTTP/1.1\r\nHost: ` + strings.TrimPrefix(ts.URL, "http://") + `\r\nUser-Agent: Go-http-client/1.1\r\nAccept-Encoding:` + "\n" + `    gzip\r\n\r\n"` + "\n"
				contentsExpected := "" +
					contentsCommon +
					"  response: \"\"\n" +
					"  err: null\n" +
					"  panic: null\n" +

					contentsCommon +
					`  response: "HTTP/1.1 200 OK\r\nContent-Length: 9\r\nContent-Type: text/plain; charset=utf-8\r\nDate:` + "\n" +
					`    ` + response.Header.Get("Date") + `\r\nHi: ` + strconv.Itoa(counter) + `\r\n\r\n` + strings.TrimSuffix(string(body), "\n") + `\n"` + "\n" +
					"  err: null\n" +
					"  panic: null\n"

				contentsGot, err := ioutil.ReadFile(cassette.PathName())
				if err != nil {
					t.Fatal(err)
				}

				assert.Equal(t, contentsExpected, string(contentsGot))
			})

			t.Run("replaying is off by default", func(t *testing.T) {
				p := playback.New().WithFile()

				httpClient := &http.Client{
					Transport: p.HTTPTransport(http.DefaultTransport),
				}

				req, _ := http.NewRequest("GET", ts.URL, nil)
				ctx := p.NewContext(req.Context())
				req = req.WithContext(ctx)
				expectedResponse, _ := httpClient.Do(req)
				expectedBody, _ := ioutil.ReadAll(expectedResponse.Body)

				cassette := playback.CassetteFromContext(ctx)
				defer removeFilename(t, cassette.PathName())

				req, _ = http.NewRequest("GET", ts.URL, nil)
				req = req.WithContext(playback.NewContextWithCassette(req.Context(), cassette))
				gotResponse, _ := httpClient.Do(req)
				gotBody, _ := ioutil.ReadAll(gotResponse.Body)

				assert.Equal(t, expectedResponse.StatusCode, gotResponse.StatusCode)
				assert.NotEqual(t, expectedResponse.Header, gotResponse.Header)
				assert.NotEqual(t, expectedBody, gotBody)

				assert.False(t, cassette.IsPlaybackSucceeded())
			})

			t.Run("can run without cassette", func(t *testing.T) {
				p := playback.New()

				httpClient := &http.Client{
					Transport: p.HTTPTransport(http.DefaultTransport),
				}

				req, _ := http.NewRequest("GET", ts.URL, nil)
				response, _ := httpClient.Do(req)
				body, _ := ioutil.ReadAll(response.Body)

				cassette := playback.CassetteFromContext(req.Context())
				assert.Nil(t, cassette)

				assert.Equal(t, response.StatusCode, http.StatusOK)
				assert.NotNil(t, response.Header)
				assert.NotEqual(t, []byte{}, body)
			})
		})

		tests := []struct {
			title          string
			serverFails    bool
			serverTimeout  bool
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
		}, {
			title:          "server timeout",
			serverFails:    false,
			serverTimeout:  true,
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
					p := playback.New().WithFile().SetDefaultMode(playback.ModeRecord)

					resultResponse := "10"

					httpClient := &http.Client{
						Transport: p.HTTPTransport(http.DefaultTransport),
					}
					handler := p.NewHTTPMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						resultStr := playback.CassetteFromContext(r.Context()).Result("test", resultResponse).(string)
						req, _ := http.NewRequest("GET", ts.URL, nil)
						req = req.WithContext(playback.ProxyCassetteContext(r.Context()))
						if test.serverTimeout {
							ctx, cancel := context.WithTimeout(req.Context(), time.Nanosecond)
							defer cancel()
							req = req.WithContext(ctx)
						}
						httpResponse, err := httpClient.Do(req)
						if err != nil {
							w.WriteHeader(http.StatusInternalServerError)
							return
						} else if httpResponse.StatusCode != http.StatusOK {
							w.WriteHeader(httpResponse.StatusCode)
							return
						}

						httpBytes, _ := ioutil.ReadAll(httpResponse.Body)
						io.WriteString(w, string(httpBytes)+resultStr)
					}))

					req, _ := http.NewRequest("POST", "http://example.com/foo", strings.NewReader("bar"))
					w := httptest.NewRecorder()
					handler.ServeHTTP(w, req)

					resp := w.Result()

					assert.Equal(t, test.expectedStatus, resp.StatusCode)
					assert.Equal(t, string(playback.PathTypeFile), resp.Header.Get(playback.HeaderCassettePathType))

					body, _ := ioutil.ReadAll(resp.Body)
					assert.Equal(t, test.expectedBody, string(body))
					assert.Equal(t, "", resp.Header.Get(playback.HeaderSuccess))

					cassettePathName := resp.Header.Get(playback.HeaderCassettePathName)
					defer removeFilename(t, cassettePathName)

					cassette, _ := p.CassetteFromFile(cassettePathName)

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

					t.Run("playbacks from cassette id in request headers", func(t *testing.T) {
						req, _ := http.NewRequest("GET", "http://example.com/foo", nil)
						req.Header.Set(playback.HeaderCassetteID, cassette.ID)

						w = httptest.NewRecorder()
						handler.ServeHTTP(w, req)

						resp := w.Result()
						body, _ = ioutil.ReadAll(resp.Body)
						assert.Equal(t, test.expectedBody, string(body))

						assert.Equal(t, playback.ModePlayback, playback.Mode(resp.Header.Get(playback.HeaderMode)))
						assert.Equal(t, cassettePathName, resp.Header.Get(playback.HeaderCassettePathName))
						assert.Equal(t, "true", resp.Header.Get(playback.HeaderSuccess))
					})

					t.Run("playbacks from cassette path in request headers", func(t *testing.T) {
						req, _ := http.NewRequest("GET", "http://example.com/foo", nil)
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
						reqExpected, _ := http.NewRequest("POST", "http://example.com/foo", strings.NewReader("bar"))
						reqExpected.RemoteAddr = ""

						dumpExpected, _ := httputil.DumpRequestOut(reqExpected, true)
						dumpGot, _ := httputil.DumpRequest(reqGot, true)

						assert.ElementsMatch(t, strings.Split(string(dumpExpected), "\n"), strings.Split(string(dumpGot), "\n"))
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

		t.Run("Make cassette manually and playback", func(t *testing.T) {
			p := playback.New()

			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
			defer ts.Close()

			httpClient := &http.Client{
				Transport: p.HTTPTransport(http.DefaultTransport),
			}

			handler := p.NewHTTPMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				resultStr := playback.CassetteFromContext(r.Context()).Result("test", "not expected").(string)
				req, _ := http.NewRequest("GET", ts.URL, nil)
				req = req.WithContext(playback.ProxyCassetteContext(r.Context()))

				httpResponse, err := httpClient.Do(req)
				if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					return
				} else if httpResponse.StatusCode != http.StatusOK {
					w.WriteHeader(httpResponse.StatusCode)
					return
				}

				httpBytes, _ := ioutil.ReadAll(httpResponse.Body)
				io.WriteString(w, string(httpBytes)+resultStr)
			}))
			handler = p.NewHTTPServiceMiddleware(handler)

			resultResponse := "10"
			httpBody := "Done"
			expectedBody := httpBody + resultResponse

			t.Run("Server returns ok: one pass AddHTTPRecord", func(t *testing.T) {
				cassette, _ := p.NewCassette()

				req, _ := http.NewRequest("GET", "http://example.com/foo", nil)
				cassette.SetHTTPRequest(req)
				serverRequest, _ := http.NewRequest("GET", ts.URL, nil)
				cassette.AddHTTPRecord(serverRequest, httphelper.ResponseFromString(httpBody), nil)
				cassette.AddResultRecord("test", resultResponse, nil)
				cassette.SetHTTPResponse(req, httphelper.ResponseFromString(expectedBody))

				cassette.SetMode(playback.ModePlayback)

				req = req.WithContext(playback.NewContextWithCassette(req.Context(), cassette))

				w := httptest.NewRecorder()
				handler.ServeHTTP(w, req)

				resp := w.Result()

				assert.True(t, cassette.IsPlaybackSucceeded())
				assert.True(t, cassette.IsHTTPResponseCorrect(resp))
				assert.Equal(t, "true", resp.Header.Get(playback.HeaderSuccess))
				assert.Equal(t, http.StatusOK, resp.StatusCode)

				body, _ := ioutil.ReadAll(resp.Body)
				assert.Equal(t, expectedBody, string(body))
			})
			t.Run("Server returns ok: two passes AddHTTPRecord", func(t *testing.T) {
				cassette, _ := p.NewCassette()

				req, _ := http.NewRequest("GET", "http://example.com/foo", nil)
				cassette.SetHTTPRequest(req)
				serverRequest, _ := http.NewRequest("GET", ts.URL, nil)
				recorder := cassette.AddHTTPRecord(serverRequest, nil, nil)
				recorder.RecordResponse(httphelper.ResponseFromString(httpBody), nil)
				cassette.AddResultRecord("test", resultResponse, nil)
				cassette.SetHTTPResponse(req, httphelper.ResponseFromString(expectedBody))

				cassette.SetMode(playback.ModePlayback)

				req = req.WithContext(playback.NewContextWithCassette(req.Context(), cassette))

				w := httptest.NewRecorder()
				handler.ServeHTTP(w, req)

				resp := w.Result()

				assert.True(t, cassette.IsPlaybackSucceeded())
				assert.True(t, cassette.IsHTTPResponseCorrect(resp))
				assert.Equal(t, "true", resp.Header.Get(playback.HeaderSuccess))
				assert.Equal(t, http.StatusOK, resp.StatusCode)

				body, _ := ioutil.ReadAll(resp.Body)
				assert.Equal(t, expectedBody, string(body))
			})
			t.Run("Server returns error", func(t *testing.T) {
				cassette, _ := p.NewCassette()

				req, _ := http.NewRequest("GET", "http://example.com/foo", nil)
				cassette.SetHTTPRequest(req)

				serverRequest, _ := http.NewRequest("GET", ts.URL, nil)
				cassette.AddHTTPRecord(serverRequest, httphelper.ResponseError(http.StatusInternalServerError), nil)

				cassette.AddResultRecord("test", resultResponse, nil)
				cassette.SetHTTPResponse(req, httphelper.ResponseError(http.StatusInternalServerError))

				cassette.SetMode(playback.ModePlayback)

				req = req.WithContext(playback.NewContextWithCassette(req.Context(), cassette))

				w := httptest.NewRecorder()
				handler.ServeHTTP(w, req)

				resp := w.Result()

				assert.True(t, cassette.IsPlaybackSucceeded())
				assert.True(t, cassette.IsHTTPResponseCorrect(resp))
				assert.Equal(t, "true", resp.Header.Get(playback.HeaderSuccess))
				assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)

				body, _ := ioutil.ReadAll(resp.Body)
				assert.Equal(t, "", string(body))
			})
			t.Run("Put cassette to server using HTTP method", func(t *testing.T) {
				cassette, _ := p.NewCassette()

				req, _ := http.NewRequest("GET", "http://example.com/foo", nil)
				cassette.SetHTTPRequest(req)
				serverRequest, _ := http.NewRequest("GET", ts.URL, nil)
				cassette.AddHTTPRecord(serverRequest, httphelper.ResponseFromString(httpBody), nil)
				cassette.AddResultRecord("test", resultResponse, nil)
				cassette.SetHTTPResponse(req, httphelper.ResponseFromString(expectedBody))

				cassette.SetMode(playback.ModePlayback)

				req = httptest.NewRequest("POST", "http://example.com/playback/add/", bytes.NewBuffer(cassette.MarshalToYAML()))
				w := httptest.NewRecorder()
				handler.ServeHTTP(w, req)
				resp := w.Result()
				body, _ := ioutil.ReadAll(resp.Body)
				cassetteID := string(body)
				assert.NotEmpty(t, cassetteID)

				req = httptest.NewRequest("GET", "http://example.com/foo", nil)
				req.Header.Set(playback.HeaderCassetteID, cassetteID)

				w = httptest.NewRecorder()
				handler.ServeHTTP(w, req)

				resp = w.Result()

				cassette = p.Get(cassetteID)
				assert.True(t, cassette.IsPlaybackSucceeded())
				assert.True(t, cassette.IsHTTPResponseCorrect(resp))
				assert.Equal(t, "true", resp.Header.Get(playback.HeaderSuccess))
				assert.Equal(t, http.StatusOK, resp.StatusCode)

				body, _ = ioutil.ReadAll(resp.Body)
				assert.Equal(t, expectedBody, string(body))
			})
		})
	})

	// TODO clean cassettes map in Playback by time or count
	// TODO? result.Func can return error.
	// TODO Can record background cassette and link it with per call cassettes
	// TODO Can be used as grpc middleware at server
	// TODO Can list created cassettes
	// TODO Can finalize cassette and drop it from active cassettes list
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
	requestDump, _ := httputil.DumpRequestOut(req, true)
	key := req.URL.Path + "?" + calcMD5(requestDump)

	return key
}

func calcMD5(data []byte) string {
	return fmt.Sprintf("%x", md5.Sum(data))
}
