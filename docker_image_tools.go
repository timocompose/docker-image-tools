package dockerimagetools

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

type SlugMetaDataConf struct {
	Interfaces     interface{}   `json:"interfaces"`
	Name           string        `json:"name"`
	Source         string        `json:"source"`
	Mounts         []interface{} `json:"mounts"`
	Application    interface{}   `json:"application"`
	Version        int           `json:"version"`
	Overlays       []interface{} `json:"overlays"`
	Metadata       interface{}   `json:"metadata"`
	Parent         string        `json:"parent"`
	IsolatedAccess bool          `json:"isolatedAccess"`
	Cgroup         interface{}   `json:"cgroup"`
	RegularAccess  bool          `json:"regularAccess"`
}

// This structure is not defined by docker
type InspectStruct struct {
	RootFS struct {
		Layers []string `json:"Layers"`
	} `json:"RootFS"`
	RepoTags []string `json:"RepoTags"`
}

// Defined in docker here
// https://github.com/containers/image/blob/17449738f2bb4c6375c20dcdcfe2a6cccf03f312/docker/tarfile/types.go
type ManifestStruct struct {
	Layers []string `json:"Layers"`
}

type Options struct {
	Quiet      bool
	BaseImage  string
	SaveDir    string
	LayerCount int
}

func AddQuietFlag(o *Options) {
	flag.BoolVar(&o.Quiet, "quiet", false, "Disable info logging.")
}

func AddFromFlag(o *Options) {
	flag.StringVar(&o.BaseImage, "from", "", "Only include layers built on top of `<base image>` layers.")
}

func AddSaveDirFlag(o *Options) {
	flag.StringVar(&o.SaveDir, "save-dir", "", "Don't run docker save, use `<dir>` containing layers from previous docker save.")
}

func AddLayerCount(o *Options) {
	flag.IntVar(&o.LayerCount, "layer-count", 0, "If `<n>` is nonzero, only combine the top n layers.")
}

func LError(err error) error {
	_, file, line, _ := runtime.Caller(1)
	return fmt.Errorf("%s:%d %s", filepath.Base(file), line, err.Error())
}

func Command(name string, arg ...string) *exec.Cmd {
	cmd := exec.Command(name, arg...)
	cmd.Stderr = os.Stderr
	return cmd
}

func NameHasTag(name string) bool {
	return strings.Contains(name, ":")
}
