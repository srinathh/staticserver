package staticserver

import (
	"bytes"
	"log"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/srinathh/memfile"
)

type node struct {
	path    string
	content string
}

var testdata = []node{
	node{"title", "Mary had a Little Lamb"},
	node{"first stanza/first line/first breath", "Mary had a"},
	node{"first stanza/first line/second breath", "little lamb"},
	node{"first stanza/second line/first breath", "little lamb"},
	node{"first stanza/second line/second breath", "little lamb"},
	node{"first stanza/third line/first breath", "Mary had a"},
	node{"first stanza/third line/second breath", "little lamb"},
	node{"first stanza/fourth line/first breath", "whose fleece was"},
	node{"first stanza/fourth line/second breath", "white as snow"},
	node{"second stanza/first line/first breath", "And everywhere that"},
	node{"second stanza/first line/second breath", "Mary went"},
	node{"second stanza/second line/first breath", "Mary went"},
	node{"second stanza/second line/second breath", "Mary went"},
	node{"second stanza/third line/first breath", "And everywhere that"},
	node{"second stanza/third line/second breath", "Mary went"},
	node{"second stanza/fourth line/first breath", "the lamb was"},
	node{"second stanza/fourth line/second breath", "sure to go"},
}

func TestClose(t *testing.T) {
	testtext := "this is a test file."
	memfile := memfile.NewSimpleMemFile("test.txt", []byte(testtext), t)

	ss := RawStaticServer(func(s string) (os.FileInfo, error) {
		return &memfile, nil
	}, memfile.Open, nil, nil)

	w := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "/test.txt", bytes.NewReader([]byte{}))

	ss.ServeHTTP(w, r)
	if w.Body.String() != testtext {
		t.Errorf("Body did not match file text:\nExpected:%s\nGot:%s", testtext, w.Body.String())
	}
	if memfile.Misopen == true {
		t.Errorf("File did not close")
	}
}

func buildMapFS() map[string]string {
	ret := make(map[string]string)
	for _, node := range testdata {
		ret[node.path] = node.content
	}
	//fmt.Println(ret)
	return ret
}

func TestVFS(t *testing.T) {
	rand.Seed(time.Now().Unix())
	ss := MapSS(buildMapFS(), nil, log.New(os.Stdout, "TestVFS", log.Ldate|log.Ltime|log.Lshortfile))

	for j := 0; j < 10; j++ {
		idx := rand.Intn(len(testdata))
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "http://example.com/"+testdata[idx].path, bytes.NewReader([]byte{}))
		ss.ServeHTTP(w, req)
		if w.Body.String() != testdata[idx].content {
			t.Errorf("Want:%s\nGot:%s", testdata[idx].content, w.Body.String())
		}
	}
}

func TestNotFound(t *testing.T) {
	tests := map[string]map[string]string{
		"http://www.example1.com/": map[string]string{},
		"http://www.example2.com/sub": map[string]string{
			"sub/test.html": "Dummy file - creates subfolder to test absent index.html",
		},
		"http://www.example3.com/": map[string]string{
			"index.html/index.html": "This should fail since the root index.html is a dir",
		},
	}

	for k, v := range tests {
		ss := MapSS(v, nil, nil)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", k, bytes.NewReader([]byte{}))
		ss.ServeHTTP(w, req)
		if w.Code != http.StatusNotFound {
			t.Errorf("Did not get not found in %s", k)
		}
	}

}

func TestIndex(t *testing.T) {
	testmap := map[string]string{
		"index.html":     "root index file",
		"sub/index.html": "sub index file",
	}
	ss := MapSS(testmap, nil, nil)
	tests := map[string]response{
		"http://www.example.com/":     response{200, "root index file"},
		"http://www.example.com/sub/": response{200, "sub index file"},
	}
	runTests(tests, ss, "TestIndex", t)
}

//this will fail if tests are not run from the staticserver directory
func TestFS(t *testing.T) {
	tests := map[string]response{
		"http://www.example.com/":     response{200, "<html><body><h1>root index file</h1></body></html>\n"},
		"http://www.example.com/sub/": response{200, "<html><body><h1>sub index file</h1></body></html>\n"},
	}
	ss := OSSS("fstest", nil, nil)
	runTests(tests, ss, "TestFS", t)
}

type response struct {
	Code int
	Body string
}

func runTests(tests map[string]response, ss StaticServer, tag string, t *testing.T) {
	for k, v := range tests {
		w := httptest.NewRecorder()
		r, _ := http.NewRequest("GET", k, bytes.NewReader([]byte{}))
		ss.ServeHTTP(w, r)
		if w.Code != v.Code || w.Body.String() != v.Body {
			t.Errorf("Error in %s:\nExpected:%v\nGot::%v\n", tag, v, response{w.Code, w.Body.String()})
		}
	}
}

func TestErrorHandlers(t *testing.T) {
	notfound := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Custom Not Found Handler"))
	}
	tests := map[string]response{
		"http://www.example.com/": {404, "Custom Not Found Handler"},
	}
	ss := MapSS(map[string]string{}, map[int]http.HandlerFunc{404: notfound}, nil)
	runTests(tests, ss, "TestErrorHandlers", t)
}
