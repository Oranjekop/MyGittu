package main

import (
	"context"
	"flag"
	"io"
	"log"
	"mygittu/internal/ipc"
	"mygittu/internal/platform"
	"mygittu/internal/ui"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/egoist/mygo"
)

func launchFlags(flags *flag.FlagSet) (path, action *string) {
	return flags.String("path", "", "repository or selected file"),
		flags.String("action", "status", "status, commit, log, diff, clone, pull, push")
}

func main() {
	path, action := launchFlags(flag.CommandLine)
	flag.Parse()
	mainWindow := *action == "status" || *action == ""
	mygo.App.SetName("MyGittu")
	if mainWindow && !mygo.App.RequestSingleInstanceLock() {
		return
	}
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
	if *action == "fetch" || *action == "pull" || *action == "push" {
		if e = runRemote(client, *path, *action); e != nil {
			log.Fatal(e)
		}
		return
	}
	desktop := ui.New(client, *path, *action)
	if mainWindow {
		mygo.App.OnSecondInstance(func(args []string, workingDir string) {
			flags := flag.NewFlagSet("MyGittu", flag.ContinueOnError)
			flags.SetOutput(io.Discard)
			path, _ := launchFlags(flags)
			if flags.Parse(args) != nil {
				return
			}
			if *path != "" && !filepath.IsAbs(*path) {
				*path = filepath.Join(workingDir, *path)
			}
			desktop.Activate(*path)
		})
	}
	if e = desktop.Run(); e != nil {
		log.Fatal(e)
	}
}
