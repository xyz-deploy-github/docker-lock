package rewrite

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/michaelperel/docker-lock/generate"
	"github.com/michaelperel/docker-lock/rewrite/internal/rewriter"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

// Rewriter rewrites Dockerfiles and docker-compose files from a Lockfile.
// If a suffix is provided, Dockerfiles will be rewritten to Dockerfile-suffix,
// and docker-compose files will be rewritten to docker-compose-suffix.yml or
// docker-compose-suffix.yaml depending on the file's ending.
// If a temporary directory path is provided, a directory will be created
// inside the path at the start of each call to Rewrite and deleted when
// the function returns, to support "transaction"-like behavior (see the docs
// for Rewrite for more details). If it is not provided, a temporary directory
// will be created and deleted in the system's default temporary directory
// location (/tmp on some Linux distros, for instance). Providing a temporary
// directory may be necessary in the case where the the system's default
// temporary directory location is on a different drive than the files
// that will be rewritten.
type Rewriter struct {
	Lockfile *generate.Lockfile
	Suffix   string
	TempDir  string
}

type renameInfo struct {
	oPath    string
	tmpOPath string
	origByt  []byte
	err      error
}

type cImageLine struct {
	svcName string
	line    string
}

// NewRewriter creates a Rewriter from command line flags.
func NewRewriter(cmd *cobra.Command) (*Rewriter, error) {
	suffix, err := cmd.Flags().GetString("suffix")
	if err != nil {
		return nil, err
	}
	tmpDir, err := cmd.Flags().GetString("tempdir")
	if err != nil {
		return nil, err
	}
	tmpDir = filepath.ToSlash(tmpDir)
	lFile, err := readLockfile(cmd)
	if err != nil {
		return nil, err
	}
	lFile.DockerfileImages = dedupeDockerfileImages(lFile)
	return &Rewriter{Lockfile: lFile, Suffix: suffix, TempDir: tmpDir}, nil
}

// Rewrite rewrites images referenced in a Lockfile to include digests.
// If a docker-compose file references a Dockerfile, and the same Dockerfile
// exists in the Lockfile elsewhere, the Dockerfile will be rewritten
// with information from the docker-compose file.
// Rewrite has "transaction"-like properties to ensure all rewrites succeed or
// fail together. The function follows the following steps:
// (1) create a temporary directory in the system default temporary directory
// location or in the location supplied via the '--tempdir' command line arg
// (2) rewrite every file to a file in the temporary directory
// (3) if all rewrites succeed, rename each temporary file to its desired
// outpath
// (4) if an error occurs during renaming, revert all files back to their
// original content
// (5) if reverting fails, return an error with the paths that failed to revert
// (6) delete the temporary directory
// Note: renaming files across drives may fail because of permissions errors,
// so if your default temporary directory is on another drive and renaming
// fails, specify a temporary directory on the same drive as the
// Dockerfiles/docker-compose files.
func (r *Rewriter) Rewrite() (err error) {
	if len(r.Lockfile.DockerfileImages) == 0 &&
		len(r.Lockfile.ComposefileImages) == 0 {
		return nil
	}
	tmpDirPath, err := ioutil.TempDir(r.TempDir, "docker-lock-tmp")
	if err != nil {
		return err
	}
	tmpDirPath = filepath.ToSlash(tmpDirPath)
	defer func() {
		if rmErr := os.RemoveAll(tmpDirPath); rmErr != nil {
			err = fmt.Errorf("%v: %v", rmErr, err)
		}
	}()
	var (
		doneCh = make(chan struct{})
		rnCh   = r.writeFiles(tmpDirPath, doneCh)
	)
	err = r.renameFiles(rnCh)
	if err != nil {
		close(doneCh)
	}
	return err
}

func (r *Rewriter) writeFiles(
	tmpDirPath string,
	doneCh <-chan struct{},
) chan *renameInfo {
	var (
		rnCh = make(chan *renameInfo)
		wg   sync.WaitGroup
	)
	wg.Add(1)
	go r.writeComposefiles(tmpDirPath, rnCh, &wg, doneCh)
	wg.Add(1)
	go r.writeDockerfiles(tmpDirPath, rnCh, &wg, doneCh)
	go func() {
		wg.Wait()
		close(rnCh)
	}()
	return rnCh
}

