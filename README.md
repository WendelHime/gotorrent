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

- This program only support ipv4 addresses.
- I've only tested with a couple torrent files like for [debian images](https://www.debian.org/CD/http-ftp/#mirrors) - look for the best mirror for you and if there's any torrent file available you can try it (don't forget to verify/validate hashes!)

### Future improvements

- Add support to ipv6 addresses
- Keep track of stored pieces and download missing pieces if the program was interrupted
- Add more unit tests/improve test coverage
- Seed data
