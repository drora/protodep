package service

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/mitchellh/go-homedir"
	"github.com/stormcat24/protodep/helper"
	"github.com/stretchr/testify/require"
)

func TestPatch(t *testing.T) {
	result := patchProtoFile([]byte(getProtoContent()), "upstream/path/to/proto", ".org.api.derived_from")
	require.Equal(t, getExpectedProtoContentPatched(), string(result))
}

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

	conf := helper.SyncConfig{
		HomeDir:   dotProtoDir,
		TargetDir: pwd,
		OutputDir: outputRootDir,
	}

	target, err := NewSync(&conf)
	require.NoError(t, err)

	c := gomock.NewController(t)
	defer c.Finish()

	httpsAuthProviderMock := helper.NewMockAuthProvider(c)
	httpsAuthProviderMock.EXPECT().AuthMethod().Return(nil, nil).AnyTimes()
	httpsAuthProviderMock.EXPECT().GetRepositoryURL("github.com/protocolbuffers/protobuf").Return("https://github.com/protocolbuffers/protobuf.git")

	sshAuthProviderMock := helper.NewMockAuthProvider(c)
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

	// fetch
	err = target.Resolve(false, false)
	require.NoError(t, err)
}

func isFileExist(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
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
