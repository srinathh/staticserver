THIS README IS OBSOLETE - NEEDS TO BE CHANGED
===============================================

StaticServer is a small Go Language library that provides a http.handler to serve
static files in a folder. Unlike the default file server, StaticServer does not
allow listing of directories - it allows access to files only with a direct URL

Fetching Static Server
----------------------
```go get github.com/srinathh/staticserver```

Usage
-----
```
ss, err := staticserver.NewStaticServer("path/to/be/served")
if err != nil{
  log.Fatal(err)
}

http.Handle("/mycustom/endpoint", mycustomhandler)
http.Handle("/",ss)
http.ListenAndServe(":8080",nil)
```
Todo
----
- Allow custom setting of file not found and bad request pages
- Setup handling of strip-prefix etc
