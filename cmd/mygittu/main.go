// Command mygittu is a diagnostic client for the same IPC used by the UI.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"mygittu/internal/ipc"
	"mygittu/internal/platform"
	"os"
	"time"
)

func main() {
	method := flag.String("method", "health.ping", "IPC method")
	params := flag.String("params", "{}", "JSON parameters")
	flag.Parse()
	var p ipc.Params
	if e := json.Unmarshal([]byte(*params), &p); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	var out any
	c := ipc.Client{Dial: platform.Dial}
	if e := c.Call(ctx, *method, p, &out); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
	b, _ := json.MarshalIndent(out, "", "  ")
	fmt.Println(string(b))
}
