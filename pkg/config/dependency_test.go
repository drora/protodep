package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoad(t *testing.T) {

	pwd, _ := os.Getwd()
	target := NewDependency(pwd, false)

	actual, err := target.Load()
	require.NoError(t, err)
	require.Equal(t, 2, len(actual.Dependencies))

	withBranch := actual.Dependencies[0]
	withRevision := actual.Dependencies[1]

	require.Equal(t, "github.com/protocolbuffers/protobuf/src", withBranch.Target)
	require.Equal(t, "master", withBranch.Branch)
	require.Equal(t, "", withBranch.Revision)
	require.Equal(t, "", withBranch.Protocol)

	require.Equal(t, "github.com/grpc-ecosystem/grpc-gateway/examples/internal/helloworld", withRevision.Target)
	require.Equal(t, "", withRevision.Branch)
	require.Equal(t, "v2.7.2", withRevision.Revision)
	require.Equal(t, "grpc-gateway/examples/internal/helloworld", withRevision.Path)
	require.Equal(t, "ssh", withRevision.Protocol)
}

// TestLoadBehavior_NoLockFile tests that without a lock file, dependencies are loaded from toml
func TestLoadBehavior_NoLockFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create only protodep.toml with 2 dependencies
	tomlContent := `proto_outdir = "./proto"

[[dependencies]]
  target = "github.com/example/repo1"
  branch = "main"

[[dependencies]]
  target = "github.com/example/repo2"
  revision = "v1.0.0"
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "protodep.toml"), []byte(tomlContent), 0644))

	// Without lock file, forceUpdate=false should still read from toml
	dep := NewDependency(tmpDir, false)
	require.True(t, dep.IsNeedWriteLockFile(), "should need to write lock file when no lock exists")

	protodep, err := dep.Load()
	require.NoError(t, err)
	require.Equal(t, 2, len(protodep.Dependencies))
	require.Equal(t, "github.com/example/repo1", protodep.Dependencies[0].Target)
	require.Equal(t, "github.com/example/repo2", protodep.Dependencies[1].Target)
}

// TestLoadBehavior_WithLockFile_NoForce tests that with a lock file and no force,
// new deps from toml are added while existing deps preserve locked revisions
func TestLoadBehavior_WithLockFile_NoForce(t *testing.T) {
	tmpDir := t.TempDir()

	// Create protodep.toml with 3 dependencies (including a new one)
	tomlContent := `proto_outdir = "./proto"

[[dependencies]]
  target = "github.com/example/repo1"
  branch = "main"

[[dependencies]]
  target = "github.com/example/repo2"
  revision = "v1.0.0"

[[dependencies]]
  target = "github.com/example/repo3-new"
  branch = "develop"
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "protodep.toml"), []byte(tomlContent), 0644))

	// Create protodep.lock with only 2 dependencies (locked revisions)
	lockContent := `proto_outdir = "./proto"

[[dependencies]]
  target = "github.com/example/repo1"
  branch = "main"
  revision = "abc123locked"

[[dependencies]]
  target = "github.com/example/repo2"
  revision = "v1.0.0"
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "protodep.lock"), []byte(lockContent), 0644))

	dep := NewDependency(tmpDir, false)
	protodep, err := dep.Load()
	require.NoError(t, err)

	// NEW BEHAVIOR: All 3 deps from toml, with locked revisions merged for existing deps
	require.Equal(t, 3, len(protodep.Dependencies),
		"without force, all dependencies from toml are loaded including new ones")
	require.Equal(t, "github.com/example/repo1", protodep.Dependencies[0].Target)
	require.Equal(t, "abc123locked", protodep.Dependencies[0].Revision,
		"locked revision should be preserved for existing deps")
	require.Equal(t, "github.com/example/repo2", protodep.Dependencies[1].Target)
	require.Equal(t, "github.com/example/repo3-new", protodep.Dependencies[2].Target,
		"new dependency from toml should be included")

	// Should need to write lock file because new dep was added
	require.True(t, dep.IsNeedWriteLockFile(), "should need to write lock file when new deps are added")
}

// TestLoadBehavior_WithLockFile_Force tests that with force flag,
// ALL dependencies are read from toml (losing locked revisions)
func TestLoadBehavior_WithLockFile_Force(t *testing.T) {
	tmpDir := t.TempDir()

	// Create protodep.toml with 3 dependencies
	tomlContent := `proto_outdir = "./proto"

