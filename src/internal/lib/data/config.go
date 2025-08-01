package data

import (
	sys "github.com/8bitalex/raid/src/internal/sys"
	"github.com/spf13/viper"
)

const ConfigFileName = "config.toml"
const ConfigDirName = ".raid"

var CfgPath string

var defaultFilePath = sys.GetHomeDir() + sys.Sep + ConfigDirName + sys.Sep

func InitConfig() {
	viper.SetConfigFile(getOrCreateConfigFile())
	viper.ReadInConfig()
}

func getOrCreateConfigFile() string {
	path := getPath()
	if !sys.FileExists(path) {
		sys.CreateFile(path)
	}
	return path
}

func getPath() string {
	if (CfgPath == "") {
		CfgPath = defaultFilePath + ConfigFileName
	}
	return CfgPath
}
