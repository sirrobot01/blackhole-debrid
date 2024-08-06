package pkg

import (
	"database/sql"
	"encoding/json"
	"goBlack/common"
	"log"
	"net/http"
	gourl "net/url"
	"os"
	"strconv"
	"strings"
)

type Arr struct {
	WatchFolder     string              `json:"watch_folder"`
	CompletedFolder string              `json:"completed_folder"`
	Debrid          common.DebridConfig `json:"debrid"`
	Token           string              `json:"token"`
	URL             string              `json:"url"`
	Client          *common.RLHTTPClient
}

type ArrHistorySchema struct {
	Page          int    `json:"page"`
	PageSize      int    `json:"pageSize"`
	SortKey       string `json:"sortKey"`
	SortDirection string `json:"sortDirection"`
	TotalRecords  int    `json:"totalRecords"`
	Records       []struct {
		ID         int    `json:"id"`
		DownloadID string `json:"downloadId"`
	} `json:"records"`
}

func (arr *Arr) GetHeaders() map[string]string {
	return map[string]string{
		"X-Api-Key": arr.Token,
	}
}

func (arr *Arr) GetURL() string {
	url, _ := gourl.JoinPath(arr.URL, "api/v3/")
	return url
}

func getEventId(eventType string) int {
	switch eventType {
	case "grabbed":
		return 1
	case "seriesFolderDownloaded":
		return 2
	case "DownloadFolderImported":
		return 3
	case "DownloadFailed":
		return 4
	case "DownloadIgnored":
		return 7
	default:
		return 0
	}
}

func (arr *Arr) GetHistory(downloadId, eventType string) *ArrHistorySchema {
	eventId := getEventId(eventType)
	query := gourl.Values{}
	if downloadId != "" {
		query.Add("downloadId", downloadId)
	}
	if eventId != 0 {
		query.Add("eventId", strconv.Itoa(eventId))

	}
	query.Add("pageSize", "100")
	url := arr.GetURL() + "history/" + "?" + query.Encode()
	resp, err := arr.Client.MakeRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil
	}
	var data *ArrHistorySchema
	err = json.Unmarshal(resp, &data)
	if err != nil {
		return nil
	}
	return data

}

type Torrent struct {
	Id       string `json:"id"`
	InfoHash string `json:"info_hash"`
	Name     string `json:"name"`
	Folder   string `json:"folder"`
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
	Magnet   string `json:"magnet"`
	Files    []File `json:"files"`
	Status   string `json:"status"`

	Arr *Arr
}

type File struct {
	Id   string `json:"id"`
	Name string `json:"name"`
	Size int64  `json:"size"`
	Path string `json:"path"`
}

func (t *Torrent) Cleanup(remove bool) {
	if remove {
		err := os.Remove(t.Filename)
		if err != nil {
			return
		}
	}
}

func (t *Torrent) UpsertDB() error {
	tx, err := common.GetDB().Begin()
	if err != nil {
		return err
	}
	defer func(tx *sql.Tx) {
		err := tx.Rollback()
		if err != nil {
			return
		}
	}(tx)

	// Insert or update torrent
	_, err = tx.Exec(`
		INSERT INTO torrent (id, info_hash, name, folder, filename, size, magnet, watch_folder, status)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
		info_hash = excluded.info_hash,
		name = excluded.name,
		folder = excluded.folder,
		filename = excluded.filename,
		size = excluded.size,
		magnet = excluded.magnet
	`, t.Id, t.InfoHash, t.Name, t.Folder, t.Filename, t.Size, t.Magnet, t.Arr.WatchFolder, t.Status)
	if err != nil {
		return err
	}

	// Insert or update files
	for _, file := range t.Files {
		_, err = tx.Exec(`
			INSERT INTO file (id, name, size, path, torrent_id)
			VALUES (?, ?, ?, ?, ?)
			ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			size = excluded.size,
			path = excluded.path,
			torrent_id = excluded.torrent_id
		`, file.Id, file.Name, file.Size, file.Path, t.Id)
		if err != nil {
			return err
		}
	}
	return err
}

func (t *Torrent) MarkAsFailed() error {
	downloadId := strings.ToUpper(t.InfoHash)
	history := t.Arr.GetHistory(downloadId, "grabbed")
	if history == nil {
		return nil
	}
	torrentId := 0
	for _, record := range history.Records {
		if strings.EqualFold(record.DownloadID, downloadId) {
			torrentId = record.ID
			break
		}
	}
	if torrentId != 0 {
		url, err := gourl.JoinPath(t.Arr.GetURL(), "history/failed/", strconv.Itoa(torrentId))
		if err != nil {
			return err
		}
		_, err = t.Arr.Client.MakeRequest(http.MethodPost, url, nil)
		if err == nil {
			log.Printf("Marked torrent: %s as failed", t.Name)
		}
	}
	return nil
}
