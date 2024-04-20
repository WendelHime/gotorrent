# GoTorrent Client

GoTorrent is a lightweight and efficient command-line BitTorrent client written in Go.

## Features

- Download torrents quickly and efficiently.
- Support for multi-file torrents.
- Simple and easy-to-use command-line interface.

### Installation

To install GoTorrent, you need to have Go installed on your system. Then, you can use the following command:

```bash
go get github.com/WendelHime/gotorrent
```

### Build
```bash
go build -o gotorrent ./cmd/gotorrent 
```

### Usage
```bash
gotorrent -torrent <torrent-filepath> -output <output-directory>
```

- `<torrent-filepath>`: Path to the torrent file you want to download
- `<output-directory>`: Directory where the downloaded files will be saved. If the directory doesn't exist, it will be created.

Example:
```bash
./gotorrent -torrent internal/integration/sample.torrent -output ./output
```

### Testing

```bash
go test -cover -race ./...
```

### Known issues

- This program currently isn't supporting UDP trackers so some torrent files won't be able to download at this moment.
- I've only tested with a couple torrent files like for [debian images](https://www.debian.org/CD/http-ftp/#mirrors) - look for the best mirror for you and if there's any torrent file available you can try it (don't forget to verify/validate hashes!)

### Future improvements

- Keep track of stored pieces and download missing pieces if the program was interrupted
- Implement correctly the UDP tracker
- Add more unit tests/improve test coverage
- Seed data
