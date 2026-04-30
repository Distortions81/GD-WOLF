//go:build js

package main

func (g *game) requestQuit() error {
	g.setNotice("Please close page to quit")
	return nil
}
