package common

import (
	"encoding/json"
	"log"
	"os"
)

type DebridConfig struct {
	Name             string `json:"name"`
	Host             string `json:"host"`
	APIKey           string `json:"api_key"`
	Folder           string `json:"folder"`
	DownloadUncached bool   `json:"download_uncached"`
}

type Config struct {
	Debrid DebridConfig `json:"debrid"`
	Arrs   []struct {
		WatchFolder     string `json:"watch_folder"`
		CompletedFolder string `json:"completed_folder"`
	} `json:"arrs"`
}

type TorrentConfig struct {
	WatchFolder     string       `json:"watch_folder"`
	CompletedFolder string       `json:"completed_folder"`
	Debrid          DebridConfig `json:"debrid"`
}

func LoadConfig(path string) (*Config, error) {
	// Load the config file
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			log.Fatal(err)
		}
	}(file)

	decoder := json.NewDecoder(file)
	config := &Config{}
	err = decoder.Decode(config)
	if err != nil {
		return nil, err
	}

	return config, nil
}
