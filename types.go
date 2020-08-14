package got

import (
	"context"
	"net/http"
)

type (
	Config struct {

		// URL to download.
		URL string

		// File destination.
		Dest string

		// Set maximum chunk size.
		MaxChunkSize uint64

		// Set min chunk size.
		MinChunkSize uint64

		// Progress interval in ms.
		Interval uint64

		// Max chunks to download at same time.
		Concurrency uint
	}

	// Info of the download url.
	Info struct {

		// File content length.
		Length uint64

		// Supports partial content?
		Rangeable bool

		// URL Redirected.
		Redirected bool
	}

	// Download represents the download URL.
	Download struct {
		// Download file info.
		Info

		Config

		// Progress...
		Progress *Progress

		// Download file chunks.
		chunks []Chunk

		// Http client.
		client *http.Client

		// Is the URL redirected to a different location.
		redirected bool

		// Split file into chunks by ChunkSize in bytes.
		chunkSize uint64

		// Context...
		ctx context.Context
	}
)
