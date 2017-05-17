package helpers

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha1"
	"errors"
	"fmt"
	"github.com/hashicorp/golang-lru"
	"github.com/jhoonb/archivex"
	"github.com/satori/go.uuid"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"time"
)

var (
	globalCache, _ = lru.New(2048)
	archiveTime    = time.Date(2016, time.October, 16, 23, 0, 0, 0, time.UTC)
)

//
// Fetch .tgz version of npm archive stored in cache
//
func GetNpmArchiveFromCache(repoUUID, repoRef string) ([]byte, error) {
	cacheKey := fmt.Sprintf("archive_%s_%s", repoUUID, repoRef)

	if item, ok := globalCache.Get(cacheKey); ok {
		if archive, ok := item.([]byte); ok {
			log.Printf("Cache hit: archive-lru %s # %s", repoUUID, repoRef)
			return archive, nil
		} else {
			globalCache.Remove(cacheKey)
			return nil, fmt.Errorf("Cache broken: archive-lru %s # %s", repoUUID, repoRef)
		}
	}

	return nil, fmt.Errorf("No cache found: archive-lru %s # %s", repoUUID, repoRef)
}

//
// Note:
//
// When project npm-shrinkwrap.json or yarn.lock used to access two different
// Pavlik instances with different GitLab instances, repository names can differ.
// In that case, mandatory "shasum" field will not match.
//
// So, for npm:
// * repack tar.gz archive recevied from GitLab: tar.gz -> tar -> tar -> tgz
// * force set constant mtime/atime for directories and files
// * cache final archive bytes
// * return final archive bytes and calculated shasum
//
func PutNpmArchiveToCache(src []byte, repoUUID, repoRef string) ([]byte, string, error) {
	// check in cache
	if npmArchive, err := GetNpmArchiveFromCache(repoUUID, repoRef); err == nil {
		log.Printf("Cache hit: archive-lru %s # %s", repoUUID, repoRef)
		return npmArchive, fmt.Sprintf("%x", sha1.Sum(npmArchive)), nil
	}

	log.Printf("Cache miss: archive-lru %s # %s", repoUUID, repoRef)

	// define some properties
	u := uuid.NewV4().String()
	t := os.TempDir()

	tarBeforeRenameDir := filepath.Join(t, fmt.Sprintf("dir_%s", u))
	tarDestinationFile := filepath.Join(t, fmt.Sprintf("%s.tar", u))
	tarDestinationDir := filepath.Join(t, fmt.Sprintf("dir_%s", u))
	tgzDestinationFile := filepath.Join(t, fmt.Sprintf("%s.tgz", u))

	if err := unGzip(src, tarDestinationFile); err != nil {
		return nil, "", err
	}

	tarArchive, err := getFileContents(tarDestinationFile)
	os.Remove(tarDestinationFile)
	if err != nil {
		return nil, "", err
	}

	if err := unTar(tarArchive, tarDestinationDir); err != nil {
		return nil, "", err
	}

	//
	files, err := ioutil.ReadDir(tarDestinationDir)
	if err != nil {
		return nil, "", err
	}
	if len(files) != 1 {
		return nil, "", errors.New("Broken archive received from GitLab")
	}

	//
	oldPath := filepath.Join(tarBeforeRenameDir, files[0].Name())
	tarDestinationDir = filepath.Join(tarBeforeRenameDir, fmt.Sprintf("%s-%s", repoUUID, repoRef))
	if err := os.Rename(oldPath, tarDestinationDir); err != nil {
		return nil, "", err
	}

	err = makeTar(tarDestinationDir, tarDestinationFile)
	os.RemoveAll(tarBeforeRenameDir)
	if err != nil {
		return nil, "", err
	}

	tarArchive, err = getFileContents(tarDestinationFile)
	os.Remove(tarDestinationFile)
	if err != nil {
		return nil, "", err
	}

	if err := makeGzip(tarArchive, tgzDestinationFile); err != nil {
		return nil, "", err
	}

	npmArchive, err := getFileContents(tgzDestinationFile)
	os.Remove(tgzDestinationFile)
	if err != nil {
		return nil, "", err
	}

	// WARNING: *never* cache master ref
	if repoRef != "master" {
		cacheKey := fmt.Sprintf("archive_%s_%s", repoUUID, repoRef)
		globalCache.Add(cacheKey, npmArchive)
	}

	return npmArchive, fmt.Sprintf("%x", sha1.Sum(npmArchive)), nil
}

