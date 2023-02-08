package managers

import (
	"archive/zip"
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"

	"github.com/splunk/kube-bundler/api/v1alpha1"
	"github.com/docker/cli/cli/config"
	cliTypes "github.com/docker/cli/cli/config/types"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/go-connections/nat"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/yaml"
)

const (
	HttpsScheme = "https"
	HttpScheme  = "http"

	DefaultRegistryImage = "registry:2.8.1"
)

type ErrorLine struct {
	Error       string      `json:"error"`
	ErrorDetail ErrorDetail `json:"errorDetail"`
}

type ErrorDetail struct {
	Message string `json:"message"`
}

type BuildManager struct {
	c                   *client.Client
	authConfigs         map[string]cliTypes.AuthConfig
	registryContainerId string

	dir     string
	appFile string
}

func NewBuildManager(dir, appFile string) (*BuildManager, error) {
	c, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, errors.Wrap(err, "couldn't start docker client")
	}

	configFile := config.LoadDefaultConfigFile(os.Stderr)
	authConfigs, err := configFile.GetAllCredentials()
	if err != nil {
		return nil, errors.Wrap(err, "could not load docker credentials")
	}

	return &BuildManager{
		c:           c,
		authConfigs: authConfigs,
		dir:         dir,
		appFile:     appFile,
	}, nil
}

func (bm *BuildManager) BuildDeployImage(ctx context.Context, dir string, argsMap map[string]*string) error {

	log.Info("Starting build deploy image...")
	app, err := bm.loadApp(ctx)
	if err != nil {
		return errors.Wrap(err, "couldn't build deploy image")
	}
	imagePath := app.Spec.DeployImage

	log.WithFields(log.Fields{"imagePath": imagePath, "dir": dir}).Info("Printing the imagePath and dir values that were passed in")

	tar, err := archive.TarWithOptions(dir, &archive.TarOptions{})
	if err != nil {
		return err
	}

	AuthConfigsRegistries := make(map[string]types.AuthConfig)

	// Embed all registries creds in AuthConfigsRegistries. This is to be passed in to imagebuildoptions later
	// which then uses the credentials to pull in any image from any of available registries.
	// Docker build needs to pull in images when Dockerfile uses "From <imageName>"
	for registry, auth := range bm.authConfigs {
		authConfig := types.AuthConfig{
			Username: auth.Username,
			Password: auth.Password,
		}
		AuthConfigsRegistries[registry] = authConfig
	}

	opts := types.ImageBuildOptions{
		Dockerfile:  "Dockerfile",
		Tags:        []string{imagePath},
		Remove:      true,
		BuildArgs:   argsMap,
		AuthConfigs: AuthConfigsRegistries,
	}
	res, err := bm.c.ImageBuild(ctx, tar, opts)
	if err != nil {
		return err
	}

	defer res.Body.Close()

	err = printLogs(res.Body)
	if err != nil {
		return err
	}
	log.Info("Deploy image successfully built!")
	return nil
}

func (bm *BuildManager) UploadDeployImage(ctx context.Context) error {
	app, err := bm.loadApp(ctx)
	if err != nil {
		return errors.Wrap(err, "couldn't upload deploy image")
	}
	imagePath := app.Spec.DeployImage

	log.Info("Starting upload deploy image.......")
	scheme := HttpsScheme
	fullImage := scheme + "://" + imagePath

	authStr, err := bm.fetchRegAuth(fullImage, scheme, false)
	if err != nil {
		return errors.Wrapf(err, "unable to fetch the registry authString")
	}

	opts := types.ImagePushOptions{RegistryAuth: authStr}

	reader, err := bm.c.ImagePush(ctx, imagePath, opts)
	if err != nil {
		return errors.Wrapf(err, "couldn't push image '%s' to registry", imagePath)
	}

	_, err = io.Copy(os.Stdout, reader)
	if err != nil {
		return errors.Wrap(err, "couldn't print push progress")
	}
	reader.Close()

	log.Info("Successfully uploaded deploy image!")
	return nil
}

