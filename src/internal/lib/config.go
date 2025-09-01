package lib

import (
	sys "github.com/8bitalex/raid/src/internal/sys"
	"github.com/spf13/viper"
)

var CfgPath string

var defaultConfigPath = sys.GetHomeDir() + sys.Sep + ConfigDirName + sys.Sep

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
	if CfgPath == "" {
		CfgPath = defaultConfigPath + ConfigFileName
	}
	return CfgPath
}

func Get(key string) any {
	if !viper.IsSet(key) {
		return nil
	}
	return viper.Get(key)
}

func Set(key string, value any) {
	viper.Set(key, value)
	Write()
}

func SetLazy(key string, value any) {
	viper.Set(key, value)
}

func Write() {
	if err := viper.WriteConfig(); err != nil {
		panic(err)
	}
}
