package main

import (
	"context"
	"flag"
	"log"
	"mygittu/internal/ipc"
	"mygittu/internal/platform"
	"mygittu/internal/ui"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

func main() {
	path := flag.String("path", "", "repository or selected file")
	action := flag.String("action", "status", "status, commit, log, diff, clone, pull, push")
	flag.Parse()
	client := &ipc.Client{Dial: platform.Dial}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	e := client.Call(ctx, "health.ping", ipc.Params{}, nil)
	cancel()
	if e != nil {
		exe, e := os.Executable()
		if e != nil {
			log.Fatal(e)
		}
		agent := filepath.Join(filepath.Dir(exe), agentName())
		cmd := exec.Command(agent)
		configureAgent(cmd)
		if e = cmd.Start(); e != nil {
			log.Fatalf("start agent: %v; build agent beside desktop first", e)
		}
		_ = cmd.Process.Release()
		ready := false
		for i := 0; i < 50; i++ {
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			err := client.Call(ctx, "health.ping", ipc.Params{}, nil)
			cancel()
			if err == nil {
				ready = true
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
		if !ready {
			log.Fatal("Agent did not become ready")
		}
	}
	if e = ui.New(client, *path, *action).Run(); e != nil {
		log.Fatal(e)
	}
}
