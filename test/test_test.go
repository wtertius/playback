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
		p := &playback.Playback{Mode: playback.ModePlaybackOrRecord, File: file}

		ctx := context.Background()
		ctx = playback.NewContext(ctx, p)

		assert.Equal(t, p, playback.FromContext(ctx))
	})

	t.Run("playback.Result: record and playback", func(t *testing.T) {
		file := tempFile(t, playback.BasenamePrefix+"*.yml")
		defer removeFile(t, file)

		p := &playback.Playback{Mode: playback.ModePlaybackOrRecord, File: file}

		// init random
		rand.Seed(time.Now().Unix())
		randRange := 100

		key := "rand.Intn"
		numberExpected := p.Result(key, rand.Intn(randRange)).(int)

		t.Run("replaying works", func(t *testing.T) {
			numberGot := p.Result(key, rand.Intn(randRange)).(int)

			assert.Equal(t, numberExpected, numberGot)
		})

		t.Run("file contents are correct", func(t *testing.T) {
			contentsExpected := "- kind: result\n" +
				"  key: rand.Intn\n" +
				"  request: \"\"\n" +
				"  response: |\n" +
				"    type: int\n" +
				"    value: " + strconv.Itoa(numberExpected) + "\n"
			contentsGot, err := ioutil.ReadFile(file.Name())
			if err != nil {
				t.Fatal(err)
			}

			assert.Equal(t, contentsExpected, string(contentsGot))
		})
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
