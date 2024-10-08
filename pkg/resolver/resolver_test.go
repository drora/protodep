package resolver

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/mitchellh/go-homedir"
	"github.com/stretchr/testify/require"

	"github.com/stormcat24/protodep/pkg/auth"
	"github.com/stormcat24/protodep/pkg/config"
)

func TestSync(t *testing.T) {

	homeDir, err := homedir.Dir()
	require.NoError(t, err)

	dotProtoDir := filepath.Join(homeDir, "protodep_ut")
	err = os.RemoveAll(dotProtoDir)
	require.NoError(t, err)

	pwd, err := os.Getwd()
	fmt.Println(pwd)

	require.NoError(t, err)

	outputRootDir := os.TempDir()

	conf := Config{
		HomeDir:   dotProtoDir,
		TargetDir: pwd,
		OutputDir: outputRootDir,
	}

	target, err := New(&conf)
	require.NoError(t, err)

	c := gomock.NewController(t)
	defer c.Finish()

	httpsAuthProviderMock := auth.NewMockAuthProvider(c)
	httpsAuthProviderMock.EXPECT().AuthMethod().Return(nil, nil).AnyTimes()
	httpsAuthProviderMock.EXPECT().GetRepositoryURL("github.com/protocolbuffers/protobuf").Return("https://github.com/protocolbuffers/protobuf.git")
	httpsAuthProviderMock.EXPECT().GetRepositoryURL("github.com/protodep/catalog").Return("https://github.com/protodep/catalog.git")

	sshAuthProviderMock := auth.NewMockAuthProvider(c)
	sshAuthProviderMock.EXPECT().AuthMethod().Return(nil, nil).AnyTimes()
	sshAuthProviderMock.EXPECT().GetRepositoryURL("github.com/opensaasstudio/plasma").Return("https://github.com/opensaasstudio/plasma.git")

	target.SetHttpsAuthProvider(httpsAuthProviderMock)
	target.SetSshAuthProvider(sshAuthProviderMock)

	// clone
	err = target.Resolve(false, false)
	require.NoError(t, err)

	if !isFileExist(filepath.Join(outputRootDir, "proto/stream.proto")) {
		t.Error("not found file [proto/stream.proto]")
	}
	if !isFileExist(filepath.Join(outputRootDir, "proto/google/protobuf/empty.proto")) {
		t.Error("not found file [proto/google/protobuf/empty.proto]")
	}

	// check ignore worked
	// hasPrefix test - backward compatibility
	if isFileExist(filepath.Join(outputRootDir, "proto/google/protobuf/test_messages_proto3.proto")) {
		t.Error("found file [proto/google/protobuf/test_messages_proto3.proto]")
	}

	// glob test 1
	if isFileExist(filepath.Join(outputRootDir, "proto/google/protobuf/test_messages_proto2.proto")) {
		t.Error("found file [proto/google/protobuf/test_messages_proto2.proto]")
	}

	// glob test 2
	if isFileExist(filepath.Join(outputRootDir, "proto/google/protobuf/test_messages_proto2.proto")) {
		t.Error("found file [proto/google/protobuf/test_messages_proto2.proto]")
	}

	// glob test 3
	if isFileExist(filepath.Join(outputRootDir, "proto/google/protobuf/util/internal/testdata/")) {
		t.Error("found file [proto/google/protobuf/util/internal/testdata/]")
	}

	// check include worked
	// glob test 1
	if !isFileExist(filepath.Join(outputRootDir, "proto/protodep/hierarchy/service.proto")) {
		t.Error("not found file [proto/protodep/hierarchy/service.proto]")
	}

	// glob test 2
	if !isFileExist(filepath.Join(outputRootDir, "proto/protodep/hierarchy/fuga/fuga.proto")) {
		t.Error("not found file [proto/protodep/hierarchy/fuga/fuga.proto]")
	}

	// fetch
	err = target.Resolve(false, false)
	require.NoError(t, err)
}

