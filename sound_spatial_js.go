//go:build js && wasm

package main

func (g *game) playWorldSound(id soundID, x, y float64) {
	g.playSound(id)
}
