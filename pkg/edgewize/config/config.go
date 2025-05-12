package config

import (
	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

type Config struct {
	PodSelector []string `yaml:"podSelector"`
}

var Cfg Config
var v *viper.Viper

func init() {
	_ = initConfig()
}

func initConfig() error {
	v = viper.New()
	v.SetConfigFile("/manifests/middleware/config.yaml")
	v.SetConfigType("yaml")
	if err := v.ReadInConfig(); err != nil {
		return err
	}
	v.WatchConfig()
	v.OnConfigChange(func(_ fsnotify.Event) {
		if err := v.ReadInConfig(); err != nil {
			return
		}
		parseCfg()
	})
	return nil
}

func parseCfg() {
	if v == nil {
		return
	}
	_ = v.Unmarshal(&Cfg)
}
