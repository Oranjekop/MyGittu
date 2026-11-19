package main

import (
	"context"
	"flag"
	"log"
	"mygittu/internal/app"
	git "mygittu/internal/git"
	"mygittu/internal/ipc"
	"mygittu/internal/platform"
	"mygittu/internal/repository"
	"mygittu/internal/settings"
	"mygittu/internal/status"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
)

func main() {
	binary := flag.String("git", "git", "Git executable")
	flag.Parse()
	r, e := git.NewRunner(*binary)
	if e != nil {
		log.Fatal(e)
	}
	dir, e := platform.ConfigDir()
	if e != nil {
		log.Fatal(e)
	}
	registry, e := repository.New(r, filepath.Join(dir, "repositories.json"))
	if e != nil {
		log.Fatal(e)
	}
	prefs, e := settings.New(filepath.Join(dir, "settings.json"))
	if e != nil {
		log.Fatal(e)
	}
	service := &app.Service{Runner: r, Registry: registry, Settings: prefs}
	cache := status.New(service)
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	listener, e := platform.Listen()
	if e != nil {
		log.Fatal(e)
	}
	go cache.Run(ctx)
	log.Print("MyGittu Agent started")
	if e = (&ipc.Server{Service: service, Cache: cache}).Serve(ctx, listener); e != nil {
		log.Fatal(e)
	}
}
