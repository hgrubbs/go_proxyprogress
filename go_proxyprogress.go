/*
   This program is free software: you can redistribute it and/or modify
   it under the terms of the GNU General Public License as published by
   the Free Software Foundation, either version 3 of the License, or
   (at your option) any later version.

   This program is distributed in the hope that it will be useful,
   but WITHOUT ANY WARRANTY; without even the implied warranty of
   MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
   GNU General Public License for more details.

   You should have received a copy of the GNU General Public License
   along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package main

import (
    "encoding/json"
    "flag"
    "fmt"
    "io/ioutil"
    "net/http"
    "os"
    "os/exec"
    "runtime"
    "strconv"
    "time"
)

type Response struct {
    Stdout string `json:"stdout"`
    Stderr string `json:"stderr"`
}

var g_temp_path string
var g_progress_path string

// timeString returns epoch in nanoseconds prefixed by "abl_", intended for temporary file names
func timeString() string {
    now := time.Now().UnixNano()
    x := strconv.FormatInt(now, 10)
    return "abl_" + x
}

// runHandler maps to HTTP /progress/ requests
func runHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != "POST" {
        http.Error(w, "Allowed method is POST", 405)
        return
    }
    querystring_args := r.URL.Query()
    db, exists := querystring_args["db"]
    if exists == false {
        http.Error(w, "Missing 'db' variable from in URL", 406)
        return
    }
    db_name := db[0]
    body, _ := ioutil.ReadAll(r.Body)

    // store 4gl query in temporary file
    file_path := g_temp_path + timeString() + ".4gl"
    f, _ := os.Create(file_path)
    f.Write(body)
    f.Close()

    // setup ENV and shell out to progress
    os.Setenv("TERM", "xterm")
    ext := exec.Command(g_progress_path)
    ext.Args = []string{g_progress_path, db_name, "-b", "-p", file_path}
    stderr, _ := ext.StderrPipe()
    stdout, _ := ext.StdoutPipe()
    err := ext.Start()
    ext_stdout, _ := ioutil.ReadAll(stdout)
    ext_stderr, _ := ioutil.ReadAll(stderr)
    ext.Wait()
    os.Remove(file_path) // remove temporary file
    if err != nil {
        http.Error(w, err.Error(), 500)
        return
    }

    // encode and return output as JSON
    m := new(Response)
    m.Stdout = string(ext_stdout)
    m.Stderr = string(ext_stderr)
    msg, err := json.Marshal(m)
    if err != nil {
        http.Error(w, err.Error(), 500)
        return
    }
    fmt.Fprint(w, string(msg)) // return JSON

}

func main() {
    build_number := "20150917.1413"
    fmt.Printf("* go_proxyprogress version %s\n", build_number)
	// Parse command-line args
	concurrency := flag.Int("cpus", 1, "Concurrency factor for multiple CPUs")
	temp_path := flag.String("temp_path", "/tmp/", "Path prefix for temporary query storage")
	progress_path := flag.String("progress_binary", "_progres", "Explicit path to _progres(eg /foo/bin/_progres)")
	bind_ip := flag.String("ip", "0.0.0.0", "Network address to listen on")
	bind_port := flag.String("port", "8080", "Network port to listen on")
	flag.Parse()

    g_temp_path = *temp_path         // global temporary path
    g_progress_path = *progress_path // global _progres path
    runtime.GOMAXPROCS(*concurrency)

    http.HandleFunc("/progress/query/", runHandler)
    fmt.Printf("Listening on %s:%s\n", *bind_ip, *bind_port)
    err := http.ListenAndServe((*bind_ip + ":" + *bind_port), nil)
    if err != nil {
        fmt.Printf("Unable to bind to %s: %s\n", (*bind_ip + ":" + *bind_port), err)
        }
}
