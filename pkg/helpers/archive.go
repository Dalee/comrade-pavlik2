package helpers

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha1"
	"fmt"
	"github.com/hashicorp/golang-lru"
	"github.com/jhoonb/archivex"
	"github.com/satori/go.uuid"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
)

var (
	globalCache, _ = lru.New(2048)
)

func DataFromCache(repoUUID, repoRef string) ([]byte, error) {
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

	return nil, fmt.Errorf("No cache for: archive-lru %s # %s", repoUUID, repoRef)
}

//
// For a given GitLab project archive, calculate sha1 hash
// and put archive to cache.
// sha1 hash is mandatory for npm.
//
func DataToCache(src []byte, repoUUID, repoRef string) (string, error) {
	// WARNING: *never* cache master ref
	if repoRef != "master" {
		cacheKey := fmt.Sprintf("archive_%s_%s", repoUUID, repoRef)
		globalCache.Add(cacheKey, src)
	}

	sum := fmt.Sprintf("%x", sha1.Sum(src))
	return sum, nil
}

//
// GitLab serves repository archive as tar.gz archive
// so, for composer:
// 	* repack gitlab archive: ungzip -> untar -> zip
// 	* cleanup all temporary files and directories
//	* return zip archive bytes
//      * cache archive forever
//
func TarGzToZip(src []byte, repoUUID, repoRef string) ([]byte, error) {
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

	tarDestinationFile := filepath.Join(t, fmt.Sprintf("%s.tar", u))
	tarDestinationDir := filepath.Join(t, fmt.Sprintf("dir_%s", u))
	zipDestinationFile := filepath.Join(t, fmt.Sprintf("%s.zip", u))

	if err := ungzip(src, tarDestinationFile); err != nil {
		return nil, err
	}

	tarBytes, err := getFileContents(tarDestinationFile)
	if err != nil {
		return nil, err
	}

	if err := os.Remove(tarDestinationFile); err != nil {
		return nil, err
	}

	if err := untar(tarBytes, tarDestinationDir); err != nil {
		return nil, err
	}

	if err := zip(tarDestinationDir, zipDestinationFile); err != nil {
		return nil, err
	}

	if err := os.RemoveAll(tarDestinationDir); err != nil {
		return nil, err
	}

	archive, err := getFileContents(zipDestinationFile)
	if err != nil {
		return nil, err
	}

	if err := os.Remove(zipDestinationFile); err != nil {
		return nil, err
	}

	// WARNING: *don't even think* to put master ref into cache
	if repoRef != "master" {
		globalCache.Add(cacheKey, archive)
	}

	return archive, nil
}

//
func getFileContents(path string) ([]byte, error) {
	fp, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer fp.Close()

	content, err := ioutil.ReadAll(fp)
	if err != nil {
		return nil, err
	}

	return content, nil
}

func putFileContents(path string, data io.Reader, flag int, mode os.FileMode) error {
	fp, err := os.OpenFile(path, flag, mode)
	if err != nil {
		return err
	}

	_, err = io.Copy(fp, data)

	fp.Close()
	return err
}

//
func untar(archive []byte, target string) error {
	byteReader := bytes.NewReader(archive)
	archiveReader := tar.NewReader(byteReader)

	if err := os.MkdirAll(target, 0755); err != nil {
		return err
	}

	for {
		var header *tar.Header
		var err error

		if header, err = archiveReader.Next(); err != nil {
			if err == io.EOF {
				break
			}

			return err
		}

		// skip global header
		if header.Name == "pax_global_header" {
			continue
		}

		path := filepath.Join(target, header.Name)
		info := header.FileInfo()

		if info.IsDir() {
			if err = os.MkdirAll(path, info.Mode()); err != nil {
				return err
			}
			continue
		}

		err = putFileContents(path, archiveReader, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode())
		if err != nil {
			return err
		}
	}

	return nil
}

//
func ungzip(archive []byte, target string) error {
	byteReader := bytes.NewReader(archive)
	archiveReader, err := gzip.NewReader(byteReader)
	if err != nil {
		return err
	}
	defer archiveReader.Close()

	target = filepath.Join(target, archiveReader.Name)
	writer, err := os.Create(target)
	if err != nil {
		return err
	}
	defer writer.Close()

	_, err = io.Copy(writer, archiveReader)
	return err
}

//
func zip(sourceDir string, targetFile string) error {
	zip := new(archivex.ZipFile)
	if err := zip.Create(targetFile); err != nil {
		return err
	}

	if err := zip.AddAll(sourceDir, false); err != nil {
		return err
	}

	zip.Close()
	return nil
}
