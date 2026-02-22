//go:build !systray

package main

func runSystray(_ string) {
	// systray não compilado; use: go build -tags systray
}
