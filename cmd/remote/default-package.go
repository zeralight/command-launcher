package remote

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/criteo/command-launcher/internal/command"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

type defaultPackageManifest struct {
	PkgName     string                    `json:"pkgName" yaml:"pkgName"`
	PkgVersion  string                    `json:"version" yaml:"version"`
	PkgCommands []*command.DefaultCommand `json:"cmds" yaml:"cmds"`
}

func (mf *defaultPackageManifest) Name() string {
	return mf.PkgName
}

func (mf *defaultPackageManifest) Version() string {
	return mf.PkgVersion
}

func (mf *defaultPackageManifest) Commands() []command.Command {
	cmds := make([]command.Command, 0)
	for _, cmd := range mf.PkgCommands {
		//newCmd := cmd
		cmds = append(cmds, cmd)
	}
	return cmds
}

type defaultPackage struct {
	Manifest command.PackageManifest
	ZipFile  string
}

func (pkg *defaultPackage) Name() string {
	return pkg.Manifest.Name()
}

func (pkg *defaultPackage) Version() string {
	return pkg.Manifest.Version()
}

func (pkg *defaultPackage) Commands() []command.Command {
	return pkg.Manifest.Commands()
}

func (pkg *defaultPackage) InstallTo(targetDir string) (command.PackageManifest, error) {
	zipReader, _ := zip.OpenReader(pkg.ZipFile)
	defer zipReader.Close()
	for _, file := range zipReader.Reader.File {
		zippedFile, err := file.Open()
		if err != nil {
			return nil, fmt.Errorf("installation failed: %s", err)
		}
		defer zippedFile.Close()

		extractedFilePath := filepath.Join(targetDir, file.Name)
		if file.FileInfo().IsDir() {
			log.Println("Directory Created:", extractedFilePath)
			err := os.MkdirAll(extractedFilePath, file.Mode())
			if err != nil {
				return nil, fmt.Errorf("directory extraction failed: %s", err)
			}
		} else {
			log.Println("File extracted:", file.Name)
			outputFile, err := os.OpenFile(
				extractedFilePath,
				os.O_WRONLY|os.O_CREATE|os.O_TRUNC,
				file.Mode(),
			)
			if err != nil {
				return nil, fmt.Errorf("file extraction failed: %s", err)
			}
			defer outputFile.Close()

			_, err = io.Copy(outputFile, zippedFile)
			if err != nil {
				return nil, fmt.Errorf("file data extraction failed: %s", err)
			}
		}
	}

	manifestfile, _ := os.Open(filepath.Join(targetDir, "manifest.mf"))
	defer manifestfile.Close()

	mf, err := ReadManifest(manifestfile)
	if err != nil {
		return nil, fmt.Errorf("cannot read package manifest: %s", err)
	}

	return mf, nil
}

func CreatePackage(zipFilename string) (command.Package, error) {
	reader, err := zip.OpenReader(zipFilename)
	if err != nil {
		return nil, fmt.Errorf("failed to open: %s", err)
	}
	defer reader.Close()
	manifestFile, err := reader.Open("manifest.mf")
	if err != nil {
		return nil, fmt.Errorf("failed to open the manifest: %s", err)
	}
	defer manifestFile.Close()

	mf, err := ReadManifest(manifestFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read the manifest: %s", err)
	}

	var pkg = defaultPackage{
		Manifest: mf,
		ZipFile:  zipFilename,
	}

	return &pkg, nil
}

func ReadManifest(file fs.File) (command.PackageManifest, error) {
	stat, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("cannot read the manifest file handle (%s)", err)
	}

	var payload = make([]byte, stat.Size())
	nb, err := file.Read(payload)
	if err != nil && err != io.EOF || nb != int(stat.Size()) {
		return nil, fmt.Errorf("cannot read the manifest (%s)", err)
	}

	var mf = defaultPackageManifest{}
	// TODO: deperacate the JSON format
	err = json.Unmarshal(payload, &mf)
	if err != nil {
		// try YAML format
		yamlErr := yaml.Unmarshal(payload, &mf)
		if yamlErr != nil {
			return nil, fmt.Errorf("cannot read the manifest content, it is neither a valid JSON (%s) nor YAML (%s)", err, yamlErr)
		}
	}

	return &mf, nil
}
