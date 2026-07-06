package wallpaper

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

type bingResponse struct {
	Images []struct {
		URL string `json:"url"`
	} `json:"images"`
}

// SyncBingDaily downloads the daily image from Bing if not already cached today.
func SyncBingDaily(workspaceDir string) error {
	wpDir := filepath.Join(workspaceDir, "wallpapers")
	os.MkdirAll(wpDir, 0755)

	today := time.Now().Format("2006-01-02")
	filename := fmt.Sprintf("bing-%s.jpg", today)
	targetPath := filepath.Join(wpDir, filename)

	// If already downloaded today, nothing to do
	if _, err := os.Stat(targetPath); err == nil {
		return nil
	}

	client := &http.Client{Timeout: 5 * time.Second}
	res, err := client.Get("https://www.bing.com/HPImageArchive.aspx?format=js&idx=0&n=1&mkt=en-US")
	if err != nil {
		return fmt.Errorf("failed fetching bing metadata: %w", err)
	}
	defer res.Body.Close()

	var data bingResponse
	if errDecode := json.NewDecoder(res.Body).Decode(&data); errDecode != nil || len(data.Images) == 0 {
		return fmt.Errorf("invalid bing response")
	}

	imgURL := data.Images[0].URL
	if len(imgURL) > 0 && imgURL[0] == '/' {
		imgURL = "https://www.bing.com" + imgURL
	}

	imgRes, err := client.Get(imgURL)
	if err != nil {
		return fmt.Errorf("failed downloading wallpaper image: %w", err)
	}
	defer imgRes.Body.Close()

	if imgRes.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status code: %d", imgRes.StatusCode)
	}

	out, err := os.Create(targetPath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, imgRes.Body)
	return err
}
