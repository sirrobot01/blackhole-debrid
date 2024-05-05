package debrid

import (
	"bufio"
	"fmt"
	"github.com/anacrolix/torrent/metainfo"
	"goBlack/pkg"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

type Service interface {
	SubmitMagnet(torrent *pkg.Torrent) (*pkg.Torrent, error)
	CheckStatus(torrent *pkg.Torrent) (*pkg.Torrent, error)
	DownloadLink(torrent *pkg.Torrent) error
	Process(magnet string) (*pkg.Torrent, error)
	IsAvailable(torrent *pkg.Torrent) bool
}

type Debrid struct {
	Host             string `json:"host"`
	APIKey           string
	DownloadUncached bool
}

func NewDebrid(name, host, apiKey string, downloadUncached bool) Service {
	switch name {
	case "realdebrid":
		return NewRealDebrid(host, apiKey, downloadUncached)
	default:
		return NewRealDebrid(host, apiKey, downloadUncached)
	}
}

func GetTorrentInfo(filePath string) (*pkg.Torrent, error) {
	// Open and read the .torrent file
	if filepath.Ext(filePath) == ".torrent" {
		return getTorrentInfo(filePath)
	} else {
		return getMagnetInfo(filePath)
	}

}

func openMagnetFile(filePath string) string {
	file, err := os.Open(filePath)
	if err != nil {
		log.Println("Error opening file:", err)
		return ""
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			return
		}
	}(file) // Ensure the file is closed after the function ends

	// Create a scanner to read the file line by line
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		magnetLink := scanner.Text()
		if magnetLink != "" {
			return magnetLink
		}
	}

	// Check for any errors during scanning
	if err := scanner.Err(); err != nil {
		log.Println("Error reading file:", err)
	}
	return ""
}

func getMagnetInfo(filePath string) (*pkg.Torrent, error) {
	magnetLink := openMagnetFile(filePath)
	if magnetLink == "" {
		return nil, fmt.Errorf("error getting magnet from file")
	}
	magnetURI, err := url.Parse(magnetLink)
	if err != nil {
		return nil, fmt.Errorf("error parsing magnet link")
	}
	query := magnetURI.Query()
	xt := query.Get("xt")
	dn := query.Get("dn")

	// Extract BTIH
	parts := strings.Split(xt, ":")
	btih := ""
	if len(parts) > 2 {
		btih = parts[2]
	}
	torrent := &pkg.Torrent{
		InfoHash: btih,
		Name:     dn,
		Size:     0,
		Magnet:   magnetLink,
		Filename: filePath,
	}
	return torrent, nil
}

func getTorrentInfo(filePath string) (*pkg.Torrent, error) {
	mi, err := metainfo.LoadFromFile(filePath)
	if err != nil {
		return nil, err
	}
	hash := mi.HashInfoBytes()
	infoHash := hash.HexString()
	info, err := mi.UnmarshalInfo()
	if err != nil {
		return nil, err
	}
	torrent := &pkg.Torrent{
		InfoHash: infoHash,
		Name:     info.Name,
		Size:     info.Length,
		Magnet:   mi.Magnet(&hash, &info).String(),
		Filename: filePath,
	}
	return torrent, nil
}