[[dependencies]]
  target = "github.com/example/repo1"
  branch = "main"

[[dependencies]]
  target = "github.com/example/repo2"
  revision = "v1.0.0"

[[dependencies]]
  target = "github.com/example/repo3-new"
  branch = "develop"
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "protodep.toml"), []byte(tomlContent), 0644))

	// Create protodep.lock with only 2 dependencies (with locked revisions)
	lockContent := `proto_outdir = "./proto"

[[dependencies]]
  target = "github.com/example/repo1"
  branch = "main"
  revision = "abc123locked"

[[dependencies]]
  target = "github.com/example/repo2"
  revision = "v1.0.0"
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "protodep.lock"), []byte(lockContent), 0644))

	// With forceUpdate=true, should read from toml
	dep := NewDependency(tmpDir, true)
	require.True(t, dep.IsNeedWriteLockFile(), "should need to write lock file when force is set")

	protodep, err := dep.Load()
	require.NoError(t, err)

	// KEY ASSERTION: All 3 deps from toml, but repo1 loses its locked revision!
	require.Equal(t, 3, len(protodep.Dependencies),
		"with force, all dependencies from toml are loaded including new ones")
	require.Equal(t, "github.com/example/repo1", protodep.Dependencies[0].Target)
	require.Equal(t, "", protodep.Dependencies[0].Revision,
		"with force, the locked revision is LOST - repo1 will resolve to latest on branch")
	require.Equal(t, "github.com/example/repo3-new", protodep.Dependencies[2].Target,
		"new dependency from toml is now included")
}

// TestLoadBehavior_MiddleGround confirms that without force:
// - New deps from toml are added
// - Locked revisions for existing deps are preserved
func TestLoadBehavior_MiddleGround(t *testing.T) {
	tmpDir := t.TempDir()

	tomlContent := `proto_outdir = "./proto"

[[dependencies]]
  target = "github.com/example/existing"
  branch = "main"

[[dependencies]]
  target = "github.com/example/new-dep"
  branch = "develop"
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "protodep.toml"), []byte(tomlContent), 0644))

	lockContent := `proto_outdir = "./proto"

[[dependencies]]
  target = "github.com/example/existing"
  branch = "main"
  revision = "locked-hash-12345"
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "protodep.lock"), []byte(lockContent), 0644))

	// Without force: new dep IS added AND locked revision IS preserved
	depNoForce := NewDependency(tmpDir, false)
	protodepNoForce, _ := depNoForce.Load()
	require.Equal(t, 2, len(protodepNoForce.Dependencies),
		"without force: new-dep IS added")
	require.Equal(t, "locked-hash-12345", protodepNoForce.Dependencies[0].Revision,
		"without force: locked revision IS preserved")
	require.Equal(t, "github.com/example/new-dep", protodepNoForce.Dependencies[1].Target,
		"without force: new dep from toml is included")

	// With force: locked revision is lost
	depForce := NewDependency(tmpDir, true)
	protodepForce, _ := depForce.Load()
	require.Equal(t, 2, len(protodepForce.Dependencies),
		"with force: new-dep IS added")
	require.Equal(t, "", protodepForce.Dependencies[0].Revision,
		"with force: locked revision is LOST (will resolve to latest)")
}

// TestLoadBehavior_IncludesIgnoresFromLock confirms that for existing deps,
// includes/ignores from lock file are used, ignoring changes in toml
func TestLoadBehavior_IncludesIgnoresFromLock(t *testing.T) {
	tmpDir := t.TempDir()

	// toml has different includes/ignores than lock
	tomlContent := `proto_outdir = "./proto"

