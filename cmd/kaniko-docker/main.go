package main

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/joho/godotenv"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	kaniko "github.com/drone/drone-kaniko"
)

const (
	// Docker file path
	dockerPath       string = "/kaniko/.docker"
	dockerConfigPath string = "/kaniko/.docker/config.json"

	v1Registry string = "https://index.docker.io/v1/" // Default registry
	v2Registry string = "https://index.docker.io/v2/" // v2 registry is not supported
)

var (
	version = "unknown"
)

func main() {
	// Load env-file if it exists first
	if env := os.Getenv("PLUGIN_ENV_FILE"); env != "" {
		godotenv.Load(env)
	}

	app := cli.NewApp()
	app.Name = "kaniko docker plugin"
	app.Usage = "kaniko docker plugin"
	app.Action = run
	app.Version = version
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:   "dockerfile",
			Usage:  "build dockerfile",
			Value:  "Dockerfile",
			EnvVar: "PLUGIN_DOCKERFILE",
		},
		cli.StringFlag{
			Name:   "context",
			Usage:  "build context",
			Value:  ".",
			EnvVar: "PLUGIN_CONTEXT",
		},
		cli.StringSliceFlag{
			Name:     "tags",
			Usage:    "build tags",
			Value:    &cli.StringSlice{"latest"},
			EnvVar:   "PLUGIN_TAGS",
			FilePath: ".tags",
		},
		cli.StringSliceFlag{
			Name:   "args",
			Usage:  "build args",
			EnvVar: "PLUGIN_BUILD_ARGS",
		},
		cli.StringFlag{
			Name:   "target",
			Usage:  "build target",
			EnvVar: "PLUGIN_TARGET",
		},
		cli.StringFlag{
			Name:   "repo",
			Usage:  "docker repository",
			EnvVar: "PLUGIN_REPO",
		},
		cli.StringSliceFlag{
			Name:   "custom-labels",
			Usage:  "additional k=v labels",
			EnvVar: "PLUGIN_CUSTOM_LABELS",
		},
		cli.StringFlag{
			Name:   "registry",
			Usage:  "docker registry",
			Value:  v1Registry,
			EnvVar: "PLUGIN_REGISTRY",
		},
		cli.StringFlag{
			Name:   "username",
			Usage:  "docker username",
			EnvVar: "PLUGIN_USERNAME",
		},
		cli.StringFlag{
			Name:   "password",
			Usage:  "docker password",
			EnvVar: "PLUGIN_PASSWORD",
		},
		cli.BoolFlag{
			Name:   "skip-tls-verify",
			Usage:  "Skip registry tls verify",
			EnvVar: "PLUGIN_SKIP_TLS_VERIFY",
		},
		cli.StringFlag{
			Name:   "snapshot-mode",
			Usage:  "Specify one of full, redo or time as snapshot mode",
			EnvVar: "PLUGIN_SNAPSHOT_MODE",
		},
		cli.BoolFlag{
			Name:   "enable-cache",
			Usage:  "Set this flag to opt into caching with kaniko",
			EnvVar: "PLUGIN_ENABLE_CACHE",
		},
		cli.StringFlag{
			Name:   "cache-repo",
			Usage:  "Remote repository that will be used to store cached layers. enable-cache needs to be set to use this flag",
			EnvVar: "PLUGIN_CACHE_REPO",
		},
		cli.IntFlag{
			Name:   "cache-ttl",
			Usage:  "Cache timeout in hours. Defaults to two weeks.",
			EnvVar: "PLUGIN_CACHE_TTL",
		},
	}

	if err := app.Run(os.Args); err != nil {
		logrus.Fatal(err)
	}
}

func run(c *cli.Context) error {
	err := createDockerCfgFile(c.String("username"), c.String("password"), c.String("registry"))
	if err != nil {
		return err
	}

	plugin := kaniko.Plugin{
		Build: kaniko.Build{
			Dockerfile:    c.String("dockerfile"),
			Context:       c.String("context"),
			Tags:          c.StringSlice("tags"),
			Args:          c.StringSlice("args"),
			Target:        c.String("target"),
			Repo:          c.String("repo"),
			Labels:        c.StringSlice("custom-labels"),
			SkipTlsVerify: c.Bool("skip-tls-verify"),
			SnapshotMode:  c.String("snapshot-mode"),
			EnableCache:   c.Bool("enable-cache"),
			CacheRepo:     c.String("cache-repo"),
			CacheTTL:      c.Int("cache-ttl"),
		},
	}
	return plugin.Exec()
}

// Create the docker config file for authentication
func createDockerCfgFile(username, password, registry string) error {
	if username == "" {
		return fmt.Errorf("Username must be specified")
	}
	if password == "" {
		return fmt.Errorf("Password must be specified")
	}
	if registry == "" {
		return fmt.Errorf("Registry must be specified")
	}

	if registry == v2Registry {
		fmt.Println("Docker v2 registry is not supported in kaniko. Refer issue: https://github.com/GoogleContainerTools/kaniko/issues/1209")
		fmt.Printf("Using v1 registry instead: %s\n", v1Registry)
		registry = v1Registry
	}

	err := os.MkdirAll(dockerPath, 0600)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("failed to create %s directory", dockerPath))
	}

	authBytes := []byte(fmt.Sprintf("%s:%s", username, password))
	encodedString := base64.StdEncoding.EncodeToString(authBytes)
	jsonBytes := []byte(fmt.Sprintf(`{"auths": {"%s": {"auth": "%s"}}}`, registry, encodedString))
	err = ioutil.WriteFile(dockerConfigPath, jsonBytes, 0644)
	if err != nil {
		return errors.Wrap(err, "failed to create docker config file")
	}
	return nil
}
