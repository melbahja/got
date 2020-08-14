package got

import (
	"testing"

	"golang.org/x/net/context"
)

func TestChunksLength(t *testing.T) {

	d, err := New(context.Background(), Config{
		URL:          "http://speedtest.ftp.otenet.gr/files/test10Mb.db",
		MinChunkSize: 5242870,
	})

	// init
	if err != nil {
		t.Error(err)
		return
	}

	chunk0 := Chunk{
		Start: 0,
		End:   5242870,
	}

	// Last chunk should not have end,
	// So the server must respond with the remaining content starting form 5242871
	chunk1 := Chunk{
		Start: 5242871,
		End:   0,
	}

	if d.chunks[0].Start != 0 {

		t.Errorf("First chunk should start from 0, but got %d", d.chunks[0].Start)
	}

	if chunk0.End != d.chunks[0].End {

		t.Errorf("Chunk 0 expecting: %d but got: %d", chunk0.End, d.chunks[0].End)
	}

	if d.chunks[1].Start != 5242871 {

		t.Errorf("Second chunk should start from: 5242871, but got %d", d.chunks[1].Start)
	}

	if chunk1.End != d.chunks[1].End {

		t.Errorf("Chunk 1 expecting: %d but got: %d", chunk1.End, d.chunks[1].End)
	}
}