func (bm *BuildManager) Build(ctx context.Context, registryImage string, allowAnonymousPull bool) error {
	app, err := bm.loadApp(ctx)
	if err != nil {
		return errors.Wrap(err, "couldn't create bundle")
	}

	var tarContents io.ReadCloser
	err = bm.launchLocalRegistry(ctx, registryImage)
	if err != nil {
		return errors.Wrap(err, "couldn't launch local registry")
	}

	defer func() {
		err := bm.stopLocalRegistry(ctx)
		if err != nil {
			log.WithError(err).WithField("containerId", bm.registryContainerId).Error("failed to stop registry")
		}
	}()

	err = bm.pullImages(ctx, app, allowAnonymousPull)
	if err != nil {
		return errors.Wrap(err, "couldn't pull bundle images")
	}

	err = bm.pushImages(ctx, app)
	if err != nil {
		return errors.Wrap(err, "couldn't push bundle images")
	}

	tarContents, err = bm.copyImages(ctx)
	if err != nil {
		return errors.Wrap(err, "couldn't copy bundle images")
	}
	defer tarContents.Close()

	err = bm.createKbFile(ctx, app, tarContents)
	if err != nil {
		return errors.Wrap(err, "couldn't create bundle")
	}

	return nil
}

func (bm *BuildManager) loadApp(ctx context.Context) (*v1alpha1.Application, error) {
	// Read application definition
	f, err := os.Open(filepath.Join(bm.dir, bm.appFile))
	if err != nil {
		return nil, errors.Wrap(err, "couldn't open application definition file")
	}
	defer f.Close()

	// Deserialize app to get name and version
	var app v1alpha1.Application
	decoder := yaml.NewYAMLOrJSONDecoder(f, 100)
	err = decoder.Decode(&app)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't decode app yaml")
	}

	err = app.Validate()
	if err != nil {
		return nil, errors.Wrap(err, "couldn't validate")
	}

	return &app, nil
}

// createKbFile creates the zip file.
func (bm *BuildManager) createKbFile(ctx context.Context, app *v1alpha1.Application, images io.ReadCloser) error {
	packageFilename := fmt.Sprintf("%s-%s.kb", app.Spec.Name, app.Spec.Version)
	bundleFile, err := os.Create(filepath.Join(bm.dir, packageFilename))
	if err != nil {
		return errors.Wrap(err, "couldn't create bundle")
	}
	defer bundleFile.Close()

	bundleZip := zip.NewWriter(bundleFile)
	defer bundleZip.Close()

	dstAppFile, err := bundleZip.Create(DefaultAppFile)
	if err != nil {
		return errors.Wrap(err, "couldn't create application definition inside bundle")
	}

	srcAppFile, err := os.Open(filepath.Join(bm.dir, bm.appFile))
	if err != nil {
		return errors.Wrap(err, "couldn't open application definition file")
	}
	defer srcAppFile.Close()

	_, err = io.Copy(dstAppFile, srcAppFile)
	if err != nil {
		return errors.Wrap(err, "couldn't copy from application definition to bundle")
	}

	imagesFile, err := bundleZip.Create(DefaultImagesFile)
	if err != nil {
		return errors.Wrap(err, "couldn't open images file")
	}

	_, err = io.Copy(imagesFile, images)
	if err != nil {
		return errors.Wrap(err, "couldn't copy from images file to bundle")
	}

	return nil
}

func (bm *BuildManager) launchLocalRegistry(ctx context.Context, image string) error {
	// TODO: use randomized port to avoid collisions
	config := &container.Config{
		Image:        image,
		Tty:          false,
		ExposedPorts: nat.PortSet{"5000/tcp": struct{}{}},
	}

	hostConfig := &container.HostConfig{
		PortBindings: nat.PortMap{
			"5000/tcp": []nat.PortBinding{
				{
					HostIP:   "0.0.0.0",
					HostPort: "5000",
				},
			},
		},
	}

	resp, err := bm.c.ContainerCreate(ctx, config, hostConfig, nil, nil, "")
	if err != nil {
		return errors.Wrap(err, "couldn't launch kb registry")
	}

	err = bm.c.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{})
	if err != nil {
		return errors.Wrap(err, "couldn't start kb registry")
	}

	bm.registryContainerId = resp.ID

	// TODO: If pushes become flakey, wait for container to start
	return nil
}

func (bm *BuildManager) stopLocalRegistry(ctx context.Context) error {
	err := bm.c.ContainerStop(ctx, bm.registryContainerId, nil)
	if err != nil {
		return errors.Wrap(err, "couldn't stop local registry container")
	}

	err = bm.c.ContainerRemove(ctx, bm.registryContainerId, types.ContainerRemoveOptions{RemoveVolumes: true})
	if err != nil {
		return errors.Wrap(err, "couldn't remove local registry container")
	}

	return nil
}

