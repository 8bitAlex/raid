package lib

import (
	"fmt"
	"path/filepath"

	sys "github.com/8bitalex/raid/src/internal/sys"
	"github.com/spf13/viper"
)

const (
	ConfigDirName       = ".raid"
	ConfigFileName      = "config.toml"
	ConfigPathDefault   = "~" + sys.Sep + ConfigDirName + sys.Sep + ConfigFileName
	ConfigPathFlag      = "config"
	ConfigPathFlagShort = "c"
	ConfigPathFlagDesc  = "configuration file path (default is " + ConfigPathDefault + ")"
)

var CfgPath string

var defaultConfigPath = filepath.Join(sys.GetHomeDir(), ConfigDirName)

func InitConfig() error {
	path, err := getOrCreateConfigFile()
	if err != nil {
		return err
	}
	viper.SetConfigFile(path)
	if err := viper.ReadInConfig(); err != nil {
		return err
	}
	return nil
}

func getOrCreateConfigFile() (string, error) {
	path := sys.ExpandPath(getPath())
	if !sys.FileExists(path) {
		f, err := sys.CreateFile(path)
		if err != nil {
			return "", fmt.Errorf("failed to create config file at %s: %w", path, err)
		}
		f.Close()
	}
	return path, nil
}

func getPath() string {
	if CfgPath == "" {
		CfgPath = filepath.Join(defaultConfigPath, ConfigFileName)
	}
	return CfgPath
}

func Set(key string, value any) {
	viper.Set(key, value)
	Write()
}

func Write() {
	if err := viper.WriteConfig(); err != nil {
		panic(err)
	}
}