func isFileExist(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

func TestWriteToml(t *testing.T) {

	config := config.ProtoDep{
		ProtoOutdir: "./proto",
		Dependencies: []config.ProtoDepDependency{
			config.ProtoDepDependency{
				Target:   "github.com/openfresh/plasma/protobuf",
				Branch:   "master",
				Revision: "d7ee1d95b6700756b293b722a1cfd4b905a351ba",
			},
			config.ProtoDepDependency{
				Target:   "github.com/grpc-ecosystem/grpc-gateway/examples/examplepb",
				Branch:   "master",
				Revision: "c6f7a5ac629444a556bb665e389e41b897ebad39",
			},
		},
	}

	destDir := os.TempDir()
	destFile := filepath.Join(destDir, "protodep.lock")

	require.NoError(t, os.MkdirAll(os.TempDir(), 0777))
	require.NoError(t, writeToml(destFile, config))

	stat, err := os.Stat(destFile)
	require.NoError(t, err)

	require.True(t, !stat.IsDir())
}

func TestWriteFileWithDirectory(t *testing.T) {
	destDir := os.TempDir()
	testDir := filepath.Join(destDir, "hoge")
	testFile := filepath.Join(testDir, "fuga.txt")

	err := writeFileWithDirectory(testFile, []byte("test"), 0644)
	require.NoError(t, err)

	stat, err := os.Stat(testFile)
	require.NoError(t, err)
	require.True(t, !stat.IsDir())

	data, err := os.ReadFile(testFile)
	require.NoError(t, err)
	require.Equal(t, string(data), "test")
}

func TestIsAvailableSSH(t *testing.T) {
	f, err := os.CreateTemp("", "id_rsa")
	require.NoError(t, err)

	found, err := isAvailableSSH(f.Name())
	require.NoError(t, err)
	require.True(t, found)

	notFound, err := isAvailableSSH(fmt.Sprintf("/tmp/IsAvailableSSH_%d", time.Now().UnixNano()))
	require.NoError(t, err)
	require.False(t, notFound)
}

func TestPatch(t *testing.T) {
	result := patchProtoFile([]byte(getProtoContent()), "upstream/path/to/proto", ".org.api.derived_from", nil, "")
	require.Equal(t, getExpectedProtoContentPatched(), string(result))
}

func getProtoContent() string {
	return `
syntax = "proto3";

package protodep.org.common;

option java_multiple_files = true;
option java_package = "com.protodep.org.common";

import "google/protobuf/wrappers.proto";

// A common thing

//This Is 2
message Two {
    google.protobuf.StringValue one = 1;
	google.protobuf.IntValue two = 2;
}

//thr33
message Three {
    message Count {
        uint32 total = 1;
    }
    google.protobuf.StringValue one = 1;
    google.protobuf.IntValue two = 2;
    Count three = 3;
}

//This Is The One
message One {
	option (.org.api.derived_from) = "some.other.Ancestor";
    boolean one = 1;
}

`
}

func getExpectedProtoContentPatched() string {
	return `
syntax = "proto3";

package upstream.path.to;

option java_multiple_files = true;
option java_package = "com.upstream.path.to";

import "google/protobuf/wrappers.proto";

// A common thing

//This Is 2
message Two {
    option (.org.api.derived_from) = "protodep.org.common.Two";
    google.protobuf.StringValue one = 1;
	google.protobuf.IntValue two = 2;
}

//thr33
message Three {
    option (.org.api.derived_from) = "protodep.org.common.Three";
    message Count {
        option (.org.api.derived_from) = "protodep.org.common.Three.Count";
        uint32 total = 1;
    }
    google.protobuf.StringValue one = 1;
    google.protobuf.IntValue two = 2;
    Count three = 3;
}

//This Is The One
message One {
    option (.org.api.derived_from) = "protodep.org.common.One";
    boolean one = 1;
}

`
}
