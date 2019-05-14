package test

import (
	"context"
	"io/ioutil"
	"math/rand"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/wtertius/playback"
)

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

		tests := []struct {
			title string
			f     func(t *testing.T, p *playback.Playback)
		}{
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

					expected := []int{10, 30}

					p.SetMode(playback.ModeRecord)
					p.Result(key, expected[0])
					p.Result(key, expected[1])

					p.Cassette().UnmarshalFromFile()

					p.SetMode(playback.ModePlayback)
					assert.Equal(t, expected[0], p.Result(key, rand.Intn(randRange)))
					assert.Equal(t, expected[1], p.Result(key, rand.Intn(randRange)))

					assert.True(t, p.Cassette().CheckPlay())
				},
			},
			{
				title: "recorded twice, replayed once: CheckPlay is false",
				f: func(t *testing.T, p *playback.Playback) {
					key := "rand.Intn"

					expected := []int{10, 30}

					p.SetMode(playback.ModeRecord)
					p.Result(key, expected[0])
					p.Result(key, expected[1])

					p.Cassette().UnmarshalFromFile()

					p.SetMode(playback.ModePlayback)
					assert.Equal(t, expected[0], p.Result(key, rand.Intn(randRange)))

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
		}

		for _, test := range tests {
			t.Run(test.title, func(t *testing.T) {
				file := tempFile(t, playback.BasenamePrefix+"*.yml")
				defer removeFile(t, file)

				p := playback.New().WithFile(file)
				test.f(t, p)
			})
		}
	})
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
