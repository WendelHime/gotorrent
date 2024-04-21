package logic

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"errors"
	"io"
	"log/slog"
	"math"
	"math/rand"
	"os"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/WendelHime/gotorrent/internal/decoder"
	"github.com/WendelHime/gotorrent/internal/p2p"
	"github.com/WendelHime/gotorrent/internal/shared/models"
	"github.com/WendelHime/gotorrent/internal/tracker"
	"github.com/schollz/progressbar/v3"
)

type Downloader interface {
	Download(metafile io.Reader, outputDir string) error
}

type downloader struct {
	clientID string
	d        decoder.MetafileDecoder
	log      *slog.Logger
}

func NewDownloader(d decoder.MetafileDecoder, logger *slog.Logger) Downloader {
	return &downloader{d: d, log: logger, clientID: generateRandomPeerID()}
}

func generateRandomPeerID() string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	// Seed the random number generator
	r := rand.New(rand.NewSource(int64(time.Now().Nanosecond())))

	// Initialize a byte slice to hold the generated peer ID
	peerID := make([]byte, 20)

	// Fill the byte slice with random characters from the charset
	for i := range peerID {
		peerID[i] = charset[r.Intn(len(charset))]
	}

	return string(peerID)
}

type peerClient struct {
	client p2p.P2PClient
	peer   *models.Peer
}

func (d *downloader) Download(metafile io.Reader, outputDir string) error {
	d.log.Info("creating output directory", slog.String("output_dir", outputDir))
	err := createOutputDir(outputDir)
	if err != nil {
		return err
	}

	d.log.Info("decoding metafile")
	meta, err := d.d.Decode(metafile)
	if err != nil {
		return err
	}

	bar := progressbar.DefaultBytes(int64(-1), "retrieving peers")
	d.log.Info("retrieving peers", slog.Any("len piece hashes", len(meta.Info.PiecesHashes)))
	peers, err := d.retrievePeers(meta)
	if err != nil {
		return err
	}

	if len(peers) == 0 {
		return errors.New("no peers found")
	}

	peerClients := make([]peerClient, 0)
	for _, peer := range peers {
		client := p2p.NewClient(d.clientID)
		peerClients = append(peerClients, peerClient{client: client, peer: &peer})
	}

	piecesQueue := make(chan models.Piece, len(meta.Info.PiecesHashes))
	writeQueue := make(chan models.Piece, len(meta.Info.PiecesHashes))
	pieces := make([]models.Piece, len(meta.Info.PiecesHashes))
	for i := range pieces {
		pieces[i] = models.Piece{
			Index: i,
			Hash:  meta.Info.PiecesHashes[i],
		}
		piecesQueue <- pieces[i]
	}

	if meta.Info.Length == 0 {
		meta.Info.Length = calculateTotalLength(meta.Info.Files)
	}
	bar.ChangeMax(meta.Info.Length)
	bar.Describe("downloading")
	var wg sync.WaitGroup
	wg.Add(len(pieces))

	for _, peerClient := range peerClients {
		go d.downloadPieces(piecesQueue, writeQueue, &wg, meta, peerClient)
	}

	var writeWaitGroup sync.WaitGroup
	writeWaitGroup.Add(1)

	// map file lengths begin and end from info total length
	filePositions := make(map[string]FilePosition)
	index := 0
	for _, file := range meta.Info.Files {
		begin := index
		dirpath := path.Join(outputDir, strings.Join(file.Path[:len(file.Path)-1], "/"))
		os.MkdirAll(dirpath, 0755)
		filepath := path.Join(dirpath, file.Path[len(file.Path)-1])
		filePositions[filepath] = FilePosition{
			Begin: index,
			End:   begin + file.Length,
		}
		index += file.Length
	}

	d.log.Info("file positions", slog.Any("file_positions", filePositions))

	go func() {
		defer writeWaitGroup.Done()
		for piece := range writeQueue {
			// if single file torrent
			if len(meta.Info.Files) == 0 {
				filepath := path.Join(outputDir, meta.Info.Name)
				n, err := d.writeFile(filepath, meta, piece)
				if err != nil {
					d.log.Error("failed to save piece to file", slog.Any("error", err))
					continue
				}
				d.log.Info("piece saved to file", slog.Any("piece", piece.Index), slog.Int("amount_pieces", len(pieces)))
				bar.Add(n)
				continue
			}

			// for multifile torrent
			for _, file := range meta.Info.Files {
				fp := path.Join(file.Path...)
				fp = path.Join(outputDir, fp)
				for _, block := range piece.Blocks {
					pieceOffset := piece.Index*meta.Info.PieceLength + block.Begin
					if pieceOffset >= filePositions[fp].Begin && pieceOffset < filePositions[fp].End {
						n, err := d.writeFile(fp, meta, piece)
						if err != nil {
							d.log.Error("failed to save piece to file", slog.Any("error", err))
							continue
						}
						bar.Add(n)
					}
				}
			}
		}
	}()
	wg.Wait()
	close(writeQueue)
	writeWaitGroup.Wait()

	return nil
}

