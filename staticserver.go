package staticserver

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

type StaticServer struct {
	root string
}

func NewStaticServer(path string) (*StaticServer, error) {
	fi, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("Error: NewStaticServer: Error in statting %s:%s", path, err)
	}

	if !fi.IsDir() {
		return nil, fmt.Errorf("Error: NewStaticServer: Root path %s is not a a folder", path)
	}

	return &StaticServer{path}, nil
}

var defaultfiles = []string{"index.html", "index.htm"}

func (fs StaticServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	// Clean the path so that it cannot possibly begin with ../.
	// If it did, the result of filepath.Join would be outside the
	// tree rooted at root.  We probably won't ever see a path
	// with .. in it, but be safe anyway.
	reqpath := filepath.Join(fs.root, filepath.Clean("/"+r.URL.Path))

	fi, err := os.Lstat(reqpath)
	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		log.Printf("StaticServer: Error finding resource %s : %s", reqpath, err)
		return
	}

	if fi.IsDir() {
		found := ""
		for _, filename := range defaultfiles {
			temppath := filepath.Join(reqpath, filename)
			if _, err := os.Lstat(temppath); err == nil {
				found = temppath
				break
			}
		}
		if found == "" {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			log.Printf("Error: StaticServer: could not find default file in at %s", reqpath)
			return
		}
		reqpath = found

	}

	http.ServeFile(w, r, reqpath)
}
