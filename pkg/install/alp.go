package install

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	myerrors "github.com/ShotaKitazawa/isucontinuous/pkg/errors"
)

func (i *Installer) Alp(ctx context.Context, version string) error {
	i.log.Info("### install alp ###")

	// ealry return if alp has already installed
	if stdout, _, _ := i.runCommand(ctx, "", "which -a alp"); len(stdout.Bytes()) != 0 {
		i.log.Info("... alp has already been installed")
		return nil
	}

	if version == "latest" {
		// TODO
		// get release
		// get latest tag
	}
	command := fmt.Sprintf(
		"curl -sL https://github.com/tkuchiki/alp/releases/download/%s/alp_linux_amd64.zip -o /tmp/alp.zip",
		version)
	stdout, stderr, err := i.runCommand(ctx, "", command)
	if err != nil {
		return myerrors.NewErrorCommandExecutionFailed(stderr)
	}
	i.log.Debug(stdout.String())

	if err := i.unzip("/tmp/alp.zip", "/usr/local/bin/"); err != nil {
		return myerrors.NewErrorCommandExecutionFailed(stderr)
	}

	i.log.Info("... installed alp!")
	return nil
}

func (i *Installer) unzip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()
	for _, f := range r.File {
		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer rc.Close()
		if f.FileInfo().IsDir() {
			path := filepath.Join(dest, f.Name)
			if err := os.MkdirAll(path, f.Mode()); err != nil {
				return err
			}
		} else {
			buf := make([]byte, f.UncompressedSize)
			_, err = io.ReadFull(rc, buf)
			if err != nil {
				return err
			}
			path := filepath.Join(dest, f.Name)
			if err = ioutil.WriteFile(path, buf, f.Mode()); err != nil {
				return err
			}
		}
	}
	return nil
}
