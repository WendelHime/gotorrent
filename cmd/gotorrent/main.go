package main

import (
	"flag"
	"log/slog"
	"os"

	"github.com/WendelHime/gotorrent/internal/decoder"
	"github.com/WendelHime/gotorrent/internal/logic"
)

func main() {
	var torrentPath string
	var outputDir string
	flag.StringVar(&torrentPath, "torrent", "~/Downloads/debian-12.5.0-amd64-netinst.iso.torrent", "Specify the input torrent file")
	flag.StringVar(&outputDir, "output", "~/Downloads", "Specify the output directory")
	flag.Parse()
	f, err := os.Open(torrentPath)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	// Create a new logger and generate log file
	logOut, err := os.Create("log.txt")
	if err != nil {
		panic(err)
	}
	defer logOut.Close()
	logger := slog.New(slog.NewJSONHandler(logOut, &slog.HandlerOptions{Level: slog.LevelError}))

	downloader := logic.NewDownloader(decoder.NewDecoder(), logger)
	err = downloader.Download(f, outputDir)
	if err != nil {
		logger.Error("failed to download torrent", slog.Any("error", err))
		return
	}
}
