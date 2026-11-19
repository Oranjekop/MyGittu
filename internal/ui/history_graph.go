package ui

import (
	"strings"

	native "github.com/egoist/mygo/ui"
	"mygittu/internal/app"
)

type graphLane struct {
	hash  string
	color int
}

type graphEdge struct {
	from, to, color int
}

type commitGraphRow struct {
	lane, color     int
	merge, incoming bool
	through         []graphEdge
	outgoing        []graphEdge
}

const graphLaneSpacing = 12
const graphStrokeWidth = 2

var graphColors = [...]native.Color{
	native.Hex("#3785df"), native.Hex("#389e65"), native.Hex("#d87a35"),
	native.Hex("#9b65d5"), native.Hex("#d15374"), native.Hex("#1b9aa2"),
}

type commitGraphBuilder struct {
	lanes     []graphLane
	nextColor int
}

func buildCommitGraph(commits []app.Commit, filtered bool) ([]commitGraphRow, float32) {
	var builder commitGraphBuilder
	return builder.appendRows(commits, filtered)
}

func (b *commitGraphBuilder) appendRows(commits []app.Commit, filtered bool) ([]commitGraphRow, float32) {
	rows := make([]commitGraphRow, len(commits))
	visible := make(map[string]bool, len(commits))
	if filtered {
		for _, commit := range commits {
			visible[commit.Hash] = true
		}
	}
	lanes, nextColor := b.lanes, b.nextColor
	find := func(hash string) int {
		for i, lane := range lanes {
			if lane.hash == hash {
				return i
			}
		}
		return -1
	}
	assign := func(lane graphLane) int {
		for i, slot := range lanes {
			if slot.hash == "" {
				lanes[i] = lane
				return i
			}
		}
		lanes = append(lanes, lane)
		return len(lanes) - 1
	}
	for i, commit := range commits {
		lane := find(commit.Hash)
		incoming := lane >= 0
		if !incoming {
			lane = assign(graphLane{commit.Hash, nextColor})
			nextColor++
		}
		color := lanes[lane].color
		row := commitGraphRow{lane: lane, color: color, merge: len(commit.Parents) > 1, incoming: incoming}
		for j, active := range lanes {
			if j != lane && active.hash != "" {
				row.through = append(row.through, graphEdge{j, j, active.color})
			}
		}
		lanes[lane] = graphLane{}
		for j, parent := range commit.Parents {
			if filtered && !visible[parent] {
				continue
			}
			dest := find(parent)
			if dest < 0 {
				if j == 0 {
					lanes[lane] = graphLane{parent, color}
					dest = lane
				} else {
					dest = assign(graphLane{parent, nextColor})
					nextColor++
				}
			}
			row.outgoing = append(row.outgoing, graphEdge{lane, dest, lanes[dest].color})
		}
		rows[i] = row
	}
	b.lanes, b.nextColor = lanes, nextColor
	return rows, max(56, float32(len(lanes)*graphLaneSpacing+14))
}

func (g commitGraphRow) draw(p *native.Painter, r native.Rect) {
	x := func(lane int) float32 { return r.X + 10 + graphLaneSpacing*float32(lane) }
	top, bottom := r.Y, r.Y+r.H
	middle := (top + bottom) / 2
	color := graphColors[g.color%len(graphColors)]
	stroke := func(from, to, y0, y1 float32, color native.Color) {
		var line native.Path
		line.Cap(native.ButtCap)
		if y0 == top {
			// Extend through the cell clip so fractional DPI does not fade
			// the two stroke ends at a shared row boundary.
			y0 -= 1
		}
		line.MoveTo(from, y0)
		if from == to {
			line.LineTo(to, y1)
		} else {
			bend := (y1 - y0) * 0.45
			line.CubeTo(from, y0+bend, to, y1-bend, to, y1)
		}
		if y1 == bottom {
			line.LineTo(to, y1+1)
		}
		p.StrokePath(&line, graphStrokeWidth, color)
	}
	for _, edge := range g.outgoing {
		stroke(x(edge.from), x(edge.to), middle, bottom, graphColors[edge.color%len(graphColors)])
	}
	for _, edge := range g.through {
		stroke(x(edge.from), x(edge.to), top, bottom, graphColors[edge.color%len(graphColors)])
	}
	if g.incoming {
		stroke(x(g.lane), x(g.lane), top, middle, color)
	}
	if g.merge {
		var node native.Path
		node.MoveTo(x(g.lane), middle-4).LineTo(x(g.lane)+4, middle).LineTo(x(g.lane), middle+4).LineTo(x(g.lane)-4, middle).Close()
		p.FillPath(&node, color)
	} else {
		p.Fill(native.Rect{X: x(g.lane) - 3, Y: middle - 3, W: 6, H: 6}, color, 3)
	}
}

func (d *Desktop) isHistoryHead() bool {
	if d.logRow < 0 || d.logRow >= len(d.logs) {
		return false
	}
	refs := d.logs[d.logRow].Refs
	return refs == "HEAD" || strings.HasPrefix(refs, "HEAD, ") || strings.HasPrefix(refs, "HEAD -> ")
}
