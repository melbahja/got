package got

import (
	"testing"
)

func TestChunksLength(t *testing.T) {

	d := &Download{
		URL:          "http://www.ovh.net/files/10Mio.dat",
		MinChunkSize: 5242870,
	}

	if err := d.Init(); err != nil {

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
		End:   10485759,
	}

	if d.info.Rangeable == false {
		t.Errorf("Chunk information could not be retrieved for the test file: %s", d.URL)
		return
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
