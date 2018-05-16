## Docker image tools

Tools to create images and tarballs from Docker images.

### Tools

#### cmd/image_export
Image export creates a tar file from all layers in image in local Docker daemon.
`-from` creates a differential tar containing only layers above given Docker 
image. 

##### Usage
```
Usage: image_export [options] <image> <tar file>
Docker save <image> and combine all layers to <tar file>.
Options:
  -from <base image>
        Only include layers built on top of <base image> layers.
  -layer-count <n>
        If <n> is nonzero, only combine the top n layers.
  -quiet
        Disable info logging.
  -save-dir <dir>
        Don't run docker save, use <dir> containing layers from previous docker save.
```

##### Notes
Layers with whiteout files for deletions between layers are not handled 
currently.

#### cmd/make_slug
Creates slug tgz file from the given tar file and metadata file.

##### Usage
```
Usage: make_slug [options] <tar file> <metadata conf file> <tgz file>
Create output <tgz file> if slug format with <tar file> as "diff.tar" and <metadata conf file> as "METADATA/conf".
Options:
  -quiet
        Disable info logging.
```



