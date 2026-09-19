package main

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/noornee/reddit-dl/internal/helper"
	"github.com/noornee/reddit-dl/internal/reddit"
	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:    "reddit-dl",
		Usage:   "A reddit multimedia downloader",
		Version: "0.66.5",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "url",
				Aliases: []string{"u"},
				Usage:   "a reddit post url",
			},
			&cli.BoolFlag{
				Name:    "dash",
				Aliases: []string{"d"},
				Usage:   "download reddit video using Dash playlist with ffmpeg",
			},
			&cli.StringFlag{
				Name:    "cookies-from-browser",
				Aliases: []string{"b"},
				Usage:   "load cookies from browser (e.g. brave, chrome, firefox, edge, opera, safari)",
			},
			&cli.StringFlag{
				Name:    "cookies",
				Aliases: []string{"c"},
				Usage:   "path to Netscape formatted cookies.txt file",
			},
		},
		Action: func(ctx *cli.Context) error {
			url := ctx.String("url")
			if url == "" && ctx.Args().Present() {
				url = ctx.Args().First()
			}

			if url == "" {
				cli.ShowAppHelp(ctx)
				return nil
			}

			cookiesBrowser := ctx.String("cookies-from-browser")
			cookiesFile := ctx.String("cookies")

			if cookiesBrowser != "" || cookiesFile != "" {
				if err := helper.InitCookies(cookiesBrowser, cookiesFile); err != nil {
					helper.ErrorLog.Printf("Warning: error initializing cookies: %v\n", err)
				}
			}

			if ctx.Bool("dash") {
				controller(url, true)
			} else {
				controller(url, false)
			}

			return nil
		},
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Println()
		helper.ErrorLog.Println(err)
	}
}

func controller(raw_url string, useDash bool) {
	title := fmt.Sprintf("%d", time.Now().Unix())

	body, err := helper.GetJSONBody(raw_url)
	if err != nil {
		helper.ErrorLog.Fatal(err)
	}

	reddit_data, err := reddit.ExtractRedditData(body, useDash)
	if err != nil {
		helper.ErrorLog.Fatal(err)
	}

	var wg sync.WaitGroup

	if reddit_data.IsDash {
		helper.InfoLog.Println("Downloading DASHPlaylist")
		helper.DownloadDashPlaylist(reddit_data.MediaUrl, title)
		return
	}

	if reddit_data.IsRedditGallery {
		for _, url := range reddit_data.GalleryUrls {
			wg.Add(1)
			go func(url string) {
				defer wg.Done()
				helper.Download(url, "", title)
			}(url)
		}
		wg.Wait()

	} else {
		media, audio := helper.GetMediaUrl(reddit_data.MediaUrl)
		helper.Download(media, audio, title)
	}
}
