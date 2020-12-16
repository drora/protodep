package service

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/gobwas/glob"
	"github.com/stormcat24/protodep/dependency"
	"github.com/stormcat24/protodep/helper"
	"github.com/stormcat24/protodep/logger"
	"github.com/stormcat24/protodep/repository"
)

type protoResource struct {
	source       string
	relativeDest string
}

type Sync interface {
	Resolve(forceUpdate bool, cleanupCache bool, overwrite bool) error
}

type SyncImpl struct {
	authProvider  helper.AuthProvider
	userHomeDir   string
	targetDir     string
	outputRootDir string
}

func NewSync(authProvider helper.AuthProvider, userHomeDir string, targetDir string, outputRootDir string) Sync {
	return &SyncImpl{
		authProvider:  authProvider,
		userHomeDir:   userHomeDir,
		targetDir:     targetDir,
		outputRootDir: outputRootDir,
	}
}

func (s *SyncImpl) Resolve(forceUpdate bool, cleanupCache bool, overwrite bool) error {

	dep := dependency.NewDependency(s.targetDir, forceUpdate)
	protodep, err := dep.Load()
	if err != nil {
		return err
	}

	newdeps := make([]dependency.ProtoDepDependency, 0, len(protodep.Dependencies))
	protodepDir := filepath.Join(s.userHomeDir, ".protodep")

	_, err = os.Stat(protodepDir)
	if cleanupCache && err == nil {
		files, err := ioutil.ReadDir(protodepDir)
		if err != nil {
			return err
		}
		for _, file := range files {
			if file.IsDir() {
				dirpath := filepath.Join(protodepDir, file.Name())
				if err := os.RemoveAll(dirpath); err != nil {
					return err
				}
			}
		}
	}

	outdir := filepath.Join(s.outputRootDir, protodep.ProtoOutdir)

	if err := cleanup(outdir, protodep.Dependencies, forceUpdate, overwrite); err != nil {
		return err
	}

	for _, dep := range protodep.Dependencies {
		gitrepo := repository.NewGitRepository(protodepDir, dep, s.authProvider)

		repo, err := gitrepo.Open()
		if err != nil {
			return err
		}

		sources := make([]protoResource, 0)

		compiledIgnores := compileIngoresToGlob(dep.Ignores)

		protoRootDir := gitrepo.ProtoRootDir()
		err = filepath.Walk(protoRootDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if strings.HasSuffix(path, ".proto") {
				if s.isIgnorePath(protoRootDir, path, dep.Ignores, compiledIgnores) {
					logger.Info("skipped %s due to ignore setting", path)
				} else {
					sources = append(sources, protoResource{
						source:       path,
						relativeDest: strings.Replace(path, protoRootDir, "", -1),
					})
				}
			}
			return nil
		})
		if err != nil {
			return err
		}

		for _, s := range sources {
			outpath := filepath.Join(outdir, dep.Path, s.relativeDest)
			_, statErr := os.Stat(outpath)
			if statErr != nil {
				content, err := ioutil.ReadFile(s.source)
				if err != nil {
					return err
				}

				if len(protodep.PatchAnnotation) > 0 {
					content = patchProtoFile(content, filepath.Join(protodep.ProtoOutdir, dep.Path, s.relativeDest), protodep.PatchAnnotation)
				}
				if err := helper.WriteFileWithDirectory(outpath, content, 0644); err != nil {
					return err
				}
			} else {
				logger.Info("skipped %s - already exists", outpath)
			}
		}

		newdeps = append(newdeps, dependency.ProtoDepDependency{
			Target:   repo.Dep.Target,
			Branch:   repo.Dep.Branch,
			Revision: repo.Hash,
			Path:     repo.Dep.Path,
			Ignores:  repo.Dep.Ignores,
		})
	}

	newProtodep := dependency.ProtoDep{
		ProtoOutdir:  protodep.ProtoOutdir,
		Dependencies: newdeps,
	}

	if dep.IsNeedWriteLockFile() {
		if err := helper.WriteToml("protodep.lock", newProtodep); err != nil {
			return err
		}
	}

	return nil
}

func cleanup(outdir string, dependencies []dependency.ProtoDepDependency, forceUpdate bool, overwrite bool) error {
	allDepsHasDefinedPath := true
	for _, dep := range dependencies {
		if dep.Path == "" {
			allDepsHasDefinedPath = false
			break
		}
	}

	if allDepsHasDefinedPath {
		for _, dep := range dependencies {
			pathdir := filepath.Join(outdir, dep.Path)
			if forceUpdate || overwrite {
				if err := os.RemoveAll(pathdir); err != nil {
					if err := os.Remove(pathdir); err != nil {
						return err
					}
				}
			}
		}
	} else {
		if err := os.RemoveAll(outdir); err != nil {
			return err
		}
	}
	return nil
}

