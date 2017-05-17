package client

// Communication with GitLab

import (
	"comrade-pavlik2/pkg/client/gitlab"
	"comrade-pavlik2/pkg/helpers"
	"encoding/json"
	"fmt"
	"github.com/hashicorp/golang-lru"
	"log"
	"net/http"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"
)

type (
	// GitLabConnection represent connection to GitLab
	GitLabConnection struct {
		visibleProjectList []*gitlab.Project // all projects visible by user
		containerRepo      *gitlab.Project   // project with repo.json
		containerRepoList  []*containerItem  // all entries from repo.json
		packageRepoList    []*containerItem  // filtered list of entries

		token  string
		client *gitlab.Client
	}

	// Represent project/package repository
	GitLabRepo struct {
		Project      *gitlab.Project
		UUID         string
		TagList      []Tag
		Metadata     *JsonMap
		MetadataLock *sync.RWMutex
	}

	// Represent project/package tag
	Tag struct {
		Name         string
		Reference    string
		Metadata     *JsonMap
		MetadataLock *sync.RWMutex
	}

	// repo.json entry
	containerItem struct {
		GitURL    string
		UUID      string
		LabelList []string
		Project   *gitlab.Project
	}

	// timed project list cache structure
	cachedProjectList struct {
		Expire      time.Time
		ProjectList []*gitlab.Project
	}
)

var (
	repoListJsonFile          string
	repoJsonFilesList         []string
	baseURL                   string
	repoPathWithNamespace     string
	repoListJsonNamespace     string
	repoListJsonFileExtraList string // temporary storage

	// predefined constants
	KindComposer = "composer"
	KindNpm      = "npm"

	composerMetadataFile = "composer.json"
	npmMetadataFile      = "package.json"

	// Cache policy:
	//
	//  * projectList - per token, for a relatively small amount of time (5-10 min)
	//  * tag metadata file (composer.json/package.json) - forever, except master.
	//  * archive []bytes - forever, except master.
	//
	globalCache, _ = lru.New(1024)
)

func init() {
	baseURL = os.Getenv("GITLAB_URL")
	repoPathWithNamespace = os.Getenv("GITLAB_REPO_NAME")
	repoListJsonFile = os.Getenv("GITLAB_REPO_FILE")
	repoListJsonFileExtraList = os.Getenv("GITLAB_REPO_FILE_EXTRA_LIST")
	repoListJsonNamespace = os.Getenv("GITLAB_FILE_NAMESPACE")

	fmt.Println("> Pavlik reporting")
	if baseURL == "" || repoPathWithNamespace == "" || repoListJsonFile == "" || repoListJsonNamespace == "" {
		fmt.Println("ERROR: Please check environment variables, some of them are not set!")
		os.Exit(1)
	}

	// parse additional files
	repoJsonFilesList = append(repoJsonFilesList, repoListJsonFile)
	if repoListJsonFileExtraList != "" {
		rawList := strings.Split(repoListJsonFileExtraList, ",")
		for _, jsonFile := range rawList {
			jsonFile = strings.TrimSpace(jsonFile)
			if jsonFile != "" {
				repoJsonFilesList = append(repoJsonFilesList, jsonFile)
			}
		}
	}

	fmt.Println("==> GitLab:", baseURL)
	fmt.Println("==> Repository:", repoPathWithNamespace)
	fmt.Println("==> Namespace:", repoListJsonNamespace)
	fmt.Println("==> Source Files:", strings.Join(repoJsonFilesList, ", "))
}

// NewConnectionFromRequest - create new GitLabConnection for a given request
func NewConnectionFromRequest(r *http.Request) (*GitLabConnection, error) {
	token := helpers.GetTokenFromRequest(r)

	driver, err := gitlab.NewClient(baseURL, token)
	if err != nil {
		// possible errors:
		//  * ErrGitLabInvalidToken
		//  * ErrGitLabInvalidEndpoint
		return nil, err
	}

	c := &GitLabConnection{
		token:  token,
		client: driver,
	}
	return c, nil
}