func (r *Rewriter) writeDockerfiles(
	tmpDirPath string,
	rnCh chan<- *renameInfo,
	wg *sync.WaitGroup,
	doneCh <-chan struct{},
) {
	defer wg.Done()
	for dPath, ims := range r.Lockfile.DockerfileImages {
		wg.Add(1)
		go r.writeDockerfile(dPath, ims, tmpDirPath, rnCh, wg, doneCh)
	}
}

func (r *Rewriter) writeComposefiles(
	tmpDirPath string,
	rnCh chan<- *renameInfo,
	wg *sync.WaitGroup,
	doneCh <-chan struct{},
) {
	defer wg.Done()
	for cPath, ims := range r.Lockfile.ComposefileImages {
		wg.Add(1)
		go r.writeComposefile(cPath, ims, tmpDirPath, rnCh, wg, doneCh)
	}
}

func (r *Rewriter) writeDockerfile(
	dPath string,
	ims []*generate.DockerfileImage,
	tmpDirPath string,
	rnCh chan<- *renameInfo,
	wg *sync.WaitGroup,
	doneCh <-chan struct{},
) {
	defer wg.Done()
	dByt, err := ioutil.ReadFile(dPath) // nolint: gosec
	if err != nil {
		select {
		case <-doneCh:
		case rnCh <- &renameInfo{err: err}:
		}
		return
	}
	var (
		stageNames = map[string]bool{}
		lines      = strings.Split(string(dByt), "\n")
		imIndex    int
	)
	for i, line := range lines {
		fields := strings.Fields(line)
		if len(fields) > 0 && strings.ToLower(fields[0]) == "from" {
			// FROM <image>
			// FROM <image> AS <stage>
			// FROM <stage> AS <another stage>
			if !stageNames[fields[1]] {
				if imIndex >= len(ims) {
					select {
					case <-doneCh:
					case rnCh <- &renameInfo{
						err: fmt.Errorf(
							"more images exist in %s than in the Lockfile",
							dPath,
						),
					}:
					}
					return
				}
				fields[1] = fmt.Sprintf(
					"%s:%s@sha256:%s", ims[imIndex].Image.Name,
					ims[imIndex].Image.Tag, ims[imIndex].Image.Digest,
				)
				imIndex++
			}
			if len(fields) == 4 {
				stageName := fields[3]
				stageNames[stageName] = true
			}
			lines[i] = strings.Join(fields, " ")
		}
	}
	if imIndex != len(ims) {
		select {
		case <-doneCh:
		case rnCh <- &renameInfo{
			err: fmt.Errorf(
				"more images exist in the Lockfile than in %s", dPath,
			),
		}:
		}
		return
	}
	var (
		oPath = dPath
		rwByt = []byte(strings.Join(lines, "\n"))
	)
	if r.Suffix != "" {
		oPath = fmt.Sprintf("%s-%s", dPath, r.Suffix)
	}
	writeTmpFile(oPath, rwByt, tmpDirPath, rnCh, doneCh)
}

func (r *Rewriter) writeComposefile(
	cPath string,
	ims []*generate.ComposefileImage,
	tmpDirPath string,
	rnCh chan<- *renameInfo,
	wg *sync.WaitGroup,
	doneCh <-chan struct{},
) {
	defer wg.Done()
	cByt, err := ioutil.ReadFile(cPath) // nolint: gosec
	if err != nil {
		select {
		case <-doneCh:
		case rnCh <- &renameInfo{err: err}:
		}
		return
	}
	var comp rewriter.Compose
	if err := yaml.Unmarshal(cByt, &comp); err != nil {
		select {
		case <-doneCh:
		case rnCh <- &renameInfo{err: err}:
		}
		return
	}
	svcIms := map[string][]*generate.ComposefileImage{}
	for _, im := range ims {
		svcIms[im.ServiceName] = append(svcIms[im.ServiceName], im)
	}
	var (
		cilCh = make(chan *cImageLine)
		cwg   sync.WaitGroup
	)
	for svcName, svc := range comp.Services {
		cwg.Add(1)
		wg.Add(1)
		go r.writeDockerfileOrGetCImageLine(
			svcName, svc, svcIms, tmpDirPath, cilCh, rnCh, &cwg, wg, doneCh,
		)
	}
	go func() {
		cwg.Wait()
		close(cilCh)
	}()
	r.writeComposefileFromCImageLines(
		cPath, cByt, tmpDirPath, cilCh, rnCh, doneCh,
	)
}

