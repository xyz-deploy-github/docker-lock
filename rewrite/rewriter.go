// Package rewrite provides functions to rewrite Dockerfiles
// and docker-compose files from a Lockfile.
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
	"gopkg.in/yaml.v2"
)

// Rewriter is used to rewrite base images in docker and docker-compose files
// with their digests.
type Rewriter struct {
	Lockfile *generate.Lockfile
	Suffix   string
	TempDir  string
}

// rnInfo contains information necessary to rename a file that has been
// written to a temporary directory. In case of failure during renaming,
// the bytes of the file that will be overwritten by a rename are stored
// so that the file can be reverted to its original.
type rnInfo struct {
	oPath    string
	tmpOPath string
	origByt  []byte
	err      error
}

// cImLine represents an image line for a docker-compose service that
// is specified as image: "..." rather than in a Dockerfile.
type cImLine struct {
	svcName string
	line    string
}

// NewRewriter creates a Rewriter from command line flags.
func NewRewriter(flags *Flags) (*Rewriter, error) {
	lfile, err := readLfile(flags.LockfilePath)
	if err != nil {
		return nil, err
	}

	lfile.DockerfileImages = dedupeDIms(lfile)

	return &Rewriter{
		Lockfile: lfile,
		Suffix:   flags.Suffix,
		TempDir:  flags.TempDir,
	}, nil
}

// Rewrite rewrites docker and docker-compose files to include base images
// with digests from a Lockfile.
//
// Rewrite has "transaction"-like properties to ensure all rewrites succeed or
// fail together. The function follows the following steps:
//
// (1) Create a temporary directory in the system default temporary directory
// location or in the location supplied via the command line arg.
//
// (2) Rewrite every file to a file in the temporary directory.
//
// (3) If all rewrites succeed, rename each temporary file to its desired
// outpath. Providing a suffix ensures that the temporary file will not
// overwrite the original. Instead, a new file of the form Dockerfile-suffix,
// docker-compose-suffix.yml, or docker-compose-suffix.yaml will be written.
//
// (4) If an error occurs during renaming, revert all files back to their
// original content.
//
// (5) If reverting fails, return an error with the paths that failed
// to revert.
//
// (6) Delete the temporary directory.
//
// Note: If the Lockfile references a Dockerfile and that same Dockerfile
// is referenced by another docker-compose file, the Dockerfile will be
// rewritten according to the docker-compose file.
func (r *Rewriter) Rewrite() (err error) {
	if len(r.Lockfile.DockerfileImages) == 0 &&
		len(r.Lockfile.ComposefileImages) == 0 {
		return nil
	}

	tmpDirPath, err := ioutil.TempDir(r.TempDir, "docker-lock-tmp")
	if err != nil {
		return err
	}

	defer func() {
		if rmErr := os.RemoveAll(tmpDirPath); rmErr != nil {
			err = fmt.Errorf("%v: %v", rmErr, err)
		}
	}()

	doneCh := make(chan struct{})
	rnCh := r.writeFiles(tmpDirPath, doneCh)

	if err := r.renameFiles(rnCh); err != nil {
		close(doneCh)
		return err
	}

	return nil
}

func (r *Rewriter) writeFiles(
	tmpDirPath string,
	doneCh <-chan struct{},
) chan *rnInfo {
	rnCh := make(chan *rnInfo)
	wg := sync.WaitGroup{}

	wg.Add(1)

	go r.writeCfiles(tmpDirPath, rnCh, &wg, doneCh)

	wg.Add(1)

	go r.writeDfiles(tmpDirPath, rnCh, &wg, doneCh)

	go func() {
		wg.Wait()
		close(rnCh)
	}()

	return rnCh
}

func (r *Rewriter) writeDfiles(
	tmpDirPath string,
	rnCh chan<- *rnInfo,
	wg *sync.WaitGroup,
	doneCh <-chan struct{},
) {
	defer wg.Done()

	for dPath, ims := range r.Lockfile.DockerfileImages {
		wg.Add(1)

		go r.writeDfile(dPath, ims, tmpDirPath, rnCh, wg, doneCh)
	}
}

func (r *Rewriter) writeCfiles(
	tmpDirPath string,
	rnCh chan<- *rnInfo,
	wg *sync.WaitGroup,
	doneCh <-chan struct{},
) {
	defer wg.Done()

	for cPath, ims := range r.Lockfile.ComposefileImages {
		wg.Add(1)

		go r.writeCfile(cPath, ims, tmpDirPath, rnCh, wg, doneCh)
	}
}