// GetArchive - get binary buffer (tar.gz) for whole project by ref
func (c *GitLabConnection) GetArchive(kind, uuid, ref string) ([]byte, error) {
	var packageRepo *containerItem
	var item interface{}
	var ok bool
	var err error
	var archive []byte

	cacheKey := fmt.Sprintf("%s_%s_%s", kind, uuid, ref)
	ok = false

	// WARNING: *never* cache master ref
	if ref != "master" {
		item, ok = globalCache.Get(cacheKey)
	}

	if !ok {
		if err = c.fetchBasicData(kind); err != nil {
			return nil, err
		}
		if packageRepo, err = c.findPackageRepoByUUID(uuid); err != nil {
			return nil, err
		}
		if archive, err = c.client.GetArchive(packageRepo.Project, ref); err != nil {
			return nil, err
		}
		// WARNING: *don't even think* to put master ref into cache
		if ref != "master" {
			globalCache.Add(cacheKey, archive)
		}
	} else {
		if archive, ok = item.([]byte); !ok {
			globalCache.Remove(cacheKey)
			return nil, fmt.Errorf("Cache broken for key: %s", cacheKey)
		}
	}

	return archive, nil
}

// GetRepo - return package repository
func (c *GitLabConnection) GetRepo(kind, uuid string) (*GitLabRepo, error) {
	if err := c.fetchBasicData(kind); err != nil {
		return nil, err
	}

	packageRepo, err := c.findPackageRepoByUUID(uuid)
	if err != nil {
		return nil, err
	}

	log.Printf("==> Fetching repository data: %s", packageRepo.Project.Name)
	return c.fetchRepoData(kind, packageRepo)
}

// GetRepoList - return list of package repositories
func (c *GitLabConnection) GetRepoList(kind string) ([]*GitLabRepo, error) {
	if err := c.fetchBasicData(kind); err != nil {
		return nil, err
	}

	list := make([]*GitLabRepo, 0)
	for _, packageRepo := range c.packageRepoList {
		log.Printf("==> Fetching repository data: %s", packageRepo.Project.Name)
		packageRepo, err := c.fetchRepoData(kind, packageRepo)
		if err != nil {
			return nil, err
		}

		list = append(list, packageRepo)
	}

	return list, nil
}

// return www items values for list of cached projects
func (c *GitLabConnection) GetCachedList() ([]string, time.Time) {
	var cachedData cachedProjectList
	var item interface{}
	var ok bool
	var expire time.Time

	cacheKey := c.getProjectListCacheKey()
	cachedProjects := make([]string, 0)

	if item, ok = globalCache.Get(cacheKey); ok {
		if cachedData, ok = item.(cachedProjectList); ok {
			expire = cachedData.Expire

			if cachedData.Expire.Before(time.Now()) {
				globalCache.Remove(cacheKey)

			} else {
				for _, project := range cachedData.ProjectList {
					cachedProjects = append(cachedProjects, project.WWWURL)
				}
			}
		}
	}

	return cachedProjects, expire
}

// ClearCachedList - force remove projectList cache key for current token
func (c *GitLabConnection) ClearCachedList() {
	cacheKey := c.getProjectListCacheKey()
	globalCache.Remove(cacheKey)
}

// EnqueueProjectCache - trigger projectList load code for current token,
// but if cache is exists, do nothing.
// Function will run in background.
func (c *GitLabConnection) EnqueueProjectCache() {
	go c.fetchProjectList()
}

//
// Private API
//

// method which wraps all fetch/filter/fetch steps into one
func (c *GitLabConnection) fetchBasicData(kind string) error {

	// few steps to bootstrap GitLab connection:
	//
	//  * load all visible project list from GitLab
	//  * locate project with repo.json
	//  * fetch repo.json
	//  * build package projects by filtering projects by kind(tag, label)
	//
	if err := c.fetchProjectList(); err != nil {
		return err
	}

	if err := c.fetchSourceRepoList(kind); err != nil {
		return err
	}

	if err := c.filterProjectList(kind); err != nil {
		return err
	}

	return nil
}

// find repo.json entry by given uuid
func (c *GitLabConnection) findPackageRepoByUUID(uuid string) (*containerItem, error) {
	for _, containerRepo := range c.packageRepoList {
		if containerRepo.UUID == uuid {
			return containerRepo, nil
		}
	}

	return nil, fmt.Errorf("Project with uuid=%s not found", uuid)
}

// return cache key for project list
func (c *GitLabConnection) getProjectListCacheKey() string {
	return fmt.Sprintf("project_list_%s", c.token)
}

