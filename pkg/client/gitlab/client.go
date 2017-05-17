package gitlab

import (
	"errors"
	"fmt"
	"gopkg.in/resty.v0"
	"net/http"
	"strconv"
	"strings"
)

type (
	//
	Client struct {
		HasV4Support bool
		HasV3Support bool
		Endpoint     string
		Token        string
		APIPrefix    string
	}
)

var (
	//
	ErrGitLabInvalidToken = errors.New("Invalid Token")

	//
	ErrGitLabInvalidEndpoint = errors.New("Invalid GitLab endpoint")
)

//
func NewClient(endpoint string, token string) (*Client, error) {
	client := &Client{
		HasV4Support: false,
		HasV3Support: false,
		Endpoint:     endpoint,
		Token:        token,
	}

	err := client.guessAPIVersion()
	if err != nil {
		return nil, err
	}

	return client, nil
}

//
// Guess API version, by making HEAD
// request to /api/vX/namespaces endpoint
//
func (c *Client) guessAPIVersion() error {
	// Checking: HEAD /api/v4/namespaces
	resp, _ := c.executeHead("/api/v4/user")
	if resp.StatusCode() == http.StatusUnauthorized {
		return ErrGitLabInvalidToken
	}

	// HEAD request succeeded.
	// Client will use API v4.
	if resp.StatusCode() == http.StatusOK {
		c.HasV4Support = true
		c.APIPrefix = "/api/v4"
		return nil
	}

	// Checking: HEAD /api/v3/namespaces
	resp, _ = c.executeHead("/api/v3/user")
	if resp.StatusCode() == http.StatusUnauthorized {
		return ErrGitLabInvalidToken
	}

	// HEAD request succeeded.
	// Client will use API v3
	if resp.StatusCode() == http.StatusOK {
		c.HasV3Support = true
		c.APIPrefix = "/api/v3"
		return nil
	}

	return ErrGitLabInvalidEndpoint
}

//
// Execute API method and return array of response bodies
//
func (c *Client) executeAPIMethod(baseRequestURI string) ([][]byte, error) {

	list := make([][]byte, 0)
	baseRequestURI = strings.TrimLeft(baseRequestURI, "/")
	baseRequestURI = fmt.Sprintf("%s/%s", c.APIPrefix, baseRequestURI)
	perPage := 30

	// performing initial request without pagination
	// will check response header for pagination support
	addArg := "?"
	if strings.Index(baseRequestURI, "?") >= 0 {
		addArg = "&"
	}

	reqURI := fmt.Sprintf("%s%sper_page=%d", baseRequestURI, addArg, perPage)
	resp, err := c.executeGet(reqURI)
	if err != nil {
		return nil, err
	}

	// store body of initial request
	list = append(list, resp.Body())
	totalPagesRaw := resp.Header().Get("X-Total-Pages")
	nextPageRaw := resp.Header().Get("X-Next-Page")

	// is resource support pagination?
	if nextPageRaw == "" {
		return list, nil
	}

	nextPage, err := strconv.Atoi(nextPageRaw)
	if err != nil {
		return nil, err
	}

	totalPages, err := strconv.Atoi(totalPagesRaw)
	if err != nil {
		return nil, err
	}

	bodyChan := make(chan []byte)
	guardChan := make(chan bool, 2)

	for i := nextPage; i <= totalPages; i++ {
		go func(i int) {
			guardChan <- true
			defer func() {
				<-guardChan
			}()

			reqURI := fmt.Sprintf("%s%sper_page=%d&page=%d", baseRequestURI, addArg, perPage, i)
			resp, err := c.executeGet(reqURI)
			if err != nil {
				bodyChan <- nil
				return
			}

			bodyChan <- resp.Body()
		}(i)
	}

	for j := nextPage; j <= totalPages; j++ {
		b := <-bodyChan
		if b != nil {
			list = append(list, b)
		}
	}

	if len(list) != totalPages {
		return nil, errors.New("Failed to get some pages..")
	}

	return list, nil
}

//
// HEAD request helper
//
func (c *Client) executeHead(requestURI string) (*resty.Response, error) {
	requestURI = strings.TrimLeft(requestURI, "/")
	requestURL := fmt.Sprintf("%s/%s", c.Endpoint, requestURI)

	return resty.R().SetHeader("PRIVATE-TOKEN", c.Token).Head(requestURL)
}

//
// GET request helper
//
func (c *Client) executeGet(requestURI string) (*resty.Response, error) {
	requestURI = strings.TrimLeft(requestURI, "/")
	requestURL := fmt.Sprintf("%s/%s", c.Endpoint, requestURI)

	return resty.R().SetHeader("PRIVATE-TOKEN", c.Token).Get(requestURL)
}
