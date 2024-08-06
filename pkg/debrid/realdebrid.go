package debrid

import (
	"encoding/json"
	"fmt"
	"goBlack/common"
	"goBlack/pkg"
	"goBlack/pkg/debrid/schema"
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
	client           *common.RLHTTPClient
}

func (r *RealDebrid) Process(arr *pkg.Arr, magnet string) (*pkg.Torrent, error) {
	torrent, err := GetTorrentInfo(magnet)
	torrent.Arr = arr
	if err != nil {
		return nil, err
	}
	err = torrent.UpsertDB()
	if err != nil {
		return nil, err
	}
	log.Printf("Torrent Name: %s", torrent.Name)
	if !r.DownloadUncached {
		if !r.IsAvailable(torrent) {
			return nil, fmt.Errorf("torrent is not cached")
		}
	}
	err = torrent.UpsertDB()
	if err != nil {
		return nil, err
	}
	torrent, err = r.SubmitMagnet(torrent)

	if err != nil || torrent.Id == "" {
		return nil, err
	}
	err = torrent.UpsertDB()
	if err != nil {
		return nil, err
	}

	return r.CheckStatus(torrent)
}

func (r *RealDebrid) IsAvailable(torrent *pkg.Torrent) bool {
	url := fmt.Sprintf("%s/torrents/instantAvailability/%s", r.Host, torrent.InfoHash)
	resp, err := r.client.MakeRequest(http.MethodGet, url, nil)
	if err != nil {
		return false
	}
	var data schema.RealDebridAvailabilityResponse
	err = json.Unmarshal(resp, &data)
	if err != nil {
		return false
	}
	hosters, exists := data[strings.ToLower(torrent.InfoHash)]
	if !exists || len(hosters) < 1 {
		log.Printf("Torrent: %s not cached", torrent.Name)
		return false
	}
	log.Printf("Torrent: %s is cached", torrent.Name)
	return true
}

func (r *RealDebrid) SubmitMagnet(torrent *pkg.Torrent) (*pkg.Torrent, error) {
	url := fmt.Sprintf("%s/torrents/addMagnet", r.Host)
	payload := gourl.Values{
		"magnet": {torrent.Magnet},
	}
	var data schema.RealDebridAddMagnetSchema
	resp, err := r.client.MakeRequest(http.MethodPost, url, strings.NewReader(payload.Encode()))
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(resp, &data)
	log.Printf("Torrent: %s added with id: %s\n", torrent.Name, data.Id)
	torrent.Id = data.Id

	return torrent, nil
}

func (r *RealDebrid) CheckStatus(torrent *pkg.Torrent) (*pkg.Torrent, error) {
	url := fmt.Sprintf("%s/torrents/info/%s", r.Host, torrent.Id)
	for {
		resp, err := r.client.MakeRequest(http.MethodGet, url, nil)
		if err != nil {
			return torrent, err
		}
		var data schema.RealDebridTorrentInfo
		err = json.Unmarshal(resp, &data)
		status := data.Status
		torrent.Folder = common.RemoveExtension(data.OriginalFilename)
		if status == "error" || status == "dead" || status == "magnet_error" {
			return torrent, fmt.Errorf("torrent: %s has error", torrent.Name)
		} else if status == "waiting_files_selection" {
			files := make([]pkg.File, 0)
			for _, f := range data.Files {
				name := f.Path
				if !common.RegexMatch(common.VIDEOMATCH, name) && !common.RegexMatch(common.SUBMATCH, name) {
					continue
				}
				fileId := f.ID
				file := &pkg.File{
					Name: name,
					Path: filepath.Join(torrent.Folder, name),
					Size: int64(f.Bytes),
					Id:   strconv.Itoa(fileId),
				}
				files = append(files, *file)
			}
			torrent.Files = files
			_ = torrent.UpsertDB()
			if len(files) == 0 {
				return torrent, fmt.Errorf("no video files found")
			}
			filesId := make([]string, 0)
			for _, f := range files {
				filesId = append(filesId, f.Id)
			}
			p := gourl.Values{
				"files": {strings.Join(filesId, ",")},
			}
			payload := strings.NewReader(p.Encode())
			_, err = r.client.MakeRequest(http.MethodPost, fmt.Sprintf("%s/torrents/selectFiles/%s", r.Host, torrent.Id), payload)
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

func NewRealDebrid(dc common.DebridConfig) *RealDebrid {
	rl := common.ParseRateLimit(dc.RateLimit)
	headers := map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", dc.APIKey),
	}
	client := common.NewRLHTTPClient(rl, headers)
	return &RealDebrid{
		Host:             dc.Host,
		APIKey:           dc.APIKey,
		DownloadUncached: dc.DownloadUncached,
		client:           client,
	}
}
