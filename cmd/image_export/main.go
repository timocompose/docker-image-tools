/*
image_export <image> <tar file>
Docker save image and combine all layers to tar file.
*/
package main

import (
	"archive/tar"
	. "github.com/timocompose/docker-image-tools"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func main() {
	var opt Options
	AddQuietFlag(&opt)
	AddFromFlag(&opt)
	AddSaveDirFlag(&opt)
	AddLayerCount(&opt)
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: image_export [options] <image> <tar file>")
		fmt.Fprintln(os.Stderr, "Docker save <image> and combine all layers to <tar file>.")
		fmt.Fprintln(os.Stderr, "Options:")
		flag.PrintDefaults()
		os.Exit(0)
	}
	flag.Parse()

	args := flag.Args()
	if len(args) != 2 {
		log.Println("usage image_export <image> <tar file>")
		log.Fatal("image_export --help for more information")
	}
	if err := imageExport(args[0], args[1], &opt); err != nil {
		log.Fatal(err)
	}
}

func imageExport(image string, tarPath string, opt *Options) error {
	tempDir, err := ioutil.TempDir(filepath.Dir(tarPath), "tmp")
	if err != nil {
		return LError(err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			log.Print(err)
		}
	}()

	if !NameHasTag(image) {
		image += ":latest"
	}

	var saveDir string
	if opt.SaveDir != "" {
		saveDir = opt.SaveDir
	} else {
		saveDir = filepath.Join(tempDir, "save")
		if err := os.Mkdir(saveDir, 0770); err != nil {
			return LError(err)
		}

		cmd := Command(
			"docker",
			"save",
			image,
		)
		stdOutRdr, err := cmd.StdoutPipe()
		if err != nil {
			return LError(err)
		}
		tarCmd := Command("tar", "-C", saveDir, "-xf", "-")
		tarCmd.Stdin = stdOutRdr
		if !opt.Quiet {
			log.Printf("saving %s", image)
		}
		if err := tarCmd.Start(); err != nil {
			return LError(err)
		}
		if err := cmd.Start(); err != nil {
			return LError(err)
		}
		if err := cmd.Wait(); err != nil {
			return LError(err)
		}
		if err := tarCmd.Wait(); err != nil {
			return LError(err)
		}
	}

	manifestPath := filepath.Join(saveDir, "manifest.json")
	file, err := os.Open(manifestPath)
	if err != nil {
		return LError(err)
	}
	jsonDec := json.NewDecoder(file)
	var manifestFile []ManifestStruct
	if err := jsonDec.Decode(&manifestFile); err != nil {
		return LError(err)
	}
	file.Close()
	if len(manifestFile) != 1 {
		return LError(fmt.Errorf("%s has unexpected format", manifestPath))
	}
	manifest := manifestFile[0]

	// number of layers to export
	layerCount := len(manifest.Layers)

	if opt.BaseImage != "" {
		if !NameHasTag(opt.BaseImage) {
			opt.BaseImage += ":latest"
		}
		cmd := Command(
			"docker",
			"inspect",
			"--type",
			"image",
			opt.BaseImage,
			image,
		)
		inspectStdOutRdr, err := cmd.StdoutPipe()
		if err != nil {
			return LError(err)
		}
		if err := cmd.Start(); err != nil {
			return LError(err)
		}
		var inspectOutput []InspectStruct
		if err := json.NewDecoder(inspectStdOutRdr).Decode(&inspectOutput); err != nil {
			return LError(err)
		}
		if err := cmd.Wait(); err != nil {
			return LError(err)
		}
		if len(inspectOutput) != 2 {
			return LError(fmt.Errorf("docker inspect returned %d images want 2", len(inspectOutput)))
		}
		inspectOutputId := map[int]string{
			0: opt.BaseImage,
			1: image,
		}
		layers := make(map[string][]string)
		for i, inspect := range inspectOutput {
			if len(inspect.RootFS.Layers) == 0 {
				return LError(fmt.Errorf("docker inspect image %s has no layers", inspectOutputId[i]))
			}
			layers[inspectOutputId[i]] = inspectOutput[i].RootFS.Layers
		}
		nCommon := 0
		// layer lists are SHAs identifying docker layers
		// count number of common layers
		for i, layer := range layers[opt.BaseImage] {
			if layer != layers[image][i] {
				break
			}
			nCommon++
		}
		nDiff := len(layers[image]) - nCommon
		if nCommon == 0 {
			return LError(fmt.Errorf("image %s is not derived from image %s", image, opt.BaseImage))
		} else if nDiff == 0 {
			// should this be an error?
			return LError(fmt.Errorf("image %s is the same as %s", image, opt.BaseImage))
		}

		// Here we are assuming that there is a one to one relationship between layer tarball list from docker save,
		// and the layer SHA list from docker inspect.
		layerCount = nDiff
	}

	if opt.LayerCount != 0 {
		layerCount = opt.LayerCount
	}

	if !opt.Quiet {
		log.Printf("combining layers")
	}
	tarFile, err := os.Create(tarPath)
	if err != nil {
		return LError(err)
	}
	tarWtr := tar.NewWriter(tarFile)

	deletedPaths := make([]string, 0, 100)
	existingPaths := make(map[string]struct{})
	firstLyr := len(manifest.Layers) - layerCount
	// build from top layer downward, so as to exclude any deleted files
	for i := len(manifest.Layers) - 1; i >= firstLyr; i-- {
		layerTar := manifest.Layers[i]
		lyrFile, err := os.Open(filepath.Join(saveDir, layerTar))
		if err != nil {
			return LError(err)
		}
		tarRdr := tar.NewReader(lyrFile)
		for {
			header, err := tarRdr.Next()
			if err == io.EOF {
				break
			} else if err != nil {
				return LError(err)
			}

			// if deleted ignore
			if matchPath(header.Name, deletedPaths) {
				continue
			}
			// build up sorted list of deleted paths
			if base := filepath.Base(header.Name); strings.HasPrefix(base, ".wh.") {
				path := filepath.Join(filepath.Dir(header.Name), strings.TrimPrefix(base, ".wh."))
				deletedPaths = append(deletedPaths, path)
				sort.Strings(deletedPaths)
				continue
			}

			if _, ok := existingPaths[header.Name]; ok {
				continue
			}
			existingPaths[header.Name] = struct{}{}

			if err := tarWtr.WriteHeader(header); err != nil {
				return LError(err)
			}

			if _, err := io.Copy(tarWtr, tarRdr); err != nil {
				return LError(err)
			}
		}
		lyrFile.Close()
	}
	if err := tarWtr.Close(); err != nil {
		LError(err)
	}
	if err := tarFile.Close(); err != nil {
		LError(err)
	}
	return nil
}

// do binary search here because sort package binary search is hard to use
func matchPath(path string, paths []string) bool {
	low := 0
	high := len(paths)
	if high == 0 {
		return false
	}
	pos := (high - low)/2
	for {
		entry := paths[pos]
		if strings.HasPrefix(path, entry) {
			if len(entry) == len(path) {
				// path == entry
				return true
			} else if path[len(entry)] == '/' {
				// If we have an entry that is the direcotry part of
				// path. Then if must be a directory.
				return true
			}
		}
		if path < entry {
			high = pos
			pos = low + (pos - low) / 2
			if pos == high {
				break
			}
		} else {
			low = pos
			pos = pos + (high - pos) / 2
			if low == pos {
				break
			}
		}
	}
	return false
}