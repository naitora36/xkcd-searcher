package xkcd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"yadro.com/course/closers"
	"yadro.com/course/update/core"
)

type Client struct {
	log    *slog.Logger
	client http.Client
	url    string
}
type XKCDInfoDTO struct {
	ID          int    `json:"num"`
	URL         string `json:"img"`
	Title       string `json:"title"`
	Description string `json:"transcript"`
}

func NewClient(url string, timeout time.Duration, log *slog.Logger) (*Client, error) {
	if url == "" {
		return nil, fmt.Errorf("empty base url specified")
	}
	return &Client{
		client: http.Client{Timeout: timeout},
		log:    log,
		url:    url,
	}, nil
}

func (c Client) Get(ctx context.Context, id int) (core.XKCDInfo, error) {
	if id == 404 {
		return core.XKCDInfo{
			ID:          id,
			URL:         "https://empty_and_funny",
			Title:       "",
			Description: "",
		}, nil
	}

	res := XKCDInfoDTO{}

	stringId := strconv.Itoa(id)
	pathToComics, err := url.JoinPath(c.url, stringId, "info.0.json")
	if err != nil {
		return core.XKCDInfo{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pathToComics, nil)
	if err != nil {
		return core.XKCDInfo{}, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return core.XKCDInfo{}, err
	}

	defer closers.CloseOrLog(resp.Body, slog.Default())

	err = json.NewDecoder(resp.Body).Decode(&res)
	if err != nil {
		return core.XKCDInfo{}, err
	}

	return core.XKCDInfo{
		ID:          res.ID,
		URL:         res.URL,
		Title:       res.Title,
		Description: res.Description,
	}, nil
}

func (c Client) LastID(ctx context.Context) (int, error) {
	xkcdAnswer := struct {
		ComicsTotal int `json:"num"`
	}{}

	pathToInfo, err := url.JoinPath(c.url, "info.0.json")
	if err != nil {
		return 0, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pathToInfo, nil)
	if err != nil {
		return 0, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return 0, err
	}

	defer closers.CloseOrLog(resp.Body, slog.Default())

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}
	err = json.Unmarshal(body, &xkcdAnswer)
	if err != nil {
		return 0, err
	}

	return xkcdAnswer.ComicsTotal, nil
}
