package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/srinathh/staticserver"
)

func main() {

	var ipport string

	flag.StringVar(&ipport, "http", "127.0.0.1:8080", "The IP address and port to host the server at")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: staticserver -flags <directory to serve>\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	if flag.NArg() != 1 {
		fmt.Fprintf(os.Stderr, "Error: You must specify a target directory to serve\n")
		flag.Usage()
		return
	}

	server, err := staticserver.NewStaticServer(flag.Arg(0))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		flag.Usage()
		return
	}

	http.ListenAndServe(ipport, server)
}