func patchProtoFile(content []byte, filepath string, messageAnnotation string) []byte {
	if len(content) > 0 {
		lineSeparator := "\n"
		dirSeparator := "/"
		packageSeparator := "."
		javaPackageLinePrefix := "option java_package"
		packageLinePrefix := "package "
		messageLinePrefix := "message "
		javaClassPrefix := "com."
		originalPackage := ""
		totalMessages := 0
		path := strings.Split(filepath, dirSeparator)
		formattedPath := strings.Join(path[0:(len(path)-1)], packageSeparator)

		lines := strings.Split(string(content), lineSeparator)
		// patch package, remember original, count messages
		for i, line := range lines {
			relativePath := strings.ReplaceAll(formattedPath, dirSeparator, packageSeparator)
			if strings.HasPrefix(strings.TrimSpace(line), packageLinePrefix) {
				originalPackage = eliminateCharIfNeeded(line, ";")
				lines[i] = fmt.Sprintf("%s%s;", packageLinePrefix, relativePath)
			} else if strings.HasPrefix(strings.TrimSpace(line), javaPackageLinePrefix) {
				if !(strings.HasPrefix(relativePath, javaClassPrefix)) {
					relativePath = javaClassPrefix + relativePath
				}
				lines[i] = fmt.Sprintf("%s = \"%s\";", javaPackageLinePrefix, relativePath)
			} else if strings.HasPrefix(strings.TrimSpace(line), messageLinePrefix) {
				totalMessages++
			}
		}

		if (originalPackage != "") && totalMessages > 0 {
			// patch messages
			patchedLines := make([]string, len(lines) + totalMessages)
			totalLinesPatched := 0
			nestingLevel := 0
			originalMessage := ""
			tab := "    "
			for _, line := range lines {
				patchedLines[totalLinesPatched] = line
				totalLinesPatched++
				if strings.HasPrefix(strings.TrimSpace(line), messageLinePrefix) {
					nestingLevel++
					topLevelMessage := eliminateCharIfNeeded(line, "{")
					if originalMessage == "" {
						originalMessage = topLevelMessage
					} else {
						originalMessage = originalMessage + packageSeparator + topLevelMessage
					}
					patchedLines[totalLinesPatched] = fmt.Sprintf("%soption (%s) = \"%s.%s\";", strings.Repeat(tab, nestingLevel), messageAnnotation, originalPackage, originalMessage)
					totalLinesPatched++
				} else if originalMessage != "" && strings.HasSuffix(strings.TrimSpace(line), "}") {
					nestingLevel--
					splitNestedMessage := strings.Split(originalMessage, packageSeparator)
					if len(splitNestedMessage) > 0 {
						originalMessage = strings.Join(splitNestedMessage[0:len(splitNestedMessage)-1], packageSeparator)
					} else {
						originalMessage = ""
					}
				}
			}
			return []byte(strings.Join(patchedLines, lineSeparator))
		}

		return []byte(strings.Join(lines, lineSeparator))
	}
	return content
}

// eliminates unwanted chars (should they exist) from the subject of a 2-word-expression
// "package some.thing ;" => "some.thing"
// "message msg{" => "msg"
func eliminateCharIfNeeded(expression string, cutset string) string {
	splittedByNonWhiteSpace := strings.Split(strings.TrimSpace(expression), " ")
	subject := splittedByNonWhiteSpace[1]
	if strings.HasSuffix(subject, cutset) {
		return subject[0:len(subject)-len(cutset)]
	}
	return subject
}

func compileIngoresToGlob(ignores []string) []glob.Glob {
	globIngores := make([]glob.Glob, len(ignores))

	for idx, ignore := range ignores {
		globIngores[idx] = glob.MustCompile(ignore)
	}

	return globIngores
}

func (s *SyncImpl) isIgnorePath(protoRootDir string, target string, ignores []string, globIgnores []glob.Glob) bool {
	// convert slashes otherwise doesnt work on windows same was as on linux
	target = filepath.ToSlash(target)

	// keeping old logic for backward compatibility
	for _, ignore := range ignores {
		// support windows paths correctly
		pathPrefix := filepath.ToSlash(filepath.Join(protoRootDir, ignore))
		if strings.HasPrefix(target, pathPrefix) {
			return true
		}
	}

	for _, ignore := range globIgnores {
		if ignore.Match(target) {
			return true
		}
	}

	return false
}