func (r *Rewriter) writeDockerfileOrGetCImageLine(
	svcName string,
	svc *rewriter.Service,
	svcIms map[string][]*generate.ComposefileImage,
	tmpDirPath string,
	cilCh chan<- *cImageLine,
	rnCh chan<- *renameInfo,
	cwg *sync.WaitGroup,
	wg *sync.WaitGroup,
	doneCh <-chan struct{},
) {
	defer wg.Done()
	defer cwg.Done()
	var hasDFile bool
	switch build := svc.Build.(type) {
	case map[interface{}]interface{}:
		if build["context"] != nil || build["dockerfile"] != nil {
			hasDFile = true
		}
	case string:
		hasDFile = true
	}
	if hasDFile {
		dIms := make([]*generate.DockerfileImage, len(svcIms[svcName]))
		dPath := svcIms[svcName][0].DockerfilePath
		for i, cIm := range svcIms[svcName] {
			dIms[i] = &generate.DockerfileImage{Image: cIm.Image}
		}
		wg.Add(1)
		go r.writeDockerfile(dPath, dIms, tmpDirPath, rnCh, wg, doneCh)
	} else {
		im := svcIms[svcName][0]
		select {
		case <-doneCh:
		case cilCh <- &cImageLine{
			svcName: svcName,
			line: fmt.Sprintf(
				"%s:%s@sha256:%s", im.Image.Name, im.Image.Tag, im.Image.Digest,
			),
		}:
		}
	}
}

func (r *Rewriter) writeComposefileFromCImageLines(
	cPath string,
	cByt []byte,
	tmpDirPath string,
	cilCh chan *cImageLine,
	rnCh chan<- *renameInfo,
	doneCh <-chan struct{},
) {
	svcNameToLine := map[string]string{}
	for cil := range cilCh {
		svcNameToLine[cil.svcName] = cil.line
	}
	if len(svcNameToLine) != 0 {
		var (
			svcName string
			lines   = strings.Split(string(cByt), "\n")
		)
		for i, line := range lines {
			posSvcName := strings.Trim(line, " :")
			if svcNameToLine[posSvcName] != "" {
				svcName = posSvcName
				continue
			}
			if svcName != "" {
				if strings.HasPrefix(strings.TrimLeft(line, " "), "image:") {
					imIndex := strings.Index(line, "image:")
					lines[i] = fmt.Sprintf(
						"%s %s", line[:imIndex+len("image:")],
						svcNameToLine[svcName],
					)
					svcName = ""
				}
			}
		}
		var (
			oPath = cPath
			rwByt = []byte(strings.Join(lines, "\n"))
		)
		if r.Suffix != "" {
			var ymlSuffix string
			if strings.HasSuffix(cPath, ".yml") {
				ymlSuffix = ".yml"
			} else if strings.HasSuffix(cPath, ".yaml") {
				ymlSuffix = ".yaml"
			}
			oPath = fmt.Sprintf(
				"%s-%s%s", cPath[:len(cPath)-len(ymlSuffix)], r.Suffix,
				ymlSuffix,
			)
		}
		writeTmpFile(oPath, rwByt, tmpDirPath, rnCh, doneCh)
	}
}

func writeTmpFile(
	oPath string,
	rwByt []byte,
	tmpDirPath string,
	rnCh chan<- *renameInfo,
	doneCh <-chan struct{},
) {
	tmpFile, err := ioutil.TempFile(tmpDirPath, "docker-lock-")
	if err != nil {
		select {
		case <-doneCh:
		case rnCh <- &renameInfo{err: err}:
		}
		return
	}
	defer tmpFile.Close()
	if _, err = tmpFile.Write(rwByt); err != nil {
		select {
		case <-doneCh:
		case rnCh <- &renameInfo{err: err}:
		}
		return
	}
	origByt, err := getOrigByt(oPath)
	if err != nil {
		select {
		case <-doneCh:
		case rnCh <- &renameInfo{err: err}:
		}
		return
	}
	select {
	case <-doneCh:
	case rnCh <- &renameInfo{
		oPath:    oPath,
		tmpOPath: filepath.ToSlash(tmpFile.Name()),
		origByt:  origByt,
	}:
	}
}

