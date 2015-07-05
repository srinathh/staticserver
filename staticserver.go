//Package staticserver implements a small flexible static assets server that can
//serve godoc virtual file systems, go-bindata assets etc with custom errors
//handlers and no directory listing
//
//The primary motivation for creating this package was to build a static asset
//server with different behaviors compared to http.FileServer (eg. preventing
//drectory listing, custom error handlers) while getting flexibility to serve
//static assets from a variety of sources.
//
//To achieve flexibility, StaticServer defines two function types - one with a
//signature matching os.LStat (takes a path & returns os.FileInfo) and the other
//with signature matching os.Open (takes a path & returns an os.ReadSeeker)
//which it uses to access and serve static assets.
//
//OS Filesystems, [string]string Maps & Zip files can be served by abstracting
//via godoc virtual file system and helper functions are provided to support them.
//go-bindata assets are similarly served by wrapping Asset(), AssetInfo() etc.
//in function closures
package staticserver

import (
	"archive/zip"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"golang.org/x/tools/godoc/vfs"
	"golang.org/x/tools/godoc/vfs/mapfs"
	"golang.org/x/tools/godoc/vfs/zipfs"
)

var defaultErrorHandlers = map[int]http.HandlerFunc{
	http.StatusNotFound: func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Not Found", http.StatusNotFound)
	},
	http.StatusInternalServerError: func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	},
}

func setupErrorHandlers(m map[int]http.HandlerFunc) map[int]http.HandlerFunc {
	if m == nil {
		return defaultErrorHandlers
	}

	ret := make(map[int]http.HandlerFunc)

	for code, def := range defaultErrorHandlers {
		if custom, ok := m[code]; ok {
			ret[code] = custom
		} else {
			ret[code] = def
		}
	}

	return ret
}

//StatFunc represents a function signature of type of os.LStat() or AssetInfo()
//from go-bindata that returns an os.FileInfo interface and error given path
type StatFunc func(string) (os.FileInfo, error)

//ReaderFunc represents a function signature that returns at least an io.ReadSeeker
//(and an error if unable to open) given a path - for instance os.Open() or
//vfs.FileSystem.Open(). If the implementation also defines a Close() method (eg. vfs.ReadSeekCloser), StaticServer will
//detect it and call Close() after serving the file to prevent dangling file pointers
type ReaderFunc func(string) (io.ReadSeeker, error)

//StaticServer provides a http.Handler to serves static assets. Use one of the
//"constructors" to get an instance of StaticServer to serve files from the
//filesystem, go-bindata asset functions, maps etc. You can provide your custom
//HandlerFuncs for http.StatusNotFound and http.StatusInternalServerError error
//if you wish and it will use those instead of basic http.Error()
type StaticServer struct {
	stat          StatFunc
	readerfn      ReaderFunc
	errorHandlers map[int]http.HandlerFunc
	logger        *log.Logger
}

//VFSStaticServer returns a StaticServer to serve a Godoc virtual file system
//as defined in golang.org/x/tools/godoc/vfs . The Godoc vfs package contains
//implementations for FileSystem, Map and Zip File based virtual file systems
//Use errorHandlers to provide custom http.HandlerFunc to handle http.StatusNotFound
//and http.StatusInternalServerError or provide nil to use default implementation
//If a log.Logger is provided (ie. not nil), StaticServer does verbose logging
func VFSStaticServer(f vfs.FileSystem, errorHandlers map[int]http.HandlerFunc, logger *log.Logger) StaticServer {
	return StaticServer{
		stat: f.Lstat,
		readerfn: func(name string) (io.ReadSeeker, error) {
			rsc, err := f.Open(name)
			return io.ReadSeeker(rsc), err
		},
		errorHandlers: setupErrorHandlers(errorHandlers),
		logger:        logger,
	}
}

//OSSS is a convenience function to return a StaticServer based on the
//OS file system by creating a Godoc virtual file system on the Root Folder
//Use errorHandlers to provide custom http.HandlerFunc to handle http.StatusNotFound
//and http.StatusInternalServerError or provide nil to use default implementation
//If a log.Logger is provided (ie. not nil), StaticServer does verbose logging
func OSSS(root string, errorHandlers map[int]http.HandlerFunc, logger *log.Logger) StaticServer {
	return VFSStaticServer(vfs.OS(root), errorHandlers, logger)
}