type FilePosition struct {
	Begin int
	End   int
}

func (d *downloader) retrievePeers(metafile models.Metafile) ([]models.Peer, error) {
	d.log.Info("retrieving peers from tracker", slog.String("announce", metafile.Announce))
	t := tracker.NewTracker(metafile.Announce, d.clientID)
	peers := make([]models.Peer, 0)
	p, err := t.GetPeers(metafile)
	if err != nil && err != io.EOF {
		d.log.Warn("failed to get peers", slog.Any("error", err))
	}
	peers = append(peers, p...)
	mutex := sync.Mutex{}
	var wg sync.WaitGroup
	for _, announceLists := range metafile.AnnounceList {
		for _, announce := range announceLists {
			if announce == metafile.Announce {
				continue
			}
			wg.Add(1)

			go func() {
				defer wg.Done()
				d.log.Info("retrieving peers from tracker", slog.String("announce", announce))
				t := tracker.NewTracker(announce, d.clientID)
				p, err := t.GetPeers(metafile)
				if err != nil && err != io.EOF {
					d.log.Warn("failed to get peers", slog.Any("error", err))
					return
				}

				mutex.Lock()
				peers = append(peers, p...)
				mutex.Unlock()
			}()
			time.Sleep(50 * time.Millisecond)
		}
	}
	wg.Wait()

	d.log.Info("retrieved peers", slog.Any("peers", peers))
	return peers, nil
}

var ErrMissingPiece = errors.New("missing piece")
var ErrPeerChoked = errors.New("peer choked")
var ErrUnexpectedMessage = errors.New("unexpected message")

func (d *downloader) downloadPiece(metafile models.Metafile, peerClient peerClient, pieceIndex int) ([]models.Block, error) {
	if !peerClient.client.Connected() {
		err := peerClient.client.Connect(peerClient.peer.Addr)
		if err != nil {
			return nil, err
		}
		defer peerClient.client.Disconnect()

		err = peerClient.client.Handshake(metafile.InfoHash)
		if err != nil {
			return nil, err
		}

		msg, err := peerClient.client.ReadMessage()
		if err != nil || msg.ID != models.MessageIDBitfield {
			return nil, err
		}

		decodeAvailablePiecesFromPeer(msg.Payload, peerClient.peer)
	}

	if _, ok := peerClient.peer.HavePieces[pieceIndex]; !ok {
		return nil, ErrMissingPiece
	}

	err := peerClient.client.WriteMessage(models.PeerMessage{ID: models.MessageIDInterested, Length: 1})
	if err != nil {
		return nil, err
	}

	pieceLength := calculatePieceLength(metafile.Info.Length, metafile.Info.PieceLength, pieceIndex)
	blockSize := 16 * 1024
	expectingBlocks := int(math.Ceil(float64(pieceLength) / float64(blockSize)))

	blocks := make([]models.Block, 0)
	var pieceRequested bool

	for {
		msg, err := peerClient.client.ReadMessage()
		if err != nil {
			return nil, err
		}

		switch msg.ID {
		case models.MessageIDChoke:
			peerClient.client.Disconnect()
			return nil, ErrPeerChoked
		case models.MessageIDUnchoke:
			if !pieceRequested {
				_, err = requestBlocks(peerClient.client, pieceIndex, pieceLength)
				if err != nil {
					return nil, err
				}
				pieceRequested = true
			}
		case models.MessageIDHave:
			peerHavePieceIndex := int(msg.Payload[0])
			peerClient.peer.HavePieces[peerHavePieceIndex] = struct{}{}
		case models.MessageIDPiece:
			block := models.Block{
				Index: int(binary.BigEndian.Uint32(msg.Payload[:4])),
				Begin: int(binary.BigEndian.Uint32(msg.Payload[4:8])),
				Data:  msg.Payload[8:],
			}
			blocks = append(blocks, block)

			if len(blocks) == expectingBlocks {
				return blocks, nil
			}
		default:
			return nil, ErrUnexpectedMessage
		}

		if len(blocks) == expectingBlocks {
			break
		}
	}

	return blocks, nil
}

