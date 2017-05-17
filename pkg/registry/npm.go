package registry

import (
	"comrade-pavlik2/pkg/client"
	"comrade-pavlik2/pkg/helpers"
	"fmt"
	"github.com/blang/semver"
	"log"
	"runtime"
	"strings"
)

type (
	NpmRegistry struct {
		conn *client.GitLabConnection
	}

	NpmPackage struct {
		Name        string                `json:"name"`
		Description string                `json:"description"`
		DistTags    map[string]string     `json:"dist-tags"`
		License     string                `json:"license"` // should always be "proprietary"
		Private     bool                  `json:"private"` // should always be true
		Versions    map[string]npmVersion `json:"versions"`
	}

	npmVersion struct {
		Version         string                  `json:"version"`
		Name            string                  `json:"name"`
		Description     string                  `json:"description,omitempty"`
		Main            string                  `json:"main,omitempty"`
		Dependencies    *map[string]interface{} `json:"dependencies,omitempty"`
		DevDependencies *map[string]interface{} `json:"devDependencies,omitempty"`
		Dist            npmDist                 `json:"dist"`
	}

	npmDist struct {
		Sha     string `json:"shasum"`
		Tarball string `json:"tarball"`
	}
)

//
func NewNpmRegistry(conn *client.GitLabConnection) *NpmRegistry {
	return &NpmRegistry{
		conn: conn,
	}
}

// find project by name in package.json in each master branch of package repository
// download project archive, calculate sha1 hash and put it to LRU-cache
// generate final NpmPackage structure
func (c *NpmRegistry) GetPackageInfo(name string, endpoint string) (*NpmPackage, error) {
	var project *client.GitLabRepo
	var err error

	// searching for package as it is provided
	if project, err = c.findPackageByName(name); err != nil {

		// not found, for backward compatibility try to cut-off scope
		slashPos := strings.Index(name, "/")
		name := name[slashPos+1:]

		if project, err = c.findPackageByName(name); err != nil {
			return nil, err
		}
	}

	rootPackage := &NpmPackage{
		DistTags: make(map[string]string, 0),
		Versions: make(map[string]npmVersion, 0),
	}

	// when filling version, connection to gitlab is required for generating sha1 hash for each tag
	if err := rootPackage.fillVersions(c.conn, project, endpoint); err != nil {
		return nil, err
	}

	// fill base parameters
	if err := rootPackage.fillBase(project); err != nil {
		return nil, err
	}

	return rootPackage, nil
}

// This method should always serve packages from cache
func (c *NpmRegistry) GetPackageArchive(uuid string, ref string) ([]byte, error) {
	// try to fetch data from cache
	if finalArchive, err := helpers.GetNpmArchiveFromCache(uuid, ref); err == nil {
		return finalArchive, nil
	}

	// fetch archive from gitlab
	rawArchive, err := c.conn.GetArchive(client.KindNpm, uuid, ref)
	if err != nil {
		return nil, err
	}

	// calculate sha1 sum and put data to cache
	npmArchive, _, err := helpers.PutNpmArchiveToCache(rawArchive, uuid, ref)
	if err != nil {
		return nil, err
	}

	return npmArchive, nil
}

//
func (c *NpmRegistry) findPackageByName(name string) (*client.GitLabRepo, error) {
	projectList, err := c.conn.GetRepoList(client.KindNpm)
	if err != nil {
		return nil, err
	}

	for _, p := range projectList {

		p.MetadataLock.RLock()
		packageName, _ := p.Metadata.GetString("name")
		p.MetadataLock.RUnlock()

		if packageName == name {
			return p, nil
		}
	}

	return nil, fmt.Errorf("Project with name: %s not found", name)
}

// fill root level fields, versions should be generated already
func (p *NpmPackage) fillBase(src *client.GitLabRepo) error {

	src.MetadataLock.RLock()
	p.Name, _ = src.Metadata.GetString("name")
	p.Description, _ = src.Metadata.GetString("description")
	src.MetadataLock.RUnlock()

	p.License = "proprietary"
	p.Private = true

	// fill dist-tags
	for n := range p.Versions {
		p.DistTags[n] = n
	}

	return nil
}

// fetch version (tag) from GitLab, calculate sha1 and store archive in cache
func (p *NpmPackage) fillVersions(c *client.GitLabConnection, src *client.GitLabRepo, endpoint string) error {

	versionChan := make(chan *npmVersion)
	guardChan := make(chan bool, runtime.NumCPU())

	log.Println("==> Processing tags:", src.Project.Name)
	for _, tag := range src.TagList {
		go func(tag client.Tag) {
			guardChan <- true
			defer func() {
				<-guardChan
			}()

			// check semantic versioning
			// "v" is not supported by semver library and not supported by npm itself
			releaseName := strings.TrimLeft(tag.Name, "v")
			releaseInfo, err := semver.Make(releaseName)
			if err != nil {
				versionChan <- nil
				return
			}

			// fetch archive for this tag and generate sha1 hash
			rawArchive, err := c.GetArchive(client.KindNpm, src.UUID, tag.Reference)
			if err != nil {
				versionChan <- nil
				return
			}

			// calculate sha1 sum and put data to cache
			_, sum, err := helpers.PutNpmArchiveToCache(rawArchive, src.UUID, tag.Reference)
			if err != nil {
				versionChan <- nil
				return
			}

			v := &npmVersion{
				Version: releaseInfo.String(),
			}

			tag.MetadataLock.RLock()
			v.Name, _ = tag.Metadata.GetString("name")
			v.Description, _ = tag.Metadata.GetString("description")
			v.Main, _ = tag.Metadata.GetString("main")
			v.Dependencies, _ = tag.Metadata.GetMapInterface("dependencies", nil)
			v.DevDependencies, _ = tag.Metadata.GetMapInterface("devDependencies", nil)
			v.Dist = npmDist{
				Sha:     sum,
				Tarball: fmt.Sprintf(endpoint, src.UUID, tag.Reference),
			}
			tag.MetadataLock.RUnlock()

			versionChan <- v
		}(tag)
	}

	for i := 0; i < len(src.TagList); i++ {
		v := <-versionChan
		if v != nil {
			p.Versions[v.Version] = *v
		}
	}

	return nil
}