//
// WARNING: this is heavy operation, result of this operation
// is cached per token for a relatively small amount of time,
// in order to allow regular npm/composer operations to be fast.
//
func (c *GitLabConnection) fetchProjectList() error {
	var cachedData cachedProjectList
	var item interface{}
	var ok bool
	var err error
	var projectList []*gitlab.Project

	// check global cache and, if something is found, check expire field.
	cacheKey := c.getProjectListCacheKey()
	if item, ok = globalCache.Get(cacheKey); ok {
		if cachedData, ok = item.(cachedProjectList); ok {
			if cachedData.Expire.Before(time.Now()) {
				ok = false
				globalCache.Remove(cacheKey)
			} else {
				ok = true
				projectList = cachedData.ProjectList
			}
		}
	}

	if !ok {
		log.Println("==> Fetching list of available projects")
		if projectList, err = c.client.GetProjectList(); err != nil {
			return err
		}

		// store data to cache
		globalCache.Add(cacheKey, cachedProjectList{
			Expire:      time.Now().Add(30 * time.Minute),
			ProjectList: projectList,
		})
	}

	projectChan := make(chan bool)
	guardChan := make(chan bool, runtime.NumCPU())

	// store visible project list and try to locate repo.json repository
	c.visibleProjectList = projectList
	for _, project := range c.visibleProjectList {
		go func(project *gitlab.Project) {
			guardChan <- true
			defer func() {
				<-guardChan
			}()

			if project.PathWithNamespace == repoPathWithNamespace {
				c.containerRepo = project
				projectChan <- true
			} else {
				projectChan <- false
			}
		}(project)
	}

	founded := false
	for i := 0; i < len(c.visibleProjectList); i++ {
		if f := <-projectChan; f {
			founded = true
		}
	}

	if founded {
		return nil
	}

	return fmt.Errorf("Project with namespace=%s not found", repoPathWithNamespace)
}

// for each repo.json entry, find corresponding GitLab project.
// project may be unavailable due membership of current token.
func (c *GitLabConnection) filterProjectList(kind string) error {

	itemChan := make(chan *containerItem)
	guardChan := make(chan bool, runtime.NumCPU())

	for _, containerRepo := range c.containerRepoList {
		go func(containerRepo *containerItem) {
			guardChan <- true
			defer func() {
				<-guardChan
			}()

			// searching for project matching containerRepo
			for _, project := range c.visibleProjectList {
				if project.HTTPURL == containerRepo.GitURL || project.SSHURL == containerRepo.GitURL {
					containerRepo.Project = project
					itemChan <- containerRepo
					return
				}
			}

			// nothing found..
			log.Printf("==> Notice: Can't find project for source: %s", containerRepo.GitURL)
			itemChan <- nil

		}(containerRepo)
	}

	c.packageRepoList = make([]*containerItem, 0)
	for i := 0; i < len(c.containerRepoList); i++ {
		item := <-itemChan
		if item != nil {
			c.packageRepoList = append(c.packageRepoList, item)
		}
	}

	return nil
}

// return package metadata file name (composer.json/package.json) for each registry
func (c *GitLabConnection) metadataFileForKind(kind string) (string, error) {
	switch kind {
	case KindComposer:
		return composerMetadataFile, nil

	case KindNpm:
		return npmMetadataFile, nil
	}

	return "", fmt.Errorf("Unknown kind: %s", kind)
}

// fetch mandatory information from project repository: such as tags, metadata file
// and create final package/project entries.
func (c *GitLabConnection) fetchRepoData(kind string, src *containerItem) (*GitLabRepo, error) {
	// WARNING: *do not cache* this api call
	tagList, err := c.client.GetTagList(src.Project)
	if err != nil {
		return nil, err
	}

	// guessing package.json/composer.json
	metadataFile, err := c.metadataFileForKind(kind)
	if err != nil {
		return nil, err
	}

	// constructing object for registry
	result := &GitLabRepo{
		Project:      src.Project,
		UUID:         src.UUID,
		TagList:      make([]Tag, 0),
		MetadataLock: new(sync.RWMutex),
	}

	// fetch metadata for master branch, mostly required for npm
	r := make(JsonMap, 0)
	if err := c.fetchJsonFile(src.Project, "master", metadataFile, &r); err != nil {
		return nil, err
	}

	result.MetadataLock.Lock()
	result.Metadata = &r
	result.MetadataLock.Unlock()

	// request metadata file for each tag, and do it fast..
	tagChan := make(chan *Tag)
	guardChan := make(chan bool, runtime.NumCPU())

	for _, tag := range tagList {
		go func(tag *gitlab.Tag) {
			guardChan <- true
			defer func() {
				<-guardChan
			}()

			r := make(JsonMap, 0)
			err := c.fetchJsonFile(src.Project, tag.Commit.ID, metadataFile, &r)
			if err != nil {
				tagChan <- nil
				return
			}

			t := &Tag{
				Name:         tag.Name,
				Reference:    tag.Commit.ID,
				MetadataLock: new(sync.RWMutex),
			}

			t.MetadataLock.Lock()
			t.Metadata = &r
			t.MetadataLock.Unlock()

			tagChan <- t
		}(tag)
	}

	// wait tags...
	for i := 0; i < len(tagList); i++ {
		t := <-tagChan
		if t != nil {
			result.TagList = append(result.TagList, *t)
		}
	}

	return result, nil
}

