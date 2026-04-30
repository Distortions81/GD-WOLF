//go:build !js

package main

import "github.com/hajimehoshi/ebiten/v2"

func (g *game) requestQuit() error {
	return ebiten.Termination
}
