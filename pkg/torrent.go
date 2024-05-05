package pkg

import (
	"goBlack/common"
	"os"
)

type Torrent struct {
	Id       string `json:"id"`
	InfoHash string `json:"info_hash"`
	Name     string `json:"name"`
	Folder   string `json:"folder"`
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
	Magnet   string `json:"magnet"`
	Files    []File `json:"files"`
}

func (t *Torrent) Cleanup(config *common.TorrentConfig, remove bool) {
	if remove {
		err := os.Remove(t.Filename)
		if err != nil {
			return
		}
	}
}

type File struct {
	Id   string `json:"id"`
	Name string `json:"name"`
	Size int64  `json:"size"`
	Path string `json:"path"`
}