func getOrigByt(oPath string) ([]byte, error) {
	fi, err := os.Stat(oPath)
	if err != nil {
		return nil, nil
	}
	if mode := fi.Mode(); !mode.IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", oPath)
	}
	origByt, err := ioutil.ReadFile(oPath) // nolint: gosec
	if err != nil {
		return nil, err
	}
	return origByt, nil
}

func (r *Rewriter) renameFiles(rnCh <-chan *renameInfo) error {
	allRnIs := []*renameInfo{}
	for rnI := range rnCh {
		if rnI.err != nil {
			return rnI.err
		}
		allRnIs = append(allRnIs, rnI)
	}
	successRnIs := []*renameInfo{}
	for _, rnI := range allRnIs {
		if rnErr := os.Rename(rnI.tmpOPath, rnI.oPath); rnErr != nil {
			if rvErr := revertRenamedFiles(successRnIs); rvErr != nil {
				return fmt.Errorf("%v: %v", rvErr, rnErr)
			}
			return rnErr
		}
		successRnIs = append(successRnIs, rnI)
	}
	return nil
}

func revertRenamedFiles(rnIs []*renameInfo) error {
	var (
		failedOPaths   = []string{}
		failedOPathsCh = make(chan string)
		wg             sync.WaitGroup
	)
	for _, rnI := range rnIs {
		wg.Add(1)
		go func(rnI *renameInfo) {
			defer wg.Done()
			if rnI.origByt != nil {
				if err := ioutil.WriteFile(
					rnI.oPath, rnI.origByt, 0644,
				); err != nil {
					failedOPathsCh <- rnI.oPath
				}
			} else {
				if err := os.Remove(rnI.oPath); err != nil {
					failedOPathsCh <- rnI.oPath
				}
			}
		}(rnI)
	}
	go func() {
		wg.Wait()
		close(failedOPathsCh)
	}()
	for oPath := range failedOPathsCh {
		failedOPaths = append(failedOPaths, oPath)
	}
	if len(failedOPaths) != 0 {
		return fmt.Errorf("failed to revert %s", failedOPaths)
	}
	return nil
}

func dedupeDockerfileImages(
	lFile *generate.Lockfile,
) map[string][]*generate.DockerfileImage {
	dPathsInCFiles := map[string]map[string]struct{}{}
	for cPath, ims := range lFile.ComposefileImages {
		for _, im := range ims {
			if im.DockerfilePath != "" {
				cPathSvc := fmt.Sprintf("%s/%s", cPath, im.ServiceName)
				if dPathsInCFiles[im.DockerfilePath] == nil {
					dPathsInCFiles[im.DockerfilePath] = map[string]struct{}{
						cPathSvc: {},
					}
				} else {
					dPathsInCFiles[im.DockerfilePath][cPathSvc] = struct{}{}
				}
			}
		}
	}
	for dPath, cPathSvcs := range dPathsInCFiles {
		if len(cPathSvcs) > 1 {
			var (
				dupCPathSvcs = make([]string, len(cPathSvcs))
				i            int
			)
			for cPathSvc := range cPathSvcs {
				dupCPathSvcs[i] = cPathSvc
				i++
			}
			fmt.Fprintf(
				os.Stderr,
				"WARNING: '%s' referenced in multiple "+
					"docker-compose services '%s', which will result in a "+
					"non-deterministic rewrite of '%s' if the docker-compose "+
					"services would lead to different rewrites.",
				dPath, dupCPathSvcs, dPath,
			)
		}
	}
	dImsNotInCFiles := map[string][]*generate.DockerfileImage{}
	for dPath, ims := range lFile.DockerfileImages {
		if dPathsInCFiles[dPath] == nil {
			dImsNotInCFiles[dPath] = ims
		}
	}
	return dImsNotInCFiles
}

func readLockfile(cmd *cobra.Command) (*generate.Lockfile, error) {
	lPath, err := cmd.Flags().GetString("lockfile-path")
	if err != nil {
		return nil, err
	}
	lPath = filepath.ToSlash(lPath)
	lByt, err := ioutil.ReadFile(lPath) // nolint: gosec
	if err != nil {
		return nil, err
	}
	var lFile generate.Lockfile
	if err = json.Unmarshal(lByt, &lFile); err != nil {
		return nil, err
	}
	return &lFile, err
}
