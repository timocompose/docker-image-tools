/*
make_slug <tar file> <metadata conf file> <slug tgz file>
Create output tgz if slug format with tar file in "diff.tar"
and metadata conf file in "METADATA/conf"
*/
package main

import (
	. "github.com/timocompose/docker-image-tools"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
)

func main() {
	var opt Options
	AddQuietFlag(&opt)
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: make_slug [options] <tar file> <metadata conf file> <tgz file>")
		fmt.Fprintln(os.Stderr, "Create output <tgz file> if slug format with <tar file> as \"diff.tar\" and <metadata conf file> as \"METADATA/conf\".")
		fmt.Fprintln(os.Stderr, "Options:")
		flag.PrintDefaults()
		os.Exit(0)
	}
	flag.Parse()

	args := flag.Args()
	if len(args) != 3 {
		log.Println("usage make_slug <tar file> <metadata conf file> <slug tgz file>")
		log.Fatal("make_slug --help for more information")
	}
	if err := makeSlug(args[0], args[1], args[2], &opt); err != nil {
		log.Fatal(err)
	}
}

func makeSlug(tarPath string, metadataConfPath string, tgzPath string, opt *Options) error {
	// need to get absolute path for symlink
	absTarPath, err := filepath.Abs(tarPath)
	if err != nil {
		return LError(err)
	}
	tempDir, err := ioutil.TempDir(filepath.Dir(tgzPath), "tmp")
	if err != nil {
		return LError(err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			log.Print(err)
		}
	}()

	metaDataDir := filepath.Join(tempDir, "METADATA")
	if err := os.Mkdir(metaDataDir, 0755); err != nil {
		return LError(err)
	}
	metadata, err := ioutil.ReadFile(metadataConfPath)
	if err != nil {
		return LError(err)
	}
	if err := ioutil.WriteFile(filepath.Join(metaDataDir, "conf"), metadata, 0666); err != nil {
		return LError(err)
	}
	if err := os.Symlink(absTarPath, filepath.Join(tempDir, "diff.tar")); err != nil {
		return LError(err)
	}
	cmd := Command(
		"tar",
		"-h",
		"-C",
		tempDir,
		"-czf",
		tgzPath,
		".",
	)
	if !opt.Quiet {
		log.Printf("creating slug %s", tgzPath)
	}
	if err := cmd.Run(); err != nil {
		return LError(err)
	}

	return nil
}
