package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/smartystreets/goconvey/web/server/contract"
)

type HTTPServer struct {
	watcher    contract.Watcher
	executor   contract.Executor
	latest     *contract.CompleteOutput
	clientChan chan chan string
}

func (self *HTTPServer) ReceiveUpdate(update *contract.CompleteOutput) {
	self.latest = update
}

func (self *HTTPServer) Watch(response http.ResponseWriter, request *http.Request) {

	// In case a web UI client disconnected (closed the tab),
	// the web UI will request, when it initially loads the page
	// and gets the watched directory, that the status channel
	// buffer be filled so that it can get the latest status updates
	// without missing a single beat.
	/*if request.URL.Query().Get("newclient") != "" {
		select {
		case self.statusUpdate <- true:
		default:
		}
	}*/

	if request.Method == "POST" {
		self.adjustRoot(response, request)
	} else if request.Method == "GET" {
		response.Write([]byte(self.watcher.Root()))
	}
}

func (self *HTTPServer) adjustRoot(response http.ResponseWriter, request *http.Request) {
	newRoot := self.parseQueryString("root", response, request)
	if newRoot == "" {
		return
	}
	err := self.watcher.Adjust(newRoot)
	if err != nil {
		http.Error(response, err.Error(), http.StatusNotFound)
	}
}

func (self *HTTPServer) Ignore(response http.ResponseWriter, request *http.Request) {
	path := self.parseQueryString("path", response, request)
	if path != "" {
		self.watcher.Ignore(path)
	}
}

func (self *HTTPServer) Reinstate(response http.ResponseWriter, request *http.Request) {
	path := self.parseQueryString("path", response, request)
	if path != "" {
		self.watcher.Reinstate(path)
	}
}

func (self *HTTPServer) parseQueryString(key string, response http.ResponseWriter, request *http.Request) string {
	value := request.URL.Query()[key]

	if len(value) == 0 {
		http.Error(response, fmt.Sprintf("No '%s' query string parameter included!", key), http.StatusBadRequest)
		return ""
	}

	path := value[0]
	if path == "" {
		http.Error(response, "You must provide a non-blank path.", http.StatusBadRequest)
	}
	return path
}

func (self *HTTPServer) Status(response http.ResponseWriter, request *http.Request) {
	status := self.executor.Status()
	response.Write([]byte(status))
}

func (self *HTTPServer) LongPollStatus(response http.ResponseWriter, request *http.Request) {
	if self.executor.ClearStatusFlag() {
		response.Write([]byte(self.executor.Status()))
		return
	}

	timeout, err := strconv.Atoi(request.URL.Query().Get("timeout"))
	if err != nil || timeout > 180000 || timeout < 0 {
		timeout = 60000 // default timeout is 60 seconds
	}

	myReqChan := make(chan string)

	select {
	case self.clientChan <- myReqChan: // this case means the executor's status is changing
	case <-time.After(time.Duration(timeout) * time.Millisecond): // this case means the executor hasn't changed status
		return
	}

	out := <-myReqChan

	if out != "" { // TODO: Why is this check necessary? Sometimes it writes empty string...
		response.Write([]byte(out))
	}
}

func (self *HTTPServer) Results(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("Content-Type", "application/json")
	response.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	response.Header().Set("Pragma", "no-cache")
	response.Header().Set("Expires", "0")
	stuff, _ := json.Marshal(self.latest)
	response.Write(stuff)
}

func (self *HTTPServer) Execute(response http.ResponseWriter, request *http.Request) {
	go self.execute()
}

func (self *HTTPServer) execute() {
	self.latest = self.executor.ExecuteTests(self.watcher.WatchedFolders())
}

func NewHTTPServer(watcher contract.Watcher, executor contract.Executor, status chan chan string) *HTTPServer {
	self := new(HTTPServer)
	self.watcher = watcher
	self.executor = executor
	self.clientChan = status
	return self
}

var _ = fmt.Sprintf("Hi")