func (bm *BuildManager) pullImages(ctx context.Context, app *v1alpha1.Application, anonymousPull bool) error {
	allImages := app.Spec.Images
	allImages = append(allImages, v1alpha1.ImageSpec{Image: app.Spec.DeployImage})

	// Bundle images
	for _, image := range allImages {
		scheme := image.Scheme
		if scheme == "" {
			scheme = HttpsScheme
		}

		fullImage := scheme + "://" + image.Image

		authStr, err := bm.fetchRegAuth(fullImage, scheme, anonymousPull)
		if err != nil {
			return errors.Wrapf(err, "unable to fetch the registry authString")
		}

		opts := types.ImagePullOptions{RegistryAuth: authStr}
		reader, err := bm.c.ImagePull(ctx, image.Image, opts)
		if err != nil {
			return errors.Wrapf(err, "couldn't pull image %s", image.Image)
		}
		_, err = io.Copy(os.Stdout, reader)
		if err != nil {
			return errors.Wrap(err, "couldn't print image pull progress")
		}
		reader.Close()
	}

	return nil
}

func (bm *BuildManager) fetchRegAuth(imagePath string, scheme string, anonymousPull bool) (string, error) {

	log.Info("Fetching registry auth string...")
	url, err := url.Parse(imagePath)
	if err != nil {
		return "", errors.Wrap(err, "couldn't parse docker image URL")
	}

	var auth cliTypes.AuthConfig
	var urlWithSchemeFound, urlWithoutSchemeFound bool

	dockerRegistryWithScheme := scheme + "://" + url.Host
	log.WithField("registry", dockerRegistryWithScheme).Debug("Checking auths for registry with scheme")
	auth, urlWithSchemeFound = bm.authConfigs[dockerRegistryWithScheme]
	if !urlWithSchemeFound {
		log.WithField("registry", url.Host).Debug("Checking auths for registry without scheme")

		auth, urlWithoutSchemeFound = bm.authConfigs[url.Host]
		if !urlWithoutSchemeFound {
			if anonymousPull {
				log.WithFields(log.Fields{"url": url.Host}).Warn("couldn't find authentication entry for registry, proceeding anyway")
			} else {
				return "", fmt.Errorf("couldn't find auth token for host '%s'", dockerRegistryWithScheme)
			}
		}
	}

	authConfig := types.AuthConfig{
		Username: auth.Username,
		Password: auth.Password,
	}

	encodedJSON, err := json.Marshal(authConfig)
	if err != nil {
		return "", errors.Wrap(err, "couldn't encode auth config")
	}
	return base64.URLEncoding.EncodeToString(encodedJSON), nil
}

func (bm *BuildManager) pushImages(ctx context.Context, app *v1alpha1.Application) error {
	allImages := app.Spec.Images
	allImages = append(allImages, v1alpha1.ImageSpec{Image: app.Spec.DeployImage})

	for _, image := range allImages {
		scheme := image.Scheme
		if scheme == "" {
			scheme = HttpsScheme
		}

		fullImage := scheme + "://" + image.Image
		url, err := url.Parse(fullImage)
		if err != nil {
			return errors.Wrap(err, "couldn't parse docker image URL")
		}

		targetImage := "localhost:5000" + url.Path
		err = bm.c.ImageTag(ctx, image.Image, targetImage)
		if err != nil {
			return errors.Wrapf(err, "couldn't tag image '%s' for local registry", targetImage)
		}

		// All pushes require a non-zero length RegistryAuth, and "123" is commonly used as a placeholder
		opts := types.ImagePushOptions{All: true, RegistryAuth: "123"}
		reader, err := bm.c.ImagePush(ctx, targetImage, opts)
		if err != nil {
			return errors.Wrapf(err, "couldn't push image '%s' to local registry", targetImage)
		}

		_, err = io.Copy(os.Stdout, reader)
		if err != nil {
			return errors.Wrap(err, "couldn't print push progress")
		}
		reader.Close()
	}

	return nil
}

func (bm *BuildManager) copyImages(ctx context.Context) (io.ReadCloser, error) {
	reader, _, err := bm.c.CopyFromContainer(ctx, bm.registryContainerId, "/var/lib/registry/docker")
	if err != nil {
		return nil, errors.Wrap(err, "couldn't copy docker layers from local registry")
	}

	return reader, nil
}

func printLogs(rd io.Reader) error {
	var lastLine string

	scanner := bufio.NewScanner(rd)
	for scanner.Scan() {
		lastLine = scanner.Text()
		fmt.Println(scanner.Text())
	}

	errLine := &ErrorLine{}
	json.Unmarshal([]byte(lastLine), errLine)
	if errLine.Error != "" {
		return errors.New(errLine.Error)
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	return nil
}
