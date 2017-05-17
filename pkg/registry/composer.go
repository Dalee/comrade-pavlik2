package registry

import (
	"comrade-pavlik2/pkg/client"
	"comrade-pavlik2/pkg/helpers"
	"errors"
	"fmt"
	"github.com/blang/semver"
	"log"
	"runtime"
	"strings"
	"sync"
)

type (
	// ComposerRegistry is struct to represent methods to display
	// gitlab repositories as composer packages
	ComposerRegistry struct {
		conn *client.GitLabConnection
	}

	// ComposerPackage is struct to represent single repository
	// as composer package
	ComposerPackage struct {
		Packages     map[string]map[string]composerVersion `json:"packages"`
		PackagesLock *sync.RWMutex                         `json:"-"`
	}

	//
	composerVersion struct {
		Name       string                  `json:"name"`
		Type       string                  `json:"type,omitempty"`
		Version    string                  `json:"version"`
		Extra      *map[string]interface{} `json:"extra,omitempty"`
		Require    *map[string]interface{} `json:"require,omitempty"`
		RequireDev *map[string]interface{} `json:"require-dev,omitempty"`
		Autoload   *map[string]interface{} `json:"autoload,omitempty"`
		Config     *map[string]interface{} `json:"config,omitempty"`
		Bin        *[]interface{}          `json:"bin,omitempty"`
		Dist       composerDist            `json:"dist"`
	}

	composerDist struct {
		Url       string `json:"url"`
		Type      string `json:"type"`
		Reference string `json:"reference"`
	}
)

// NewComposerRegistry - construct composer emulator for GitLab
func NewComposerRegistry(conn *client.GitLabConnection) *ComposerRegistry {
	return &ComposerRegistry{
		conn: conn,
	}
}

// GetPackageInfo - get single package info, debug method,
// format package same way as /packages.json should be.
func (c *ComposerRegistry) GetPackageInfo(uuid string, endpoint string) (*ComposerPackage, error) {
	repo, err := c.conn.GetRepo(client.KindComposer, uuid)
	if err != nil {
		return nil, err
	}

	rootPackage := &ComposerPackage{
		Packages:     make(map[string]map[string]composerVersion, 0),
		PackagesLock: new(sync.RWMutex),
	}

	if err := rootPackage.fillVersions(c.conn, repo, endpoint); err != nil {
		return nil, err
	}

	return rootPackage, nil
}

// GetPackageInfoList - get whole bunch of packages visible for provided token
func (c *ComposerRegistry) GetPackageInfoList(endpoint string) (*ComposerPackage, error) {
	repoList, err := c.conn.GetRepoList(client.KindComposer)
	if err != nil {
		return nil, err
	}

	rootPackage := &ComposerPackage{
		Packages:     make(map[string]map[string]composerVersion, 0),
		PackagesLock: new(sync.RWMutex),
	}

	resultChan := make(chan bool)
	guardChan := make(chan bool, 2)

	for _, repo := range repoList {
		go func(repo *client.GitLabRepo) {
			guardChan <- true
			defer func() {
				<-guardChan
			}()

			err := rootPackage.fillVersions(c.conn, repo, endpoint)
			if err != nil {
				resultChan <- false
				return
			}

			resultChan <- true
		}(repo)
	}

	success := true
	for i := 0; i < len(repoList); i++ {
		if processed := <-resultChan; !processed {
			success = false
		}
	}

	if success {
		return rootPackage, nil
	}

	return nil, errors.New("Error while fetchig packages")
}

// GetPackageArchive - get whole package as zip archive
func (c *ComposerRegistry) GetPackageArchive(uuid string, ref string) ([]byte, error) {
	archive, err := c.conn.GetArchive(client.KindComposer, uuid, ref)
	if err != nil {
		return nil, err
	}

	pkg, err := helpers.TarGzToZip(archive, uuid, ref)
	if err != nil {
		return nil, err
	}

	return pkg, nil
}

// fill all versions of package
func (p *ComposerPackage) fillVersions(c *client.GitLabConnection, src *client.GitLabRepo, endpoint string) error {
	versionList := make([]composerVersion, 0)
	versionList = append(versionList, p.versionListFromTags(src, endpoint)...)

	for _, v := range versionList {
		p.PackagesLock.Lock()
		if p.Packages[v.Name] == nil {
			p.Packages[v.Name] = make(map[string]composerVersion, 0)
		}

		p.Packages[v.Name][v.Version] = v
		p.PackagesLock.Unlock()
	}

	return nil
}

// fill metadata from tag's metadata composer.json
func (p *ComposerPackage) fillMetadata(v *composerVersion, uuid string, tag client.Tag, endpoint string) {
	// fill optional composer fields
	tag.MetadataLock.RLock()
	v.Type, _ = tag.Metadata.GetString("type")
	v.Extra, _ = tag.Metadata.GetMapInterface("extra", nil)
	v.Require, _ = tag.Metadata.GetMapInterface("require", nil)
	v.RequireDev, _ = tag.Metadata.GetMapInterface("require-dev", nil)
	v.Autoload, _ = tag.Metadata.GetMapInterface("autoload", nil)
	v.Config, _ = tag.Metadata.GetMapInterface("config", nil)
	v.Bin, _ = tag.Metadata.GetListInterface("bin", nil)
	tag.MetadataLock.RUnlock()

	// fill distribution struct
	v.Dist = composerDist{
		Url:       fmt.Sprintf(endpoint, uuid, tag.Reference),
		Type:      "zip",
		Reference: tag.Reference,
	}
}

// fill all available versions of package from valid tags
func (p *ComposerPackage) versionListFromTags(src *client.GitLabRepo, endpoint string) []composerVersion {

	versionChan := make(chan *composerVersion)
	guardChan := make(chan bool, runtime.NumCPU())

	log.Println("==> Processing tags:", src.Project.Name)
	for _, tag := range src.TagList {
		go func(tag client.Tag) {
			guardChan <- true
			defer func() {
				<-guardChan
			}()

			// prefix "v" is not supported by semver library, but supported by composer
			releaseName := strings.TrimLeft(tag.Name, "v")
			releaseInfo, err := semver.Make(releaseName)
			if err != nil {
				versionChan <- nil
				return
			}

			// create base object with version string including "v"
			v := &composerVersion{
				Version: fmt.Sprintf("v%s", releaseInfo.String()),
			}

			// check mandatory field
			tag.MetadataLock.RLock()
			v.Name, err = tag.Metadata.GetString("name")
			tag.MetadataLock.RUnlock()

			if err != nil {
				versionChan <- nil
				return
			}

			p.fillMetadata(v, src.UUID, tag, endpoint)
			versionChan <- v
		}(tag)
	}

	list := make([]composerVersion, 0)
	for i := 0; i < len(src.TagList); i++ {
		v := <-versionChan
		if v != nil {
			list = append(list, *v)
		}
	}

	return list
}
