package resolver

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/gobwas/glob"
	"github.com/stormcat24/protodep/pkg/auth"
	"github.com/stormcat24/protodep/pkg/config"
	"github.com/stormcat24/protodep/pkg/logger"
	"github.com/stormcat24/protodep/pkg/repository"
)

type protoResource struct {
	source       string
	relativeDest string
}

type Resolver interface {
	Resolve(forceUpdate bool, cleanupCache bool) error

	SetHttpsAuthProvider(provider auth.AuthProvider)
	SetSshAuthProvider(provider auth.AuthProvider)
}

type resolver struct {
	conf *Config

	httpsProvider auth.AuthProvider
	sshProvider   auth.AuthProvider
}

func New(conf *Config) (Resolver, error) {
	s := &resolver{
		conf: conf,
	}

	err := s.initAuthProviders()
	if err != nil {
		return nil, err
	}

	return s, nil
}

func (s *resolver) Resolve(forceUpdate bool, cleanupCache bool) error {

	dep := config.NewDependency(s.conf.TargetDir, forceUpdate)
	protodep, err := dep.Load()
	if err != nil {
		return err
	}

	newdeps := make([]config.ProtoDepDependency, 0, len(protodep.Dependencies))
	protodepDir := filepath.Join(s.conf.HomeDir, ".protodep")

	_, err = os.Stat(protodepDir)
	if cleanupCache && err == nil {
		files, err := os.ReadDir(protodepDir)
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

	outdir := filepath.Join(s.conf.OutputDir, protodep.ProtoOutdir)
	if err := os.RemoveAll(outdir); err != nil {
		return err
	}

	for _, dep := range protodep.Dependencies {
		var authProvider auth.AuthProvider

		if s.conf.UseHttps {
			authProvider = s.httpsProvider
		} else {
			switch dep.Protocol {
			case "https":
				authProvider = s.httpsProvider
			case "ssh", "":
				authProvider = s.sshProvider
			default:
				return fmt.Errorf("%s protocol is not accepted (ssh or https only)", dep.Protocol)
			}
		}

		gitrepo := repository.NewGit(protodepDir, dep, authProvider)

		repo, err := gitrepo.Open()
		if err != nil {
			return err
		}

		sources := make([]protoResource, 0)

		compiledIgnores := compileIgnoreToGlob(dep.Ignores)
		compiledIncludes := compileIgnoreToGlob(dep.Includes)

		hasIncludes := len(dep.Includes) > 0

		protoRootDir := gitrepo.ProtoRootDir()
		filepath.Walk(protoRootDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if strings.HasSuffix(path, ".proto") {
				isIncludePath := s.isMatchPath(protoRootDir, path, dep.Includes, compiledIncludes)
				isIgnorePath := s.isMatchPath(protoRootDir, path, dep.Ignores, compiledIgnores)

				if hasIncludes && !isIncludePath {
					logger.Info("skipped %s due to include setting", path)
				} else if isIgnorePath {
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

		for _, s := range sources {
			outpath := filepath.Join(outdir, dep.Path, s.relativeDest)

			content, err := os.ReadFile(s.source)
			if err != nil {
				return err
			}

			if len(protodep.PatchAnnotation) > 0 {
				content = patchProtoFile(content, filepath.Join(protodep.ProtoOutdir, dep.Path, s.relativeDest), protodep.PatchAnnotation, protodep.Dependencies, protodep.ProtoOutdir)
			}

			if err := writeFileWithDirectory(outpath, content, 0644); err != nil {
				return err
			}
		}

		newdeps = append(newdeps, config.ProtoDepDependency{
			Target:   repo.Dep.Target,
			Branch:   repo.Dep.Branch,
			Revision: repo.Hash,
			Path:     repo.Dep.Path,
			Includes: repo.Dep.Includes,
			Ignores:  repo.Dep.Ignores,
			Protocol: repo.Dep.Protocol,
			Subgroup: repo.Dep.Subgroup,
		})
	}

	newProtodep := config.ProtoDep{
		ProtoOutdir:     protodep.ProtoOutdir,
		PatchAnnotation: protodep.PatchAnnotation,
		Dependencies:    newdeps,
	}

	if dep.IsNeedWriteLockFile() {
		if err := writeToml("protodep.lock", newProtodep); err != nil {
			return err
		}
	}

	return nil
}

func (s *resolver) SetHttpsAuthProvider(provider auth.AuthProvider) {
	s.httpsProvider = provider
}

func (s *resolver) SetSshAuthProvider(provider auth.AuthProvider) {
	s.sshProvider = provider
}

func (s *resolver) initAuthProviders() error {
	s.httpsProvider = auth.NewAuthProvider(auth.WithHTTPS(s.conf.BasicAuthUsername, s.conf.BasicAuthPassword))

	if s.conf.IdentityFile == "" && s.conf.IdentityPassword == "" {
		s.sshProvider = auth.NewAuthProvider()

		return nil
	}

	identifyPath := filepath.Join(s.conf.HomeDir, ".ssh", s.conf.IdentityFile)
	isSSH, err := isAvailableSSH(identifyPath)
	if err != nil {
		return err
	}

	if isSSH {
		s.sshProvider = auth.NewAuthProvider(auth.WithPemFile(identifyPath, s.conf.IdentityPassword))
	} else {
		logger.Warn("The identity file path has been passed but is not available. Falling back to ssh-agent, the default authentication method.")
		s.sshProvider = auth.NewAuthProvider()
	}

	return nil
}

func compileIgnoreToGlob(ignores []string) []glob.Glob {
	globIgnores := make([]glob.Glob, len(ignores))

	for idx, ignore := range ignores {
		globIgnores[idx] = glob.MustCompile(ignore)
	}

	return globIgnores
}

func (s *resolver) isMatchPath(protoRootDir string, target string, paths []string, globMatch []glob.Glob) bool {
	// convert slashes otherwise doesnt work on windows same was as on linux
	target = filepath.ToSlash(target)

	// keeping old logic for backward compatibility
	for _, pathToMatch := range paths {
		// support windows paths correctly
		pathPrefix := filepath.ToSlash(filepath.Join(protoRootDir, pathToMatch))
		if strings.HasPrefix(target, pathPrefix) {
			return true
		}
	}

	for _, pathToMatch := range globMatch {
		if pathToMatch.Match(target) {
			return true
		}
	}

	return false
}

func patchProtoFile(content []byte, filepath string, messageAnnotation string, sources []config.ProtoDepDependency, localBaseDir string) []byte {
	if len(content) > 0 {
		lineSeparator := "\n"
		dirSeparator := "/"
		packageSeparator := "."
		javaPackageLinePrefix := "option java_package"
		importLinePrefix := "import "
		packageLinePrefix := "package "
		messageLinePrefix := "message "
		oneOfLinePrefix := "oneof "
		enumLinePrefix := "enum "
		javaClassPrefix := "com."
		originalPackage := ""
		totalMessages := 0
		totalMessagesAlreadyPatched := 0
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
			} else if strings.HasPrefix(strings.TrimSpace(line), fmt.Sprintf("option (%s)", messageAnnotation)) {
				totalMessagesAlreadyPatched++
			}
		}

		if (originalPackage != "") && totalMessages > 0 {
			// patch messages
			patchedLines := make([]string, len(lines)+totalMessages-totalMessagesAlreadyPatched)
			totalLinesPatched := 0
			nestingLevel := 0
			originalMessage := ""
			tab := "    "
			inNestedBlock := false
			for _, line := range lines {
				if strings.HasPrefix(strings.TrimSpace(line), importLinePrefix) {
					targetExists, localTarget := getImportTargetFromRange(line, sources, localBaseDir)
					if targetExists {
						line = fmt.Sprintf("%s\"%s\";", importLinePrefix, localTarget)
					}
				}
				if nestingLevel == 0 || !strings.HasPrefix(strings.TrimSpace(line), fmt.Sprintf("option (%s)", messageAnnotation)) {
					patchedLines[totalLinesPatched] = line
					totalLinesPatched++
				}
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
				} else if strings.HasPrefix(strings.TrimSpace(line), oneOfLinePrefix) || strings.HasPrefix(strings.TrimSpace(line), enumLinePrefix) {
					inNestedBlock = true
				} else if inNestedBlock && strings.HasSuffix(strings.TrimSpace(line), "}") {
					inNestedBlock = false
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

func getImportTargetFromRange(line string, sources []config.ProtoDepDependency, localBaseDir string) (bool, string) {
	line = strings.ReplaceAll(line, " ", "")
	line = strings.Trim(line, "import\"")
	line = strings.Trim(line, "\";")
	for _, s := range sources {
		if strings.Contains(s.Target, line) {
			return true, fmt.Sprintf("%s/%s", strings.TrimPrefix(localBaseDir, "./"), s.Path)
		}
	}
	return false, ""
}

// eliminates unwanted chars (should they exist) from the subject of a 2-word-expression
// "package some.thing ;" => "some.thing"
// "message msg{" => "msg"
func eliminateCharIfNeeded(expression string, cutset string) string {
	splittedByNonWhiteSpace := strings.Split(strings.TrimSpace(expression), " ")
	subject := splittedByNonWhiteSpace[1]
	if strings.HasSuffix(subject, cutset) {
		return subject[0 : len(subject)-len(cutset)]
	}
	return subject
}

func writeToml(dest string, input interface{}) error {
	var buffer bytes.Buffer
	encoder := toml.NewEncoder(&buffer)
	if err := encoder.Encode(input); err != nil {
		return fmt.Errorf("encode config to toml format: %w", err)
	}

	if err := os.WriteFile(dest, buffer.Bytes(), 0644); err != nil {
		return fmt.Errorf("write to %s: %w", dest, err)
	}

	return nil
}

func writeFileWithDirectory(path string, data []byte, perm os.FileMode) error {

	path = filepath.ToSlash(path)
	s := strings.Split(path, "/")

	var dir string
	if len(s) > 1 {
		dir = strings.Join(s[0:len(s)-1], "/")
	} else {
		dir = path
	}

	dir = filepath.FromSlash(dir)
	path = filepath.FromSlash(path)

	if err := os.MkdirAll(dir, 0777); err != nil {
		return fmt.Errorf("create directory %s: %w", dir, err)
	}

	if err := os.WriteFile(path, data, perm); err != nil {
		return fmt.Errorf("write data to %s: %w", path, err)
	}

	return nil
}

// isAvailableSSH is Check whether this machine can use git protocol
func isAvailableSSH(identifyPath string) (bool, error) {
	if _, err := os.Stat(identifyPath); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	// TODO: validate ssh key
	return true, nil
}
