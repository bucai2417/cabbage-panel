package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/bucai2417/cabbage-panel/internal/panel"
)

var version = "1.0.0"
var commit = "dev"
var updateKey = ""

func main() {
	addr := flag.String("addr", ":5700", "HTTP listen address")
	data := flag.String("data", "data", "Persistent data directory")
	webDir := flag.String("web", "", "Serve frontend from a local directory instead of embedded assets (development)")
	showVersion := flag.Bool("version", false, "Print version")
	flag.Parse()
	if *showVersion {
		fmt.Printf("大白菜面板 %s (%s)\n", version, commit)
		return
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	app, err := panel.New(*data, version, commit, updateKey)
	if err != nil {
		log.Fatal(err)
	}
	if err := app.SetWebDir(*webDir); err != nil {
		log.Fatal(err)
	}
	if err := app.Serve(ctx, *addr); err != nil {
		log.Fatal(err)
	}
}