func calculatePieceLength(totalLength, pieceLength, index int) int {
	pieceOffset := index * pieceLength
	left := totalLength - pieceOffset
	return min(left, pieceLength)
}

func decodeAvailablePiecesFromPeer(bitfield []byte, peer *models.Peer) {
	for byteIndex, bitfieldByte := range bitfield {
		for i := 0; i < 8; i++ {
			bitIndex := byteIndex*8 + i
			if byteIndex == peer.PiecesWanted {
				return
			}
			havePiece := bitfieldByte>>uint(7-i)&1 == 1
			if havePiece {
				peer.HavePieces[bitIndex] = struct{}{}
			}
		}
	}
}

func requestBlocks(client p2p.P2PClient, index int, length int) (int, error) {
	blockSize := 16 * 1024
	blocksRequested := 0
	for bytesToRequest := length; bytesToRequest > 0; bytesToRequest -= blockSize {
		payload := make([]byte, 12)
		binary.BigEndian.PutUint32(payload, uint32(index))
		binary.BigEndian.PutUint32(payload[4:], uint32(blockSize*blocksRequested))
		binary.BigEndian.PutUint32(payload[8:], uint32(min(blockSize, bytesToRequest)))
		blocksRequested++

		msg := models.PeerMessage{ID: models.MessageIDRequest, Payload: payload, Length: len(payload) + 1}
		err := client.WriteMessage(msg)
		if err != nil {
			return blocksRequested, err
		}
	}
	return blocksRequested, nil
}

func (d *downloader) downloadPieces(piecesQueue chan models.Piece, writeQueue chan models.Piece, wg *sync.WaitGroup, metainfo models.Metafile, peerClient peerClient) {
	var err error
	for piece := range piecesQueue {
		piece.Blocks, err = d.downloadPiece(metainfo, peerClient, piece.Index)
		if err != nil {
			piecesQueue <- piece
			d.log.Warn("failed to download piece", slog.Any("error", err))
			continue
		}

		piece.Blocks = sortBlocks(piece.Blocks)

		isValid, err := checkHash(piece)
		if err != nil {
			piecesQueue <- piece
			d.log.Warn("failed to check hash", slog.Any("error", err))
			continue
		}

		if !isValid {
			piecesQueue <- piece
			d.log.Warn("piece is invalid")
			continue
		}

		writeQueue <- piece
		wg.Done()
	}
}

func sortBlocks(blocks []models.Block) []models.Block {
	sort.Slice(blocks, func(i, j int) bool {
		return blocks[i].Begin < blocks[j].Begin
	})
	return blocks
}

func checkHash(piece models.Piece) (bool, error) {
	hash := sha1.New()
	for _, block := range piece.Blocks {
		_, err := hash.Write(block.Data)
		if err != nil {
			return false, err
		}
	}
	return bytes.Equal(hash.Sum(nil), piece.Hash.Hash[:]), nil
}

func createOutputDir(outputDir string) error {
	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		err := os.Mkdir(outputDir, 0755)
		if err != nil {
			return err
		}
	}
	return nil
}

func (d *downloader) writeFile(filepath string, meta models.Metafile, piece models.Piece) (int, error) {
	file, err := os.OpenFile(filepath, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	writtenBytes := 0
	for _, block := range piece.Blocks {
		pieceOffset := piece.Index*meta.Info.PieceLength + block.Begin
		n, err := file.WriteAt(block.Data, int64(pieceOffset))
		if err != nil {
			return writtenBytes, err
		}

		writtenBytes += n
	}

	return writtenBytes, nil
}

func calculateTotalLength(files []models.File) int {
	totalLength := 0
	for _, file := range files {
		totalLength += file.Length
	}
	return totalLength
}
