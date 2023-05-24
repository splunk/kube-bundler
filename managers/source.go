/*
   Copyright 2023 Splunk Inc.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package managers

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strconv"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/splunk/kube-bundler/api/v1alpha1"
	"k8s.io/apimachinery/pkg/util/yaml"
)

const (
	DefaultAppFile    = "app.yaml"
	DefaultImagesFile = "images.tar"
	Latest            = "latest"
)

var (
	ErrNotFound       = errors.New("bundle not found")
	ErrNotImplemented = errors.New("not implemented")
)

type Source interface {
	// Get returns a bundle from the source, if it exists
	Get(bundleRef BundleRef) (*BundleFile, error)

	// Put puts a bundle and associated metadata file to the source
	Put(bundleFile *BundleFile) error
}

type BundleRef struct {
	// Name is the bundle version
	Name string

	// Version is the bundle version
	Version string
}

// Filename returns the bundle's filename which is always a combination of name and version
func (bf *BundleRef) Filename() string {
	return fmt.Sprintf("%s-%s.kb", bf.Name, bf.Version)
}

// MetadataFilename returns a filename used to store metadata about the bundle's available versions
func (bf *BundleRef) MetadataFilename() string {
	return fmt.Sprintf("%s.json", bf.Name)
}

type BundleFile struct {
	BundleRef

	// Contents are the contents of the bundle
	Contents io.ReaderAt

	// Size is the size of the bundle in bytes
	Size int64
}

type PublishMetadata struct {
	Latest ReleaseMetadata `json:"latest"`
}

type ReleaseMetadata struct {
	Version string `json:"version"`
	Size    string `json:"size"`
}

// NewBundleFromFile returns a BundleFile from a filename, or an error if the file couldn't be parsed.
// The caller should Close() the returned BundleFile when finished.
func NewBundleFromFile(filename string) (*BundleFile, error) {
	fileInfo, err := os.Stat(filename)
	if err != nil {
		return nil, errors.Wrapf(err, "couldn't stat bundle '%s'", filename)
	}

	f, err := os.Open(filename)
	if err != nil {
		return nil, errors.Wrapf(err, "couldn't open bundle '%s'", filename)
	}
	// Don't close to allow bundle file contents to be read

	bf := &BundleFile{
		Contents: f,
		Size:     fileInfo.Size(),
	}

	app, err := bf.Application("default")
	if err != nil {
		return nil, errors.Wrap(err, "couldn't open bundle application definition")
	}

	bf.Name = app.Spec.Name
	bf.Version = app.Spec.Version

	return bf, nil
}

// Application returns the parsed structure from app.yaml
func (bf *BundleFile) Application(namespace string) (*v1alpha1.Application, error) {
	zipReader, err := zip.NewReader(bf.Contents, bf.Size)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't open bundle reader")
	}

	f, err := zipReader.Open(DefaultAppFile)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't read bundle application definition")
	}
	defer f.Close()

	var app v1alpha1.Application
	decoder := yaml.NewYAMLOrJSONDecoder(f, 100)
	err = decoder.Decode(&app)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't decode bundle application definition")
	}

	app.Name = fmt.Sprintf("%s-%s", app.Spec.Name, app.Spec.Version)
	app.Namespace = namespace

	// TODO: support more than 1 provides
	if len(app.Spec.Provides) > 1 {
		return nil, fmt.Errorf("bundle '%s' can only provide 1 dependency", app.Spec.Name)
	}

	// If the provides list is empty, the bundle provides itself
	if len(app.Spec.Provides) == 0 {
		app.Spec.Provides = []v1alpha1.ProvidesList{
			{Name: app.Spec.Name},
		}
	}

	return &app, nil
}

func (bf *BundleFile) String() string {
	return fmt.Sprintf("%s-%s", bf.Name, bf.Version)
}

func (bf *BundleFile) Close() error {
	contents, ok := bf.Contents.(io.Closer)
	if ok {
		return contents.Close()
	}
	return nil
}

// NewSource creates a new source based on the provided sourceType and path
func NewSource(sourceType string, path string, options map[string]string, section, release string) (Source, error) {
	switch sourceType {
	case "directory":
		return &DirectorySource{
			Path:    path,
			Options: options,
			Section: section,
			Release: release,
		}, nil
	case "s3":
		return &S3Source{
			Path:    path,
			Options: options,
			Section: section,
			Release: release,
		}, nil
	}

	return nil, fmt.Errorf("unrecognized source: %s", sourceType)
}

// NewMultiSource searches multiple sources for the first match
func NewMultiSource(sources []Source) Source {
	return &MultiSource{
		sources: sources,
	}
}

type MultiSource struct {
	sources []Source
}

func (ms *MultiSource) Get(bundleRef BundleRef) (*BundleFile, error) {
	for _, source := range ms.sources {
		bundleFile, err := source.Get(bundleRef)
		if err == ErrNotFound {
			continue
		} else if err != nil {
			return nil, err
		}

		return bundleFile, nil
	}

	return nil, ErrNotFound
}

// Put is not implemented in MultiSource
func (ms *MultiSource) Put(bundleFile *BundleFile) error {
	return ErrNotImplemented
}

// DirectorySource returns bundle found in the directory
type DirectorySource struct {
	Path    string
	Options map[string]string
	Section string
	Release string
}

func (ds *DirectorySource) Get(bundleRef BundleRef) (*BundleFile, error) {
	metadataPath := filepath.Join(ds.Path, ds.Section, ds.Release, bundleRef.MetadataFilename())

	if bundleRef.Version == Latest {
		f, err := os.Open(metadataPath)
		if err != nil && os.IsNotExist(err) {
			return nil, ErrNotFound
		} else if err != nil {
			return nil, errors.Wrapf(err, "couldn't open metadata file '%s'", metadataPath)
		}
		defer f.Close()

		decoder := json.NewDecoder(f)
		var publishMetadata PublishMetadata
		err = decoder.Decode(&publishMetadata)
		if err != nil {
			return nil, errors.Wrapf(err, "couldn't decode metadata file '%s'", metadataPath)
		}

		bundleRef.Version = publishMetadata.Latest.Version
	}

	fullPath := filepath.Join(ds.Path, ds.Section, ds.Release, bundleRef.Filename())
	return NewBundleFromFile(fullPath)
}

func (ds *DirectorySource) Put(bundleFile *BundleFile) error {
	folderPath := filepath.Join(ds.Path, ds.Section, ds.Release)
	fullPath := filepath.Join(folderPath, bundleFile.Filename())

	if _, err := os.Stat(folderPath); os.IsNotExist(err) {
		os.MkdirAll(folderPath, 0700)
	}
	f, err := os.Create(fullPath)
	if err != nil {
		return errors.Wrapf(err, "couldn't open path '%s' for writing", fullPath)
	}

	reader := io.NewSectionReader(bundleFile.Contents, 0, bundleFile.Size)
	_, err = io.Copy(f, reader)
	if err != nil {
		return errors.Wrap(err, "couldn't copy file content")
	}

	err = f.Close()
	if err != nil {
		return errors.Wrap(err, "couldn't close bundle")
	}

	// Put bundle metadata
	// TODO: instead of overwriting unconditionally, merge with existing metadata
	publishMetadata := PublishMetadata{
		Latest: ReleaseMetadata{
			Version: bundleFile.Version,
			Size:    strconv.FormatInt(bundleFile.Size, 10),
		},
	}

	b, err := json.MarshalIndent(publishMetadata, "", "  ")
	if err != nil {
		return errors.Wrap(err, "couldn't encode metadata json")
	}

	fullPath = filepath.Join(ds.Path, ds.Section, ds.Release, bundleFile.MetadataFilename())
	err = os.WriteFile(fullPath, b, 0755)
	if err != nil {
		return errors.Wrap(err, "couldn't write metadata file")
	}

	return nil
}

// MultiFileSource returns a bundle found in the file list
type MultiFileSource struct {
	Files   []string
	Options map[string]string
}

// NewMultiFileSource allows creating a source from a collection of files
func NewMultiFileSource(files []string) Source {
	return &MultiFileSource{
		Files: files,
	}
}

func (mfs *MultiFileSource) Get(bundleRef BundleRef) (*BundleFile, error) {
	for _, file := range mfs.Files {
		if filepath.Base(file) == bundleRef.Filename() {
			return NewBundleFromFile(file)
		}
	}
	return nil, ErrNotFound
}

// Put is not implemented in MultiFileSource
func (mfs *MultiFileSource) Put(bundleFile *BundleFile) error {
	return ErrNotImplemented
}

// S3Source returns bundle found in the bucket path
type S3Source struct {
	Path    string
	Options map[string]string
	Section string
	Release string
}

func (s *S3Source) Get(bundleRef BundleRef) (*BundleFile, error) {
	bucket := s.Options["bucket"]
	region := s.Options["region"]
	metadataKey := path.Join(s.Path, s.Section, s.Release, bundleRef.MetadataFilename())

	if bucket == "" {
		return nil, errors.New("missing bucket")
	}
	if region == "" {
		return nil, errors.New("missing region")
	}

	sess, _ := session.NewSession(&aws.Config{
		Region: aws.String(region)},
	)

	// Use the metadata file if requesting the latest version
	if bundleRef.Version == Latest {
		s3Client := s3.New(sess)
		out, err := s3Client.GetObject(&s3.GetObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(metadataKey),
		})
		if err != nil {
			awsErr, ok := err.(awserr.Error)
			if ok && awsErr.Code() == s3.ErrCodeNoSuchKey {
				return nil, ErrNotFound
			}
			return nil, errors.Wrapf(err, "couldn't download metadata file 's3://%s'", path.Join(bucket, metadataKey))
		}

		decoder := json.NewDecoder(out.Body)
		var publishMetadata PublishMetadata
		err = decoder.Decode(&publishMetadata)

		// Exhaust any remaining buffer
		_, _ = io.Copy(io.Discard, out.Body)

		if err != nil {
			return nil, errors.Wrapf(err, "couldn't decode metadata file 's3://%s'", path.Join(bucket, metadataKey))
		}

		// Use the latest version
		bundleRef.Version = publishMetadata.Latest.Version
	}

	var f *os.File
	var err error

	f, err = os.CreateTemp("", fmt.Sprintf("%s.*.kb", bundleRef.Filename()))
	if err != nil {
		return nil, errors.Wrap(err, "couldn't create temp file")
	}

	// TODO: a better filesystem abstraction could serve range requests instead of having to
	// download the entire file (which could have large docker layers in the future)
	log.WithFields(log.Fields{"bundle": bundleRef.Name, "version": bundleRef.Version}).Info("Downloading bundle from s3")
	key := path.Join(s.Path, s.Section, s.Release, bundleRef.Filename())
	downloader := s3manager.NewDownloader(sess)
	downloader.PartSize = 100 * 1024 * 1024 // 100MB
	_, err = downloader.Download(
		f,
		&s3.GetObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
		})

	if err != nil {
		awsErr, ok := err.(awserr.Error)
		if ok && awsErr.Code() == "NotFound" {
			return nil, ErrNotFound
		}
		return nil, errors.Wrapf(err, "couldn't download s3://%s", path.Join(bucket, key))
	}

	err = f.Close()
	if err != nil {
		return nil, errors.Wrapf(err, "couldn't close bundle file")
	}

	// TODO: delete temp file on close
	return NewBundleFromFile(f.Name())
}

func (s *S3Source) Put(bundleFile *BundleFile) error {
	bucket := s.Options["bucket"]
	region := s.Options["region"]
	key := path.Join(s.Path, s.Section, s.Release, bundleFile.Filename())

	if bucket == "" {
		return errors.New("missing bucket")
	}
	if region == "" {
		return errors.New("missing region")
	}

	sess, _ := session.NewSession(&aws.Config{
		Region: aws.String(region)},
	)

	uploader := s3manager.NewUploader(sess)
	uploader.PartSize = 100 * 1024 * 1024 // 100MB

	reader := io.NewSectionReader(bundleFile.Contents, 0, bundleFile.Size)
	_, err := uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   reader,
	})

	if err != nil {
		return errors.Wrap(err, "couldn't upload to s3")
	}

	// Put bundle metadata
	// TODO: instead of overwriting unconditionally, merge with existing metadata
	publishMetadata := PublishMetadata{
		Latest: ReleaseMetadata{
			Version: bundleFile.Version,
			Size:    strconv.FormatInt(bundleFile.Size, 10),
		},
	}

	b, err := json.MarshalIndent(publishMetadata, "", "  ")
	if err != nil {
		return errors.Wrap(err, "couldn't encode metadata json")
	}

	key = path.Join(s.Path, s.Section, s.Release, bundleFile.MetadataFilename())
	bytesReader := bytes.NewReader(b)

	_, err = uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   bytesReader,
	})

	if err != nil {
		return errors.Wrap(err, "couldn't upload bundle metadata to s3")
	}

	return nil
}
