package integration

import (
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"io"
	"log/slog"
	"os"
	"testing"

	"github.com/WendelHime/gotorrent/internal/decoder"
	"github.com/WendelHime/gotorrent/internal/logic"
	"github.com/cucumber/godog"
)

type IntegrationTest struct {
	Downloader logic.Downloader
	file       io.ReadCloser
}

func (i *IntegrationTest) iDownloadTheFile() error {
	return i.Downloader.Download(i.file, "./")
}

func (i *IntegrationTest) iHaveATorrentFile(torrentPath string) error {
	f, err := os.Open(torrentPath)
	if err != nil {
		return err
	}

	i.file = f

	return nil
}

func (i *IntegrationTest) theOutputHashShouldMatch(expectedHash string) error {
	defer i.file.Close()
	filename := "./sample.txt"
	output, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer output.Close()

	hash := sha1.New()
	if _, err := io.Copy(hash, output); err != nil {
		return err
	}
	hashSum := hash.Sum(nil)
	hashSumString := hex.EncodeToString(hashSum)

	if hashSumString != expectedHash {
		return errors.New("hashes do not match")
	}

	return os.Remove(filename)
}

func InitializeScenario(ctx *godog.ScenarioContext) {
	i := &IntegrationTest{
		Downloader: logic.NewDownloader(decoder.NewDecoder(), slog.New(slog.NewTextHandler(io.Discard, nil))),
	}
	ctx.Step(`^I download the file$`, i.iDownloadTheFile)
	ctx.Step(`^I have a torrent file "([^"]*)"$`, i.iHaveATorrentFile)
	ctx.Step(`^The output hash should match "([^"]*)"$`, i.theOutputHashShouldMatch)
}

func TestFeatures(t *testing.T) {
	suite := godog.TestSuite{
		ScenarioInitializer: InitializeScenario,
		Options: &godog.Options{
			Format:   "pretty",
			Paths:    []string{"features"},
			TestingT: t, // Testing instance that will run subtests.
		},
	}

	if suite.Run() != 0 {
		t.Fatal("non-zero status returned, failed to run feature tests")
	}
}