func (r *Rewriter) writeDfile(
	dPath string,
	ims []*generate.DockerfileImage,
	tmpDirPath string,
	rnCh chan<- *rnInfo,
	wg *sync.WaitGroup,
	doneCh <-chan struct{},
) {
	defer wg.Done()

	dByt, err := ioutil.ReadFile(dPath) // nolint: gosec
	if err != nil {
		addErrToRnCh(err, rnCh, doneCh)
		return
	}

	stageNames := map[string]bool{}
	statements := strings.Split(string(dByt), "\n")
	imIndex := 0

	for i, s := range statements {
		fields := strings.Fields(s)

		const instIndex = 0

		if len(fields) > 0 && strings.ToLower(fields[instIndex]) == "from" {
			// FROM <image>
			// FROM <image> AS <stage>
			// FROM <stage> AS <another stage>
			const imLineIndex = 1
			if !stageNames[fields[imLineIndex]] {
				if imIndex >= len(ims) {
					err := fmt.Errorf(
						"more images exist in %s than in the Lockfile",
						dPath,
					)
					addErrToRnCh(err, rnCh, doneCh)

					return
				}

				fields[imLineIndex] = fmt.Sprintf(
					"%s:%s@sha256:%s", ims[imIndex].Image.Name,
					ims[imIndex].Image.Tag, ims[imIndex].Image.Digest,
				)
				imIndex++
			}
			// FROM <image> AS <stage>
			// FROM <stage> AS <another stage>
			const maxNumFields = 4
			if len(fields) == maxNumFields {
				const stageIndex = 3
				stageName := fields[stageIndex]
				stageNames[stageName] = true
			}

			statements[i] = strings.Join(fields, " ")
		}
	}

	if imIndex != len(ims) {
		err := fmt.Errorf(
			"more images exist in the Lockfile than in %s", dPath,
		)
		addErrToRnCh(err, rnCh, doneCh)

		return
	}

	rwByt := []byte(strings.Join(statements, "\n"))

	oPath := dPath

	if r.Suffix != "" {
		oPath = fmt.Sprintf("%s-%s", dPath, r.Suffix)
	}

	writeTmpFile(oPath, rwByt, tmpDirPath, rnCh, doneCh)
}

func (r *Rewriter) writeCfile(
	cPath string,
	ims []*generate.ComposefileImage,
	tmpDirPath string,
	rnCh chan<- *rnInfo,
	wg *sync.WaitGroup,
	doneCh <-chan struct{},
) {
	defer wg.Done()

	cByt, err := ioutil.ReadFile(cPath) // nolint: gosec
	if err != nil {
		addErrToRnCh(err, rnCh, doneCh)
		return
	}

	comp := compose{}
	if err := yaml.Unmarshal(cByt, &comp); err != nil {
		addErrToRnCh(err, rnCh, doneCh)
		return
	}

	svcIms := map[string][]*generate.ComposefileImage{}

	for _, im := range ims {
		svcIms[im.ServiceName] = append(svcIms[im.ServiceName], im)
	}

	if len(comp.Services) != len(svcIms) {
		err := fmt.Errorf(
			"%s has %d service(s), yet the Lockfile has %d", cPath,
			len(comp.Services), len(svcIms),
		)
		addErrToRnCh(err, rnCh, doneCh)

		return
	}

	cilCh := make(chan *cImLine)
	cwg := sync.WaitGroup{}

	for svcName, svc := range comp.Services {
		if _, ok := svcIms[svcName]; !ok {
			err := fmt.Errorf(
				"service %s exists in %s, but not in the Lockfile", svcName,
				cPath,
			)
			addErrToRnCh(err, rnCh, doneCh)

			return
		}

		cwg.Add(1)
		wg.Add(1)

		go r.writeDfileOrGetCImLine(
			svcName, svc, svcIms, tmpDirPath, cilCh, rnCh, &cwg, wg, doneCh,
		)
	}

	go func() {
		cwg.Wait()
		close(cilCh)
	}()

	r.writeCfileFromCImLine(
		cPath, cByt, tmpDirPath, cilCh, rnCh, doneCh,
	)
}

func (r *Rewriter) writeDfileOrGetCImLine(
	svcName string,
	svc *service,
	svcIms map[string][]*generate.ComposefileImage,
	tmpDirPath string,
	cilCh chan<- *cImLine,
	rnCh chan<- *rnInfo,
	cwg *sync.WaitGroup,
	wg *sync.WaitGroup,
	doneCh <-chan struct{},
) {
	defer wg.Done()
	defer cwg.Done()

	hasDfile := false

	switch build := svc.Build.(type) {
	case map[interface{}]interface{}:
		if build["context"] != nil || build["dockerfile"] != nil {
			hasDfile = true
		}
	case string:
		hasDfile = true
	}

	switch hasDfile {
	case true:
		dIms := make([]*generate.DockerfileImage, len(svcIms[svcName]))
		dPath := svcIms[svcName][0].DockerfilePath

		for i, cIm := range svcIms[svcName] {
			dIms[i] = &generate.DockerfileImage{Image: cIm.Image}
		}

		wg.Add(1)

		go r.writeDfile(dPath, dIms, tmpDirPath, rnCh, wg, doneCh)
	default:
		im := svcIms[svcName][0]

		select {
		case <-doneCh:
		case cilCh <- &cImLine{
			svcName: svcName,
			line: fmt.Sprintf(
				"%s:%s@sha256:%s", im.Image.Name, im.Image.Tag,
				im.Image.Digest,
			),
		}:
		}
	}
}

