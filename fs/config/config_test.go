package config

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/ncw/rclone/fs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCRUD(t *testing.T) {
	configKey = nil // reset password
	// create temp config file
	tempFile, err := ioutil.TempFile("", "crud.conf")
	assert.NoError(t, err)
	path := tempFile.Name()
	defer func() {
		err := os.Remove(path)
		assert.NoError(t, err)
	}()
	assert.NoError(t, tempFile.Close())

	// temporarily adapt configuration
	oldOsStdout := os.Stdout
	oldConfigPath := ConfigPath
	oldConfig := fs.Config
	oldConfigData := configData
	oldReadLine := ReadLine
	os.Stdout = nil
	ConfigPath = path
	fs.Config = &fs.ConfigInfo{}
	configData = nil
	defer func() {
		os.Stdout = oldOsStdout
		ConfigPath = oldConfigPath
		ReadLine = oldReadLine
		fs.Config = oldConfig
		configData = oldConfigData
	}()

	LoadConfig()
	assert.Equal(t, []string{}, configData.GetSectionList())

	// Fake a remote
	fs.Register(&fs.RegInfo{Name: "config_test_remote"})

	// add new remote
	i := 0
	ReadLine = func() string {
		answers := []string{
			"config_test_remote", // type
			"y",                  // looks good, save
		}
		i = i + 1
		return answers[i-1]
	}

	NewRemote("test")
	assert.Equal(t, []string{"test"}, configData.GetSectionList())

	// Reload the config file to workaround this bug
	// https://github.com/Unknwon/goconfig/issues/39
	configData, err = loadConfigFile()
	require.NoError(t, err)

	// normal rename, test → asdf
	ReadLine = func() string { return "asdf" }
	RenameRemote("test")
	assert.Equal(t, []string{"asdf"}, configData.GetSectionList())

	// no-op rename, asdf → asdf
	RenameRemote("asdf")
	assert.Equal(t, []string{"asdf"}, configData.GetSectionList())

	// delete remote
	DeleteRemote("asdf")
	assert.Equal(t, []string{}, configData.GetSectionList())
}

// Test some error cases
func TestReveal(t *testing.T) {
	for _, test := range []struct {
		in      string
		wantErr string
	}{
		{"YmJiYmJiYmJiYmJiYmJiYp*gcEWbAw", "base64 decode failed when revealing password - is it obscured?: illegal base64 data at input byte 22"},
		{"aGVsbG8", "input too short when revealing password - is it obscured?"},
		{"", "input too short when revealing password - is it obscured?"},
	} {
		gotString, gotErr := Reveal(test.in)
		assert.Equal(t, "", gotString)
		assert.Equal(t, test.wantErr, gotErr.Error())
	}
}

func TestConfigLoad(t *testing.T) {
	oldConfigPath := ConfigPath
	ConfigPath = "./testdata/plain.conf"
	defer func() {
		ConfigPath = oldConfigPath
	}()
	configKey = nil // reset password
	c, err := loadConfigFile()
	if err != nil {
		t.Fatal(err)
	}
	sections := c.GetSectionList()
	var expect = []string{"RCLONE_ENCRYPT_V0", "nounc", "unc"}
	assert.Equal(t, expect, sections)

	keys := c.GetKeyList("nounc")
	expect = []string{"type", "nounc"}
	assert.Equal(t, expect, keys)
}

func TestConfigLoadEncrypted(t *testing.T) {
	var err error
	oldConfigPath := ConfigPath
	ConfigPath = "./testdata/encrypted.conf"
	defer func() {
		ConfigPath = oldConfigPath
		configKey = nil // reset password
	}()

	// Set correct password
	err = setConfigPassword("asdf")
	require.NoError(t, err)
	c, err := loadConfigFile()
	require.NoError(t, err)
	sections := c.GetSectionList()
	var expect = []string{"nounc", "unc"}
	assert.Equal(t, expect, sections)

	keys := c.GetKeyList("nounc")
	expect = []string{"type", "nounc"}
	assert.Equal(t, expect, keys)
}

func TestConfigLoadEncryptedFailures(t *testing.T) {
	var err error

	// This file should be too short to be decoded.
	oldConfigPath := ConfigPath
	ConfigPath = "./testdata/enc-short.conf"
	defer func() { ConfigPath = oldConfigPath }()
	_, err = loadConfigFile()
	require.Error(t, err)

	// This file contains invalid base64 characters.
	ConfigPath = "./testdata/enc-invalid.conf"
	_, err = loadConfigFile()
	require.Error(t, err)

	// This file contains invalid base64 characters.
	ConfigPath = "./testdata/enc-too-new.conf"
	_, err = loadConfigFile()
	require.Error(t, err)

	// This file does not exist.
	ConfigPath = "./testdata/filenotfound.conf"
	c, err := loadConfigFile()
	assert.Equal(t, errorConfigFileNotFound, err)
	assert.Nil(t, c)
}

func TestPassword(t *testing.T) {
	defer func() {
		configKey = nil // reset password
	}()
	var err error
	// Empty password should give error
	err = setConfigPassword("  \t  ")
	require.Error(t, err)

	// Test invalid utf8 sequence
	err = setConfigPassword(string([]byte{0xff, 0xfe, 0xfd}) + "abc")
	require.Error(t, err)

	// Simple check of wrong passwords
	hashedKeyCompare(t, "mis", "match", false)

	// Check that passwords match after unicode normalization
	hashedKeyCompare(t, "ﬀ\u0041\u030A", "ffÅ", true)

	// Check that passwords preserves case
	hashedKeyCompare(t, "abcdef", "ABCDEF", false)

}

func hashedKeyCompare(t *testing.T, a, b string, shouldMatch bool) {
	err := setConfigPassword(a)
	require.NoError(t, err)
	k1 := configKey

	err = setConfigPassword(b)
	require.NoError(t, err)
	k2 := configKey

	if shouldMatch {
		assert.Equal(t, k1, k2)
	} else {
		assert.NotEqual(t, k1, k2)
	}
}
