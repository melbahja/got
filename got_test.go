package got_test

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
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

			if r.Method == http.MethodGet && strings.Contains(r.Header.Get("range"), "3-") {

				time.Sleep(3 * time.Second)
			}

			http.ServeFile(w, r, "go.mod")
			return

		case "/file_name":

			w.Header().Set("Content-Disposition", `attachment; filename="go.mod"`)
			http.ServeFile(w, r, "go.mod")
			return

		case "/header_values":
			if r.Header.Get("x-test-header") == "foobar" {
				http.ServeFile(w, r, "go.mod")
				return
			}

			w.WriteHeader(403)
			return

		case "/not_found":
		}

		w.WriteHeader(http.StatusNotFound)
	}))
}

var testUrl = httpt.URL + "/ok_file"

func ExampleGot() {

	// Just for testing
	destPath := createTemp()
	defer clean(destPath)

	g := got.New()

	err := g.Download(testUrl, destPath)

	if err != nil {
		log.Fatal(err)
		return
	}

	fmt.Println("done")

	// Output: done
}

func ExampleGot_withContext() {

	// Just for testing
	destPath := createTemp()
	defer clean(destPath)

	ctx := context.Background()

	g := got.NewWithContext(ctx)

	err := g.Download(testUrl, destPath)

	if err != nil {
		log.Fatal(err)
		return
	}

	fmt.Println("done")

	// Output: done
}
