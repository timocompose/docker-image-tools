package test

import (
	. "compose/build-tools"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type mockFile struct {
	Name     string
	Contents []byte
}

// add user/group id?
type mockFilesystem struct {
	Dirs  []string
	Files map[string][]mockFile
	Perms map[string]os.FileMode
}

func newMockFilesystem() *mockFilesystem {
	return &mockFilesystem{
		Dirs:  make([]string, 0),
		Files: make(map[string][]mockFile),
		Perms: make(map[string]os.FileMode),
	}
}

var dockerfilesFs = mockFilesystem{
	Dirs: []string{"/image1", "/image2", "/image2/rootfs", "/image2/rootfs/dir_example"},
	Files: map[string][]mockFile{
		"/image1": []mockFile{
			mockFile{
				"Dockerfile",
				[]byte(`
					FROM alpine
				`),
			},
		},
		"/image2": []mockFile{
			mockFile{
				"Dockerfile",
				[]byte(`
					FROM image1
					ADD rootfs /
					RUN echo -n "mockFile created in run" > /run_file
				`),
			},
		},
		"/image2/rootfs": []mockFile{mockFile{"file_example", []byte("this is a example mockFile")}},
	},
	Perms: map[string]os.FileMode{
		"/image2/rootfs/dir_example":  0770,
		"/image2/rootfs/file_example": 0600,
	},
}

// differences between image1 and image2 docker images
var imageDiffFs = mockFilesystem{
	Dirs: []string{"/dir_example"},
	Files: map[string][]mockFile{
		"/": []mockFile{
			mockFile{"file_example", []byte("this is a example mockFile")},
			mockFile{"run_file", []byte("mockFile created in run")},
		},
	},
	Perms: map[string]os.FileMode{
		"/dir_example":  0770,
		"/file_example": 0600,
		"/run_file":     0644,
	},
}

func populateDirFromMock(root string, mock *mockFilesystem) error {
	for _, dir := range mock.Dirs {
		perm := mock.Perms[dir]
		if perm == 0 {
			perm = 0777
		}
		if err := os.MkdirAll(filepath.Join(root, dir), perm); err != nil {
			return err
		}
	}
	for dir, files := range mock.Files {
		for _, file := range files {
			filePath := filepath.Join(dir, file.Name)
			perm := mock.Perms[filePath]
			if perm == 0 {
				perm = 0666
			}
			err := ioutil.WriteFile(filepath.Join(root, filePath), file.Contents, perm)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func createMockFromDir(root string) (*mockFilesystem, error) {
	root = filepath.Clean(root)
	mock := newMockFilesystem()
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if path == root {
			return nil
		}
		subPath := strings.TrimPrefix(path, root)
		if info.IsDir() {
			mock.Dirs = append(mock.Dirs, subPath)
			mock.Perms[subPath] = info.Mode() & os.ModePerm
		} else {
			contents, err := ioutil.ReadFile(path)
			if err != nil {
				return err
			}
			dir := filepath.Dir(subPath)
			name := filepath.Base(subPath)
			mock.Files[dir] = append(mock.Files[dir], mockFile{name, contents})
			mock.Perms[subPath] = info.Mode() & os.ModePerm
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return mock, nil
}

// return true if a is subset of b
func mockFilesystemsSubset(a *mockFilesystem, b *mockFilesystem, reason *string) bool {
	var reasonVar string
	defer func() {
		if reason != nil {
			*reason = reasonVar
		}
	}()
	for _, dir := range a.Dirs {
		found := false
		for _, dir2 := range b.Dirs {
			if dir == dir2 {
				found = true
				break
			}
		}
		if !found {
			reasonVar = fmt.Sprintf("%s dir not found", dir)
			return false
		}
		if dirPerm, ok := a.Perms[dir]; ok {
			bDirPerm, ok := b.Perms[dir]
			if ok {
				if dirPerm != bDirPerm {
					reasonVar = fmt.Sprintf("%s dir permissions differ", dir)
					return false
				}
			}
		}
	}

	for _, dir := range append(a.Dirs, "/") {
		for _, file := range a.Files[dir] {
			bfiles, ok := b.Files[dir]
			if !ok {
				reasonVar = fmt.Sprintf("%s file not found", file)
				return false
			}
			found := false
			nameFound := false
			for _, bfile := range bfiles {
				if bfile.Name != file.Name {
					continue
				}
				nameFound = true
				if string(bfile.Contents) != string(file.Contents) {
					continue
				}
				found = true
				break
			}
			if !found {
				if nameFound {
					reasonVar = fmt.Sprintf("%s file contents different", file)
				} else {
					reasonVar = fmt.Sprintf("%s file not found", file)
				}
				return false
			}
			filePath := filepath.Join(dir, file.Name)
			if filePerm, ok := a.Perms[filePath]; ok {
				bFilePerm, ok := b.Perms[filePath]
				if ok {
					if filePerm != bFilePerm {
						reasonVar = fmt.Sprintf("%s file permissions differ", file)
						return false
					}
				}
			}
		}
	}

	return true
}

func mockFilesystemEqual(a *mockFilesystem, b *mockFilesystem, reason *string) bool {
	if mockFilesystemsSubset(a, b, reason) {
		if mockFilesystemsSubset(b, a, reason) {
			return true
		}
	}
	return false
}

var keepDat = flag.Bool("keep-data", false, "keep test files/images")

func TestMain(m *testing.M) {
	tempDir, err := ioutil.TempDir("", "test_dockerfiles")
	if err != nil {
		log.Fatal(err)
	}

	flag.Parse()

	if *keepDat {
		log.Printf("test docker files kept in dir: %s", tempDir)
	}

	if err := populateDirFromMock(tempDir, &dockerfilesFs); err != nil {
		log.Fatal(err)
	}

	wd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	images := []string{"image1", "image2"}
	for _, image := range images {
		if err := os.Chdir(filepath.Join(tempDir, image)); err != nil {
			log.Fatal(err)
		}
		cmd := Command("docker", "build", ".", "-t", image)
		if err := cmd.Run(); err != nil {
			log.Fatal(err)
		}
	}
	if err := os.Chdir(wd); err != nil {
		log.Fatal(err)
	}

	extCode := m.Run()

	if !*keepDat {
		for _, image := range images {
			removeImgFn := func(removeImg string) {
				cmd := Command("docker", "rmi", removeImg)
				if err := cmd.Run(); err != nil {
					log.Print(err)
				}
			}
			removeImgFn(image)
		}
		os.RemoveAll(tempDir)
	}
	os.Exit(extCode)
}

// TODO: Test user/group ownership of files in tar created by image_export.
func TestImageExportDiff(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "test_image_export")
	if err != nil {
		t.Fatal(err)
	}
	if *keepDat {
		t.Logf("impage export test files kept in dir: %s", tempDir)
	}

	tarPath := filepath.Join(tempDir, "save.tar")
	cmd := Command(
		"go",
		"run",
		"../cmd/image_export/main.go",
		"--from",
		"image1",
		"image2",
		tarPath,
	)
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	rootDir := filepath.Join(tempDir, "root")
	if err := os.Mkdir(rootDir, 0770); err != nil {
		t.Fatal(err)
	}

	cmd = Command(
		"tar",
		"-C",
		rootDir,
		"-xf",
		tarPath,
	)
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	got, err := createMockFromDir(rootDir)
	if err != nil {
		t.Fatal(err)
	}
	var reason string
	if !mockFilesystemEqual(&imageDiffFs, got, &reason) {
		t.Fatal(fmt.Errorf("unexpected filesystem produced from diffing image1 and image2, %s", reason))
	}

	if !*keepDat {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Fatal(err)
		}
	}
}

func TestMakeSlug(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "test_make_slug")
	if err != nil {
		t.Fatal(err)
	}
	if *keepDat {
		t.Logf("make slug test files kept in dir: %s", tempDir)
	}

	imgRoot := filepath.Join(tempDir, "image_root")
	if err := os.Mkdir(imgRoot, 0770); err != nil {
		t.Fatal(err)
	}
	if err := populateDirFromMock(imgRoot, &imageDiffFs); err != nil {
		t.Fatal(err)
	}
	tarPath := filepath.Join(tempDir, "root.tar")
	cmd := Command(
		"tar",
		"-C",
		imgRoot,
		"-cf",
		tarPath,
		".",
	)
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	var slugMetCnf SlugMetaDataConf
	slugMetCnf.RegularAccess = true
	slugMetCnf.IsolatedAccess = true
	slugMetCnf.Name = "image2"
	slugMetCnf.Version = 2
	slugMetCnf.Parent = "image1"
	slugMetCnf.Source = "slug"
	slgMetCnfPath := filepath.Join(tempDir, "slug_metadata_conf.json")
	slgMetCnfFile, err := os.Create(slgMetCnfPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.NewEncoder(slgMetCnfFile).Encode(slugMetCnf); err != nil {
		t.Fatal(err)
	}
	slgMetCnfFile.Close()

	slugPath := filepath.Join(tempDir, "image2_slug.tgz")
	cmd = Command(
		"go",
		"run",
		"../cmd/make_slug/main.go",
		tarPath,
		slgMetCnfPath,
		slugPath,
	)
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	slgContentDir := filepath.Join(tempDir, "slug_content")
	if err := os.Mkdir(slgContentDir, 0770); err != nil {
		t.Fatal(err)
	}
	cmd = Command(
		"tar",
		"-C",
		slgContentDir,
		"-zxf",
		slugPath,
	)
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	cnfPath := "METADATA/conf"
	fileContent, err := ioutil.ReadFile(filepath.Join(slgContentDir, cnfPath))
	if err != nil {
		t.Fatal(err)
	}
	var gotCnf SlugMetaDataConf
	if err := json.Unmarshal(fileContent, &gotCnf); err != nil {
		t.Fatal(err)
	}
	if gotCnf.Name != slugMetCnf.Name {
		t.Fatal(fmt.Errorf("unexpected contents of %s", cnfPath))
	}

	diffContentDir := filepath.Join(tempDir, "diff_content")
	if err := os.Mkdir(diffContentDir, 0770); err != nil {
		t.Fatal(err)
	}
	diff := "diff.tar"
	cmd = Command(
		"tar",
		"-C",
		diffContentDir,
		"-xf",
		filepath.Join(slgContentDir, diff),
	)
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	gotMock, err := createMockFromDir(diffContentDir)
	if err != nil {
		t.Fatal(err)
	}
	var reason string
	if !mockFilesystemEqual(&imageDiffFs, gotMock, &reason) {
		t.Fatal(fmt.Errorf("unexpected contents of %s, %s", diff, reason))
	}

	if !*keepDat {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Fatal(err)
		}
	}
}
