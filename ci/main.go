package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"dagger.io/dagger"
	platformFormat "github.com/containerd/containerd/platforms"
)

var (
	cleanCacheOnly    = flag.Bool("clean-cache-only", false, "only clean the cached modules directory, then exit without doing the build")
	cleanCacheAtStart = flag.Bool("clean-cache", false, "clean the cached modules directory, then continue with the build")
	cleanBuildAtStart = flag.Bool("clean-build", false, "clean the build directory before commencing the build")
	cleanBuildOnly    = flag.Bool("clean-build-only", false, "clean the build directory, then exist without doing the build")
)

// util that returns the architecture of the provided platform
func architectureOf(platform dagger.Platform) string {
	return platformFormat.MustParse(string(platform)).Architecture
}

// util that returns the OS of the provided platform
func osOf(platform dagger.Platform) string {
	return platformFormat.MustParse(string(platform)).OS
}

func main() {
	if err := build(context.Background()); err != nil {
		fmt.Println(err)
	}
}

func build(ctx context.Context) (err error) {

	fmt.Println("Building with Dagger")

	var platforms = []dagger.Platform{
		"linux/amd64", // a.k.a. x86_64
		"linux/arm64", // a.k.a. aarch64
	}

	// This will be populated by the build process and contain the distroless
	// images from the linux build
	imageVariants := make([]*dagger.Container, 0, len(platforms))

	// initialize Dagger client
	client, err := dagger.Connect(ctx, dagger.WithLogOutput(os.Stderr))
	if err != nil {
		return err
	}
	defer client.Close()

	// create a cache volume
	goCache := client.CacheVolume("go-1.21.5")
	goModCache := client.CacheVolume("go-1.21.5-mod")

	// Create a reference to the source directory on our host
	src := client.Host().Directory(".")

	// create empty directory to put build outputs
	outputs := client.Directory()
	cleanChecked := false

	for _, platform := range platforms {
		// get `golang` image and a reference to the local project
		golang := client.Container().From("golang:1.21.5-alpine")

		// Check for various maintenance commands related to cleaning and maintaining the build environment
		// that dont require a full build
		if !cleanChecked {
			if *cleanCacheOnly || *cleanCacheAtStart {
				golang = golang.WithExec([]string{"go", "clean", "-cache"})
				golang = golang.WithExec([]string{"go", "clean", "-modcache"})
				if *cleanCacheOnly {
					return nil
				}
			}
			if *cleanBuildAtStart || *cleanBuildOnly {
				golang = golang.WithExec([]string{"rm", "-r", "build/"})
				if *cleanBuildOnly {
					return nil
				}
			}
			cleanChecked = true
		}

		// Add caching speeds up to the build container
		golang = golang.WithMountedCache("/src/go_cache", goCache).WithEnvVariable("GO_CACHE", "/src/go_cache")
		golang = golang.WithMountedCache("/src/go_mod_cache", goModCache).WithEnvVariable("GOMODCACHE", "/src/go_mod_cache")

		// mount cloned repository into the golang image
		golang = golang.WithDirectory("/src", src).WithWorkdir("/src")

		// create a directory for each os and arch
		outputPath := fmt.Sprintf("build/%s/%s/", osOf(platform), architectureOf(platform))

		build := golang.WithEnvVariable("CGO_ENABLED", "0")
		build = build.WithEnvVariable("GOOS", osOf(platform))
		build = build.WithEnvVariable("GOARCH", architectureOf(platform))

		build = build.WithExec([]string{"go", "build", "-ldflags=-extldflags=-static", "-tags=osusergo netgo", "-o", outputPath, "./cmd/..."})

		// get reference to build output directory in container
		outputs = outputs.WithDirectory(outputPath, build.Directory(outputPath))

		// If we just did a linux build, add the distroless images
		if osOf(platform) == "linux" {
			ctr := client.Container(dagger.ContainerOpts{Platform: platform}).
				From("gcr.io/distroless/static-debian11").
				WithRootfs(build.Directory(outputPath))
			imageVariants = append(imageVariants, ctr)
			ctr.Export(ctx, filepath.Join(outputPath, fmt.Sprintf("distroless-%s-%s.img", osOf(platform), architectureOf(platform))))
			// ctr.Publish(ctx, "docker.orb.internal/wild-"+osOf(platform)+"-"+architectureOf(platform)+":latest", dagger.ContainerPublishOpts{})
		}
	}

	// Export the linux distroless images to the host in the build directory is not yet working
	// commented out for now
	client.Container().Export(ctx, "build/image:latest", dagger.ContainerExportOpts{PlatformVariants: imageVariants})
	// client.Container().
	//	Publish(ctx, "docker.orb.internal/wild:latest", dagger.ContainerPublishOpts{PlatformVariants: imageVariants})

	// write build executable artifacts to host
	if _, err = outputs.Export(ctx, "."); err != nil {
		return err
	}

	return nil
}