// Convert entries in repo.json into containerItem structures
func (c *GitLabConnection) fetchSourceRepoList(kind string) error {

	//
	// unpack repo.json
	//
	jsonChan := make(chan []JsonMap)
	gitlabChan := make(chan bool, 2)

	sourceJsonData := make([]JsonMap, 0)
	for _, jsonFile := range repoJsonFilesList {
		go func(jsonFile string) {
			gitlabChan <- true
			defer func() {
				<-gitlabChan
			}()

			jsonSource := make([]JsonMap, 0)
			if err := c.fetchJsonFile(c.containerRepo, "master", jsonFile, &jsonSource); err != nil {
				jsonChan <- nil
				return
			}
			jsonChan <- jsonSource
		}(jsonFile)
	}

	for i := 0; i < len(repoJsonFilesList); i++ {
		jsonSource := <-jsonChan
		if jsonSource != nil {
			if len(jsonSource) > 0 {
				sourceJsonData = append(sourceJsonData, jsonSource...)
			}
		}
	}

	//
	// transform repo.json entries into real GitLab repositories
	//
	containerChan := make(chan *containerItem)
	allCPUChan := make(chan bool, runtime.NumCPU())

	for _, data := range sourceJsonData {
		go func(data JsonMap) {
			allCPUChan <- true
			defer func() {
				<-allCPUChan
			}()

			cloneUrl, _ := data.GetString(repoListJsonNamespace)
			uuid, _ := data.GetString("uuid")
			tagsInterface, _ := data.GetListInterface("tag", nil)
			if cloneUrl == "" || uuid == "" || tagsInterface == nil {
				containerChan <- nil
				return
			}

			containerRepo := &containerItem{
				GitURL:    cloneUrl,
				UUID:      uuid,
				LabelList: make([]string, 0),
			}

			for _, tag := range *tagsInterface {
				if tagValue, _ := tag.(string); tagValue == kind {
					containerRepo.LabelList = append(containerRepo.LabelList, tagValue)
				}
			}

			containerChan <- containerRepo
		}(data)
	}

	c.containerRepoList = make([]*containerItem, 0)
	for i := 0; i < len(sourceJsonData); i++ {
		containerRepo := <-containerChan
		if containerRepo != nil {
			// append any source repo with non empty label list
			if len(containerRepo.LabelList) > 0 {
				c.containerRepoList = append(c.containerRepoList, containerRepo)
			}
		}
	}

	return nil
}

// Get json file from repository and auto-unpack it into provided interface
func (c *GitLabConnection) fetchJsonFile(p *gitlab.Project, ref, path string, rec interface{}) error {
	var fileContent []byte
	var ok bool
	var err error
	var item interface{}

	cacheKey := fmt.Sprintf("json_%d_%s", p.ID, ref)
	ok = false

	// WARNING: *never* cache master ref
	if ref != "master" {
		item, ok = globalCache.Get(cacheKey)
	}

	if !ok {
		if fileContent, err = c.client.GetFile(p, path, ref); err != nil {
			return err
		}
		// WARNING: *don't even think* to put master ref into cache
		if ref != "master" {
			globalCache.Add(cacheKey, fileContent)
		}
	} else {
		if fileContent, ok = item.([]byte); !ok {
			globalCache.Remove(cacheKey)
			return fmt.Errorf("Cache broken for key: %s", cacheKey)
		}
	}

	return json.Unmarshal(fileContent, rec)
}
