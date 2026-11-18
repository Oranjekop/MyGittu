package ui

import (
	"context"
	"path/filepath"
	"strings"

	native "github.com/egoist/mygo/ui"
	"mygittu/internal/ipc"
	"mygittu/internal/repository"
)

func (d *Desktop) repositoryMenu(c *native.Context) {
	label := d.text("repo.recent")
	paths := make(map[string]int, len(d.repos))
	for _, repo := range d.repos {
		name := filepath.Base(repo.Root)
		if repo.LastOpened > 0 || repo.ID == d.selected {
			paths[strings.ToLower(name)]++
		}
		if repo.ID == d.selected {
			label = name
		}
	}
	native.MenuButton(c, label, func(menu *native.Menu) {
		for _, repo := range d.repos {
			if repo.LastOpened == 0 && repo.ID != d.selected {
				continue
			}
			name := filepath.Base(repo.Root)
			if paths[strings.ToLower(name)] > 1 {
				name = repo.Root
			}
			current := repo.ID == d.selected
			if menu.Item(name).Radio(current).Tooltip(repo.Root).Chosen() && !current {
				d.open(repo.Root)
			}
		}
		if len(paths) > 0 {
			menu.Separator()
		}
		if menu.Item(d.text("repo.search")).Shortcut(native.Cmd, native.KeyK).Chosen() {
			d.repositorySearch = ""
			d.repositoriesOpen = true
		}
		if menu.Item(d.text("repo.open") + "…").Chosen() {
			d.chooseRepository()
		}
	}).Height(27).Padding(0, 8).MaxWidth(200).Tooltip(d.text("repo.recent")).Disabled(d.busy)
}

func (d *Desktop) repositoryDialog(c *native.Context) {
	native.Modal(c, &d.repositoriesOpen, func() {
		native.Column(c).Width(620).Gap(panelSpacing).Children(func() {
			native.Row(c).Gap(controlSpacing).AlignItems(native.Center).Children(func() {
				native.Text(c, d.text("repo.recent")).FontSize(19).Bold().Grow(1)
				if IconButton(c, d.text("close"), "close", false).Clicked() {
					d.repositoriesOpen = false
				}
			})
			native.TextInput(c, &d.repositorySearch).Placeholder(d.text("repo.search")).FillWidth()
			query := strings.ToLower(strings.TrimSpace(d.repositorySearch))
			native.Scroll(c).Height(300).FillWidth().Children(func() {
				native.Column(c).FillWidth().Gap(controlSpacing).Children(func() {
					found := false
					for _, repo := range d.repos {
						if repo.LastOpened == 0 || !strings.Contains(strings.ToLower(repo.Root), query) {
							continue
						}
						found = true
						native.Row(c).FillWidth().Gap(controlSpacing).AlignItems(native.Center).Children(func() {
							label := filepath.Base(repo.Root)
							var open native.Element
							if repo.ID == d.selected {
								open = native.PrimaryButton(c, "")
							} else {
								open = native.Button(c, "")
							}
							open.Label(label).Grow(1).Padding(controlSpacing).Radius(8).Disabled(d.busy).Tooltip(repo.Root)
							open.Gap(controlSpacing).AlignItems(native.Center).Children(func() {
								native.Icon(c, icons["open"]).Width(14).Height(14)
								native.Column(c).Grow(1).Gap(detailSpacing).Children(func() {
									native.Text(c, label).Bold().Ellipsis("…")
									native.Text(c, repo.Root).FontSize(11).Ellipsis("…")
								})
							})
							if open.Clicked() {
								d.repositoriesOpen = false
								d.open(repo.Root)
							}
							label = d.text("repo.forget")
							if IconButton(c, "", "remove", false).Label(label).Tooltip(label).Disabled(d.busy).Clicked() {
								d.forgetRepository(repo.ID)
							}
						})
					}
					if !found {
						native.Text(c, d.text("repo.noMatches")).TextColor(c.Theme().TextMuted).Padding(controlSpacing)
					}
				})
			})
			if IconButton(c, d.text("repo.open")+"…", "open", true).Disabled(d.busy).Clicked() {
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
