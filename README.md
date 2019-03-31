## Docker image tools

Tools to work with Docker images.

### Tools

#### cmd/image_export
Image export creates a tar file of all layers from an image in a Docker daemon (only local for now).
Layers with whiteout files for deletions and rewritten files between layers are
handled correctly. `-from` creates a differential tar containing only layers above given Docker
image. This can be used for example: to create a tar that can be used to update scripts in a
deployed service, create redistributable packages.

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