//MapSS is a convenience function to return a StaticServer based on a map
//mapping forward slash separated filepaths to strings. The paths cannot have
//a leading slash. This is implemented via a godoc virtual file system.
//Use errorHandlers to provide custom http.HandlerFunc to handle http.StatusNotFound
//and http.StatusInternalServerError or provide nil to use default implementation
//If a log.Logger is provided (ie. not nil), StaticServer does verbose logging
func MapSS(fs map[string]string, errorHandlers map[int]http.HandlerFunc, logger *log.Logger) StaticServer {
	return VFSStaticServer(mapfs.New(fs), errorHandlers, logger)
}

//ZipSS is a convenience function to return a StaticServer based on godoc.zipfs
//For more info look at documentation for golang.org/x/tools/godoc/vfs/zipfs/
//Use errorHandlers to provide custom http.HandlerFunc to handle http.StatusNotFound
//and http.StatusInternalServerError or provide nil to use default implementation
//If a log.Logger is provided (ie. not nil), StaticServer does verbose logging
func ZipSS(rc *zip.ReadCloser, name string, errorHandlers map[int]http.HandlerFunc, logger *log.Logger) StaticServer {
	return VFSStaticServer(zipfs.New(rc, name), errorHandlers, logger)
}

//RawStaticServer returns a static server where the caller supplies the StatFunc
//and ReaderFunc functions that the StaticServer uses to find and serve content
func RawStaticServer(stat StatFunc, readerfn ReaderFunc, errorHandlers map[int]http.HandlerFunc, logger *log.Logger) StaticServer {
	return StaticServer{
		stat:          stat,
		readerfn:      readerfn,
		errorHandlers: errorHandlers,
		logger:        logger,
	}
}

func (ss *StaticServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	if ss.logger != nil {
		ss.logger.Printf("StaticServer: Request for :%s", r.URL.Path)
	}

	// clean the path so that it cannot possibly begin with ../.
	// to prevent access to files outside root path in case we're using real FS
	reqpath := filepath.Clean("/" + r.URL.Path)

	//try to find the requested file
	info, err := ss.stat(reqpath)
	if err != nil {
		if ss.logger != nil {
			ss.logger.Printf("StaticServer: Error finding requested path. Returning http.StatusNotFound:%s", reqpath)
		}
		ss.errorHandlers[http.StatusNotFound](w, r)
		return
	}

	//we don't allow listing of directories. If the path was a directory, try
	//to find an index.html in it else return an error
	if info.IsDir() {
		if ss.logger != nil {
			ss.logger.Printf("StaticServer: The requested path was a directory. Trying to find index.html file in it:%s", reqpath)
		}
		reqpath = filepath.Join(reqpath, "index.html")
		info, err = ss.stat(reqpath)
		if err != nil {
			if ss.logger != nil {
				ss.logger.Printf("StaticServer: index.html was not found. Returning http.StatusNotFound:%s", reqpath)
			}
			ss.errorHandlers[http.StatusNotFound](w, r)
			return
		}
		//if index.html itself was a directory (however unlikely that is), just
		//send a not found error message since we don't want to serve directories
		//and don't want to get caught in a possible infinite recursion loop
		if info.IsDir() {
			if ss.logger != nil {
				ss.logger.Printf("StaticServer: index.html is a folder. Returning http.StatusNotFound:%s", reqpath)
			}
			ss.errorHandlers[http.StatusNotFound](w, r)
			return
		}
	}

	//at this stage, reqpath should be a valid asset that is available. We
	//try to get an io.Reader onto the file.
	rds, err := ss.readerfn(reqpath)
	if err != nil {
		if ss.logger != nil {
			ss.logger.Printf("StaticServer: Error obtaining a reader to requested path. Returning http.StatusInternalServerError:%s", reqpath)
		}
		ss.errorHandlers[http.StatusInternalServerError](w, r)
		return
	}

	if ss.logger != nil {
		ss.logger.Printf("StaticServer: Calling ServeContent for:%s", reqpath)
	}
	//Now we have both a reader and an os.FileInfo on the asset. we call http.ServeContent
	http.ServeContent(w, r, info.Name(), info.ModTime(), rds)

	//if the ReaderSeeker defines a Close() method, call it to avoid dangling file handles
	rdsc, ok := rds.(vfs.ReadSeekCloser)
	if ok {
		rdsc.Close()
	}

}
