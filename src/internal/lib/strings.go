package lib

import "github.com/8bitalex/raid/src/internal/sys"

const (
	ConfigDirName       = ".raid"
	ConfigFileName      = "config.toml"
	ConfigPathDefault   = "~" + sys.Sep + ConfigDirName + sys.Sep + ConfigFileName
	ConfigPathFlag      = "config"
	ConfigPathFlagShort = "c"
	ConfigPathFlagDesc  = "configuration file path (default is " + ConfigPathDefault + ")"
)
