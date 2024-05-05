package debrid

import (
	"encoding/json"
	"fmt"
	"goBlack/common"
	"goBlack/pkg"
	"log"
	"net/http"
	gourl "net/url"
	"path/filepath"
	"strconv"
	"strings"
)

type RealDebrid struct {
	Host             string `json:"host"`
	APIKey           string
	DownloadUncached bool
}

func (r *RealDebrid) Process(magnet string) (*pkg.Torrent, error) {
	torrent, err := GetTorrentInfo(magnet)
	if err != nil {
		return nil, err
	}
	log.Printf("Torrent Name: %s", torrent.Name)
	if !r.DownloadUncached {
		if !r.IsAvailable(torrent) {
			return nil, fmt.Errorf("torrent is not cached")
		}
	}
	torrent, err = r.SubmitMagnet(torrent)
	if err != nil || torrent.Id == "" {
		return nil, err
	}
	return r.CheckStatus(torrent)
}

func (r *RealDebrid) IsAvailable(torrent *pkg.Torrent) bool {
	url := fmt.Sprintf("%s/torrents/instantAvailability/%s", r.Host, torrent.InfoHash)
	headers := map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", r.APIKey),
	}
	resp, err := common.NewRequest(http.MethodGet, url, nil, headers)
	if err != nil {
		return false
	}
	var data map[string]any
	err = json.Unmarshal(resp, &data)
	if err != nil {
		return false
	}
	v, ok := data[strings.ToLower(torrent.InfoHash)].(map[string]any)
	if !ok || v == nil {
		log.Printf("Torrent: %s not cached", torrent.Name)
		return false
	}
	s, ok := v["rd"].([]any)
	if !ok || len(s) == 0 || s == nil {
		log.Printf("Torrent: %s not cached", torrent.Name)
		return false
	}
	log.Printf("Torrent: %s is cached", torrent.Name)
	return true
}

func (r *RealDebrid) SubmitMagnet(torrent *pkg.Torrent) (*pkg.Torrent, error) {
	headers := map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", r.APIKey),
	}

	url := fmt.Sprintf("%s/torrents/addMagnet", r.Host)
	payload := gourl.Values{
		"magnet": {torrent.Magnet},
	}
	var data map[string]any
	resp, err := common.NewRequest(http.MethodPost, url, strings.NewReader(payload.Encode()), headers)

	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(resp, &data)
	torrentId := data["id"].(string)
	log.Printf("Torrent: %s added with id: %s\n", torrent.Name, torrentId)
	torrent.Id = torrentId

	return torrent, nil
}

func (r *RealDebrid) CheckStatus(torrent *pkg.Torrent) (*pkg.Torrent, error) {
	url := fmt.Sprintf("%s/torrents/info/%s", r.Host, torrent.Id)
	headers := map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", r.APIKey),
	}
	for {
		resp, err := common.NewRequest(http.MethodGet, url, nil, headers)
		if err != nil {
			return torrent, err
		}
		var data map[string]any
		err = json.Unmarshal(resp, &data)
		status := data["status"].(string)
		torrent.Folder = common.RemoveExtension(data["original_filename"].(string))
		if status == "error" || status == "dead" || status == "magnet_error" {
			return torrent, fmt.Errorf("torrent: %s has error", torrent.Name)
		} else if status == "waiting_files_selection" {
			files := make([]pkg.File, 0)
			filesObj := data["files"].([]any)
			for _, fs := range filesObj {
				f := fs.(map[string]any)
				name := f["path"].(string)
				if !common.RegexMatch(common.VIDEOMATCH, name) && !common.RegexMatch(common.SUBMATCH, name) {
					continue
				}
				fileId := int(f["id"].(float64))
				file := &pkg.File{
					Name: name,
					Path: filepath.Join(torrent.Folder, name),
					Size: int64(f["bytes"].(float64)),
					Id:   strconv.Itoa(fileId),
				}
				files = append(files, *file)
			}
			torrent.Files = files
			if len(files) == 0 {
				return torrent, fmt.Errorf("no video files found")
			}
			filesId := make([]string, 0)
			for _, f := range files {
				filesId = append(filesId, f.Id)
			}
			payload := gourl.Values{
				"files": {strings.Join(filesId, ",")},
			}
			_, err = common.NewRequest(http.MethodPost, fmt.Sprintf("%s/torrents/selectFiles/%s", r.Host, torrent.Id), strings.NewReader(payload.Encode()), headers)
			if err != nil {
				return torrent, err
			}
		} else if status == "downloaded" {
			log.Printf("Torrent: %s downloaded\n", torrent.Name)
			err = r.DownloadLink(torrent)
			if err != nil {
				return torrent, err
			}
			break
		} else if status == "downloading" {
			return torrent, fmt.Errorf("torrent is uncached")
		}

	}
	return torrent, nil
}

func (r *RealDebrid) DownloadLink(torrent *pkg.Torrent) error {
	return nil
}

func NewRealDebrid(host, apiKey string, downloadUncached bool) *RealDebrid {
	return &RealDebrid{
		Host:             host,
		APIKey:           apiKey,
		DownloadUncached: downloadUncached,
	}
}
