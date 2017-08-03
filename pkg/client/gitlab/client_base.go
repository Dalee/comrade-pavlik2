package gitlab

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
)

// @see https://gitlab.com/gitlab-org/gitlab-ce/blob/8-5-stable/doc/api/projects.md#list-projects
// https://docs.gitlab.com/ee/api/projects.html#list-projects
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

// @see https://gitlab.com/gitlab-org/gitlab-ce/blob/8-5-stable/doc/api/projects.md#get-single-project
// @see https://docs.gitlab.com/ee/api/projects.html#get-single-project
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

// @see https://gitlab.com/gitlab-org/gitlab-ce/blob/8-5-stable/doc/api/tags.md#list-project-repository-tags
// @see https://docs.gitlab.com/ee/api/tags.html#list-project-repository-tags
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

// @see https://gitlab.com/gitlab-org/gitlab-ce/blob/8-5-stable/doc/api/repositories.md#get-file-archive
// @see https://docs.gitlab.com/ee/api/repositories.html#get-file-archive
//
func (c *Client) GetArchive(project *Project, ref string) ([]byte, error) {
	endpoint := fmt.Sprintf(
		"projects/%d/repository/archive.tar.gz?sha=%s",
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

// @see https://gitlab.com/gitlab-org/gitlab-ce/blob/8-5-stable/doc/api/repository_files.md#get-file-from-repository
// for v3 file_path should be QueryString parameter.
//
// @see https://docs.gitlab.com/ee/api/repository_files.html#get-file-from-repository
// for v4 file_path is not a parameter but part of URI. Should be encoded anyway.
// update, right now this one doesn't seem's to work.
//
// GitLab < v9.4.2: v4 method doesn't work as documented, uses v3 signature.
// GitLab >= v9.4.2: v4 work as documented.
//
// To maintain compatibility between all v3, v4-pre and v4 versions,
// one extra HEAD request should be executed.
//
func (c *Client) GetFile(project *Project, path, ref string) ([]byte, error) {
	var endpoint string

	// v3 and v4:legacy method for accessing files
	endpoint = fmt.Sprintf(
		"projects/%d/repository/files?file_path=%s&ref=%s",
		project.ID,
		url.QueryEscape(path),
		url.QueryEscape(ref),
	)

	if c.HasV4Support {
		// check broken v4 api
		r, _ := c.executeHead(endpoint)
		if r.StatusCode() != 200 {
			// ok, gitlab has correct v4 support
			endpoint = fmt.Sprintf(
				"projects/%d/repository/files/%s?ref=%s",
				project.ID,
				url.QueryEscape(path),
				url.QueryEscape(ref),
			)
		}
	}

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
