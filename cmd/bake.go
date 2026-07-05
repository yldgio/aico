package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	images "github.com/yldgio/aico/images"
	"github.com/yldgio/aico/internal/agents"
	"github.com/yldgio/aico/internal/auth"
	"github.com/yldgio/aico/internal/container"
	"github.com/yldgio/aico/internal/platform"
	"github.com/yldgio/aico/internal/runtime"
)

type bakeOpts struct {
	tag              string
	includeWorkspace bool
	newContainer     bool
	image            string
	runtime          string
	dryRun           bool
	verbose          bool
}

func newBakeCmd() *cobra.Command {
	o := &bakeOpts{}
	c := &cobra.Command{
		Use:   "bake <agent> [path] -t <tag>",
		Short: "Snapshot an agent container into a pushable image",
		Long: "Bake a fully-configured aico container into a taggable, pushable OCI\n" +
			"image, without ever launching the agent UI.\n\n" +
			"If a container for <agent>+[path] already exists, bake commits that\n" +
			"container's current state (running or stopped); otherwise it creates one\n" +
			"(via `docker create`, never started) and commits it. The resulting\n" +
			"container is a normal aico container: a later `aico run` resumes it.\n\n" +
			"Auth/login is never baked in: it lives in named volumes and env vars,\n" +
			"which `docker commit` excludes by construction. Push the image yourself\n" +
			"with `docker push <tag>` -- aico never touches the registry.",
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			agentName := args[0]
			if _, err := agents.Lookup(agentName); err != nil {
				return err
			}
			if o.tag == "" {
				return fmt.Errorf(
					"-t/--tag is required\n\nfix: pass an image tag, e.g. aico bake %s -t myorg/%s:latest",
					agentName, agentName)
			}
			path := ""
			if len(args) == 2 {
				path = args[1]
			}
			return bake(agentName, path, o)
		},
	}
	f := c.Flags()
	f.StringVarP(&o.tag, "tag", "t", "", "tag for the resulting image (required)")
	f.BoolVarP(&o.includeWorkspace, "include-workspace", "w", false,
		"copy the project folder into the image (honoring .dockerignore) and set WORKDIR to it")
	f.BoolVar(&o.newContainer, "new", false, "discard any existing container and create a fresh one before baking")
	f.StringVar(&o.image, "image", "",
		"base image used only when bake must create the container (ignored if it already exists)")
	f.StringVar(&o.runtime, "runtime", "", "container runtime to use (default: auto-detect docker, then podman)")
	f.BoolVar(&o.dryRun, "dry-run", false, "print the commit/build plan without executing")
	f.BoolVar(&o.verbose, "verbose", false, "print extra signal, e.g. whether an existing container was reused")
	return c
}

func bake(agentName, path string, o *bakeOpts) error {
	agent, err := agents.Lookup(agentName)
	if err != nil {
		return err
	}
	absPath, err := resolvePath(path)
	if err != nil {
		return err
	}

	rtBin := runtime.Resolve(o.runtime)
	image := o.image
	if image == "" {
		image = images.DefaultTag
	}
	name := container.Name(agent.Name, absPath)
	authPlan := auth.Build(agent, false)
	mountSrc, workdir := platform.WorkspaceMount(absPath)
	shortName := resolveContainerName("", agent.Name, absPath)

	commonArgs := []string{"--name", name,
		"-v", fmt.Sprintf("%s:%s", mountSrc, workdir), "-w", workdir}
	commonArgs = append(commonArgs, containerLabels(agent.Name, absPath, shortName)...)
	commonArgs = append(commonArgs, authPlan.Args...)

	interactiveFlag := "-i"
	if isTTY() {
		interactiveFlag = "-it"
	}

	// Baking with -w commits to a throwaway intermediate image first, then
	// builds the final -t tag from it (so the workspace COPY + WORKDIR happen
	// in a second, .dockerignore-respecting `docker build`). Without -w the
	// commit goes straight to the user's tag.
	intermediateTag := o.tag
	if o.includeWorkspace {
		intermediateTag = "aico-bake-intermediate:" + name
	}

	if o.dryRun {
		printBakeDryRun(rtBin, image, name, interactiveFlag, commonArgs, agent, o, intermediateTag, absPath, workdir)
		return nil
	}

	rt, err := runtime.Detect(o.runtime)
	if err != nil {
		return err
	}

	if o.newContainer {
		_ = rt.Remove(name)
	}

	if rt.Exists(name) {
		if o.verbose {
			fmt.Fprintf(os.Stderr, "aico: container %s already exists; baking its current state\n", name)
		}
	} else {
		if o.image == "" {
			if err := images.EnsureBuilt(rt); err != nil {
				return err
			}
		}
		createArgs := append([]string{interactiveFlag}, commonArgs...)
		createArgs = append(createArgs, image)
		createArgs = append(createArgs, agent.Command...)
		if _, err := rt.Create(createArgs...); err != nil {
			return fmt.Errorf("create container: %w", err)
		}
		if o.verbose {
			fmt.Fprintf(os.Stderr, "aico: created container %s (not started)\n", name)
		}
	}

	if err := rt.Commit(name, intermediateTag); err != nil {
		return fmt.Errorf("commit container: %w", err)
	}

	if o.includeWorkspace {
		if err := bakeWorkspace(rt, intermediateTag, o.tag, absPath, workdir); err != nil {
			return err
		}
	}

	fmt.Fprintf(os.Stderr,
		"aico: caution: auth volumes/env vars are excluded by docker commit, but double-check %s "+
			"for anything else sensitive before pushing it to a public registry.\n", o.tag)
	fmt.Fprintf(os.Stderr, "aico: image %s ready. Push it with: docker push %s\n", o.tag, o.tag)
	return nil
}

