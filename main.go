package main

import (
	"github.com/bitrise-io/go-utils/log"
	"os"
	"github.com/bitrise-io/go-utils/command"
	"fmt"
	"runtime"
	"github.com/mholt/archiver"
	"io"
	"net/http"
	"io/ioutil"
	"github.com/bitrise-io/go-utils/pathutil"
	"strings"
	"path/filepath"
)

func main() {
	if err := ensureAndroidSdkSetup(); err != nil {
		log.Errorf("Could not setup Android SDK, error: %s", err)
		os.Exit(6)
	}

	configs := createConfigsModelFromEnvs()

	if err := configs.validate(); err != nil {
		log.Errorf("Could not validate config, error: %s", err)
		os.Exit(4)
	}
	configs.dump()

	flutterSdkDir, err := getSdkDestinationDir()
	if err != nil {
		log.Errorf("Could not validate config, error: %s", err)
		os.Exit(5)
	}

	flutterSdkExists, err := pathutil.IsDirExists(flutterSdkDir)
	if err != nil {
		log.Errorf("Could not check if Flutter SDK is installed, error: %s", err)
		os.Exit(1)
	}

	if !flutterSdkExists {
		if err := extractSdk(configs.Version, flutterSdkDir); err != nil {
			log.Errorf("Could not extract Flutter SDK, error: %s", err)
			os.Exit(2)
		}
	} else {
		log.Infof("Flutter SDK folder already exists, skipping installation.")
	}

	for _, flutterCommand := range configs.Commands {
		log.Infof("Executing Flutter command: %s", flutterCommand)

		flutterExecutablePath := filepath.Join(flutterSdkDir, "bin/flutter")
		bashCommand := fmt.Sprintf("%s %s", flutterExecutablePath, flutterCommand)
		err := command.RunCommandInDir(configs.WorkingDir, "bash", "-c", bashCommand)
		if err != nil {
			log.Errorf("Flutter invocation failed, error: %s", err)
			os.Exit(3)
		}
	}
}

func getArchiveExtension() string {
	if runtime.GOOS == "linux" {
		return "tar.xz"
	}
	return "zip"
}

func extractSdk(flutterVersion, flutterSdkDestinationDir string) error {
	log.Infof("Extracting Flutter SDK to %s", flutterSdkDestinationDir)

	versionComponents := strings.Split(flutterVersion, "-")
	channel := versionComponents[len(versionComponents)-1]

	flutterSdkSourceURL := fmt.Sprintf(
		"https://storage.googleapis.com/flutter_infra/releases/%s/%s/flutter_%s_v%s.%s",
		channel,
		getFlutterPlatform(),
		getFlutterPlatform(),
		flutterVersion,
		getArchiveExtension())

	flutterSdkParentDir := filepath.Join(flutterSdkDestinationDir, "..")

	if runtime.GOOS == "darwin" {
		return command.DownloadAndUnZIP(flutterSdkSourceURL, flutterSdkParentDir)
	} else if runtime.GOOS == "linux" {

		file, err := ioutil.TempFile(os.TempDir(), "flutter")
		if err != nil {
			return err
		}

		defer func() {
			if err := os.Remove(file.Name()); err != nil {
				log.Errorf("Failed to remove temporary file:", err)
			}
		}()

		if err := downloadFile(flutterSdkSourceURL, file); err != nil {
			return err
		}

		return archiver.TarXZ.Open(file.Name(), flutterSdkParentDir)
	} else {
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

func getSdkDestinationDir() (string, error) {
	if runtime.GOOS == "darwin" {
		return filepath.Join(pathutil.UserHomeDir(), "Library/flutter"), nil
	} else if runtime.GOOS == "linux" {
		return "/opt/flutter", nil
	}
	return "", fmt.Errorf("unsupported OS: %s", runtime.GOOS)
}

func getFlutterPlatform() string {
	if runtime.GOOS == "darwin" {
		return "macos"
	}
	return runtime.GOOS
}

func downloadFile(downloadURL string, outFile *os.File) error {
	resp, err := http.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("failed to download from %s, error: %s", downloadURL, err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Warnf("Failed to close (%s) body", downloadURL)
		}
	}()

	_, err = io.Copy(outFile, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to save file %s, error: %s", outFile.Name(), err)
	}

	return nil
}
