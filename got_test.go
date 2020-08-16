package got_test

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"time"

	"github.com/melbahja/got"
)

func NewHttptestServer() *httptest.Server {

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		switch r.URL.String() {

		case "/ok_file":
			http.ServeFile(w, r, "go.mod")
			return

		case "/found_and_head_not_allowed":

			if r.Method == http.MethodHead {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}

			fmt.Fprint(w, "helloworld")
			return

		case "/not_found_and_method_not_allowed":

			w.WriteHeader(http.StatusMethodNotAllowed)
			return

		case "/ok_file_with_range_delay":

			if strings.Contains(r.Header.Get("range"), "3-") {

				time.Sleep(2 * time.Second)
			}

			http.ServeFile(w, r, "go.mod")
			return

		case "/not_found":
		}

		w.WriteHeader(http.StatusNotFound)
	}))
}

var testUrl = httpt.URL + "/ok_file"

func ExampleGot() {

	defer os.Remove("/tmp/got_dl_file_test")

	g := got.New()

	err := g.Download(testUrl, "/tmp/got_dl_file_test")

	if err != nil {
		log.Fatal(err)
		return
	}

	fmt.Println("done")

	// Output: done
}

func ExampleGot_withContext() {

	defer os.Remove("/tmp/got_dl_file_test")

	ctx := context.Background()

	g := got.NewWithContext(ctx)

	err := g.Download(testUrl, "/tmp/got_dl_file_test")

	if err != nil {
		log.Fatal(err)
		return
	}

	fmt.Println("done")

	// Output: done
}