func (r *Rewriter) writeCfileFromCImLine(
	cPath string,
	cByt []byte,
	tmpDirPath string,
	cilCh chan *cImLine,
	rnCh chan<- *rnInfo,
	doneCh <-chan struct{},
) {
	svcNameToLine := map[string]string{}

	for cil := range cilCh {
		svcNameToLine[cil.svcName] = cil.line
	}

	if len(svcNameToLine) != 0 {
		svcName := ""
		statements := strings.Split(string(cByt), "\n")

		for i, s := range statements {
			posSvcName := strings.Trim(s, " :")

			if svcNameToLine[posSvcName] != "" {
				svcName = posSvcName
				continue
			}

			if svcName != "" &&
				strings.HasPrefix(strings.TrimLeft(s, " "), "image:") {
				imIndex := strings.Index(s, "image:")
				statements[i] = fmt.Sprintf(
					"%s %s", s[:imIndex+len("image:")], svcNameToLine[svcName],
				)
				svcName = ""
			}
		}

		rwByt := []byte(strings.Join(statements, "\n"))

		oPath := cPath

		if r.Suffix != "" {
			ymlSuffix := ""

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
	rnCh chan<- *rnInfo,
	doneCh <-chan struct{},
) {
	tmpFile, err := ioutil.TempFile(tmpDirPath, "docker-lock-")

	if err != nil {
		addErrToRnCh(err, rnCh, doneCh)
		return
	}
	defer tmpFile.Close()

	if _, err = tmpFile.Write(rwByt); err != nil {
		addErrToRnCh(err, rnCh, doneCh)
		return
	}

	origByt, err := getOrigByt(oPath)
	if err != nil {
		addErrToRnCh(err, rnCh, doneCh)
		return
	}

	select {
	case <-doneCh:
	case rnCh <- &rnInfo{
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

func (r *Rewriter) renameFiles(rnCh <-chan *rnInfo) error {
	allRns := []*rnInfo{}

	for rn := range rnCh {
		if rn.err != nil {
			return rn.err
		}

		allRns = append(allRns, rn)
	}

	successRns := []*rnInfo{}

	for _, rn := range allRns {
		if rnErr := os.Rename(rn.tmpOPath, rn.oPath); rnErr != nil {
			if rvErr := revertRnFiles(successRns); rvErr != nil {
				return fmt.Errorf("%v: %v", rvErr, rnErr)
			}

			return rnErr
		}

		successRns = append(successRns, rn)
	}

	return nil
}

func revertRnFiles(rns []*rnInfo) error {
	failedOPathsCh := make(chan string)
	wg := sync.WaitGroup{}

	for _, rn := range rns {
		wg.Add(1)

		go func(rn *rnInfo) {
			defer wg.Done()

			switch rn.origByt {
			case nil:
				if err := os.Remove(rn.oPath); err != nil {
					failedOPathsCh <- rn.oPath
				}
			default:
				if err := ioutil.WriteFile(
					rn.oPath, rn.origByt, 0644,
				); err != nil {
					failedOPathsCh <- rn.oPath
				}
			}
		}(rn)
	}

	go func() {
		wg.Wait()
		close(failedOPathsCh)
	}()

	failedOPaths := []string{}
	for oPath := range failedOPathsCh {
		failedOPaths = append(failedOPaths, oPath)
	}

	if len(failedOPaths) != 0 {
		return fmt.Errorf("failed to revert %s", failedOPaths)
	}

	return nil
}

func dedupeDIms(
	lfile *generate.Lockfile,
) map[string][]*generate.DockerfileImage {
	dPathsInCfiles := map[string]map[string]struct{}{}

	for cPath, ims := range lfile.ComposefileImages {
		for _, im := range ims {
			if im.DockerfilePath != "" {
				cPathSvc := fmt.Sprintf("%s/%s", cPath, im.ServiceName)

				if dPathsInCfiles[im.DockerfilePath] == nil {
					dPathsInCfiles[im.DockerfilePath] = map[string]struct{}{
						cPathSvc: {},
					}
				} else {
					dPathsInCfiles[im.DockerfilePath][cPathSvc] = struct{}{}
				}
			}
		}
	}

	for dPath, cPathSvcs := range dPathsInCfiles {
		if len(cPathSvcs) > 1 {
			dupCPathSvcs := make([]string, len(cPathSvcs))

			i := 0

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

	dImsNotInCfiles := map[string][]*generate.DockerfileImage{}

	for dPath, ims := range lfile.DockerfileImages {
		if dPathsInCfiles[dPath] == nil {
			dImsNotInCfiles[dPath] = ims
		}
	}

	return dImsNotInCfiles
}

func readLfile(lPath string) (*generate.Lockfile, error) {
	lByt, err := ioutil.ReadFile(lPath) // nolint: gosec
	if err != nil {
		return nil, err
	}

	lfile := generate.Lockfile{}
	if err = json.Unmarshal(lByt, &lfile); err != nil {
		return nil, err
	}

	return &lfile, err
}

func addErrToRnCh(err error, rnCh chan<- *rnInfo, doneCh <-chan struct{}) {
	select {
	case <-doneCh:
	case rnCh <- &rnInfo{err: err}:
	}
}