//
// Note:
//
// GitLab serves repository archive as tar.gz archive, so, for composer:
// 	* repack gitlab archive: ungzip -> untar -> zip
//	* during repack, set directory and file mtime/atime to predefined constant
// 	* cleanup all temporary files and directories
//      * cache archive bytes
//	* return zip archive bytes
//
func GetComposerArchive(src []byte, repoUUID, repoRef string) ([]byte, error) {
	cacheKey := fmt.Sprintf("archive_%s_%s", repoUUID, repoRef)

	// WARNING: *never* cache master ref
	if repoRef != "master" {
		if item, ok := globalCache.Get(cacheKey); ok {
			if archive, ok := item.([]byte); ok {
				log.Printf("Cache hit: archive-lru %s # %s", repoUUID, repoRef)
				return archive, nil
			} else {
				globalCache.Remove(cacheKey)
				return nil, fmt.Errorf("Cache broken: archive-lru %s # %s", repoUUID, repoRef)
			}
		}
	}

	log.Printf("Cache miss: archive-lru %s # %s", repoUUID, repoRef)

	// define some properties
	u := uuid.NewV4().String()
	t := os.TempDir()

	tarBeforeRenameDir := filepath.Join(t, fmt.Sprintf("dir_%s", u))
	tarDestinationFile := filepath.Join(t, fmt.Sprintf("%s.tar", u))
	tarDestinationDir := filepath.Join(t, fmt.Sprintf("dir_%s", u))
	zipDestinationFile := filepath.Join(t, fmt.Sprintf("%s.zip", u))

	if err := unGzip(src, tarDestinationFile); err != nil {
		return nil, err
	}

	tarArchive, err := getFileContents(tarDestinationFile)
	os.Remove(tarDestinationFile)
	if err != nil {
		return nil, err
	}

	if err := unTar(tarArchive, tarDestinationDir); err != nil {
		return nil, err
	}

	//
	files, err := ioutil.ReadDir(tarDestinationDir)
	if err != nil {
		return nil, err
	}
	if len(files) != 1 {
		return nil, errors.New("Broken archive received from GitLab")
	}

	//
	oldPath := filepath.Join(tarBeforeRenameDir, files[0].Name())
	tarDestinationDir = filepath.Join(tarBeforeRenameDir, fmt.Sprintf("%s-%s", repoUUID, repoRef))
	if err := os.Rename(oldPath, tarDestinationDir); err != nil {
		return nil, err
	}

	err = makeZip(tarDestinationDir, zipDestinationFile)
	os.RemoveAll(tarBeforeRenameDir)
	if err != nil {
		return nil, err
	}

	composerArchive, err := getFileContents(zipDestinationFile)
	os.Remove(zipDestinationFile)
	if err != nil {
		return nil, err
	}

	// WARNING: *don't even think* to put master ref into cache
	if repoRef != "master" {
		globalCache.Add(cacheKey, composerArchive)
	}

	return composerArchive, nil
}

//
func getFileContents(targetFile string) ([]byte, error) {
	f, err := os.Open(targetFile)
	if err != nil {
		return nil, err
	}

	defer f.Close()

	return ioutil.ReadAll(f)
}

//
func putFileContents(targetFile string, src io.Reader) error {
	f, err := os.Create(targetFile)
	if err != nil {
		return err
	}

	io.Copy(f, src)
	f.Close()

	return recursiveSetFileTime(targetFile)
}

//
func makeTar(sourceDir string, targetFile string) error {
	recursiveSetFileTime(sourceDir)

	w := new(archivex.TarFile)
	if err := w.Create(targetFile); err != nil {
		return err
	}

	w.AddAll(sourceDir, true)
	w.Close()

	return recursiveSetFileTime(targetFile)
}

//
func unTar(archive []byte, targetDir string) error {
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return err
	}

	r := tar.NewReader(bytes.NewReader(archive))
	for {
		entryItem, err := r.Next()
		if err != nil && err == io.EOF {
			break
		}

		if err != nil && err != io.EOF {
			return err
		}

		if entryItem == nil || entryItem.Name == "pax_global_header" {
			continue
		}

		entryPath := filepath.Join(targetDir, entryItem.Name)
		if entryItem.FileInfo().IsDir() {
			if err := os.MkdirAll(entryPath, 0755); err != nil {
				return err
			}

		} else {
			if err := putFileContents(entryPath, r); err != nil {
				return err
			}
		}
	}

	//
	return recursiveSetFileTime(targetDir)
}

//
func makeGzip(archive []byte, targetFile string) error {
	dst, err := os.Create(targetFile)
	if err != nil {
		return err
	}

	writer := gzip.NewWriter(dst)
	defer dst.Close()
	defer writer.Close()

	_, err = writer.Write(archive)
	return err
}

//
func unGzip(archive []byte, targetFile string) error {
	src, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return err
	}
	defer src.Close()

	writer, err := os.Create(filepath.Join(targetFile, src.Name))
	if err != nil {
		return err
	}
	defer writer.Close()

	_, err = io.Copy(writer, src)
	return err
}

//
func makeZip(sourceDir string, targetFile string) error {
	recursiveSetFileTime(sourceDir)

	w := new(archivex.ZipFile)
	if err := w.Create(targetFile); err != nil {
		return err
	}

	w.AddAll(sourceDir, true)
	w.Close()

	return recursiveSetFileTime(targetFile)
}

//
func recursiveSetFileTime(rootEntry string) error {
	s, err := os.Stat(rootEntry)
	if err != nil {
		return err
	}

	if s.IsDir() {
		return filepath.Walk(rootEntry, func(entry string, f os.FileInfo, err error) error {
			return os.Chtimes(entry, archiveTime, archiveTime)
		})

	} else {
		return os.Chtimes(rootEntry, archiveTime, archiveTime)
	}
}