// bakeWorkspace builds the final workspace-including image from the committed
// intermediate: FROM <intermediateTag>, COPY the project folder to workdir,
// WORKDIR workdir. The Dockerfile is written to a temp dir (never into the
// user's project) while absPath remains the build context, so absPath's
// .dockerignore is honored. The intermediate image is removed afterward.
func bakeWorkspace(rt *runtime.Runtime, intermediateTag, finalTag, absPath, workdir string) error {
	dir, err := os.MkdirTemp("", "aico-bake-*")
	if err != nil {
		return fmt.Errorf("create build context: %w", err)
	}
	defer os.RemoveAll(dir)

	dockerfilePath := filepath.Join(dir, "Dockerfile")
	content := fmt.Sprintf("FROM %s\nCOPY . %s\nWORKDIR %s\n", intermediateTag, workdir, workdir)
	if err := os.WriteFile(dockerfilePath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write Dockerfile: %w", err)
	}

	buildErr := rt.BuildWithDockerfile(finalTag, dockerfilePath, absPath)
	_ = rt.RemoveImage(intermediateTag)
	if buildErr != nil {
		return fmt.Errorf("build workspace image: %w", buildErr)
	}
	return nil
}

func printBakeDryRun(rtBin, image, name, interactiveFlag string, commonArgs []string, agent agents.Agent,
	o *bakeOpts, intermediateTag, absPath, workdir string) {
	if rtBin == "" {
		rtBin = "(none detected — install docker or podman)"
	}
	fmt.Fprintf(os.Stderr, "[dry-run] runtime:   %s\n", rtBin)
	fmt.Fprintf(os.Stderr, "[dry-run] container: %s (used as-is if it exists; otherwise created via `docker create`, not started)\n", name)

	createArgs := append([]string{"create", interactiveFlag}, commonArgs...)
	createArgs = append(createArgs, image)
	createArgs = append(createArgs, agent.Command...)
	fmt.Fprintf(os.Stderr, "[dry-run] create:    %s %s\n", rtBin, strings.Join(createArgs, " "))
	fmt.Fprintf(os.Stderr, "[dry-run] commit:    %s commit %s %s\n", rtBin, name, intermediateTag)

	if o.includeWorkspace {
		fmt.Fprintf(os.Stderr, "[dry-run] build:     %s build -t %s -f <tmp-Dockerfile> %s\n", rtBin, o.tag, absPath)
		fmt.Fprintf(os.Stderr, "[dry-run]   Dockerfile: FROM %s / COPY . %s / WORKDIR %s\n", intermediateTag, workdir, workdir)
		fmt.Fprintf(os.Stderr, "[dry-run] cleanup:   %s rmi -f %s\n", rtBin, intermediateTag)
	}
	fmt.Fprintln(os.Stderr, "[dry-run] no changes made.")
}
