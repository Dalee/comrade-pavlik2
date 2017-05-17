package gitlab

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
)

//
//
//
func (c *Client) GetProjectList() ([]*Project, error) {

	endpoint := "projects"
	pageList, err := c.executeAPIMethod(endpoint)
	if err != nil {
		return nil, err
	}

	projectList := make([]*Project, 0)
	for _, body := range pageList {
		page := make([]*Project, 0)
		if err := json.Unmarshal(body, &page); err != nil {
			return nil, err
		}

		projectList = append(projectList, page...)
	}

	return projectList, nil
}

//
//
//
func (c *Client) GetProjectById(projectId int) (*Project, error) {

	endpoint := fmt.Sprintf("projects/%d", projectId)
	pageList, err := c.executeAPIMethod(endpoint)
	if err != nil {
		return nil, err
	}

	if len(pageList) == 0 {
		return nil, errors.New("No such project")
	}

	result := &Project{}
	if err := json.Unmarshal(pageList[0], result); err != nil {
		return nil, err
	}

	return result, nil
}

//
//
//
func (c *Client) GetTagList(project *Project) ([]*Tag, error) {

	endpoint := fmt.Sprintf("projects/%d/repository/tags", project.ID)
	pageList, err := c.executeAPIMethod(endpoint)
	if err != nil {
		return nil, err
	}

	tagList := make([]*Tag, 0)

	for _, body := range pageList {
		page := make([]*Tag, 0)
		if err := json.Unmarshal(body, &page); err != nil {
			return nil, err
		}

		tagList = append(tagList, page...)
	}

	return tagList, nil
}

//
//
//
func (c *Client) GetArchive(project *Project, ref string) ([]byte, error) {
	endpoint := fmt.Sprintf(
		"projects/%d/repository/archive.tar.gz?ref=%s",
		project.ID,
		url.QueryEscape(ref),
	)

	pageList, err := c.executeAPIMethod(endpoint)
	if err != nil {
		return nil, err
	}

	if len(pageList) == 0 {
		return nil, errors.New("Archive operation failed")
	}

	return pageList[0], nil
}

//
//
//
func (c *Client) GetFile(project *Project, path, ref string) ([]byte, error) {

	endpoint := fmt.Sprintf(
		"projects/%d/repository/files?file_path=%s&ref=%s",
		project.ID,
		url.QueryEscape(path),
		url.QueryEscape(ref),
	)
	pageList, err := c.executeAPIMethod(endpoint)
	if err != nil {
		return nil, err
	}

	// should be only one page
	if len(pageList) == 0 {
		return nil, errors.New("No such file")
	}

	// decode response
	file := &File{}
	if err := json.Unmarshal(pageList[0], file); err != nil {
		return nil, err
	}

	// check encoding, should be base64
	if file.Encoding != "base64" {
		return nil, fmt.Errorf("Unknown encoding: %s", file.Encoding)
	}

	// decode file content
	fileContent, err := base64.StdEncoding.DecodeString(file.Content)
	if err != nil {
		return nil, err
	}

	return fileContent, nil
}
