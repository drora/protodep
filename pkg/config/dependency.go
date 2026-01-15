package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type Dependency interface {
	Load() (*ProtoDep, error)
	IsNeedWriteLockFile() bool
}

type DependencyImpl struct {
	targetDir   string
	tomlPath    string
	lockPath    string
	forceUpdate bool
	hasNewDeps  bool // set after Load() is called
}

func NewDependency(targetDir string, forceUpdate bool) Dependency {
	return &DependencyImpl{
		targetDir:   targetDir,
		tomlPath:    filepath.Join(targetDir, "protodep.toml"),
		lockPath:    filepath.Join(targetDir, "protodep.lock"),
		forceUpdate: forceUpdate,
	}
}

func (d *DependencyImpl) Load() (*ProtoDep, error) {
	// Always read from toml to get all dependencies (including new ones)
	tomlContent, err := os.ReadFile(d.tomlPath)
	if err != nil {
		return nil, fmt.Errorf("load %s: %w", d.tomlPath, err)
	}

	var conf ProtoDep
	if _, err := toml.Decode(string(tomlContent), &conf); err != nil {
		return nil, fmt.Errorf("decode toml: %w", err)
	}

	if err := conf.Validate(); err != nil {
		return nil, fmt.Errorf("found invalid configuration: %w", err)
	}

	// If not force update and lock file exists, merge locked info for existing deps
	if !d.forceUpdate && d.hasLockFile() {
		lockContent, err := os.ReadFile(d.lockPath)
		if err != nil {
			return nil, fmt.Errorf("load %s: %w", d.lockPath, err)
		}

		var lockConf ProtoDep
		if _, err := toml.Decode(string(lockContent), &lockConf); err != nil {
			return nil, fmt.Errorf("decode lock toml: %w", err)
		}

		// Build a map of locked deps by target for quick lookup
		lockedDeps := make(map[string]ProtoDepDependency)
		for _, dep := range lockConf.Dependencies {
			lockedDeps[dep.Target] = dep
		}

		// Merge: for deps that exist in lock, use locked revision/includes/ignores
		for i, dep := range conf.Dependencies {
			if locked, exists := lockedDeps[dep.Target]; exists {
				conf.Dependencies[i].Revision = locked.Revision
				conf.Dependencies[i].Includes = locked.Includes
				conf.Dependencies[i].Ignores = locked.Ignores
			} else {
				// New dep found in toml but not in lock
				d.hasNewDeps = true
			}
		}
	} else if !d.hasLockFile() {
		// No lock file means all deps are "new"
		d.hasNewDeps = true
	}

	return &conf, nil
}

func (d *DependencyImpl) hasLockFile() bool {
	_, err := os.Stat(d.lockPath)
	return err == nil
}

func (d *DependencyImpl) IsNeedWriteLockFile() bool {
	// Write lock file if: force update, no lock file exists, or new deps were added
	return d.forceUpdate || !d.hasLockFile() || d.hasNewDeps
}
