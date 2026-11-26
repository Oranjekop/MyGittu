package status

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	git "mygittu/internal/git"
	"mygittu/internal/repository"
	"mygittu/internal/settings"
)

type overlayPublication struct {
	roots  string
	mask   settings.BadgeMask
	states map[string]git.State
}

type overlayPublisher struct {
	dir     string
	written map[string]overlayPublication
}

func (c *Cache) EnableExplorerSnapshots(configDir string) {
	if runtime.GOOS != "windows" {
		return
	}
	dir := filepath.Join(configDir, "explorer-overlays-v1")
	c.overlayPublisher = &overlayPublisher{dir: dir, written: make(map[string]overlayPublication)}
}

// Explorer-discovered repositories are normally in memory only. Restore their
// watches after restarting so local lookups do not bypass rediscovery forever.
func (c *Cache) restoreExplorerRepositories(ctx context.Context) {
	p := c.overlayPublisher
	if p == nil {
		return
	}
	entries, _ := os.ReadDir(p.dir)
	for _, entry := range entries {
		if ctx.Err() != nil {
			return
		}
		id, ok := strings.CutSuffix(entry.Name(), ".overlay")
		if !ok || entry.IsDir() {
			continue
		}
		if _, err := c.Service.Registry.Get(id); err == nil {
			continue
		}
		path := filepath.Join(p.dir, entry.Name())
		file, err := os.Open(path)
		if err != nil {
			continue
		}
		scan := bufio.NewScanner(file)
		valid := scan.Scan() && scan.Text() == "MYGITTU-OVERLAY-1" && scan.Scan() && scan.Scan()
		root, _, _ := strings.Cut(scan.Text(), "\t")
		_ = file.Close()
		if valid && filepath.IsAbs(root) {
			if info, err := c.Service.Registry.Discover(ctx, root); err == nil && info.ID == id {
				if info, err = c.Service.Registry.RegisterDiscovered(ctx, root); err == nil {
					c.ScheduleRefresh(info.ID)
					continue
				}
			}
		}
		if ctx.Err() == nil {
			c.mu.Lock()
			if _, err := c.Service.Registry.Get(id); err != nil {
				p.remove(id)
			}
			c.mu.Unlock()
		}
	}
}

func (p *overlayPublisher) remove(id string) {
	delete(p.written, id)
	if err := os.Remove(filepath.Join(p.dir, id+".overlay")); err != nil && !os.IsNotExist(err) {
		log.Printf("remove Explorer snapshot: %v", err)
	}
}

// Called under the cache lock so unregister and visibility changes cannot
// overtake a publication. Normal reconciliation skips unchanged snapshots.
func (c *Cache) publishOverlay(repo repository.Info, states map[string]git.State) bool {
	p := c.overlayPublisher
	if p == nil {
		return false
	}
	roots := strings.Join(append([]string{repo.Root}, repo.Aliases...), "\t")
	var mask settings.BadgeMask
	if c.Service.Settings != nil {
		mask = c.Service.Settings.HiddenBadges()
	}
	previous, exists := p.written[repo.ID]
	if exists && previous.roots == roots && previous.mask == mask && maps.Equal(previous.states, states) {
		p.written[repo.ID] = overlayPublication{roots, mask, states}
		return false
	}
	if _, err := c.Service.Registry.Get(repo.ID); err != nil {
		return false
	}
	if !exists {
		for id := range p.written {
			if _, err := c.Service.Registry.Get(id); err != nil {
				p.remove(id)
			}
		}
	}
	if err := p.write(repo.ID, roots, mask, states); err != nil {
		log.Printf("publish Explorer snapshot: %v", err)
		return false
	}
	p.written[repo.ID] = overlayPublication{roots, mask, states}
	return true
}

func (p *overlayPublisher) write(id, roots string, mask settings.BadgeMask, states map[string]git.State) error {
	if err := os.MkdirAll(p.dir, 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(p.dir, ".pending-")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	w := bufio.NewWriter(f)
	_, err = fmt.Fprintf(w, "MYGITTU-OVERLAY-1\n%d\n%s\n", mask, roots)
	for path, state := range states {
		if err != nil {
			break
		}
		_, err = fmt.Fprintf(w, "%s\t%s\n", state, path)
	}
	if err == nil {
		err = w.Flush()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(f.Name(), filepath.Join(p.dir, id+".overlay"))
}