[[dependencies]]
  target = "github.com/example/repo1"
  branch = "main"
  includes = ["new_include_from_toml/**"]
  ignores = ["new_ignore_from_toml/**"]
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "protodep.toml"), []byte(tomlContent), 0644))

	// lock has the original includes/ignores that should be preserved
	lockContent := `proto_outdir = "./proto"

[[dependencies]]
  target = "github.com/example/repo1"
  branch = "main"
  revision = "abc123"
  includes = ["locked_include/**"]
  ignores = ["locked_ignore/**"]
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "protodep.lock"), []byte(lockContent), 0644))

	// Without force: includes/ignores from lock should be used
	dep := NewDependency(tmpDir, false)
	protodep, err := dep.Load()
	require.NoError(t, err)

	require.Equal(t, 1, len(protodep.Dependencies))
	require.Equal(t, []string{"locked_include/**"}, protodep.Dependencies[0].Includes,
		"includes from lock file should be preserved, toml changes ignored")
	require.Equal(t, []string{"locked_ignore/**"}, protodep.Dependencies[0].Ignores,
		"ignores from lock file should be preserved, toml changes ignored")
}

// TestLoadBehavior_IncludesIgnoresNewDep confirms that for new deps,
// includes/ignores from toml are used (since they don't exist in lock)
func TestLoadBehavior_IncludesIgnoresNewDep(t *testing.T) {
	tmpDir := t.TempDir()

	tomlContent := `proto_outdir = "./proto"

[[dependencies]]
  target = "github.com/example/existing"
  branch = "main"
  includes = ["toml_include/**"]

[[dependencies]]
  target = "github.com/example/new-dep"
  branch = "develop"
  includes = ["new_dep_include/**"]
  ignores = ["new_dep_ignore/**"]
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "protodep.toml"), []byte(tomlContent), 0644))

	lockContent := `proto_outdir = "./proto"

[[dependencies]]
  target = "github.com/example/existing"
  branch = "main"
  revision = "abc123"
  includes = ["locked_include/**"]
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "protodep.lock"), []byte(lockContent), 0644))

	dep := NewDependency(tmpDir, false)
	protodep, err := dep.Load()
	require.NoError(t, err)

	require.Equal(t, 2, len(protodep.Dependencies))

	// Existing dep: uses includes from lock
	require.Equal(t, []string{"locked_include/**"}, protodep.Dependencies[0].Includes,
		"existing dep should use includes from lock")

	// New dep: uses includes/ignores from toml
	require.Equal(t, []string{"new_dep_include/**"}, protodep.Dependencies[1].Includes,
		"new dep should use includes from toml")
	require.Equal(t, []string{"new_dep_ignore/**"}, protodep.Dependencies[1].Ignores,
		"new dep should use ignores from toml")
}

// TestLoadBehavior_NoNewDeps_NoLockWrite confirms that when all deps are already
// locked and no new deps exist, lock file doesn't need to be rewritten
func TestLoadBehavior_NoNewDeps_NoLockWrite(t *testing.T) {
	tmpDir := t.TempDir()

	tomlContent := `proto_outdir = "./proto"

[[dependencies]]
  target = "github.com/example/repo1"
  branch = "main"
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "protodep.toml"), []byte(tomlContent), 0644))

	lockContent := `proto_outdir = "./proto"

[[dependencies]]
  target = "github.com/example/repo1"
  branch = "main"
  revision = "abc123"
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "protodep.lock"), []byte(lockContent), 0644))

	dep := NewDependency(tmpDir, false)
	_, err := dep.Load()
	require.NoError(t, err)

	require.False(t, dep.IsNeedWriteLockFile(),
		"should NOT need to write lock file when no new deps and not force")
}
