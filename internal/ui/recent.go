package ui

import (
	"context"
	"path/filepath"
	"strings"

	native "github.com/egoist/mygo/ui"
	"mygittu/internal/ipc"
	"mygittu/internal/repository"
)

func desktopTheme(c *native.Context) *native.Theme {
	t := *c.Theme()
	if t.Dark {
		t.Background = native.Hex("#172223")
		t.Surface = native.Hex("#101a1b")
		t.Border = native.Hex("#354848")
		t.Accent = native.Hex("#88dfc1")
		t.AccentText = native.Hex("#102822")
		t.AccentHover = native.Hex("#a0ebd1")
		t.AccentPressed = native.Hex("#73c5a8")
	} else {
		t.Background = native.Hex("#fcfdfb")
		t.Surface = native.Hex("#edf3ef")
		t.Border = native.Hex("#cadbd3")
		t.Accent = native.Hex("#12644f")
		t.AccentText = native.Hex("#ffffff")
		t.AccentHover = native.Hex("#1a755d")
		t.AccentPressed = native.Hex("#0b5743")
	}
	c.SetTheme(&t)
	return &t
}

func (d *Desktop) repositoryDialog(c *native.Context) {
	native.Modal(c, &d.repositoriesOpen, func() {
		native.Column(c).Width(620).Gap(10).Children(func() {
			native.Row(c).Gap(8).AlignItems(native.Center).Children(func() {
				native.Text(c, d.text("repo.recent")).FontSize(22).Bold().Grow(1)
				if native.Button(c, d.text("close")).Clicked() {
					d.repositoriesOpen = false
				}
			})
			native.TextInput(c, &d.repositorySearch).Placeholder(d.text("repo.search")).FillWidth()
			query := strings.ToLower(strings.TrimSpace(d.repositorySearch))
			native.Scroll(c).Height(300).FillWidth().Children(func() {
				native.Column(c).FillWidth().Gap(6).Children(func() {
					found := false
					for _, repo := range d.repos {
						if repo.LastOpened == 0 || !strings.Contains(strings.ToLower(repo.Root), query) {
							continue
						}
						found = true
						native.Row(c).FillWidth().Gap(8).AlignItems(native.Center).Children(func() {
							label := filepath.Base(repo.Root)
							var open native.Element
							if repo.ID == d.selected {
								open = native.PrimaryButton(c, "")
							} else {
								open = native.Button(c, "")
							}
							open.Label(label).Grow(1).Padding(8).Radius(8).Disabled(d.busy).Tooltip(repo.Root)
							open.Children(func() {
								native.Column(c).Grow(1).Gap(2).Children(func() {
									native.Text(c, label).Bold().Ellipsis("…")
									native.Text(c, repo.Root).FontSize(12).Ellipsis("…")
								})
							})
							if open.Clicked() {
								d.repositoriesOpen = false
								d.open(repo.Root)
							}
							label = d.text("repo.forget")
							if iconButton(c, "", "remove", false).Label(label).Tooltip(label).Disabled(d.busy).Clicked() {
								d.forgetRepository(repo.ID)
							}
						})
					}
					if !found {
						native.Text(c, d.text("repo.noMatches")).TextColor(c.Theme().TextMuted).Padding(8)
					}
				})
			})
			if iconButton(c, d.text("repo.open")+"…", "open", true).Disabled(d.busy).Clicked() {
				d.repositoriesOpen = false
				d.chooseRepository()
			}
		})
	})
}

func (d *Desktop) forgetRepository(id string) {
	d.task(func(ctx context.Context) (func(), error) {
		if err := d.Client.Call(ctx, "repo.unregister", ipc.Params{RepoID: id}, nil); err != nil {
			return nil, err
		}
		var repos []repository.Info
		err := d.Client.Call(ctx, "repo.list", ipc.Params{}, &repos)
		return func() {
			d.repos = repos
			if d.selected == id {
				d.selectRepository("", "")
				d.page = 0
				if len(repos) > 0 && repos[0].LastOpened > 0 {
					d.selectRepository(repos[0].ID, repos[0].Root)
					d.refresh()
				}
			}
		}, err
	})
}
