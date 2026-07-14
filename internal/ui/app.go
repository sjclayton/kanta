package ui

import (
	"context"
	"fmt"
)

type PlaceholderKey struct {
	Label string `json:"label"`
	Row   int    `json:"row"`
	Col   int    `json:"col"`
}

type PlaceholderGrid struct {
	Width  int             `json:"width"`
	Height int             `json:"height"`
	Keys   []PlaceholderKey `json:"keys"`
}

type App struct {
	ctx context.Context
}

func New() *App {
	return &App{}
}

func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) PlaceholderGrid() PlaceholderGrid {
	rows := []string{"1234567890", "qwertyuiop", "asdfghjkl", "zxcvbnm"}
	g := PlaceholderGrid{Width: 10, Height: len(rows), Keys: make([]PlaceholderKey, 0, 40)}
	for r, row := range rows {
		for c, ch := range row {
			g.Keys = append(g.Keys, PlaceholderKey{
				Label: fmt.Sprintf("%c", ch),
				Row:   r,
				Col:   c,
			})
		}
	}
	return g
}
